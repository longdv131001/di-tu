package game

import (
	"context"
	"encoding/json"
	"math/rand"
	"strings"
	"testing"
	"time"

	"dituonline/internal/store"
	"dituonline/internal/world"
)

type fx struct {
	t     *testing.T
	inst  *Instance
	st    *store.Memory
	now   time.Time
	saved map[int64]*store.Character
	gone  []int64
}

func newFx(t *testing.T) *fx {
	f := &fx{t: t, st: store.NewMemory(), now: time.Unix(1_700_000_000, 0), saved: map[int64]*store.Character{}}
	f.inst = NewInstance(Config{ID: "t1", Max: 24, DaySeconds: 600, Map: world.Build(), Store: f.st,
		Clock:    func() time.Time { return f.now },
		Save:     func(c *store.Character) { f.saved[c.ID] = c },
		OnRemove: func(id int64, _ string) { f.gone = append(f.gone, id) },
		RouteDM:  nil})
	f.inst.rng = rand.New(rand.NewSource(7))
	f.inst.RouteDMHook(func(from, to, text string) bool {
		if p := f.inst.byName[lower(to)]; p != nil {
			f.inst.deliverDM(dmEv{from, to, text})
			return true
		}
		return false
	})
	return f
}

func lower(s string) string { return strings.ToLower(s) }

type pl struct {
	*Player
	cl *Client
}

func (f *fx) add(name string) *Player {
	ctx := context.Background()
	a, _ := f.st.CreateAccount(ctx, name, "h")
	c := &store.Character{AccountID: a.ID, Name: name, Money: 50, Data: store.CharData{Inv: map[string]int{}}}
	f.st.CreateCharacter(ctx, c)
	return f.join(c, a)
}

func (f *fx) join(c *store.Character, a store.Account) *Player {
	cl := NewClient(512, func(string) {})
	f.inst.dispatch(joinEv{c, a, cl})
	return f.inst.players[c.ID]
}

func (f *fx) ticks(n int) {
	for k := 0; k < n; k++ {
		f.now = f.now.Add(50 * time.Millisecond)
		f.inst.tick()
	}
}

func (f *fx) send(p *Player, m Msg) { f.inst.handle(p, m) }

func (f *fx) drain(p *Player) []map[string]any {
	var out []map[string]any
	for {
		select {
		case b := <-p.cl.Out:
			var m map[string]any
			json.Unmarshal(b, &m)
			out = append(out, m)
		default:
			return out
		}
	}
}

func last(msgs []map[string]any, t string) map[string]any {
	var r map[string]any
	for _, m := range msgs {
		if m["t"] == t {
			r = m
		}
	}
	return r
}

func has(msgs []map[string]any, t string) map[string]any {
	for _, m := range msgs {
		if m["t"] == t {
			return m
		}
	}
	return nil
}

func (f *fx) place(p *Player, x, y, z float64) {
	f.inst.teleport(p, x, y, z)
	f.drain(p)
}

func TestJoinWelcomeAndMovement(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	msgs := f.drain(p)
	w := has(msgs, "welcome")
	if w == nil || w["id"] != float64(p.id()) || w["me"] == nil {
		t.Fatalf("welcome: %v", w)
	}
	z0 := p.body.Z
	for s := uint32(1); s <= 10; s++ {
		f.send(p, Msg{T: "in", S: s, MZ: -1, Yaw: 0})
		f.ticks(1)
	}
	if p.body.Z >= z0-1 || p.ack != 10 {
		t.Fatalf("did not move: z %.2f -> %.2f ack=%d", z0, p.body.Z, p.ack)
	}
	f.ticks(2)
	snap := last(f.drain(p), "snap")
	if snap == nil || snap["a"] != float64(10) {
		t.Fatalf("snap/ack: %v", snap)
	}
}

func TestOthersVisibleAndGone(t *testing.T) {
	f := newFx(t)
	a, b := f.add("Anna"), f.add("Binh")
	f.place(a, 40, 0, 40)
	f.place(b, 42, 0, 40)
	f.ticks(2)
	s := has(f.drain(a), "snap")
	ents := s["p"].([]any)
	if len(ents) < 2 { // Binh + guard maybe
		t.Fatalf("expected other players in snapshot, got %v", ents)
	}
	if s["n"] == nil {
		t.Fatal("missing metadata for new entities")
	}
	f.place(b, 2, 0, 2) // still within AOI 45? 38,38 -> 54m: out of range
	f.ticks(2)
	s = has(f.drain(a), "snap")
	if s["x"] == nil {
		t.Fatalf("expected removal of out-of-range player: %v", s)
	}
}

