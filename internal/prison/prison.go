package prison

import (
	"dituonline/internal/game"
	"dituonline/internal/proto"
	"encoding/json"
	"fmt"
	"math/rand"
	"time"
)

type Request struct {
	Token string
	Msg   proto.Message
	Out   chan []byte
	Reply chan any
}
type Prison struct {
	ID           string
	Max          int
	DaySeconds   int
	SentenceDays int
	req          chan Request
}
type playerView struct {
	Token      string `json:"id"`
	Name       string `json:"name"`
	Zone       string `json:"zone"`
	Color      string `json:"color"`
	Money      int    `json:"money"`
	Reputation int    `json:"rep"`
	Discipline int    `json:"disc"`
	Stamina    int    `json:"stamina"`
	Sentence   int    `json:"sentence"`
	Total      int    `json:"total"`
	Crime      string `json:"crime"`
	Finished   bool   `json:"finished"`
	Cell       int    `json:"cell"`
}
type state struct {
	p            map[string]*game.Player
	out          map[string]chan []byte
	offline      map[string]time.Time
	currentDay   int
	room         int
	daySeconds   int
	sentenceDays int
	lastHour     int
	started      time.Time
	tick         int
	schedule     int
	trades       map[string]trade
	chat         map[string][]map[string]any
	friends      map[string]map[string]bool
	friendReq    map[string]map[string]bool
	crews        map[string]*crew
	playerCrew   map[string]string
	listings     map[string]*listing
	nextListing  int64
	seasonScore  map[string]int
	eventUntil   time.Time
	eventName    string
}
type trade struct {
	From, To string
	Amount   int
	Expires  time.Time
}

