// Package world holds the static prison map and the movement physics.
// The same algorithm is mirrored in web/static/js/physics.js so the client can
// predict movement; the server remains the authority.
package world

import "math"

const (
	R      = 0.4  // player radius
	H      = 1.8  // player height
	Step   = 0.45 // max step-up
	Walk   = 3.2
	Run    = 6.0
	Grav   = 22.0
	Jump   = 7.2
	DT     = 0.05 // fixed simulation step (20 Hz)
	lowCut = 0.35 // collisions ignore things lower than this above the feet
)

// Box is an axis-aligned block: centre X,Z, base Y, size W,H,D.
type Box struct {
	X, Y, Z float64
	W, H, D float64
	C, F    int
	Solid   bool
}

func (b Box) wire() [9]float64 {
	s := 0.0
	if b.Solid {
		s = 1
	}
	return [9]float64{b.X, b.Y, b.Z, b.W, b.H, b.D, float64(b.C), float64(b.F), s}
}

type Slab struct {
	X0, Z0, X1, Z1, Y float64
	C                 int
}
type Ramp struct {
	X0, Z0, X1, Z1, Y0, Y1 float64
	Axis                   string
}
type Door struct {
	ID      string
	X, Y, Z float64
	W       float64
	Axis    string // "x": the door sits in a wall that runs along x
	Locked  bool
	C       int
}
type Room struct {
	ID, Name       string
	X0, Z0, X1, Z1 float64
	F              int
	Cap            int
}
type Obj struct {
	ID, Kind, Label string
	X, Y, Z         float64
}

// RN is a guard route node; Act is "open:<door>" or "close:<door>".
type RN struct {
	X, Y, Z float64
	Act     string
}

type Map struct {
	Version int
	W, D    float64
	Boxes   []Box
	Slabs   []Slab
	Ramps   []Ramp
	Doors   []Door
	Rooms   []Room
	Objs    []Obj
	Spawn   [3]float64
	Iso     [2][3]float64
	Route   []RN

	solids   []Box
	doorBox  map[string]Box
	doorByID map[string]Door
	objByID  map[string]Obj
}

func (m *Map) finalize() {
	m.doorBox = map[string]Box{}
	m.doorByID = map[string]Door{}
	m.objByID = map[string]Obj{}
	for _, b := range m.Boxes {
		if b.Solid {
			m.solids = append(m.solids, b)
		}
	}
	for _, d := range m.Doors {
		b := Box{X: d.X, Y: d.Y, Z: d.Z, W: d.W, H: 3, D: 0.3, C: d.C, F: floorOf(d.Y), Solid: true}
		if d.Axis != "x" {
			b.W, b.D = 0.3, d.W
		}
		m.doorBox[d.ID] = b
		m.doorByID[d.ID] = d
	}
	for _, o := range m.Objs {
		m.objByID[o.ID] = o
	}
}

func floorOf(y float64) int {
	if y >= 2 {
		return 1
	}
	return 0
}

func (m *Map) Door(id string) (Door, bool) { d, ok := m.doorByID[id]; return d, ok }
func (m *Map) Obj(id string) (Obj, bool)   { o, ok := m.objByID[id]; return o, ok }

// Wire is the compact structure served to the client.
func (m *Map) Wire() map[string]any {
	boxes := make([][9]float64, 0, len(m.Boxes))
	for _, b := range m.Boxes {
		boxes = append(boxes, b.wire())
	}
	slabs := make([][6]float64, 0, len(m.Slabs))
	for _, s := range m.Slabs {
		slabs = append(slabs, [6]float64{s.X0, s.Z0, s.X1, s.Z1, s.Y, float64(s.C)})
	}
	ramps := make([]map[string]any, 0, len(m.Ramps))
	for _, r := range m.Ramps {
		ramps = append(ramps, map[string]any{"x0": r.X0, "z0": r.Z0, "x1": r.X1, "z1": r.Z1, "y0": r.Y0, "y1": r.Y1, "axis": r.Axis})
	}
	doors := make([]map[string]any, 0, len(m.Doors))
	for _, d := range m.Doors {
		doors = append(doors, map[string]any{"id": d.ID, "x": d.X, "y": d.Y, "z": d.Z, "w": d.W, "axis": d.Axis, "locked": d.Locked, "c": d.C})
	}
	rooms := make([]map[string]any, 0, len(m.Rooms))
	for _, r := range m.Rooms {
		rooms = append(rooms, map[string]any{"id": r.ID, "name": r.Name, "x0": r.X0, "z0": r.Z0, "x1": r.X1, "z1": r.Z1, "f": r.F, "cap": r.Cap})
	}
	objs := make([]map[string]any, 0, len(m.Objs))
	for _, o := range m.Objs {
		objs = append(objs, map[string]any{"id": o.ID, "kind": o.Kind, "label": o.Label, "x": o.X, "y": o.Y, "z": o.Z})
	}
	return map[string]any{"version": m.Version, "w": m.W, "d": m.D, "boxes": boxes, "slabs": slabs, "ramps": ramps,
		"doors": doors, "rooms": rooms, "objs": objs, "spawn": m.Spawn}
}

