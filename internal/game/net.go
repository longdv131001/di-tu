package game

import (
	"math"
	"strings"
	"time"
)

const guardID = -1

func (i *Instance) broadcast(v any) {
	for _, p := range i.players {
		if p.cl != nil {
			p.cl.Send(v)
		}
	}
}

func (i *Instance) toast(p *Player, kind, text string) {
	if p.cl != nil {
		p.cl.Send(map[string]any{"t": "toast", "k": kind, "text": text})
	}
}

func (i *Instance) broadcastToast(kind, text string) {
	i.broadcast(map[string]any{"t": "toast", "k": kind, "text": text})
}

func (i *Instance) meMsg(p *Player, now time.Time) map[string]any {
	d := p.Char.Data
	inv := map[string]int{}
	for k, v := range d.Inv {
		if v > 0 {
			inv[k] = v
		}
	}
	daily := make([]map[string]any, 0, len(d.Quests.Daily))
	for _, q := range d.Quests.Daily {
		def, _ := qdefByID(q.ID)
		daily = append(daily, map[string]any{"id": q.ID, "label": def.Label, "prog": q.Prog, "target": def.Target, "done": q.Done, "money": def.Money})
	}
	isoLeft := 0
	if p.iso(now) {
		isoLeft = int(p.isoUntil.Sub(now).Seconds()) + 1
	}
	return map[string]any{"t": "me", "me": map[string]any{
		"id": p.id(), "name": p.name(), "money": p.Char.Money, "rep": p.Char.Rep, "points": p.Char.Points,
		"stamina": d.Stamina, "disc": d.Discipline, "sentence": d.Sentence, "cell": d.Cell, "inv": inv,
		"escapes": p.Char.Escapes, "releases": p.Char.Releases, "outfit": p.Char.Outfit, "face": d.Face,
		"iso": isoLeft, "muted": now.Before(p.mutedUntil), "room": p.room,
		"quests": map[string]any{"daily": daily, "week": map[string]any{"prog": d.Quests.WeekProg, "target": weeklyTarget, "done": d.Quests.WeekDone, "money": weeklyMoney}},
	}}
}

func (i *Instance) sendWelcome(p *Player) {
	open := []string{}
	for id := range i.open {
		open = append(open, id)
	}
	now := i.now()
	me := i.meMsg(p, now)
	p.cl.Send(map[string]any{"t": "welcome", "id": p.id(), "inst": i.cfg.ID, "mapv": i.cfg.Map.Version,
		"pos": []float64{r2(p.body.X), r2(p.body.Y), r2(p.body.Z), r2(p.yaw)}, "doors": open, "clock": i.clockMsg(),
		"me": me["me"], "dt": 50})
}

type meta struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Outfit any    `json:"outfit"`
	Guard  bool   `json:"guard,omitempty"`
	Ver    int    `json:"v"`
}

