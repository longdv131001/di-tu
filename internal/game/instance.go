package game

import (
	"math"
	"math/rand"
	"strings"
	"sync/atomic"
	"time"

	"dituonline/internal/store"
	"dituonline/internal/world"
)

type Config struct {
	ID         string
	Max        int
	DaySeconds int
	Linger     time.Duration
	Map        *world.Map
	Store      store.Store
	Save       func(*store.Character)          // asynchronous persistence
	OnJoin     func(charID int64, name string) // a new player entered the instance
	OnRemove   func(charID int64, name string) // player left the instance for good
	RouteDM    func(from, to, text string) bool
	Clock      func() time.Time
	AOI        float64
}

type (
	joinEv struct {
		char *store.Character
		acct store.Account
		cl   *Client
	}
	leaveEv struct{ cl *Client }
	msgEv   struct {
		cl *Client
		m  Msg
	}
	dmEv   struct{ from, to, text string }
	kickEv struct {
		name, reason string
	}
	muteEv struct {
		name  string
		until time.Time
	}
	stopEv struct{ done chan struct{} }
)

type trade struct {
	id       int
	from, to int64
	item     string
	qty      int
	price    int
	expires  time.Time
}

type escape struct {
	members []int64
	end     time.Time
}

type Instance struct {
	cfg      Config
	ev       chan any
	players  map[int64]*Player
	byName   map[string]*Player
	byClient map[*Client]*Player
	open     map[string]time.Time
	guard    guard
	started  time.Time
	tickN    uint32
	dayIdx   int
	phase    string
	hour     float64
	trades   map[int]*trade
	tradeSeq int
	esc      *escape
	rng      *rand.Rand
	n        atomic.Int32
}

func NewInstance(cfg Config) *Instance {
	if cfg.Linger == 0 {
		cfg.Linger = 60 * time.Second
	}
	if cfg.Clock == nil {
		cfg.Clock = time.Now
	}
	if cfg.AOI == 0 {
		cfg.AOI = 45
	}
	if cfg.DaySeconds < 10 {
		cfg.DaySeconds = 600
	}
	i := &Instance{cfg: cfg, ev: make(chan any, 4096), players: map[int64]*Player{}, byName: map[string]*Player{},
		byClient: map[*Client]*Player{}, open: map[string]time.Time{}, trades: map[int]*trade{},
		rng: rand.New(rand.NewSource(time.Now().UnixNano()))}
	i.started = cfg.Clock()
	i.guard = newGuard(cfg.Map)
	i.hour, i.dayIdx = i.clockAt(i.started)
	i.phase, _ = phaseOf(i.hour)
	return i
}

func (i *Instance) ID() string     { return i.cfg.ID }
func (i *Instance) Count() int     { return int(i.n.Load()) }
func (i *Instance) now() time.Time { return i.cfg.Clock() }

// ---- public, goroutine-safe API (all enqueue events) ----

func (i *Instance) Join(c *store.Character, a store.Account, cl *Client) { i.ev <- joinEv{c, a, cl} }
func (i *Instance) Leave(cl *Client)                                     { i.ev <- leaveEv{cl} }
func (i *Instance) DM(from, to, text string)                             { i.ev <- dmEv{from, to, text} }
func (i *Instance) Kick(name, reason string)                             { i.ev <- kickEv{name, reason} }
func (i *Instance) Mute(name string, until time.Time)                    { i.ev <- muteEv{name, until} }
func (i *Instance) Handle(cl *Client, m Msg) {
	select {
	case i.ev <- msgEv{cl, m}:
	default: // overloaded: drop input rather than block the reader
	}
}

// Run drives the simulation until Stop is called.
func (i *Instance) Run() {
	t := time.NewTicker(time.Duration(world.DT * float64(time.Second)))
	defer t.Stop()
	for {
		select {
		case e := <-i.ev:
			if s, ok := e.(stopEv); ok {
				i.saveAll()
				close(s.done)
				return
			}
			i.dispatch(e)
		case <-t.C:
			i.tick()
		}
	}
}

func (i *Instance) Stop() {
	d := make(chan struct{})
	i.ev <- stopEv{d}
	<-d
}