func TestRoomCapacityAndRestricted(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	f.place(p, 63, 0, 44) // office door is locked, but walk straight into the room through the wall is impossible;
	if p.room == "office" {
		// teleported inside on purpose; step out and try to come back through a locked door
		f.place(p, 56, 0, 44)
	}
	f.place(p, 56, 0, 44)
	for s := uint32(1); s < 80; s++ {
		f.send(p, Msg{T: "in", S: s, MX: -1, Yaw: 0}) // +x, towards the door at x=60
		f.ticks(1)
	}
	if p.room == "office" || p.body.X > 59.6 {
		t.Fatalf("entered locked/restricted office, x=%.2f room=%s", p.body.X, p.room)
	}
	// room cap: fill cell1 (cap 6) with other players, 7th is refused
	m := f.inst.cfg.Map
	cell, _ := m.Room("cell1")
	var ps []*Player
	for k := 0; k < 7; k++ {
		q := f.add("P" + itoa(k))
		f.place(q, cell.X0+3, 4, 10)
		ps = append(ps, q)
	}
	f.place(ps[6], 11.5, 4, 20)
	ps[6].room = "corridor"
	f.inst.open["d_c1"] = f.now
	for s := uint32(1); s < 30; s++ {
		f.send(ps[6], Msg{T: "in", S: s, MZ: -1, Yaw: 0})
		f.ticks(1)
	}
	if n := f.inst.occupancy("cell1", nil); n > 6 {
		t.Fatalf("cell over capacity: %d", n)
	}
}

func TestWorkRewardsAndQuests(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	b, _ := f.inst.cfg.Map.Obj("bench1")
	f.place(p, b.X, 0, b.Z)
	money := p.Char.Money
	f.send(p, Msg{T: "use", ID: "bench1"})
	if p.act == nil || p.act.kind != "work" {
		t.Fatalf("work did not start: %v", f.drain(p))
	}
	f.ticks(130)
	if p.Char.Money <= money || p.act != nil || p.Char.Data.Stamina >= 100 {
		t.Fatalf("no reward: money %d -> %d, stamina %d", money, p.Char.Money, p.Char.Data.Stamina)
	}
	if len(p.Char.Data.Quests.Daily) != 3 {
		t.Fatal("daily quests missing")
	}
	// moving cancels the action
	f.send(p, Msg{T: "use", ID: "bench1"})
	for s := uint32(1); s < 12; s++ {
		f.send(p, Msg{T: "in", S: s, MX: 1, Yaw: 0, R: true})
		f.ticks(1)
	}
	if p.act != nil {
		t.Fatal("moving should cancel the action")
	}
	// too far away: refused
	f.place(p, 60, 0, 58)
	f.send(p, Msg{T: "use", ID: "bench1"})
	if p.act != nil {
		t.Fatal("use from far away accepted")
	}
}

func TestQuestCompletionAndWeekly(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	id := p.Char.Data.Quests.Daily[0].ID
	def, _ := qdefByID(id)
	m0 := p.Char.Money
	for k := 0; k < def.Target+2; k++ {
		f.inst.questProgress(p, id)
	}
	if p.Char.Money != m0+def.Money || !p.Char.Data.Quests.Daily[0].Done || p.Char.Data.Quests.WeekProg != 1 {
		t.Fatalf("quest reward wrong: money %d->%d week=%d", m0, p.Char.Money, p.Char.Data.Quests.WeekProg)
	}
	// next UTC day resets dailies
	f.now = f.now.Add(25 * time.Hour)
	f.inst.ensureQuests(p, f.now)
	if p.Char.Data.Quests.Daily[0].Done {
		t.Fatal("daily quests did not reset")
	}
}