func (i *Instance) broadcastSnap() {
	aoi2 := i.cfg.AOI * i.cfg.AOI
	for _, r := range i.players {
		if r.cl == nil {
			continue
		}
		ents := make([][7]any, 0, 16)
		var news []meta
		seen := map[int64]bool{}
		add := func(id int64, x, y, z, yaw float64, anim, face int, mk func() meta, ver int) {
			seen[id] = true
			ents = append(ents, [7]any{id, r2(x), r2(y), r2(z), r2(yaw), anim, face})
			if v, ok := r.known[id]; !ok || v != ver {
				r.known[id] = ver
				news = append(news, mk())
			}
		}
		for _, o := range i.players {
			if o == r {
				continue
			}
			if dist2(r.body.X, r.body.Z, o.body.X, o.body.Z) > aoi2 && !(o.room != "yard" && o.room == r.room) {
				continue
			}
			o := o
			add(o.id(), o.body.X, o.body.Y, o.body.Z, o.yaw, o.anim, o.Char.Data.Face,
				func() meta { return meta{ID: o.id(), Name: o.name(), Outfit: o.Char.Outfit, Ver: o.metaVer} }, o.metaVer)
		}
		g := i.guard
		if dist2(r.body.X, r.body.Z, g.x, g.z) <= aoi2 {
			add(guardID, g.x, g.y, g.z, g.yaw, g.anim, 0,
				func() meta { return meta{ID: guardID, Name: "Cai ngục", Guard: true, Ver: 0} }, 0)
		}
		var gone []int64
		for id := range r.known {
			if !seen[id] {
				gone = append(gone, id)
				delete(r.known, id)
			}
		}
		msg := map[string]any{"t": "snap", "k": i.tickN, "a": r.ack,
			"me": []any{r2(r.body.X), r2(r.body.Y), r2(r.body.Z), r2(r.body.VY), boolInt(r.body.Grounded)}, "p": ents, "rm": r.room}
		if len(news) > 0 {
			msg["n"] = news
		}
		if len(gone) > 0 {
			msg["x"] = gone
		}
		r.cl.Send(msg)
	}
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// ---- incoming messages ----

func (i *Instance) handle(p *Player, m Msg) {
	now := i.now()
	switch m.T {
	case "in":
		if p.iso(now) {
			return
		}
		if len(p.queue) >= 4 {
			p.queue = p.queue[1:]
		}
		if math.IsNaN(m.MX) || math.IsNaN(m.MZ) || math.IsNaN(m.Yaw) {
			return
		}
		p.queue = append(p.queue, inp{m.S, worldInput(m)})
	case "ping":
		p.cl.Send(map[string]any{"t": "pong", "ts": m.TS})
	case "chat":
		i.chat(p, m, now)
	case "door":
		i.toggleDoor(p, m.ID, now)
	case "use":
		i.use(p, m.ID, now)
	case "eat":
		i.eat(p, m.Item)
	case "buy":
		i.buy(p, m.Item)
	case "craft":
		i.craft(p, m.ID)
	case "offer":
		i.offer(p, m, now)
	case "offer_reply":
		i.offerReply(p, m, now)
	case "cancel":
		if p.act != nil {
			i.cancelAct(p, "")
		}
	case "emote":
		if m.N >= 1 && m.N <= 4 && p.act == nil && !p.iso(now) {
			p.emote, p.emoteEnd = m.N, now.Add(4*time.Second)
			if m.N == 4 {
				p.emoteEnd = now.Add(time.Hour) // sitting lasts until you move
			}
		}
	case "face":
		if m.N >= 0 && m.N <= 4 {
			p.Char.Data.Face = m.N
			p.dirty = true
		}
	case "outfit":
		if o := m.Outfit; o != nil && o.Shirt >= 0 && o.Shirt < 12 && o.Pants >= 0 && o.Pants < 8 && o.Skin >= 0 && o.Skin < 6 &&
			o.Hair >= 0 && o.Hair < 4 && o.HairC >= 0 && o.HairC < 8 && o.Hat >= 0 && o.Hat < 3 {
			p.Char.Outfit = *o
			p.metaVer++
			p.dirty = true
			p.cl.Send(i.meMsg(p, now))
		}
	case "report":
		i.report(p, m, now)
	}
}

func (i *Instance) toggleDoor(p *Player, id string, now time.Time) {
	d, ok := i.cfg.Map.Door(id)
	if !ok || p.iso(now) || dist2(p.body.X, p.body.Z, d.X, d.Z) > 2.8*2.8 || math.Abs(p.body.Y-d.Y) > 1.5 {
		return
	}
	if d.Locked {
		i.toast(p, "warn", "Cửa này bị khóa.")
		return
	}
	_, isOpen := i.open[id]
	if isOpen {
		delete(i.open, id)
	} else {
		i.open[id] = now
	}
	i.broadcast(map[string]any{"t": "door", "id": id, "o": !isOpen})
}

func (i *Instance) report(p *Player, m Msg, now time.Time) {
	if now.Sub(p.reportAt) < 30*time.Second {
		i.toast(p, "warn", "Bạn báo cáo quá nhanh, hãy đợi một chút.")
		return
	}
	t := i.byName[strings.ToLower(m.To)]
	reason := cleanText(m.Text, 200)
	if t == nil || t == p || reason == "" {
		i.toast(p, "warn", "Không thể gửi báo cáo này.")
		return
	}
	p.reportAt = now
	st, from, to := i.cfg.Store, p.id(), t.id()
	go st.AddReport(contextBG(), from, to, reason)
	i.toast(p, "ok", "Đã gửi báo cáo tới quản trị viên.")
}
