package prison

import (
	"dituonline/internal/game"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"
)

type crew struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Leader  string          `json:"leader"`
	Members map[string]bool `json:"-"`
	Created time.Time       `json:"created"`
	Score   int             `json:"score"`
}
type listing struct {
	ID         string    `json:"id"`
	Seller     string    `json:"seller"`
	SellerName string    `json:"sellerName"`
	Item       string    `json:"item"`
	Quantity   int       `json:"quantity"`
	Price      int       `json:"price"`
	Created    time.Time `json:"created"`
}

var marketItems = map[string]bool{"meal": true, "noodles": true, "snack": true, "rope": true, "file": true, "cigarette": true}

func (s *state) socialView(token string) map[string]any {
	p := s.p[token]
	friends := []map[string]any{}
	for id := range s.friends[token] {
		if x := s.p[id]; x != nil {
			friends = append(friends, map[string]any{"id": id, "name": x.Name, "online": s.out[id] != nil})
		}
	}
	sort.Slice(friends, func(i, j int) bool { return fmt.Sprint(friends[i]["name"]) < fmt.Sprint(friends[j]["name"]) })
	requests := []map[string]any{}
	for id := range s.friendReq[token] {
		if x := s.p[id]; x != nil {
			requests = append(requests, map[string]any{"id": id, "name": x.Name})
		}
	}
	var myCrew any
	if cid := s.playerCrew[token]; cid != "" {
		if c := s.crews[cid]; c != nil {
			members := []map[string]any{}
			for id := range c.Members {
				if x := s.p[id]; x != nil {
					role := "Thành viên"
					if id == c.Leader {
						role = "Trưởng nhóm"
					}
					members = append(members, map[string]any{"id": id, "name": x.Name, "role": role})
				}
			}
			myCrew = map[string]any{"id": c.ID, "name": c.Name, "leader": c.Leader, "members": members, "score": c.Score}
		}
	}
	listings := []*listing{}
	for _, l := range s.listings {
		if l.Seller != token {
			cp := *l
			listings = append(listings, &cp)
		}
	}
	sort.Slice(listings, func(i, j int) bool { return listings[i].Created.After(listings[j].Created) })
	if len(listings) > 30 {
		listings = listings[:30]
	}
	type rank struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		Money      int    `json:"money"`
		Reputation int    `json:"rep"`
		Score      int    `json:"score"`
	}
	ranks := []rank{}
	for id, x := range s.p {
		ranks = append(ranks, rank{id, x.Name, x.Money, x.Reputation, s.seasonScore[id]})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].Score == ranks[j].Score {
			return ranks[i].Reputation > ranks[j].Reputation
		}
		return ranks[i].Score > ranks[j].Score
	})
	if len(ranks) > 10 {
		ranks = ranks[:10]
	}
	inventory := map[string]int{}
	if p != nil {
		for k, v := range p.Items {
			inventory[k] = v
		}
	}
	availableCrews := []map[string]any{}
	for _, c := range s.crews {
		if s.playerCrew[token] != c.ID {
			availableCrews = append(availableCrews, map[string]any{"id": c.ID, "name": c.Name, "members": len(c.Members), "score": c.Score})
		}
	}
	sort.Slice(availableCrews, func(i, j int) bool {
		return fmt.Sprint(availableCrews[i]["name"]) < fmt.Sprint(availableCrews[j]["name"])
	})
	return map[string]any{"friends": friends, "requests": requests, "crew": myCrew, "crews": availableCrews, "listings": listings, "leaderboard": ranks, "inventory": inventory, "score": s.seasonScore[token], "event": s.eventView()}
}

func (s *state) eventView() map[string]any {
	if s.eventName == "" || time.Now().After(s.eventUntil) {
		return map[string]any{"active": false}
	}
	return map[string]any{"active": true, "name": s.eventName, "until": s.eventUntil.Unix()}
}

