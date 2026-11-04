package game

import (
	"strings"
	"time"
)

func (i *Instance) chatMsg(ch, from, text, to string) map[string]any {
	m := map[string]any{"t": "chat", "ch": ch, "from": from, "text": text, "ts": i.now().UnixMilli()}
	if to != "" {
		m["to"] = to
	}
	return m
}

func (i *Instance) broadcastChat(ch, from, text, to string) {
	i.broadcast(i.chatMsg(ch, from, text, to))
}

func (i *Instance) chat(p *Player, m Msg, now time.Time) {
	text := filterBad(cleanText(m.Text, 200))
	if text == "" {
		return
	}
	if now.Before(p.mutedUntil) {
		i.toast(p, "bad", "Bạn đang bị cấm chat.")
		return
	}
	p.chatTokens = minFloat(3, p.chatTokens+now.Sub(p.chatAt).Seconds()/0.8)
	p.chatAt = now
	if p.chatTokens < 1 {
		i.toast(p, "warn", "Bạn chat quá nhanh.")
		return
	}
	p.chatTokens--
	switch m.Ch {
	case "global":
		i.broadcastChat("global", p.name(), text, "")
	case "room":
		msg := i.chatMsg("room", p.name(), text, "")
		msg["room"] = p.room
		for _, o := range i.players {
			if o.cl != nil && o.room == p.room {
				o.cl.Send(msg)
			}
		}
	case "dm":
		to := cleanText(m.To, 16)
		if strings.EqualFold(to, p.name()) || to == "" {
			return
		}
		if i.cfg.RouteDM == nil || !i.cfg.RouteDM(p.name(), to, text) {
			i.toast(p, "warn", "Không thấy người chơi \""+to+"\" đang online.")
			return
		}
		p.cl.Send(i.chatMsg("dm", p.name(), text, to))
	}
}

func (i *Instance) deliverDM(e dmEv) {
	if t := i.byName[strings.ToLower(e.to)]; t != nil && t.cl != nil {
		t.cl.Send(i.chatMsg("dm", e.from, e.text, t.name()))
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