func (i *Instance) dispatch(e any) {
	switch e := e.(type) {
	case joinEv:
		i.join(e)
	case leaveEv:
		if p := i.byClient[e.cl]; p != nil {
			delete(i.byClient, e.cl)
			if p.cl == e.cl {
				p.cl = nil
				p.discAt = i.now()
				p.queue = nil
			}
		}
	case msgEv:
		if p := i.byClient[e.cl]; p != nil {
			i.handle(p, e.m)
		}
	case dmEv:
		i.deliverDM(e)
	case kickEv:
		if p := i.byName[strings.ToLower(e.name)]; p != nil {
			if p.cl != nil {
				p.cl.Send(map[string]any{"t": "err", "code": "kicked", "msg": e.reason})
				p.cl.Close("kicked")
			}
			i.remove(p)
		}
	case muteEv:
		if p := i.byName[strings.ToLower(e.name)]; p != nil {
			p.mutedUntil = e.until
		}
	}
}

// ---- clock ----

func (i *Instance) clockAt(now time.Time) (hour float64, dayIdx int) {
	total := 8 + now.Sub(i.started).Seconds()*24/float64(i.cfg.DaySeconds)
	return math.Mod(total, 24), int(math.Floor((total - 6) / 24))
}

func (i *Instance) clockMsg() map[string]any {
	ph, label := phaseOf(i.hour)
	return map[string]any{"t": "clock", "day": i.dayIdx + 1, "h": r2(i.hour), "ph": ph, "phl": label, "len": i.cfg.DaySeconds}
}

func (i *Instance) isOpen(id string) bool { _, ok := i.open[id]; return ok }

// ---- join / leave ----

func (i *Instance) join(e joinEv) {
	c := e.char
	now := i.now()
	if p, ok := i.players[c.ID]; ok { // reconnect: same character, same place
		if p.cl != nil && p.cl != e.cl {
			p.cl.Send(map[string]any{"t": "err", "code": "other_login", "msg": "Tài khoản đăng nhập ở nơi khác"})
			delete(i.byClient, p.cl)
			p.cl.Close("replaced")
		}
		p.cl, p.discAt, p.known, p.queue, p.ack = e.cl, time.Time{}, map[int64]int{}, nil, 0
		p.Acct = e.acct
		p.mutedUntil = e.acct.MutedUntil
		i.byClient[e.cl] = p
		i.sendWelcome(p)
		return
	}
	if len(i.players) >= i.cfg.Max {
		e.cl.Send(map[string]any{"t": "err", "code": "full", "msg": "Trại đã đầy, hãy thử lại."})
		e.cl.Close("full")
		return
	}
	p := &Player{Char: c, Acct: e.acct, cl: e.cl, known: map[int64]int{}, mutedUntil: e.acct.MutedUntil, chatTokens: 3, lastSave: now, chatAt: now}
	d := &c.Data
	if d.Inv == nil {
		d.Inv = map[string]int{}
	}
	if d.Stamina == 0 {
		d.Stamina = 100
	}
	if d.Discipline == 0 {
		d.Discipline = 70
	}
	if d.Sentence == 0 {
		d.Sentence = 20 + i.rng.Intn(21)
	}
	if d.Cell == 0 {
		d.Cell = i.pickCell()
	}
	sp := i.cfg.Map.Spawn
	p.body = world.Body{X: d.X, Y: d.Y, Z: d.Z, Grounded: true}
	if (d.X == 0 && d.Z == 0) || d.Y < 0 || d.Y > 8 || d.X < 1 || d.Z < 1 || d.X > i.cfg.Map.W-1 || d.Z > i.cfg.Map.D-1 ||
		strings.HasPrefix(i.cfg.Map.RoomAt(d.X, d.Y, d.Z), "iso") || i.cfg.Map.RoomAt(d.X, d.Y, d.Z) == "office" {
		p.body = world.Body{X: sp[0], Y: sp[1], Z: sp[2], Grounded: true}
	}
	p.yaw = d.Yaw
	if d.X == 0 && d.Z == 0 {
		p.yaw = math.Pi // new characters start facing north, into the compound
	}
	p.room = i.cfg.Map.RoomAt(p.body.X, p.body.Y, p.body.Z)
	i.ensureQuests(p, now)
	i.players[c.ID] = p
	i.byName[strings.ToLower(c.Name)] = p
	i.byClient[e.cl] = p
	i.n.Store(int32(len(i.players)))
	if i.cfg.OnJoin != nil {
		i.cfg.OnJoin(c.ID, c.Name)
	}
	i.sendWelcome(p)
	i.broadcastChat("sys", "", c.Name+" vừa nhập trại.", "")
}

