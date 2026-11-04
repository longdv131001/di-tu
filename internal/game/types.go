// Package game contains the authoritative simulation of one prison instance.
package game

import (
	"encoding/json"
	"math"
	"strings"
	"sync/atomic"
	"time"
	"unicode"

	"dituonline/internal/store"
	"dituonline/internal/world"
)

// Client is the server side of one WebSocket connection.
type Client struct {
	Out     chan []byte
	CloseFn func(reason string)
	closed  atomic.Bool
}

func NewClient(buf int, closeFn func(string)) *Client {
	return &Client{Out: make(chan []byte, buf), CloseFn: closeFn}
}

func (c *Client) Send(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	c.SendRaw(b)
}

func (c *Client) SendRaw(b []byte) {
	if c.closed.Load() {
		return
	}
	select {
	case c.Out <- b:
	default: // client cannot keep up: drop it rather than block the simulation
		c.Close("slow client")
	}
}

func (c *Client) Close(reason string) {
	if c.closed.CompareAndSwap(false, true) && c.CloseFn != nil {
		c.CloseFn(reason)
	}
}

// Msg is the union of all client->server messages (short keys on the wire).
type Msg struct {
	T      string        `json:"t"`
	S      uint32        `json:"s,omitempty"`
	MX     float64       `json:"mx,omitempty"`
	MZ     float64       `json:"mz,omitempty"`
	Yaw    float64       `json:"yaw,omitempty"`
	J      bool          `json:"j,omitempty"`
	R      bool          `json:"r,omitempty"`
	Ch     string        `json:"ch,omitempty"`
	To     string        `json:"to,omitempty"`
	Text   string        `json:"text,omitempty"`
	ID     string        `json:"id,omitempty"`
	Item   string        `json:"item,omitempty"`
	Qty    int           `json:"qty,omitempty"`
	Price  int           `json:"price,omitempty"`
	N      int           `json:"n,omitempty"`
	TS     int64         `json:"ts,omitempty"`
	Accept bool          `json:"accept,omitempty"`
	Outfit *store.Outfit `json:"outfit,omitempty"`
}

// animation codes shared with the client
const (
	animIdle = iota
	animWalk
	animRun
	animJump
	animSleep
	animWork
	animStudy
	animSit
	animWave
	animDance
	animLaugh
	animGym
	animEat
	animCut
)

type itemDef struct {
	Name   string
	Contra bool
	Eat    int
	Price  int
}

var items = map[string]itemDef{
	"noodle":    {"Mì tôm", false, 18, 4},
	"cigarette": {"Thuốc lá", true, 0, 9},
	"wire":      {"Dây thép", false, 0, 0},
	"handle":    {"Cán gỗ", false, 0, 0},
	"pliers":    {"Kìm cắt", true, 0, 0},
}

const mealPrice = 5

type actDef struct {
	Dur     time.Duration
	Anim    int
	Stamina int
	Room    string
	Label   string
}

var acts = map[string]actDef{
	"rest":  {6 * time.Second, animSleep, 0, "", "Nghỉ ngơi"},
	"work":  {6 * time.Second, animWork, 8, "workshop", "Lao động"},
	"study": {6 * time.Second, animStudy, 5, "classroom", "Học tập"},
	"gym":   {5 * time.Second, animGym, 10, "", "Tập thể dục"},
	"cut":   {8 * time.Second, animCut, 0, "", "Cắt hàng rào"},
}

var objAct = map[string]string{"bed": "rest", "bench": "work", "desk": "study", "gym": "gym", "fence": "cut"}

type qdef struct {
	ID, Label string
	Target    int
	Money     int
	Rep       int
}

var questDefs = []qdef{
	{"work", "Lao động 3 lần", 3, 30, 2},
	{"eat", "Ăn 2 bữa", 2, 15, 1},
	{"study", "Học 2 lần", 2, 25, 3},
	{"gym", "Tập thể dục 2 lần", 2, 20, 2},
	{"craft", "Chế tạo 1 món đồ", 1, 35, 3},
	{"roll", "Điểm danh đúng giờ", 1, 20, 2},
}

const weeklyTarget = 10
const weeklyMoney, weeklyRep = 150, 10

func qdefByID(id string) (qdef, bool) {
	for _, q := range questDefs {
		if q.ID == id {
			return q, true
		}
	}
	return qdef{}, false
}

type inp struct {
	seq uint32
	in  world.Input
}

type action struct {
	kind   string
	obj    string
	sx, sz float64
	end    time.Time
	group  bool
}

type Player struct {
	Char *store.Character
	Acct store.Account
	cl   *Client

	discAt   time.Time
	body     world.Body
	yaw      float64
	anim     int
	room     string
	queue    []inp
	ack      uint32
	known    map[int64]int
	metaVer  int
	act      *action
	emote    int
	emoteEnd time.Time

	isoUntil   time.Time
	lastViol   time.Time
	mutedUntil time.Time
	chatTokens float64
	chatAt     time.Time
	reportAt   time.Time
	denyAt     time.Time
	dirty      bool
	lastSave   time.Time
	offerOut   int
}

func (p *Player) id() int64              { return p.Char.ID }
func (p *Player) name() string           { return p.Char.Name }
func (p *Player) iso(now time.Time) bool { return now.Before(p.isoUntil) }

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func r2(v float64) float64 { return math.Round(v*100) / 100 }

func dist2(ax, az, bx, bz float64) float64 { return (ax-bx)*(ax-bx) + (az-bz)*(az-bz) }

var badWords = []string{"đm", "dm", "đ.m", "vcl", "vl", "cặc", "lồn", "đụ", "địt", "fuck", "shit"}

func cleanText(s string, max int) string {
	var b strings.Builder
	n := 0
	space := false
	for _, r := range s {
		if unicode.IsControl(r) || r == '\u200b' || r == '\ufeff' {
			continue
		}
		if unicode.IsSpace(r) {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteRune(r)
		if n++; n >= max {
			break
		}
	}
	return b.String()
}

func filterBad(s string) string {
	words := strings.Split(s, " ")
	for i, w := range words {
		lw := strings.ToLower(strings.Trim(w, ".,!?;:"))
		for _, bad := range badWords {
			if lw == bad {
				words[i] = strings.Repeat("*", len([]rune(w)))
			}
		}
	}
	return strings.Join(words, " ")
}

// CleanName validates a display name: 2-16 chars, letters/digits/space/_ only.
func CleanName(s string) string {
	s = cleanText(s, 16)
	for _, r := range s {
		if !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '_') {
			return ""
		}
	}
	if len([]rune(s)) < 2 {
		return ""
	}
	return s
}

func phaseOf(h float64) (id, label string) {
	switch {
	case h >= 6 && h < 7:
		return "roll", "Điểm danh"
	case h >= 7 && h < 12:
		return "work", "Giờ lao động"
	case h >= 12 && h < 13:
		return "lunch", "Ăn trưa"
	case h >= 13 && h < 17:
		return "free", "Giờ tự do"
	case h >= 17 && h < 19:
		return "study", "Giờ học tập"
	case h >= 19 && h < 22:
		return "evening", "Buổi tối"
	}
	return "night", "Tắt đèn"
}
