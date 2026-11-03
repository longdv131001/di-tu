package game

import (
	"math/rand"
	"strings"
	"time"
	"unicode/utf8"
)

var Crimes = []string{"Tội danh: giấu dép của cả phòng", "Tội danh: nấu mì thơm quá mức quy định", "Tội danh: chiếm dụng remote tivi", "Tội danh: ngủ nướng có tổ chức", "Tội danh: buôn bán bí kíp gấp chăn"}
var Zones = map[string]bool{"cell": true, "canteen": true, "workshop": true, "yard": true, "classroom": true, "solitary": true, "gate": true}
var banned = []string{"đồ ngu", "thằng ngu", "địt", "đụ", "fuck", "shit"}

type Player struct {
	Token, Name, Color, Hair, Crime, Zone  string
	Cell                                   int
	Money, Reputation, Discipline, Stamina int
	Sentence, TotalSentence, Reform        int
	Earned, Trades, SolitaryCount          int
	Joined, LastAction                     time.Time
	SolitaryUntil                          time.Time
	WorkStarted                            time.Time
	WorkTarget                             int
	Finished                               bool
	Items                                  map[string]int
}

func NewPlayer(token, name, color, hair string, now time.Time) *Player {
	return &Player{Token: token, Name: name, Color: color, Hair: hair, Crime: Crimes[rand.Intn(len(Crimes))], Zone: "cell", Money: 100, Reputation: 50, Discipline: 70, Stamina: 80, Sentence: 20 + rand.Intn(21), Joined: now, Items: map[string]int{}}
}
func CleanName(s string) string {
	s = strings.TrimSpace(s)
	if len([]rune(s)) < 2 || len([]rune(s)) > 16 {
		return ""
	}
	for _, b := range banned {
		if strings.Contains(strings.ToLower(s), b) {
			return ""
		}
	}
	return s
}
func CleanText(s string) string {
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > 200 {
		s = string(r[:200])
	}
	for _, b := range banned {
		for {
			i := strings.Index(strings.ToLower(s), b)
			if i < 0 {
				break
			}
			s = s[:i] + strings.Repeat("*", utf8.RuneCountInString(b)) + s[i+len(b):]
		}
	}
	return s
}
func Clamp(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}
func WorkReward(score int, elapsed time.Duration) (int, bool) {
	if elapsed < 2*time.Second || elapsed > 45*time.Second || score < 0 || score > 100 {
		return 0, false
	}
	return 5 + score/10, true
}
func ApplyWork(p *Player, score int, elapsed time.Duration) bool {
	reward, ok := WorkReward(score, elapsed)
	if !ok {
		return false
	}
	p.Money += reward
	p.Earned += reward
	p.Reform += 2 + score/25
	p.Discipline = Clamp(p.Discipline + 1)
	p.Stamina = Clamp(p.Stamina - 5)
	ReduceSentence(p)
	return true
}
func ReduceSentence(p *Player) int {
	if p.Reform >= 20 && p.Sentence > 0 {
		p.Reform -= 20
		p.Sentence--
		return 1
	}
	return 0
}
func AttendRollCall(p *Player, present bool) {
	if present {
		p.Discipline = Clamp(p.Discipline + 3)
		p.Reform += 2
	} else {
		p.Discipline = Clamp(p.Discipline - 5)
		p.Reputation = Clamp(p.Reputation - 2)
	}
	ReduceSentence(p)
}
func Buy(p *Player, item string) bool {
	prices := map[string]int{"meal": 12, "noodles": 8, "cigarette": 5, "rope": 35, "file": 30, "snack": 6}
	cost, ok := prices[item]
	if !ok || p.Money < cost {
		return false
	}
	p.Money -= cost
	p.Items[item]++
	if item == "meal" {
		p.Stamina = Clamp(p.Stamina + 25)
	}
	if item == "noodles" {
		p.Stamina = Clamp(p.Stamina + 15)
	}
	if item == "snack" {
		p.Stamina = Clamp(p.Stamina + 8)
	}
	return true
}
func Rest(p *Player) { p.Stamina = Clamp(p.Stamina + 18); p.LastAction = time.Now() }
func Exercise(p *Player) {
	p.Stamina = Clamp(p.Stamina - 8)
	p.Reputation = Clamp(p.Reputation + 2)
	p.Reform++
}
func Study(p *Player) {
	if p.Stamina < 5 {
		return
	}
	p.Stamina -= 5
	p.Reform += 5
	p.Reputation = Clamp(p.Reputation + 1)
	ReduceSentence(p)
}
func EscapeChance(players, items int, night bool, alert int) int {
	chance := 10 + (players-3)*8 + items*12 + alert*-3
	if night {
		chance += 20
	}
	if chance < 1 {
		return 1
	}
	if chance > 85 {
		return 85
	}
	return chance
}
func Escape(p []*Player, night bool, alert int, roll int) bool {
	items := 0
	for _, x := range p {
		items += x.Items["rope"] + x.Items["file"]
	}
	chance := EscapeChance(len(p), items, night, alert)
	if len(p) < 3 || items < len(p) || roll >= chance {
		for _, x := range p {
			x.Sentence += 5
			x.SolitaryUntil = time.Now().Add(2 * time.Minute)
			x.SolitaryCount++
			x.Zone = "solitary"
		}
		return false
	}
	for _, x := range p {
		x.Finished = true
	}
	return true
}