func (i *Instance) pickCell() int {
	count := map[int]int{}
	for _, p := range i.players {
		count[p.Char.Data.Cell]++
	}
	best, bc := 1, 1<<30
	for k := 1; k <= 4; k++ {
		if count[k] < bc {
			best, bc = k, count[k]
		}
	}
	return best
}

func (i *Instance) remove(p *Player) {
	i.saveChar(p)
	delete(i.players, p.id())
	delete(i.byName, strings.ToLower(p.name()))
	if p.cl != nil {
		delete(i.byClient, p.cl)
	}
	i.n.Store(int32(len(i.players)))
	if i.cfg.OnRemove != nil {
		i.cfg.OnRemove(p.id(), p.name())
	}
}

func (i *Instance) saveChar(p *Player) {
	d := &p.Char.Data
	d.X, d.Y, d.Z, d.Yaw = p.body.X, p.body.Y, p.body.Z, p.yaw
	cp := *p.Char
	cp.Data.Inv = map[string]int{}
	for k, v := range p.Char.Data.Inv {
		cp.Data.Inv[k] = v
	}
	cp.Data.Quests.Daily = append([]store.QState(nil), p.Char.Data.Quests.Daily...)
	if i.cfg.Save != nil {
		i.cfg.Save(&cp)
	}
	p.dirty = false
	p.lastSave = i.now()
}

func (i *Instance) saveAll() {
	for _, p := range i.players {
		i.saveChar(p)
	}
}

// ---- tick ----

func (i *Instance) tick() {
	now := i.now()
	i.tickN++
	for _, p := range i.players {
		i.stepPlayer(p, now)
	}
	i.stepGuard(now)
	i.stepEscape(now)
	if i.tickN%2 == 0 {
		i.broadcastSnap()
	}
	if i.tickN%20 == 0 {
		i.second(now)
	}
}

func (i *Instance) stepPlayer(p *Player, now time.Time) {
	m := i.cfg.Map
	if p.isoUntil.After(time.Time{}) {
		if p.iso(now) {
			p.anim = animIdle
			return
		}
		p.isoUntil = time.Time{}
		i.teleport(p, m.Spawn[0], m.Spawn[1], m.Spawn[2])
		i.toast(p, "ok", "Bạn đã được thả khỏi biệt giam. Hãy giữ kỷ luật nhé!")
	}
	var in world.Input
	if len(p.queue) > 0 {
		q := p.queue[0]
		p.queue = p.queue[1:]
		in, p.ack = q.in, q.seq
	}
	old := p.body
	facing, moving := m.StepBody(&p.body, in, i.isOpen)
	jumped := in.Jump && old.Grounded && !p.body.Grounded
	if room := m.RoomAt(p.body.X, p.body.Y, p.body.Z); room != p.room {
		if def, ok := m.Room(room); ok && i.occupancy(room, p) >= def.Cap {
			p.body = old
			p.body.VY = 0
			if now.Sub(p.denyAt) > 2*time.Second {
				p.denyAt = now
				msg := def.Name + " đã đầy."
				if def.Cap == 0 {
					msg = "Khu vực cấm vào."
				}
				i.toast(p, "warn", msg)
			}
		} else {
			p.room = room
		}
	}
	if p.body.Y < -5 {
		i.teleport(p, m.Spawn[0], m.Spawn[1], m.Spawn[2])
	}
	if moving && !math.IsNaN(facing) {
		p.yaw = facing
	}
	if p.act != nil {
		if jumped || dist2(p.body.X, p.body.Z, p.act.sx, p.act.sz) > 0.9*0.9 {
			i.cancelAct(p, "Đã hủy vì bạn di chuyển.")
		} else if !now.Before(p.act.end) && !p.act.group {
			i.finishAct(p, now)
		}
	}
	if (moving || jumped) && p.emote != 0 {
		p.emote = 0
	}
	switch {
	case p.act != nil:
		p.anim = acts[p.act.kind].Anim
	case p.emote != 0 && now.Before(p.emoteEnd):
		p.anim = emoteAnim[p.emote]
	case !p.body.Grounded:
		p.anim = animJump
	case moving && in.Run:
		p.anim = animRun
	case moving:
		p.anim = animWalk
	default:
		p.anim = animIdle
	}
}

var emoteAnim = map[int]int{1: animWave, 2: animDance, 3: animLaugh, 4: animSit}

func (i *Instance) occupancy(room string, except *Player) int {
	n := 0
	for _, o := range i.players {
		if o != except && o.room == room {
			n++
		}
	}
	return n
}

