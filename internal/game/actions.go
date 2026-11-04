package game

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"dituonline/internal/store"
)

// near reports whether p stands within r metres (same floor) of a point.
func near(p *Player, x, y, z, r float64) bool {
	return dist2(p.body.X, p.body.Z, x, z) <= r*r && math.Abs(p.body.Y-y) < 1.6
}

func (i *Instance) use(p *Player, id string, now time.Time) {
	o, ok := i.cfg.Map.Obj(id)
	if !ok || p.iso(now) || !near(p, o.X, o.Y, o.Z, 2.8) {
		return
	}
	if p.act != nil {
		i.toast(p, "warn", "Bạn đang bận.")
		return
	}
	if o.Kind == "counter" {
		shop := []map[string]any{{"id": "meal", "name": "Suất cơm", "price": mealPrice}}
		for _, k := range []string{"noodle", "cigarette"} {
			shop = append(shop, map[string]any{"id": k, "name": items[k].Name, "price": items[k].Price})
		}
		p.cl.Send(map[string]any{"t": "shop", "items": shop})
		return
	}
	kind, ok := objAct[o.Kind]
	if !ok {
		return
	}
	if kind == "cut" {
		i.startEscape(p, o, now)
		return
	}
	def := acts[kind]
	d := &p.Char.Data
	switch {
	case def.Room != "" && p.room != def.Room:
		i.toast(p, "warn", "Bạn phải ở trong "+roomName(i, def.Room)+" để "+strings.ToLower(def.Label)+".")
		return
	case def.Stamina > 0 && d.Stamina < def.Stamina:
		i.toast(p, "warn", "Bạn quá mệt. Hãy nghỉ hoặc ăn gì đó.")
		return
	case kind == "rest" && d.Stamina >= 100:
		i.toast(p, "warn", "Bạn chưa cần nghỉ.")
		return
	case kind == "rest" && p.room != fmt.Sprintf("cell%d", d.Cell):
		i.toast(p, "warn", "Đây không phải giường của bạn (buồng giam "+itoa(d.Cell)+").")
		return
	}
	p.emote = 0
	p.act = &action{kind: kind, obj: id, sx: p.body.X, sz: p.body.Z, end: now.Add(def.Dur)}
	p.cl.Send(map[string]any{"t": "act", "k": kind, "label": def.Label, "ms": def.Dur.Milliseconds()})
}

func roomName(i *Instance, id string) string {
	if r, ok := i.cfg.Map.Room(id); ok {
		return r.Name
	}
	return id
}

func (i *Instance) cancelAct(p *Player, why string) {
	if p.act == nil {
		return
	}
	p.act = nil
	if p.cl != nil {
		p.cl.Send(map[string]any{"t": "act", "k": ""})
		if why != "" {
			i.toast(p, "warn", why)
		}
	}
}

func (i *Instance) finishAct(p *Player, now time.Time) {
	a := p.act
	p.act = nil
	d := &p.Char.Data
	def := acts[a.kind]
	if p.cl != nil {
		p.cl.Send(map[string]any{"t": "act", "k": ""})
	}
	if def.Stamina > 0 && d.Stamina < def.Stamina {
		return
	}
	d.Stamina -= def.Stamina
	switch a.kind {
	case "rest":
		d.Stamina = clampInt(d.Stamina+35, 0, 100)
		i.toast(p, "ok", "Bạn đã nghỉ ngơi. Thể lực +35.")
	case "work":
		reward := 5 + i.rng.Intn(5)
		if i.phase == "work" {
			reward = reward * 3 / 2
		}
		p.Char.Money += reward
		msg := fmt.Sprintf("Lao động xong: +%d tiền trại", reward)
		if i.rng.Intn(100) < 30 {
			d.Inv["wire"]++
			msg += ", nhặt được Dây thép"
		}
		if i.rng.Intn(100) < 20 {
			d.Inv["handle"]++
			msg += ", nhặt được Cán gỗ"
		}
		i.toast(p, "ok", msg+".")
		i.questProgress(p, "work")
	case "study":
		pts := 1
		if i.phase == "study" {
			pts = 2
		}
		for k := 0; k < pts; k++ {
			p.Char.Points++
			if p.Char.Points%5 == 0 && d.Sentence > 1 {
				d.Sentence--
				i.toast(p, "ok", "Điểm cải tạo đủ mốc: giảm án 1 ngày!")
			}
		}
		i.toast(p, "ok", fmt.Sprintf("Học xong: điểm cải tạo +%d.", pts))
		i.questProgress(p, "study")
	case "gym":
		p.Char.Rep++
		i.toast(p, "ok", "Tập xong: uy tín +1.")
		i.questProgress(p, "gym")
	}
	p.dirty = true
	if p.cl != nil {
		p.cl.Send(i.meMsg(p, now))
	}
}