func TestShopCraftEatTrade(t *testing.T) {
	f := newFx(t)
	a, b := f.add("Anna"), f.add("Binh")
	c, _ := f.inst.cfg.Map.Obj("counter")
	f.place(a, c.X, 0, c.Z)
	a.Char.Money = 30
	f.send(a, Msg{T: "buy", Item: "noodle"})
	f.send(a, Msg{T: "buy", Item: "noodle"})
	if a.Char.Data.Inv["noodle"] != 2 || a.Char.Money != 22 {
		t.Fatalf("buy: inv=%v money=%d", a.Char.Data.Inv, a.Char.Money)
	}
	a.Char.Data.Stamina = 40
	f.send(a, Msg{T: "eat", Item: "noodle"})
	if a.Char.Data.Stamina != 58 {
		t.Fatalf("eat: %d", a.Char.Data.Stamina)
	}
	f.place(a, 100-92, 0, 10) // away from bench
	f.send(a, Msg{T: "buy", Item: "noodle"})
	if a.Char.Data.Inv["noodle"] != 1 {
		t.Fatal("bought away from the counter")
	}
	// craft needs the workshop
	a.Char.Data.Inv["wire"], a.Char.Data.Inv["handle"] = 2, 1
	f.send(a, Msg{T: "craft", ID: "pliers"})
	if a.Char.Data.Inv["pliers"] != 0 {
		t.Fatal("crafted away from a bench")
	}
	bench, _ := f.inst.cfg.Map.Obj("bench2")
	f.place(a, bench.X, 0, bench.Z)
	f.send(a, Msg{T: "craft", ID: "pliers"})
	if a.Char.Data.Inv["pliers"] != 1 || a.Char.Data.Inv["wire"] != 0 {
		t.Fatalf("craft: %v", a.Char.Data.Inv)
	}
	// trade pliers for coins
	f.place(b, bench.X+1, 0, bench.Z)
	b.Char.Money = 100
	aMoney := a.Char.Money
	f.drain(b)
	f.send(a, Msg{T: "offer", To: "Binh", Item: "pliers", Qty: 1, Price: 60})
	o := has(f.drain(b), "offer")
	if o == nil {
		t.Fatal("buyer got no offer")
	}
	f.send(b, Msg{T: "offer_reply", N: int(o["id"].(float64)), Accept: true})
	if b.Char.Data.Inv["pliers"] != 1 || a.Char.Data.Inv["pliers"] != 0 || b.Char.Money != 40 || a.Char.Money != aMoney+60 {
		t.Fatalf("trade failed: a=%v/%d b=%v/%d", a.Char.Data.Inv, a.Char.Money, b.Char.Data.Inv, b.Char.Money)
	}
	// cannot sell what you do not have
	f.send(a, Msg{T: "offer", To: "Binh", Item: "pliers", Qty: 1, Price: 1})
	if len(f.inst.trades) != 0 {
		t.Fatal("offer without stock accepted")
	}
}

func TestChatChannelsAndLimits(t *testing.T) {
	f := newFx(t)
	a, b, c := f.add("Anna"), f.add("Binh"), f.add("Chi")
	f.place(a, 20, 0, 10)
	f.place(b, 21, 0, 10)
	f.place(c, 60, 0, 58)
	f.send(a, Msg{T: "chat", Ch: "room", Text: "xin chào buồng"})
	if has(f.drain(b), "chat") == nil || has(f.drain(c), "chat") != nil {
		t.Fatal("room chat leaked or missing")
	}
	f.send(a, Msg{T: "chat", Ch: "global", Text: "cả trại nghe này"})
	if has(f.drain(c), "chat") == nil {
		t.Fatal("global chat missing")
	}
	f.send(a, Msg{T: "chat", Ch: "dm", To: "Chi", Text: "thì thầm"})
	d := has(f.drain(c), "chat")
	if d == nil || d["ch"] != "dm" || d["from"] != "Anna" {
		t.Fatalf("dm: %v", d)
	}
	f.send(a, Msg{T: "chat", Ch: "dm", To: "Nobody", Text: "ai đó?"})
	if has(f.drain(a), "toast") == nil {
		t.Fatal("expected failure toast for unknown dm target")
	}
	f.drain(a)
	for k := 0; k < 6; k++ { // flood
		f.send(a, Msg{T: "chat", Ch: "global", Text: "spam"})
	}
	got := 0
	for _, m := range f.drain(b) {
		if m["t"] == "chat" {
			got++
		}
	}
	if got > 3 {
		t.Fatalf("rate limit let %d messages through", got)
	}
	f.inst.dispatch(muteEv{"Anna", f.now.Add(time.Hour)})
	f.now = f.now.Add(10 * time.Second)
	f.drain(b)
	f.send(a, Msg{T: "chat", Ch: "global", Text: "bị cấm"})
	if has(f.drain(b), "chat") != nil {
		t.Fatal("muted player could chat")
	}
	if filterBad("đm mày") == "đm mày" {
		t.Fatal("profanity not filtered")
	}
}