func (i *Instance) teleport(p *Player, x, y, z float64) {
	p.body = world.Body{X: x, Y: y, Z: z, Grounded: true}
	p.room = i.cfg.Map.RoomAt(x, y, z)
	p.queue = nil
	if p.cl != nil {
		p.cl.Send(map[string]any{"t": "tp", "pos": []float64{r2(x), r2(y), r2(z)}})
	}
}

// second runs once per second of real time.
func (i *Instance) second(now time.Time) {
	prev := i.phase
	prevDay := i.dayIdx
	i.hour, i.dayIdx = i.clockAt(now)
	i.phase, _ = phaseOf(i.hour)
	if i.phase != prev {
		i.phaseChanged(prev, i.phase)
	}
	if i.dayIdx != prevDay {
		i.newDay()
	}
	for _, p := range i.players {
		d := &p.Char.Data
		if p.act == nil && d.Stamina < 100 && p.cl != nil {
			d.Stamina = clampInt(d.Stamina+1, 0, 100)
			p.dirty = true
		}
		i.ensureQuests(p, now)
		if p.cl == nil && now.Sub(p.discAt) > i.cfg.Linger {
			i.remove(p)
			continue
		}
		if p.dirty && now.Sub(p.lastSave) > 30*time.Second {
			i.saveChar(p)
		}
		if p.cl != nil {
			p.cl.Send(i.meMsg(p, now))
		}
	}
	for id, at := range i.open { // doors close themselves when nobody is near
		if now.Sub(at) > 20*time.Second && !i.someoneNear(id) {
			delete(i.open, id)
			i.broadcast(map[string]any{"t": "door", "id": id, "o": false})
		}
	}
	for id, t := range i.trades {
		if now.After(t.expires) {
			delete(i.trades, id)
			if p := i.players[t.from]; p != nil {
				p.offerOut = 0
			}
		}
	}
	i.guardChecks(now)
	if i.tickN%40 == 0 && i.cfg.Clock != nil {
		i.broadcast(i.clockMsg())
	}
}

func (i *Instance) someoneNear(doorID string) bool {
	d, ok := i.cfg.Map.Door(doorID)
	if !ok {
		return false
	}
	for _, p := range i.players {
		if dist2(p.body.X, p.body.Z, d.X, d.Z) < 3*3 && math.Abs(p.body.Y-d.Y) < 2 {
			return true
		}
	}
	g := i.guard
	return dist2(g.x, g.z, d.X, d.Z) < 3*3
}

func (i *Instance) phaseChanged(from, to string) {
	i.broadcast(i.clockMsg())
	switch to {
	case "roll":
		i.broadcastToast("warn", "Điểm danh! Tập trung ở sảnh tầng 1 trước 7 giờ.")
	case "night":
		i.broadcastToast("warn", "Đã đến giờ tắt đèn. Về buồng giam của mình ngay!")
	}
	if from == "roll" {
		for _, p := range i.players {
			if p.cl == nil || p.iso(i.now()) {
				continue
			}
			d := &p.Char.Data
			if p.room == "hall" {
				d.Discipline = clampInt(d.Discipline+3, 0, 100)
				i.questProgress(p, "roll")
				i.toast(p, "ok", "Điểm danh đủ. Kỷ luật +3.")
			} else {
				d.Discipline = clampInt(d.Discipline-5, 0, 100)
				fine := minInt(p.Char.Money, 5)
				p.Char.Money -= fine
				i.toast(p, "bad", "Vắng mặt điểm danh! Kỷ luật -5, phạt 5 tiền trại.")
			}
			p.dirty = true
		}
	}
}

func (i *Instance) newDay() {
	for _, p := range i.players {
		d := &p.Char.Data
		d.Sentence--
		p.dirty = true
		if d.Sentence <= 0 {
			p.Char.Releases++
			p.Char.Money += 100
			p.Char.Rep += 10
			d.Sentence = 20 + i.rng.Intn(21)
			i.toast(p, "ok", "Mãn hạn! Thưởng 100 tiền trại. Bản án mới: "+itoa(d.Sentence)+" ngày.")
			i.broadcastChat("sys", "", p.name()+" đã mãn hạn tù.", "")
		}
	}
	i.broadcastToast("ok", "Sang ngày mới trong trại.")
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	k := len(b)
	for n > 0 {
		k--
		b[k] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		k--
		b[k] = '-'
	}
	return string(b[k:])
}
