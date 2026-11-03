package main

import (
	"compress/gzip"
	"context"
	"embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"dituonline/internal/hub"
	"dituonline/internal/proto"
	"github.com/coder/websocket"
)

//go:embed all:web
var assets embed.FS

func env(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func num(k string, d int) int {
	v, e := strconv.Atoi(env(k, strconv.Itoa(d)))
	if e != nil || v < 1 {
		return d
	}
	return v
}

type limiter struct {
	mu    sync.Mutex
	times []time.Time
}

func (l *limiter) allow() bool { return l.allowAt(time.Now()) }
func (l *limiter) allowAt(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	j := 0
	for _, t := range l.times {
		if now.Sub(t) < time.Second {
			l.times[j] = t
			j++
		}
	}
	l.times = l.times[:j]
	if len(l.times) >= 8 {
		return false
	}
	l.times = append(l.times, now)
	return true
}
func main() {
	h := hub.New(num("MAX_PLAYERS_PER_PRISON", 60), num("GAME_DAY_SECONDS", 90), num("SENTENCE_DAYS", 30))
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, e := websocket.Accept(w, r, nil)
		if e != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		conn.SetReadLimit(4096)
		ctx, cancelConn := context.WithCancel(r.Context())
		defer cancelConn()
		out := make(chan []byte, 64)
		var token string
		lim := &limiter{}
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				select {
				case b := <-out:
					ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
					e := conn.Write(ctx2, websocket.MessageText, b)
					cancel()
					if e != nil {
						cancelConn()
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()
		for {
			_, data, e := conn.Read(ctx)
			if e != nil {
				break
			}
			if len(data) > 4096 {
				_ = conn.Close(websocket.StatusMessageTooBig, "message too large")
				break
			}
			var m proto.Message
			if json.Unmarshal(data, &m) != nil {
				continue
			}
			if runes := []rune(m.Text); len(runes) > 200 {
				m.Text = string(runes[:200])
			}
			if m.Type == "join" {
				if token == "" {
					token = m.Token
				}
				if !validToken(token) {
					token = hub.Token()
				}
				v, _ := h.Join(token, m, out)
				if result, ok := v.(map[string]any); ok {
					if msg, bad := result["error"]; bad {
						b, _ := json.Marshal(map[string]any{"t": "error", "text": msg})
						select {
						case out <- b:
						default:
						}
						continue
					}
				}
				b, _ := json.Marshal(map[string]any{"token": token, "prison": v})
				select {
				case out <- b:
				default:
				}
				continue
			}
			if token == "" {
				continue
			}
			if !lim.allow() {
				select {
				case out <- []byte(`{"t":"error","text":"Tối đa 8 thao tác mỗi giây."}`):
				default:
				}
				continue
			}
			if m.Type == "ping" {
				pong, _ := json.Marshal(map[string]any{"t": "pong", "sentAt": m.SentAt})
				select {
				case out <- pong:
				default:
				}
				continue
			}
			h.Handle(token, m, out)
		}
		if token != "" {
			h.Disconnect(token)
		}
		select {
		case <-done:
		case <-time.After(time.Second):
		}
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if strings.Contains(path, "..") || strings.Contains(path, "\\") {
			http.NotFound(w, r)
			return
		}
		f, e := assets.Open("web/" + path)
		if e != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		contentType := mime.TypeByExtension("." + strings.TrimPrefix(path[strings.LastIndex(path, ".")+1:], "."))
		switch {
		case strings.HasSuffix(path, ".html"):
			contentType = "text/html; charset=utf-8"
		case strings.HasSuffix(path, ".js"):
			contentType = "text/javascript; charset=utf-8"
		case strings.HasSuffix(path, ".css"):
			contentType = "text/css; charset=utf-8"
		case strings.HasSuffix(path, ".json"):
			contentType = "application/json; charset=utf-8"
		}
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") && isText(path) {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			if r.Method == http.MethodGet {
				_, _ = io.Copy(gz, f)
			}
			return
		}
		if r.Method == http.MethodGet {
			_, _ = io.Copy(w, f)
		}
	})
	addr := ":" + env("PORT", "8080")
	log.Printf("Đi tù online đang chạy tại http://localhost%s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
func isText(p string) bool {
	return strings.HasSuffix(p, ".html") || strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".css")
}

func validToken(token string) bool {
	if len(token) != 48 {
		return false
	}
	_, err := hex.DecodeString(token)
	return err == nil
}