func (s *state) socialAction(r Request, p *game.Player) {
	m := r.Msg
	find := func(target string) string {
		if s.p[target] != nil {
			return target
		}
		for id, x := range s.p {
			if strings.EqualFold(x.Name, target) {
				return id
			}
		}
		return ""
	}
	switch m.Type {
	case "friend_request":
		target := find(m.Target)
		if target == "" || target == r.Token || s.friends[r.Token][target] {
			s.send(r.Token, map[string]any{"t": "social_notice", "text": "Không thể gửi lời mời kết bạn này."})
			return
		}
		if s.friendReq[target] == nil {
			s.friendReq[target] = map[string]bool{}
		}
		s.friendReq[target][r.Token] = true
		s.send(target, map[string]any{"t": "social_notice", "text": p.Name + " đã gửi lời mời kết bạn.", "refresh": true})
		s.send(r.Token, map[string]any{"t": "social_notice", "text": "Đã gửi lời mời kết bạn."})
	case "friend_reply":
		target := find(m.Target)
		if target != "" && s.friendReq[r.Token][target] {
			delete(s.friendReq[r.Token], target)
			if m.Choice == "accept" {
				if s.friends[r.Token] == nil {
					s.friends[r.Token] = map[string]bool{}
				}
				if s.friends[target] == nil {
					s.friends[target] = map[string]bool{}
				}
				s.friends[r.Token][target], s.friends[target][r.Token] = true, true
				s.seasonScore[r.Token] += 10
				s.seasonScore[target] += 10
				p.Reputation = game.Clamp(p.Reputation + 1)
				if x := s.p[target]; x != nil {
					x.Reputation = game.Clamp(x.Reputation + 1)
					s.send(target, map[string]any{"t": "social_notice", "text": p.Name + " đã chấp nhận lời mời kết bạn.", "refresh": true})
				}
			}
		}
	case "friend_remove":
		target := find(m.Target)
		delete(s.friends[r.Token], target)
		delete(s.friends[target], r.Token)
	case "crew_create":
		name := strings.TrimSpace(m.Name)
		if s.playerCrew[r.Token] != "" || len([]rune(name)) < 3 || len([]rune(name)) > 20 {
			s.send(r.Token, map[string]any{"t": "social_notice", "text": "Tên nhóm 3–20 ký tự; bạn phải chưa thuộc nhóm nào."})
			return
		}
		id := fmt.Sprintf("B-%03d", len(s.crews)+1)
		c := &crew{ID: id, Name: name, Leader: r.Token, Members: map[string]bool{r.Token: true}, Created: time.Now()}
		s.crews[id], s.playerCrew[r.Token] = c, id
	case "crew_join":
		id := strings.TrimSpace(m.Target)
		c := s.crews[id]
		if c == nil || s.playerCrew[r.Token] != "" || len(c.Members) >= 20 {
			s.send(r.Token, map[string]any{"t": "social_notice", "text": "Không thể tham gia nhóm này."})
			return
		}
		c.Members[r.Token], s.playerCrew[r.Token] = true, id
		c.Score += 5
		s.seasonScore[r.Token] += 5
	case "crew_leave":
		id := s.playerCrew[r.Token]
		if c := s.crews[id]; c != nil {
			delete(c.Members, r.Token)
			delete(s.playerCrew, r.Token)
			if c.Leader == r.Token {
				c.Leader = ""
				for member := range c.Members {
					c.Leader = member
					break
				}
			}
			if len(c.Members) == 0 {
				delete(s.crews, id)
			}
		}
	case "crew_chat":
		if c := s.crews[s.playerCrew[r.Token]]; c != nil {
			msg := map[string]any{"t": "chat", "from": p.Name, "text": game.CleanText(m.Text), "ch": "crew"}
			for member := range c.Members {
				s.send(member, msg)
			}
		}
	case "market_list":
		if !marketItems[m.Item] || m.Amount < 1 || m.Amount > 20 || m.Price < 1 || m.Price > 10000 || p.Items[m.Item] < m.Amount {
			s.send(r.Token, map[string]any{"t": "social_notice", "text": "Vật phẩm, số lượng hoặc giá bán không hợp lệ."})
			return
		}
		p.Items[m.Item] -= m.Amount
		s.nextListing++
		id := fmt.Sprintf("M-%d", s.nextListing)
		s.listings[id] = &listing{ID: id, Seller: r.Token, SellerName: p.Name, Item: m.Item, Quantity: m.Amount, Price: m.Price, Created: time.Now()}
	case "market_buy":
		l := s.listings[m.ID]
		if l == nil || l.Seller == r.Token || p.Money < l.Price {
			s.send(r.Token, map[string]any{"t": "social_notice", "text": "Tin đăng không còn hoặc bạn không đủ tiền."})
			return
		}
		seller := s.p[l.Seller]
		if seller == nil {
			s.send(r.Token, map[string]any{"t": "social_notice", "text": "Người bán đã rời trại; tin đăng bị huỷ."})
			delete(s.listings, m.ID)
			return
		}
		p.Money -= l.Price
		seller.Money += l.Price
		s.seasonScore[r.Token] += 5
		s.seasonScore[l.Seller] += 5
		p.Items[l.Item] += l.Quantity
		seller.Trades++
		p.Trades++
		delete(s.listings, m.ID)
		s.send(l.Seller, map[string]any{"t": "social_notice", "text": "Đã bán " + l.Item + " cho " + p.Name + "."})
	case "market_cancel":
		if l := s.listings[m.ID]; l != nil && l.Seller == r.Token {
			p.Items[l.Item] += l.Quantity
			delete(s.listings, m.ID)
		}
	case "whisper":
		target, txt := find(m.Target), game.CleanText(m.Text)
		if target != "" && target != r.Token && txt != "" {
			msg := map[string]any{"t": "chat", "from": p.Name, "text": txt, "ch": "private", "private": true}
			s.send(target, msg)
			s.send(r.Token, msg)
		}
	case "emote":
		allowed := map[string]bool{"wave": true, "laugh": true, "angry": true, "heart": true, "dance": true}
		if allowed[m.Choice] {
			s.seasonScore[r.Token]++
			s.broadcast(map[string]any{"t": "emote", "id": r.Token, "name": p.Name, "emote": m.Choice}, p.Zone, "")
		}
	case "profile_update":
		if m.Color != "" && len(m.Color) <= 20 {
			p.Color = m.Color
		}
		if m.Hair != "" && map[string]bool{"short": true, "spiky": true, "bald": true}[m.Hair] {
			p.Hair = m.Hair
		}
		if m.Name != "" {
			if n := game.CleanName(m.Name); n != "" {
				p.Name = n
			}
		}
	case "leaderboard":
		s.send(r.Token, map[string]any{"t": "social", "social": s.socialView(r.Token)})
	}
	s.send(r.Token, map[string]any{"t": "social", "social": s.socialView(r.Token)})
}

func (s *state) startSocialEvent() {
	events := []string{"Ngày hội kết nối: gửi lời chào cho 3 phạm nhân!", "Phiên chợ náo nhiệt: đăng bán hoặc mua một món đồ!", "Thi đua phòng giam: cùng nhau tích luỹ danh tiếng!"}
	s.eventName = events[rand.Intn(len(events))]
	s.eventUntil = time.Now().Add(90 * time.Second)
	s.broadcast(map[string]any{"t": "event", "kind": "social_event", "text": s.eventName}, "", "")
}
