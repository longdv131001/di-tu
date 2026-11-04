package game

import (
	"math"
	"strings"
	"time"

	"dituonline/internal/world"
)

const guardSpeed = 2.2

type guard struct {
	x, y, z, yaw float64
	idx          int
	anim         int
	route        []world.RN
}

func newGuard(m *world.Map) guard {
	r := m.Route
	return guard{x: r[0].X, y: r[0].Y, z: r[0].Z, idx: 1, route: r}
}

func (i *Instance) stepGuard(now time.Time) {
	g := &i.guard
	t := g.route[g.idx]
	dx, dy, dz := t.X-g.x, t.Y-g.y, t.Z-g.z
	d := math.Sqrt(dx*dx + dy*dy + dz*dz)
	step := guardSpeed * world.DT
	if d <= step {
		g.x, g.y, g.z = t.X, t.Y, t.Z
		i.guardAct(t.Act, now)
		g.idx = (g.idx + 1) % len(g.route)
		return
	}
	g.x += dx / d * step
	g.y += dy / d * step
	g.z += dz / d * step
	g.yaw = math.Atan2(dx, dz)
	g.anim = animWalk
}

func (i *Instance) guardAct(act string, now time.Time) {
	var id string
	var open bool
	switch {
	case strings.HasPrefix(act, "open:"):
		id, open = act[5:], true
	case strings.HasPrefix(act, "close:"):
		id = act[6:]
	default:
		return
	}
	if open {
		if !i.isOpen(id) {
			i.open[id] = now
			i.broadcast(map[string]any{"t": "door", "id": id, "o": true})
		}
	} else if i.isOpen(id) {
		delete(i.open, id)
		i.broadcast(map[string]any{"t": "door", "id": id, "o": false})
	}
}

func contraband(p *Player) bool {
	for k, n := range p.Char.Data.Inv {
		if n > 0 && items[k].Contra {
			return true
		}
	}
	return false
}

// guardChecks lets the patrolling guard catch players who break the rules.
func (i *Instance) guardChecks(now time.Time) {
	g := i.guard
	for _, p := range i.players {
		if p.cl == nil || p.iso(now) || now.Sub(p.lastViol) < 90*time.Second {
			continue
		}
		d2 := dist2(p.body.X, p.body.Z, g.x, g.z)
		if d2 > 7*7 || math.Abs(p.body.Y-g.y) > 3 {
			continue
		}
		switch {
		case p.room == "office":
			i.punish(p, now, "xâm nhập văn phòng cai ngục", 40*time.Second, 12, 8, 2)
		case i.phase == "night" && p.room != "cell"+itoa(p.Char.Data.Cell):
			i.punish(p, now, "ra ngoài buồng giam sau giờ tắt đèn", 40*time.Second, 10, 8, 2)
		case d2 <= 3.5*3.5 && contraband(p) && i.rng.Intn(100) < 35:
			for k, n := range p.Char.Data.Inv {
				if n > 0 && items[k].Contra {
					p.Char.Data.Inv[k] = 0
				}
			}
			i.punish(p, now, "tàng trữ đồ cấm (bị tịch thu)", 30*time.Second, 12, 8, 2)
		}
	}
}

// punish fines the player, lowers discipline and reputation and sends them to isolation.
func (i *Instance) punish(p *Player, now time.Time, why string, dur time.Duration, fine, disc, rep int) {
	d := &p.Char.Data
	fine = minInt(fine, p.Char.Money)
	p.Char.Money -= fine
	d.Discipline = clampInt(d.Discipline-disc, 0, 100)
	if d.Discipline < 30 && d.Sentence < 99 {
		d.Sentence++
	}
	p.Char.Rep = clampInt(p.Char.Rep-rep, 0, 1<<30)
	p.lastViol = now
	p.isoUntil = now.Add(dur)
	p.act = nil
	p.emote = 0
	room := 0
	if i.occupancy("iso1", p) > i.occupancy("iso2", p) {
		room = 1
	}
	pos := i.cfg.Map.Iso[room]
	i.teleport(p, pos[0], pos[1], pos[2])
	i.toast(p, "bad", "Cai ngục bắt bạn vì "+why+": phạt "+itoa(fine)+" tiền trại, biệt giam "+itoa(int(dur.Seconds()))+" giây.")
	i.broadcastChat("sys", "", p.name()+" bị cai ngục bắt vì "+why+".", "")
	p.dirty = true
	if p.cl != nil {
		p.cl.Send(map[string]any{"t": "act", "k": ""})
		p.cl.Send(i.meMsg(p, now))
	}
}
