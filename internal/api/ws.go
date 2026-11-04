package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"

	"dituonline/internal/game"
	"dituonline/internal/store"
)

// bucket is a small token bucket used to rate limit one connection.
type bucket struct {
	tokens, rate, burst float64
	last                time.Time
}

func (b *bucket) take() bool {
	now := time.Now()
	b.tokens = min(b.burst, b.tokens+now.Sub(b.last).Seconds()*b.rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: s.cfg.AllowOrigins})
	if err != nil {
		return
	}
	conn.SetReadLimit(4096)
	// NB: cancelling a context passed to conn.Read closes the connection, so reads use
	// timeouts only; shutting down is signalled through `closing` and done by the writer.
	closing := make(chan struct{})
	var once sync.Once
	reason := "bye"
	closeConn := func(why string) {
		once.Do(func() {
			reason = why
			close(closing)
		})
	}
	cl := game.NewClient(256, closeConn)

	wdone := make(chan struct{})
	write := func(b []byte) error {
		wctx, c := context.WithTimeout(context.Background(), 3*time.Second)
		defer c()
		return conn.Write(wctx, websocket.MessageText, b)
	}
	go func() { // forwards queued messages and flushes what is left (e.g. a kick notice) before closing
		defer close(wdone)
		for {
			select {
			case b := <-cl.Out:
				if write(b) != nil {
					closeConn("write")
					return
				}
			case <-closing:
				for {
					select {
					case b := <-cl.Out:
						if write(b) != nil {
							return
						}
					default:
						if len(reason) > 100 {
							reason = reason[:100]
						}
						conn.Close(websocket.StatusNormalClosure, reason)
						return
					}
				}
			}
		}
	}()
	defer func() {
		closeConn("bye")
		select {
		case <-wdone:
		case <-time.After(10 * time.Second):
		}
		conn.CloseNow()
	}()

	sendErr := func(code, msg string) {
		b, _ := json.Marshal(map[string]string{"t": "err", "code": code, "msg": msg})
		cl.SendRaw(b)
	}

	// the first message must authenticate
	hctx, hcancel := context.WithTimeout(context.Background(), 8*time.Second)
	_, data, err := conn.Read(hctx)
	hcancel()
	var hello struct{ T, Token string }
	if err != nil || json.Unmarshal(data, &hello) != nil || hello.T != "hello" {
		sendErr("auth", "Thiếu thông tin đăng nhập")
		return
	}
	uid, err := s.signer.Verify(hello.Token)
	if err != nil {
		sendErr("auth", "Phiên đăng nhập hết hạn, hãy đăng nhập lại")
		return
	}
	qctx, qcancel := context.WithTimeout(context.Background(), 10*time.Second)
	acct, err1 := s.st.AccountByID(qctx, uid)
	var ch *store.Character
	var err2 error
	if err1 == nil {
		ch, err2 = s.st.CharacterByAccount(qctx, uid)
	}
	qcancel()
	if err1 != nil || err2 != nil {
		sendErr("auth", "Tài khoản không tồn tại")
		return
	}
	if time.Now().Before(acct.BannedUntil) {
		sendErr("banned", "Tài khoản bị khóa: "+acct.BanReason)
		return
	}
	if !s.hub.attach(acct, ch, cl) {
		sendErr("full", "Máy chủ đang đầy, hãy thử lại sau.")
		return
	}
	defer func() {
		if inst := s.hub.instanceOf(ch.ID); inst != nil {
			inst.Leave(cl)
		}
	}()

	lim := &bucket{tokens: 120, rate: 60, burst: 120, last: time.Now()}
	drops := 0
	dropWin := time.Now()
	for {
		rctx, rc := context.WithTimeout(context.Background(), 45*time.Second)
		_, data, err := conn.Read(rctx)
		rc()
		if err != nil {
			return
		}
		if !lim.take() {
			if time.Since(dropWin) > 10*time.Second {
				drops, dropWin = 0, time.Now()
			}
			if drops++; drops > 300 {
				closeConn("flood")
				return
			}
			continue
		}
		var m game.Msg
		if json.Unmarshal(data, &m) != nil || m.T == "" {
			continue
		}
		if cur := s.hub.instanceOf(ch.ID); cur != nil {
			cur.Handle(cl, m)
		}
	}
}