type Body struct {
	X, Y, Z, VY float64
	Grounded    bool
}

type Input struct {
	MX, MZ, Yaw float64
	Jump, Run   bool
}

// RoomAt returns the id of the room containing the point, or "yard".
func (m *Map) RoomAt(x, y, z float64) string {
	f := floorOf(y)
	for _, r := range m.Rooms {
		if r.F == f && x >= r.X0 && x < r.X1 && z >= r.Z0 && z < r.Z1 {
			return r.ID
		}
	}
	return "yard"
}

func (m *Map) Room(id string) (Room, bool) {
	for _, r := range m.Rooms {
		if r.ID == id {
			return r, true
		}
	}
	return Room{}, false
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }

func pushOut(x, z *float64, y float64, b Box) {
	if y+H <= b.Y || y+lowCut >= b.Y+b.H {
		return
	}
	hx, hz := b.W/2, b.D/2
	cx := clamp(*x, b.X-hx, b.X+hx)
	cz := clamp(*z, b.Z-hz, b.Z+hz)
	dx, dz := *x-cx, *z-cz
	d2 := dx*dx + dz*dz
	if d2 >= R*R {
		return
	}
	if d2 > 1e-9 {
		d := math.Sqrt(d2)
		k := (R - d) / d
		*x += dx * k
		*z += dz * k
		return
	}
	px, pz := hx-math.Abs(*x-b.X), hz-math.Abs(*z-b.Z)
	if px < pz {
		if *x >= b.X {
			*x = b.X + hx + R
		} else {
			*x = b.X - hx - R
		}
	} else if *z >= b.Z {
		*z = b.Z + hz + R
	} else {
		*z = b.Z - hz - R
	}
}

func (m *Map) resolve(b *Body, open func(string) bool) {
	for i := 0; i < 2; i++ {
		for _, s := range m.solids {
			pushOut(&b.X, &b.Z, b.Y, s)
		}
		for _, d := range m.Doors {
			if !open(d.ID) {
				pushOut(&b.X, &b.Z, b.Y, m.doorBox[d.ID])
			}
		}
	}
}

func (r Ramp) height(x, z float64) (float64, bool) {
	if x < r.X0 || x > r.X1 || z < r.Z0 || z > r.Z1 {
		return 0, false
	}
	var t float64
	if r.Axis == "z" {
		t = (z - r.Z0) / (r.Z1 - r.Z0)
	} else {
		t = (x - r.X0) / (r.X1 - r.X0)
	}
	return r.Y0 + t*(r.Y1-r.Y0), true
}

// GroundAt returns the height of the highest walkable surface the body can stand on.
func (m *Map) GroundAt(x, z, y float64) float64 {
	best := 0.0
	for _, s := range m.Slabs {
		if x >= s.X0 && x <= s.X1 && z >= s.Z0 && z <= s.Z1 && s.Y <= y+Step && s.Y > best {
			best = s.Y
		}
	}
	for _, r := range m.Ramps {
		if h, ok := r.height(x, z); ok && h <= y+Step && h > best {
			best = h
		}
	}
	return best
}

// StepBody advances one fixed step. It returns the facing yaw (NaN when the
// body did not try to move) and whether it moved.
func (m *Map) StepBody(b *Body, in Input, open func(string) bool) (float64, bool) {
	mx, mz := clamp(in.MX, -1, 1), clamp(in.MZ, -1, 1)
	if l := math.Hypot(mx, mz); l > 1 {
		mx, mz = mx/l, mz/l
	}
	// forward = (sin yaw, cos yaw); right = forward x up = (-cos yaw, sin yaw)
	wx := -mx*math.Cos(in.Yaw) + mz*math.Sin(in.Yaw)
	wz := mx*math.Sin(in.Yaw) + mz*math.Cos(in.Yaw)
	moving := math.Hypot(mx, mz) > 0.05
	speed := Walk
	if in.Run {
		speed = Run
	}
	b.X += wx * speed * DT
	m.resolve(b, open)
	b.Z += wz * speed * DT
	m.resolve(b, open)
	b.X = clamp(b.X, R, m.W-R)
	b.Z = clamp(b.Z, R, m.D-R)

	wasGrounded := b.Grounded
	if wasGrounded && in.Jump {
		b.VY = Jump
		b.Grounded = false
		wasGrounded = false
	}
	b.VY -= Grav * DT
	b.Y += b.VY * DT
	g := m.GroundAt(b.X, b.Z, b.Y)
	switch {
	case b.Y <= g:
		b.Y, b.VY, b.Grounded = g, 0, true
	case wasGrounded && b.VY <= 0 && b.Y-g <= Step:
		b.Y, b.VY, b.Grounded = g, 0, true
	default:
		b.Grounded = false
	}
	facing := math.NaN()
	if moving {
		facing = math.Atan2(wx, wz)
	}
	return facing, moving
}
