package store

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// Memory is a non-durable Store used for tests and for running without a database.
type Memory struct {
	mu      sync.Mutex
	nextAcc int64
	nextCh  int64
	accs    map[int64]*Account
	byName  map[string]int64
	chars   map[int64]*Character // by account id
	reports []Report
}

func NewMemory() *Memory {
	return &Memory{accs: map[int64]*Account{}, byName: map[string]int64{}, chars: map[int64]*Character{}}
}

func cloneChar(c *Character) *Character {
	cp := *c
	cp.Data.Inv = map[string]int{}
	for k, v := range c.Data.Inv {
		cp.Data.Inv[k] = v
	}
	cp.Data.Quests.Daily = append([]QState(nil), c.Data.Quests.Daily...)
	return &cp
}

func (m *Memory) CreateAccount(_ context.Context, username, hash string) (Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	u := strings.ToLower(username)
	if _, ok := m.byName[u]; ok {
		return Account{}, ErrExists
	}
	m.nextAcc++
	a := &Account{ID: m.nextAcc, Username: u, PassHash: hash}
	m.accs[a.ID] = a
	m.byName[u] = a.ID
	return *a, nil
}

func (m *Memory) AccountByName(_ context.Context, username string) (Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byName[strings.ToLower(username)]
	if !ok {
		return Account{}, ErrNotFound
	}
	return *m.accs[id], nil
}

func (m *Memory) AccountByID(_ context.Context, id int64) (Account, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	a, ok := m.accs[id]
	if !ok {
		return Account{}, ErrNotFound
	}
	return *a, nil
}

func (m *Memory) CreateCharacter(_ context.Context, c *Character) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.chars[c.AccountID]; ok {
		return ErrExists
	}
	for _, o := range m.chars {
		if strings.EqualFold(o.Name, c.Name) {
			return ErrExists
		}
	}
	m.nextCh++
	c.ID = m.nextCh
	m.chars[c.AccountID] = cloneChar(c)
	return nil
}

func (m *Memory) CharacterByAccount(_ context.Context, accountID int64) (*Character, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.chars[accountID]
	if !ok {
		return nil, ErrNotFound
	}
	return cloneChar(c), nil
}

func (m *Memory) CharacterByName(_ context.Context, name string) (*Character, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.chars {
		if strings.EqualFold(c.Name, name) {
			return cloneChar(c), nil
		}
	}
	return nil, ErrNotFound
}

func (m *Memory) SaveCharacter(_ context.Context, c *Character) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.chars[c.AccountID]; !ok {
		return ErrNotFound
	}
	m.chars[c.AccountID] = cloneChar(c)
	return nil
}

func (m *Memory) Top(_ context.Context, by string, n int) ([]TopRow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	by = TopColumn(by)
	rows := make([]TopRow, 0, len(m.chars))
	for _, c := range m.chars {
		rows = append(rows, TopRow{c.Name, c.Money, c.Rep, c.Points, c.Escapes, c.Releases})
	}
	key := func(r TopRow) int {
		switch by {
		case "money":
			return r.Money
		case "points":
			return r.Points
		case "escapes":
			return r.Escapes
		case "releases":
			return r.Releases
		}
		return r.Rep
	}
	sort.Slice(rows, func(i, j int) bool { return key(rows[i]) > key(rows[j]) })
	if len(rows) > n {
		rows = rows[:n]
	}
	return rows, nil
}

func (m *Memory) SetBan(_ context.Context, username string, until time.Time, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byName[strings.ToLower(username)]
	if !ok {
		return ErrNotFound
	}
	m.accs[id].BannedUntil, m.accs[id].BanReason = until, reason
	return nil
}

func (m *Memory) SetMute(_ context.Context, username string, until time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	id, ok := m.byName[strings.ToLower(username)]
	if !ok {
		return ErrNotFound
	}
	m.accs[id].MutedUntil = until
	return nil
}

func (m *Memory) AddReport(_ context.Context, reporter, target int64, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := func(id int64) string {
		for _, c := range m.chars {
			if c.ID == id {
				return c.Name
			}
		}
		return "?"
	}
	m.reports = append(m.reports, Report{ID: int64(len(m.reports) + 1), Reporter: name(reporter), Target: name(target), Reason: reason, At: time.Now()})
	return nil
}

func (m *Memory) Reports(_ context.Context, n int) ([]Report, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := append([]Report(nil), m.reports...)
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	if len(out) > n {
		out = out[:n]
	}
	return out, nil
}

func (m *Memory) Close() error { return nil }
