package game

import (
	"fmt"
	"math"
	"time"

	"dituonline/internal/world"
)

const (
	escapeRadius  = 5.0
	escapeMinSize = 3
)

func (i *Instance) escapeGroup(fence world.Obj, now time.Time) []*Player {
	var g []*Player
	for _, o := range i.players {
		if o.cl != nil && !o.iso(now) && o.act == nil && o.Char.Data.Inv["pliers"] > 0 && near(o, fence.X, fence.Y, fence.Z, escapeRadius) {
			g = append(g, o)
		}
	}
	return g
}

// startEscape begins a cooperative fence cut: at least three players, each with pliers,
// must stand next to the weak fence section and stay put for the whole duration.
func (i *Instance) startEscape(p *Player, fence world.Obj, now time.Time) {
	if i.esc != nil {
		i.toast(p, "warn", "Đã có một nhóm đang cắt hàng rào.")
		return
	}
	if p.Char.Data.Inv["pliers"] < 1 {
		i.toast(p, "warn", "Bạn cần Kìm cắt (chế tạo ở xưởng từ 2 Dây thép + 1 Cán gỗ).")
		return
	}
	g := i.escapeGroup(fence, now)
	if len(g) < escapeMinSize {
		i.toast(p, "warn", fmt.Sprintf("Cần ít nhất %d người có Kìm cắt đứng gần hàng rào (hiện có %d).", escapeMinSize, len(g)))
		return
	}
	def := acts["cut"]
	e := &escape{end: now.Add(def.Dur)}
	for _, o := range g {
		o.emote = 0
		o.act = &action{kind: "cut", obj: fence.ID, sx: o.body.X, sz: o.body.Z, end: e.end, group: true}
		e.members = append(e.members, o.id())
		o.cl.Send(map[string]any{"t": "act", "k": "cut", "label": def.Label, "ms": def.Dur.Milliseconds()})
	}
	i.esc = e
	i.broadcastChat("sys", "", "Có tiếng động lạ gần hàng rào phía đông...", "")
}

func (i *Instance) stepEscape(now time.Time) {
	e := i.esc
	if e == nil {
		return
	}
	var members []*Player
	for _, id := range e.members {
		p := i.players[id]
		if p == nil || p.cl == nil || p.act == nil || p.act.kind != "cut" {
			i.abortEscape("Nhóm vượt ngục bị gián đoạn: có người rời vị trí.")
			return
		}
		members = append(members, p)
	}
	if now.Before(e.end) {
		return
	}
	i.esc = nil
	fence, _ := i.cfg.Map.Obj("fence")
	night := i.phase == "night"
	g := i.guard
	far := math.Sqrt(dist2(g.x, g.z, fence.X, fence.Z)) > 25
	chance := 30 + 6*(len(members)-escapeMinSize)
	if night {
		chance += 20
	}
	if far {
		chance += 15
	}
	chance = clampInt(chance, 5, 85)
	alarm := i.rng.Intn(100) < 12
	free, caught := 0, 0
	for _, p := range members {
		p.act = nil
		p.cl.Send(map[string]any{"t": "act", "k": ""})
		p.Char.Data.Inv["pliers"]--
		if !alarm && i.rng.Intn(100) < chance {
			free++
			p.Char.Escapes++
			p.Char.Money += 150
			p.Char.Rep += 15
			i.toast(p, "ok", "Vượt ngục thành công! Bạn lẻn ra ngoài, kiếm được 150 tiền trại rồi chui về trước giờ điểm danh.")
		} else {
			caught++
			i.punish(p, now, "vượt ngục bất thành", 90*time.Second, 10, 15, 5)
		}
		p.dirty = true
	}
	outcome := "toàn bộ nhóm bị bắt"
	switch {
	case caught == 0:
		outcome = "cả nhóm trốn thoát"
	case free > 0:
		outcome = fmt.Sprintf("%d người trốn thoát, %d người bị bắt", free, caught)
	}
	i.broadcastChat("sys", "", "Vụ vượt ngục kết thúc: "+outcome+".", "")
}

func (i *Instance) abortEscape(why string) {
	e := i.esc
	i.esc = nil
	for _, id := range e.members {
		if p := i.players[id]; p != nil && p.act != nil && p.act.kind == "cut" {
			p.act = nil
			if p.cl != nil {
				p.cl.Send(map[string]any{"t": "act", "k": ""})
				i.toast(p, "warn", why)
			}
		}
	}
}