func (i *Instance) eat(p *Player, item string) {
	def, ok := items[item]
	d := &p.Char.Data
	if !ok || def.Eat == 0 || d.Inv[item] < 1 || p.iso(i.now()) {
		return
	}
	d.Inv[item]--
	d.Stamina = clampInt(d.Stamina+def.Eat, 0, 100)
	p.dirty = true
	i.toast(p, "ok", "Đã ăn "+def.Name+". Thể lực +"+itoa(def.Eat)+".")
	i.questProgress(p, "eat")
	p.cl.Send(i.meMsg(p, i.now()))
}

func (i *Instance) buy(p *Player, item string) {
	c, _ := i.cfg.Map.Obj("counter")
	if p.iso(i.now()) || !near(p, c.X, c.Y, c.Z, 3.5) {
		return
	}
	price := 0
	if item == "meal" {
		price = mealPrice
	} else if def, ok := items[item]; ok && def.Price > 0 {
		price = def.Price
	} else {
		return
	}
	if p.Char.Money < price {
		i.toast(p, "warn", "Không đủ tiền trại.")
		return
	}
	p.Char.Money -= price
	d := &p.Char.Data
	if item == "meal" {
		d.Stamina = clampInt(d.Stamina+30, 0, 100)
		i.toast(p, "ok", "Đã ăn suất cơm. Thể lực +30.")
		i.questProgress(p, "eat")
	} else {
		d.Inv[item]++
		i.toast(p, "ok", "Đã mua "+items[item].Name+".")
	}
	p.dirty = true
	p.cl.Send(i.meMsg(p, i.now()))
}

type recipe struct {
	Out string
	In  map[string]int
}

var recipes = map[string]recipe{"pliers": {"pliers", map[string]int{"wire": 2, "handle": 1}}}

func (i *Instance) craft(p *Player, id string) {
	r, ok := recipes[id]
	if !ok || p.iso(i.now()) {
		return
	}
	nearBench := false
	for _, o := range i.cfg.Map.Objs {
		if o.Kind == "bench" && near(p, o.X, o.Y, o.Z, 3) {
			nearBench = true
		}
	}
	if !nearBench {
		i.toast(p, "warn", "Hãy đứng gần bàn làm việc trong xưởng để chế tạo.")
		return
	}
	d := &p.Char.Data
	for k, n := range r.In {
		if d.Inv[k] < n {
			i.toast(p, "warn", "Thiếu nguyên liệu: cần Dây thép x2 và Cán gỗ x1.")
			return
		}
	}
	for k, n := range r.In {
		d.Inv[k] -= n
	}
	d.Inv[r.Out]++
	p.dirty = true
	i.toast(p, "ok", "Đã chế tạo "+items[r.Out].Name+".")
	i.questProgress(p, "craft")
	p.cl.Send(i.meMsg(p, i.now()))
}

// ---- trading ----

func (i *Instance) offer(p *Player, m Msg, now time.Time) {
	t := i.byName[strings.ToLower(m.To)]
	if t == nil || t == p || t.cl == nil || p.iso(now) || t.iso(now) || m.Qty < 1 || m.Qty > 99 || m.Price < 0 || m.Price > 100000 {
		i.toast(p, "warn", "Không thể gửi đề nghị này.")
		return
	}
	if _, ok := items[m.Item]; !ok || p.Char.Data.Inv[m.Item] < m.Qty {
		i.toast(p, "warn", "Bạn không đủ vật phẩm.")
		return
	}
	if dist2(p.body.X, p.body.Z, t.body.X, t.body.Z) > 6*6 || math.Abs(p.body.Y-t.body.Y) > 2 {
		i.toast(p, "warn", "Người kia ở quá xa.")
		return
	}
	if p.offerOut != 0 {
		i.toast(p, "warn", "Bạn đang có một đề nghị chưa được trả lời.")
		return
	}
	i.tradeSeq++
	tr := &trade{id: i.tradeSeq, from: p.id(), to: t.id(), item: m.Item, qty: m.Qty, price: m.Price, expires: now.Add(30 * time.Second)}
	i.trades[tr.id] = tr
	p.offerOut = tr.id
	t.cl.Send(map[string]any{"t": "offer", "id": tr.id, "from": p.name(), "item": m.Item, "name": items[m.Item].Name, "qty": m.Qty, "price": m.Price})
	i.toast(p, "ok", "Đã gửi đề nghị cho "+t.name()+".")
}