func TestReconnectKeepsCharacter(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	f.place(p, 33, 0, 40)
	p.Char.Money = 777
	oldCl := p.cl
	f.inst.dispatch(leaveEv{oldCl})
	f.ticks(20)
	if f.inst.players[p.id()] == nil {
		t.Fatal("player removed immediately instead of lingering")
	}
	a, _ := f.st.AccountByID(context.Background(), p.Char.AccountID)
	p2 := f.join(p.Char, a)
	if p2 != p || p2.body.X != 33 || p2.Char.Money != 777 || has(f.drain(p2), "welcome") == nil {
		t.Fatal("reconnect did not restore the same character")
	}
	f.inst.dispatch(leaveEv{p2.cl})
	f.now = f.now.Add(70 * time.Second)
	f.ticks(20)
	if f.inst.players[p.id()] != nil || len(f.gone) != 1 {
		t.Fatalf("lingering player not removed: gone=%v", f.gone)
	}
	if s := f.saved[p.id()]; s == nil || s.Money != 777 || int(s.Data.X) != 33 {
		t.Fatalf("character not persisted on removal: %+v", s)
	}
	// joining again later restores position from the saved data
	c2 := f.saved[p.id()]
	p3 := f.join(c2, a)
	if int(p3.body.X) != 33 {
		t.Fatalf("restored at %.1f", p3.body.X)
	}
	if f.inst.Count() != 1 {
		t.Fatalf("count=%d", f.inst.Count())
	}
}

func TestSecondLoginReplacesFirst(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	first := p.cl
	a, _ := f.st.AccountByID(context.Background(), p.Char.AccountID)
	f.join(p.Char, a)
	if p.cl == first || !first.closed.Load() {
		t.Fatal("old connection not replaced")
	}
}

func TestGuardCatchesNightViolation(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	f.inst.phase = "night"
	f.inst.guard.x, f.inst.guard.y, f.inst.guard.z = 40, 0, 40
	f.place(p, 42, 0, 40)
	f.inst.guardChecks(f.now)
	if !p.iso(f.now) {
		t.Fatal("night violation went unpunished")
	}
	if p.room != "iso1" && p.room != "iso2" {
		t.Fatalf("not in isolation room: %s", p.room)
	}
	// isolated players cannot move
	x := p.body.X
	f.send(p, Msg{T: "in", S: 1, MX: 1})
	f.ticks(5)
	if p.body.X != x {
		t.Fatal("isolated player moved")
	}
	f.now = f.now.Add(41 * time.Second)
	f.ticks(2)
	if p.iso(f.now) || p.room != "yard" {
		t.Fatalf("not released: room=%s", p.room)
	}
	// cooldown prevents an immediate second catch
	f.place(p, 42, 0, 40)
	f.inst.guardChecks(f.now)
	if p.iso(f.now) {
		t.Fatal("punished twice within cooldown")
	}
}

func TestGuardContrabandAndPatrol(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	f.inst.phase = "free"
	p.Char.Data.Inv["cigarette"] = 3
	f.inst.guard.x, f.inst.guard.z = 40, 40
	f.place(p, 41, 0, 40)
	for k := 0; k < 200 && !p.iso(f.now); k++ {
		p.lastViol = time.Time{}
		f.inst.guardChecks(f.now)
	}
	if !p.iso(f.now) || p.Char.Data.Inv["cigarette"] != 0 {
		t.Fatal("contraband should be confiscated")
	}
	// the patrol visits every waypoint and opens/closes doors without getting stuck
	g := newFx(t)
	g.ticks(1)
	seen := map[int]bool{}
	for k := 0; k < 20*400 && len(seen) < len(g.inst.guard.route); k++ {
		g.ticks(1)
		seen[g.inst.guard.idx] = true
	}
	if len(seen) < len(g.inst.guard.route) {
		t.Fatalf("guard visited only %d/%d nodes", len(seen), len(g.inst.guard.route))
	}
}

func TestDoorsToggleAndAutoClose(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	f.place(p, 16, 0, 26)
	f.send(p, Msg{T: "door", ID: "d_b1_main"})
	if !f.inst.isOpen("d_b1_main") {
		t.Fatal("door did not open")
	}
	f.place(p, 16, 0, 40)
	f.inst.guard.x, f.inst.guard.z = 60, 60
	f.now = f.now.Add(25 * time.Second)
	f.inst.second(f.now)
	if f.inst.isOpen("d_b1_main") {
		t.Fatal("abandoned door stayed open")
	}
	f.send(p, Msg{T: "door", ID: "d_office"}) // far and locked
	if f.inst.isOpen("d_office") {
		t.Fatal("locked door opened")
	}
}