func New(id string, max, daySeconds, sentenceDays int) *Prison {
	p := &Prison{ID: id, Max: max, DaySeconds: daySeconds, SentenceDays: sentenceDays, req: make(chan Request, 2048)}
	go p.loop()
	return p
}
func (p *Prison) Call(r Request) any { r.Reply = make(chan any, 1); p.req <- r; return <-r.Reply }
func (p *Prison) loop() {
	s := &state{p: map[string]*game.Player{}, out: map[string]chan []byte{}, daySeconds: p.DaySeconds, sentenceDays: p.SentenceDays, started: time.Now(), trades: map[string]trade{}, offline: map[string]time.Time{}, chat: map[string][]map[string]any{}, friends: map[string]map[string]bool{}, friendReq: map[string]map[string]bool{}, crews: map[string]*crew{}, playerCrew: map[string]string{}, listings: map[string]*listing{}, seasonScore: map[string]int{}}
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case r := <-p.req:
			s.handle(r)
		case <-ticker.C:
			s.tick++
			s.onTick()
		}
	}
}
func (s *state) send(token string, v any) {
	b, e := json.Marshal(v)
	if e != nil {
		return
	}
	if c := s.out[token]; c != nil {
		select {
		case c <- b:
		default:
		}
	}
}
func (s *state) broadcast(v any, zone, cell string) {
	b, e := json.Marshal(v)
	if e != nil {
		return
	}
	for t, p := range s.p {
		if zone != "" && p.Zone != zone {
			continue
		}
		if cell != "" && fmt.Sprint(p.Cell) != cell {
			continue
		}
		if c := s.out[t]; c != nil {
			select {
			case c <- b:
			default:
			}
		}
	}
}
func view(p *game.Player) playerView {
	return playerView{p.Token, p.Name, p.Zone, p.Color, p.Money, p.Reputation, p.Discipline, p.Stamina, p.Sentence, p.TotalSentence, p.Crime, p.Finished, p.Cell}
}
func (s *state) snapshot(token string) map[string]any {
	me := s.p[token]
	others := []playerView{}
	for t, p := range s.p {
		if t != token && (p.Zone == me.Zone || p.Cell == me.Cell) {
			others = append(others, view(p))
		}
	}
	history := append([]map[string]any{}, s.chat["prison"]...)
	history = append(history, s.chat[fmt.Sprintf("cell-%d", me.Cell)]...)
	if len(history) > 50 {
		history = history[len(history)-50:]
	}
	return map[string]any{"t": "welcome", "me": view(me), "players": others, "clock": s.clock(), "events": s.eventView(), "chat": history, "social": s.socialView(token)}
}
func (s *state) clock() map[string]any {
	sec := int(time.Since(s.started).Seconds())
	day := sec/s.daySeconds + 1
	minute := (sec % s.daySeconds) * 24 * 60 / s.daySeconds
	hour := minute / 60
	activity := "Nghỉ ngơi"
	switch {
	case hour >= 6 && hour < 8:
		activity = "Điểm danh"
	case hour >= 8 && hour < 12:
		activity = "Lao động"
	case hour >= 12 && hour < 15:
		activity = "Ăn trưa"
	case hour >= 15 && hour < 19:
		activity = "Sân tập"
	case hour >= 19 && hour < 22:
		activity = "Học tập"
	default:
		activity = "Tắt đèn"
	}
	return map[string]any{"day": day, "hour": hour, "minute": minute % 60, "activity": activity}
}
func (s *state) handle(r Request) {
	p := s.p[r.Token]
	switch r.Msg.Type {
	case "join":
		name := game.CleanName(r.Msg.Name)
		if name == "" {
			r.Reply <- map[string]any{"error": "Biệt danh cần 2–16 ký tự và không chứa từ nhạy cảm."}
			return
		}
		if p == nil {
			p = game.NewPlayer(r.Token, name, r.Msg.Color, r.Msg.Hair, time.Now())
			p.Sentence = max(20, min(40, s.sentenceDays-10+rand.Intn(21)))
			p.TotalSentence = p.Sentence
			used := map[int]int{}
			for _, existing := range s.p {
				used[existing.Cell]++
			}
			for cell := 0; cell < 10; cell++ {
				if used[cell] < 6 {
					p.Cell = cell
					break
				}
			}
			s.p[r.Token] = p
		}
		s.out[r.Token] = r.Out
		delete(s.offline, r.Token)
		r.Reply <- s.snapshot(r.Token)
		s.broadcast(map[string]any{"t": "delta", "players": s.playersFor(p.Zone), "clock": s.clock()}, p.Zone, "")
	case "count":
		r.Reply <- len(s.p)
		return
	case "resume":
		if p == nil {
			r.Reply <- nil
			return
		}
		s.out[r.Token] = r.Out
		delete(s.offline, r.Token)
		r.Reply <- s.snapshot(r.Token)
	case "disconnect":
		delete(s.out, r.Token)
		if p != nil {
			s.offline[r.Token] = time.Now()
		}
		r.Reply <- true
	default:
		if p == nil {
			r.Reply <- nil
			return
		}
		s.action(r, p)
		r.Reply <- true
	}
}
func (s *state) playersFor(zone string) []playerView {
	a := []playerView{}
	for _, p := range s.p {
		if p.Zone == zone {
			a = append(a, view(p))
		}
	}
	return a
}
func (s *state) action(r Request, p *game.Player) {
	oldZone := p.Zone
	now := time.Now()
	if now.Sub(p.LastAction) < 125*time.Millisecond {
		s.send(r.Token, map[string]any{"t": "error", "text": "Chậm thôi, cai ngục đang nhìn."})
		return
	}
	p.LastAction = now
	switch r.Msg.Type {
	case "friend_request", "friend_reply", "friend_remove", "crew_create", "crew_join", "crew_leave", "crew_chat", "market_list", "market_buy", "market_cancel", "whisper", "emote", "profile_update", "leaderboard":
		s.socialAction(r, p)
	case "move":
		if game.Zones[r.Msg.Zone] && !(p.Zone == "solitary" && r.Msg.Zone != "solitary") {
			p.Zone = r.Msg.Zone
		}
	case "act":
		switch r.Msg.Act {
		case "work":
			if p.Zone != "workshop" {
				s.send(r.Token, map[string]any{"t": "error", "text": "Phải tới xưởng mới được chấm công."})
				break
			}
			if !p.WorkStarted.IsZero() {
				s.send(r.Token, map[string]any{"t": "error", "text": "Bạn đang làm dở một ca rồi."})
				break
			}
			p.WorkStarted = now
			p.WorkTarget = 43 + rand.Intn(16)
			s.send(r.Token, map[string]any{"t": "event", "kind": "minigame", "target": p.WorkTarget, "text": "Bấm khi kim chạm vùng xanh!"})
		case "meal", "noodles", "cigarette", "rope", "file", "snack":
			if !game.Buy(p, r.Msg.Act) {
				s.send(r.Token, map[string]any{"t": "error", "text": "Không đủ tiền trại hoặc món này không bán."})
			}
		case "rest":
			game.Rest(p)
		case "exercise":
			game.Exercise(p)
		case "study":
			game.Study(p)
		case "rollcall":
			game.AttendRollCall(p, true)
		case "escape":
			s.escape(r.Token, p)
		}
	case "minigame_result":
		elapsed := now.Sub(p.WorkStarted)
		score := 0
		if r.Msg.Position >= 0 && r.Msg.Position <= 100 {
			score = max(0, 100-abs(r.Msg.Position-p.WorkTarget)*3)
		}
		if p.Zone != "workshop" || p.WorkStarted.IsZero() || !game.ApplyWork(p, score, elapsed) {
			s.send(r.Token, map[string]any{"t": "error", "text": "Kết quả lao động không hợp lệ hoặc hết giờ."})
		}
		p.WorkStarted = time.Time{}
		p.WorkTarget = 0
	case "chat":
		if p.Zone == "solitary" {
			return
		}
		txt := game.CleanText(r.Msg.Text)
		if txt != "" {
			cell := ""
			channel := "prison"
			if r.Msg.Channel == "cell" {
				cell = fmt.Sprint(p.Cell)
				channel = fmt.Sprintf("cell-%d", p.Cell)
			}
			msg := map[string]any{"t": "chat", "from": p.Name, "text": txt, "ch": r.Msg.Channel}
			s.chat[channel] = append(s.chat[channel], msg)
			if len(s.chat[channel]) > 50 {
				s.chat[channel] = s.chat[channel][len(s.chat[channel])-50:]
			}
			s.broadcast(msg, "", cell)
		}
	case "trade_offer":
		target := r.Msg.Target
		if s.p[target] == nil {
			for id, candidate := range s.p {
				if candidate.Name == r.Msg.Target {
					target = id
					break
				}
			}
		}
		if r.Msg.Amount > 0 && r.Msg.Amount <= p.Money && s.p[target] != nil && target != r.Token {
			id := fmt.Sprintf("%d", rand.Int63())
			s.trades[id] = trade{r.Token, target, r.Msg.Amount, now.Add(20 * time.Second)}
			s.send(target, map[string]any{"t": "trade", "id": id, "from": p.Name, "amount": r.Msg.Amount, "text": "Có lời đề nghị chuyển tiền."})
		}
	case "trade_reply":
		tr, ok := s.trades[r.Msg.ID]
		if ok && tr.To == r.Token && now.Before(tr.Expires) {
			delete(s.trades, r.Msg.ID)
			a, b := s.p[tr.From], s.p[tr.To]
			if r.Msg.Choice == "accept" && a != nil && b != nil && a.Money >= tr.Amount {
				a.Money -= tr.Amount
				b.Money += tr.Amount
				a.Trades++
				b.Trades++
				s.send(tr.From, map[string]any{"t": "trade", "text": "Giao dịch thành công."})
				s.send(tr.To, map[string]any{"t": "trade", "text": "Giao dịch thành công."})
			} else {
				s.send(tr.From, map[string]any{"t": "trade", "text": "Giao dịch bị từ chối hoặc hết hạn."})
			}
		}
	case "event_choice":
		if r.Msg.Choice == "help" {
			p.Reputation = game.Clamp(p.Reputation + 2)
		} else {
			p.Money = max(0, p.Money-5)
		}
	}
	s.send(r.Token, map[string]any{"t": "delta", "me": view(p), "players": s.playersFor(p.Zone), "clock": s.clock(), "social": s.socialView(r.Token)})
	if oldZone != p.Zone {
		s.broadcast(map[string]any{"t": "delta", "players": s.playersFor(oldZone)}, oldZone, "")
	}
	s.broadcast(map[string]any{"t": "delta", "players": s.playersFor(p.Zone)}, p.Zone, "")
}
func (s *state) escape(token string, p *game.Player) {
	group := []*game.Player{}
	for _, x := range s.p {
		if x.Cell == p.Cell && x.Zone == "gate" {
			group = append(group, x)
		}
	}
	hour := s.clock()["hour"].(int)
	night := hour >= 22 || hour < 6
	success := game.Escape(group, night, 2, rand.Intn(100))
	if success {
		for _, escaped := range group {
			s.send(escaped.Token, map[string]any{"t": "end", "title": "Huyền thoại cổng sau", "text": "Bạn đã vượt ngục thành công!", "stats": view(escaped), "earned": escaped.Earned, "trades": escaped.Trades, "solitary": escaped.SolitaryCount, "days": s.clock()["day"]})
		}
	} else {
		s.send(token, map[string]any{"t": "event", "kind": "escape_failed", "text": "Kế hoạch bị phát hiện. Chúc mừng bạn nhận thêm 5 ngày nghỉ dưỡng biệt giam."})
	}
}
func (s *state) onTick() {
	now := time.Now()
	elapsed := int(now.Sub(s.started).Seconds())
	hour := (elapsed % s.daySeconds) * 24 / s.daySeconds
	if hour != s.lastHour && hour == 6 {
		for _, p := range s.p {
			game.AttendRollCall(p, p.Zone == "cell")
		}
	}
	s.lastHour = hour
	day := elapsed/s.daySeconds + 1
	if day > s.currentDay {
		if s.currentDay > 0 {
			for _, p := range s.p {
				if !p.Finished && p.Sentence > 0 {
					p.Sentence--
				}
			}
		}
		s.currentDay = day
	}
	for token, at := range s.offline {
		if now.Sub(at) > 60*time.Second {
			delete(s.p, token)
			delete(s.offline, token)
		}
	}
	for id, tr := range s.trades {
		if now.After(tr.Expires) {
			delete(s.trades, id)
		}
	}
	for token, p := range s.p {
		if p.Finished {
			continue
		}
		if p.SolitaryUntil.After(now) {
			p.Zone = "solitary"
		} else if p.Zone == "solitary" {
			p.Zone = "cell"
		}
		if p.Sentence <= 0 {
			p.Finished = true
			s.send(token, map[string]any{"t": "end", "title": "Mãn hạn!", "text": "Bạn đã chính thức được tự do.", "stats": view(p), "earned": p.Earned, "trades": p.Trades, "solitary": p.SolitaryCount, "days": s.clock()["day"]})
		}
		if s.tick%2 == 0 {
			s.send(token, map[string]any{"t": "delta", "me": view(p), "clock": s.clock()})
		}
	}
	if s.tick%240 == 0 {
		s.startSocialEvent()
	}
	if s.tick%120 == 0 {
		s.broadcast(map[string]any{"t": "event", "kind": "inspection", "text": "Cai ngục kiểm tra bất ngờ! Ai giấu mì tôm thì tự biết nhé."}, "", "")
	}
}
func (p *Prison) Count() int {
	v := p.Call(Request{Msg: proto.Message{Type: "count"}})
	n, _ := v.(int)
	return n
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