func (i *Instance) offerReply(p *Player, m Msg, now time.Time) {
	tr := i.trades[m.N]
	if tr == nil || tr.to != p.id() {
		return
	}
	delete(i.trades, tr.id)
	seller := i.players[tr.from]
	if seller != nil {
		seller.offerOut = 0
	}
	if !m.Accept {
		if seller != nil {
			i.toast(seller, "warn", p.name()+" đã từ chối đề nghị.")
		}
		return
	}
	switch {
	case seller == nil || seller.cl == nil || seller.iso(now) || p.iso(now):
		i.toast(p, "warn", "Giao dịch không còn hiệu lực.")
	case dist2(p.body.X, p.body.Z, seller.body.X, seller.body.Z) > 8*8:
		i.toast(p, "warn", "Hai người đã đi quá xa nhau.")
	case seller.Char.Data.Inv[tr.item] < tr.qty:
		i.toast(p, "warn", "Người bán không còn đủ vật phẩm.")
	case p.Char.Money < tr.price:
		i.toast(p, "warn", "Bạn không đủ tiền trại.")
	default:
		seller.Char.Data.Inv[tr.item] -= tr.qty
		p.Char.Data.Inv[tr.item] += tr.qty
		p.Char.Money -= tr.price
		seller.Char.Money += tr.price
		seller.dirty, p.dirty = true, true
		i.toast(p, "ok", "Đã mua "+items[tr.item].Name+" x"+itoa(tr.qty)+".")
		i.toast(seller, "ok", p.name()+" đã mua đồ của bạn: +"+itoa(tr.price)+" tiền trại.")
		seller.cl.Send(i.meMsg(seller, now))
		p.cl.Send(i.meMsg(p, now))
	}
}

// ---- quests ----

func isoWeekKey(t time.Time) string {
	y, w := t.UTC().ISOWeek()
	return fmt.Sprintf("%d-W%02d", y, w)
}

func (i *Instance) ensureQuests(p *Player, now time.Time) {
	q := &p.Char.Data.Quests
	dk := now.UTC().Format("2006-01-02")
	if q.DayKey != dk || len(q.Daily) == 0 {
		q.DayKey = dk
		h := 0
		for _, c := range dk {
			h = h*31 + int(c)
		}
		ids := make([]string, 0, len(questDefs))
		for _, d := range questDefs {
			ids = append(ids, d.ID)
		}
		sort.Strings(ids)
		start := h % len(ids)
		q.Daily = q.Daily[:0]
		for k := 0; k < 3; k++ {
			q.Daily = append(q.Daily, store.QState{ID: ids[(start+k*2)%len(ids)]})
		}
		p.dirty = true
	}
	if wk := isoWeekKey(now); q.WeekKey != wk {
		q.WeekKey, q.WeekProg, q.WeekDone = wk, 0, false
		p.dirty = true
	}
}

func (i *Instance) questProgress(p *Player, id string) {
	q := &p.Char.Data.Quests
	for k := range q.Daily {
		qs := &q.Daily[k]
		def, ok := qdefByID(qs.ID)
		if qs.ID != id || qs.Done || !ok {
			continue
		}
		if qs.Prog++; qs.Prog >= def.Target {
			qs.Done = true
			p.Char.Money += def.Money
			p.Char.Rep += def.Rep
			i.toast(p, "ok", fmt.Sprintf("Hoàn thành nhiệm vụ \"%s\": +%d tiền trại, +%d uy tín.", def.Label, def.Money, def.Rep))
			if q.WeekProg++; q.WeekProg >= weeklyTarget && !q.WeekDone {
				q.WeekDone = true
				p.Char.Money += weeklyMoney
				p.Char.Rep += weeklyRep
				i.toast(p, "ok", fmt.Sprintf("Hoàn thành nhiệm vụ tuần: +%d tiền trại, +%d uy tín!", weeklyMoney, weeklyRep))
			}
		}
	}
	p.dirty = true
}