func TestGroupEscape(t *testing.T) {
	f := newFx(t)
	fence, _ := f.inst.cfg.Map.Obj("fence")
	var ps []*Player
	for k := 0; k < 3; k++ {
		p := f.add("E" + itoa(k))
		p.Char.Data.Inv["pliers"] = 1
		f.place(p, fence.X-float64(k)*0.4, 0, fence.Z+float64(k)-1)
		ps = append(ps, p)
	}
	f.inst.guard.x, f.inst.guard.z = 5, 5
	f.send(ps[0], Msg{T: "use", ID: "fence"})
	if f.inst.esc == nil {
		t.Fatalf("escape did not start: %v", f.drain(ps[0]))
	}
	// breaking up the group aborts it without consuming tools
	f.send(ps[1], Msg{T: "cancel"})
	f.ticks(2)
	if f.inst.esc != nil || ps[0].Char.Data.Inv["pliers"] != 1 {
		t.Fatal("abort should keep the pliers")
	}
	// two people are not enough
	f.send(ps[0], Msg{T: "cancel"})
	f.place(ps[2], 10, 0, 10)
	f.send(ps[0], Msg{T: "use", ID: "fence"})
	if f.inst.esc != nil {
		t.Fatal("started with only two players")
	}
	f.place(ps[2], fence.X-0.8, 0, fence.Z+1)
	f.send(ps[0], Msg{T: "use", ID: "fence"})
	if f.inst.esc == nil {
		t.Fatal("full group should start")
	}
	f.ticks(170)
	if f.inst.esc != nil {
		t.Fatal("escape never resolved")
	}
	free, jailed := 0, 0
	for _, p := range ps {
		if p.Char.Data.Inv["pliers"] != 0 {
			t.Fatal("pliers must be consumed")
		}
		if p.Char.Escapes > 0 {
			free++
		}
		if p.iso(f.now) {
			jailed++
		}
	}
	if free+jailed != 3 {
		t.Fatalf("every member needs an outcome: free=%d jailed=%d", free, jailed)
	}
}

func TestClockPhasesRollCallAndRelease(t *testing.T) {
	f := newFx(t)
	if f.inst.hour < 7.9 || f.inst.hour > 8.1 {
		t.Fatalf("start hour %.2f", f.inst.hour)
	}
	p := f.add("Hung")
	p.Char.Data.Sentence = 1
	secs := func(h float64) { f.now = f.now.Add(time.Duration(h / 24 * 600 * float64(time.Second))) }
	secs(22) // 08:00 -> 06:00 next day
	f.place(p, 20, 0, 10)
	f.inst.second(f.now)
	if f.inst.phase != "roll" {
		t.Fatalf("phase=%s hour=%.2f", f.inst.phase, f.inst.hour)
	}
	if p.Char.Releases != 1 || p.Char.Data.Sentence < 20 {
		t.Fatalf("release not applied at new day: releases=%d sentence=%d", p.Char.Releases, p.Char.Data.Sentence)
	}
	disc := p.Char.Data.Discipline
	secs(1.1) // past 07:00, player stayed in the hall
	f.inst.second(f.now)
	if p.Char.Data.Discipline != disc+3 {
		t.Fatalf("roll call reward missing: %d -> %d", disc, p.Char.Data.Discipline)
	}
}

func TestOutfitValidationAndSaveOnStop(t *testing.T) {
	f := newFx(t)
	p := f.add("Hung")
	f.send(p, Msg{T: "outfit", Outfit: &store.Outfit{Shirt: 99}})
	if p.Char.Outfit.Shirt == 99 {
		t.Fatal("invalid outfit accepted")
	}
	f.send(p, Msg{T: "outfit", Outfit: &store.Outfit{Shirt: 5, Pants: 2, Skin: 1, Hair: 2, HairC: 3, Hat: 1}})
	if p.Char.Outfit.Shirt != 5 || p.metaVer != 1 {
		t.Fatal("valid outfit rejected")
	}
	f.inst.saveAll()
	if f.saved[p.id()].Outfit.Shirt != 5 {
		t.Fatal("outfit not persisted")
	}
}
