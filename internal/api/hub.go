// Package api wires HTTP endpoints, WebSocket sessions and game instances together.
package api

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"dituonline/internal/game"
	"dituonline/internal/store"
	"dituonline/internal/world"
)

type Config struct {
	MaxPlayers        int
	MaxInstances      int
	DaySeconds        int
	AdminKey          string
	TrustProxy        bool
	DisableRateLimits bool // load tests only
	AllowOrigins      []string
}

type Hub struct {
	cfg    Config
	st     store.Store
	m      *world.Map
	mu     sync.Mutex
	insts  []*game.Instance
	byChar map[int64]*game.Instance
	names  map[string]*game.Instance
	recent map[int64]*store.Character // saved but maybe not yet written
	saveQ  chan *store.Character
	wg     sync.WaitGroup
	done   chan struct{}
}

func NewHub(cfg Config, st store.Store, m *world.Map) *Hub {
	if cfg.MaxPlayers < 1 {
		cfg.MaxPlayers = 24
	}
	if cfg.MaxInstances < 1 {
		cfg.MaxInstances = 200
	}
	h := &Hub{cfg: cfg, st: st, m: m, byChar: map[int64]*game.Instance{}, names: map[string]*game.Instance{},
		recent: map[int64]*store.Character{}, saveQ: make(chan *store.Character, 8192), done: make(chan struct{})}
	for k := 0; k < 4; k++ {
		h.wg.Add(1)
		go h.saver()
	}
	return h
}

func (h *Hub) saver() {
	defer h.wg.Done()
	for c := range h.saveQ {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := h.st.SaveCharacter(ctx, c); err != nil {
			log.Printf("save character %d: %v", c.ID, err)
		}
		cancel()
		h.mu.Lock()
		if h.recent[c.ID] == c {
			delete(h.recent, c.ID)
		}
		h.mu.Unlock()
	}
}

func (h *Hub) save(c *store.Character) {
	h.mu.Lock()
	h.recent[c.ID] = c
	h.mu.Unlock()
	h.saveQ <- c
}

// Shutdown stops every instance (saving all players) and flushes pending writes.
func (h *Hub) Shutdown() {
	h.mu.Lock()
	insts := append([]*game.Instance(nil), h.insts...)
	h.mu.Unlock()
	for _, i := range insts {
		i.Stop()
	}
	close(h.saveQ)
	h.wg.Wait()
}

func (h *Hub) newInstance() *game.Instance {
	inst := game.NewInstance(game.Config{
		ID: "trai-" + itoa(len(h.insts)+1), Max: h.cfg.MaxPlayers, DaySeconds: h.cfg.DaySeconds, Map: h.m, Store: h.st,
		Save: h.save, OnRemove: h.onRemove, RouteDM: h.routeDM,
	})
	h.insts = append(h.insts, inst)
	go inst.Run()
	return inst
}

func (h *Hub) onRemove(id int64, name string) {
	h.mu.Lock()
	delete(h.byChar, id)
	delete(h.names, strings.ToLower(name))
	h.mu.Unlock()
}

func (h *Hub) routeDM(from, to, text string) bool {
	h.mu.Lock()
	inst := h.names[strings.ToLower(to)]
	h.mu.Unlock()
	if inst == nil {
		return false
	}
	inst.DM(from, to, text)
	return true
}

// attach places a character into an instance (its current one when reconnecting).
func (h *Hub) attach(acct store.Account, c *store.Character, cl *game.Client) bool {
	h.mu.Lock()
	if r := h.recent[c.ID]; r != nil { // newest data not yet in the database
		cp := *r
		cp.Data.Inv = map[string]int{}
		for k, v := range r.Data.Inv {
			cp.Data.Inv[k] = v
		}
		cp.Data.Quests.Daily = append([]store.QState(nil), r.Data.Quests.Daily...)
		c = &cp
	}
	inst := h.byChar[c.ID]
	if inst == nil {
		for _, i := range h.insts {
			if i.Count() < h.cfg.MaxPlayers && (inst == nil || i.Count() > inst.Count()) {
				inst = i
			}
		}
		if inst == nil {
			if len(h.insts) >= h.cfg.MaxInstances {
				h.mu.Unlock()
				return false
			}
			inst = h.newInstance()
		}
		h.byChar[c.ID] = inst
		h.names[strings.ToLower(c.Name)] = inst
	}
	h.mu.Unlock()
	inst.Join(c, acct, cl)
	return true
}

func (h *Hub) instanceOf(charID int64) *game.Instance {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.byChar[charID]
}

func (h *Hub) instanceByName(name string) *game.Instance {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.names[strings.ToLower(name)]
}

// Stats is shown on /healthz.
func (h *Hub) Stats() (instances, players int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, i := range h.insts {
		players += i.Count()
	}
	return len(h.insts), players
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	k := len(b)
	for n > 0 {
		k--
		b[k] = byte('0' + n%10)
		n /= 10
	}
	return string(b[k:])
}
