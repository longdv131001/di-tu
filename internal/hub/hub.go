package hub

import (
	"crypto/rand"
	"dituonline/internal/prison"
	"dituonline/internal/proto"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type Hub struct {
	mu                            sync.Mutex
	prisons                       []*prison.Prison
	byToken                       map[string]*prison.Prison
	generation                    map[string]uint64
	max, daySeconds, sentenceDays int
}

func New(max, daySeconds, sentenceDays int) *Hub {
	return &Hub{byToken: map[string]*prison.Prison{}, generation: map[string]uint64{}, max: max, daySeconds: daySeconds, sentenceDays: sentenceDays}
}
func Token() string {
	b := make([]byte, 24)
	if _, e := rand.Read(b); e != nil {
		panic(e)
	}
	return hex.EncodeToString(b)
}
func (h *Hub) Join(token string, m proto.Message, out chan []byte) (any, string) {
	h.mu.Lock()
	p := h.byToken[token]
	if p == nil {
		for _, x := range h.prisons {
			if x.Count() < h.max {
				p = x
				break
			}
		}
		if p == nil {
			id := fmt.Sprintf("P-%03d", len(h.prisons)+1)
			p = prison.New(id, h.max, h.daySeconds, h.sentenceDays)
			h.prisons = append(h.prisons, p)
		}
		h.byToken[token] = p
	}
	m.Type = "join"
	v := p.Call(prison.Request{Token: token, Msg: m, Out: out})
	if result, ok := v.(map[string]any); ok {
		if _, bad := result["error"]; bad {
			h.mu.Unlock()
			return v, p.ID
		}
	}
	h.generation[token]++
	h.mu.Unlock()
	return v, p.ID
}
func (h *Hub) Handle(token string, m proto.Message, out chan []byte) any {
	h.mu.Lock()
	p := h.byToken[token]
	h.mu.Unlock()
	if p == nil {
		return nil
	}
	return p.Call(prison.Request{Token: token, Msg: m, Out: out})
}
func (h *Hub) Disconnect(token string) {
	h.mu.Lock()
	p := h.byToken[token]
	gen := h.generation[token]
	h.mu.Unlock()
	if p == nil {
		return
	}
	p.Call(prison.Request{Token: token, Msg: proto.Message{Type: "disconnect"}})
	time.AfterFunc(61*time.Second, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.generation[token] == gen && h.byToken[token] == p {
			delete(h.byToken, token)
			delete(h.generation, token)
		}
	})
}
