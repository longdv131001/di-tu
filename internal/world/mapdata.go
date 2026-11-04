package world

import "sort"

const (
	wallH = 3.6
	wallT = 0.4
	floor = 4.0 // upper floor level

	// Colours are consumed by the client: outer wall colours (cB1..cB3) are also how the
	// client recognises walls that get windows, so keep them distinct from interior colours.
	cB1    = 0xbcd5ee // cell block, ground floor
	cB1Up  = 0xd4e5f7 // cell block, upper floor
	cB2    = 0xf4cf93 // canteen + classroom
	cB3    = 0xf2e08a // workshop
	cB4    = 0x56627a // guard block
	cIn    = 0xf3ecdc // interior walls
	cIn4   = 0x7a8499
	cDark  = 0x6b6a66
	cFloor = 0xe6e0d0
	cYard  = 0xd9d3c4
	cFence = 0xc9a98a
	cWood  = 0xa9764a
	cMetal = 0x7b8794
	cBed   = 0x5b7fa6
	cDoor  = 0x8a5a3a
	cLock  = 0x4a4f57
	cLeaf  = 0x4f8a4a
)

type gap struct{ c, w float64 }

type builder struct{ m *Map }

func (b *builder) box(x0, z0, x1, z1, y, h float64, c int, solid bool) {
	f := floorOf(y)
	b.m.Boxes = append(b.m.Boxes, Box{X: (x0 + x1) / 2, Y: y, Z: (z0 + z1) / 2, W: x1 - x0, H: h, D: z1 - z0, C: c, F: f, Solid: solid})
}

// wallX builds a wall running along x at the given z, leaving openings.
func (b *builder) wallX(x0, x1, z, y, h float64, c int, gaps ...gap) {
	sort.Slice(gaps, func(i, j int) bool { return gaps[i].c < gaps[j].c })
	cur := x0
	for _, g := range gaps {
		if g.c-g.w/2 > cur {
			b.box(cur, z-wallT/2, g.c-g.w/2, z+wallT/2, y, h, c, true)
		}
		b.box(g.c-g.w/2, z-wallT/2, g.c+g.w/2, z+wallT/2, y+3.0, h-3.0, c, true) // lintel
		cur = g.c + g.w/2
	}
	if x1 > cur {
		b.box(cur, z-wallT/2, x1, z+wallT/2, y, h, c, true)
	}
}

func (b *builder) wallZ(z0, z1, x, y, h float64, c int, gaps ...gap) {
	sort.Slice(gaps, func(i, j int) bool { return gaps[i].c < gaps[j].c })
	cur := z0
	for _, g := range gaps {
		if g.c-g.w/2 > cur {
			b.box(x-wallT/2, cur, x+wallT/2, g.c-g.w/2, y, h, c, true)
		}
		b.box(x-wallT/2, g.c-g.w/2, x+wallT/2, g.c+g.w/2, y+3.0, h-3.0, c, true)
		cur = g.c + g.w/2
	}
	if z1 > cur {
		b.box(x-wallT/2, cur, x+wallT/2, z1, y, h, c, true)
	}
}

func (b *builder) door(id string, x, y, z, w float64, axis string, locked bool) {
	c := cDoor
	if locked {
		c = cLock
	}
	b.m.Doors = append(b.m.Doors, Door{ID: id, X: x, Y: y, Z: z, W: w, Axis: axis, Locked: locked, C: c})
}

func (b *builder) room(id, name string, x0, z0, x1, z1 float64, f, cap int) {
	b.m.Rooms = append(b.m.Rooms, Room{ID: id, Name: name, X0: x0, Z0: z0, X1: x1, Z1: z1, F: f, Cap: cap})
}

func (b *builder) obj(id, kind, label string, x, y, z float64) {
	b.m.Objs = append(b.m.Objs, Obj{ID: id, Kind: kind, Label: label, X: x, Y: y, Z: z})
}

// Build creates the single, unified prison map: a walled compound with a
// two-storey cell block, canteen, classroom, workshop and a guard block.
func Build() *Map {
	m := &Map{Version: 1, W: 84, D: 64, Spawn: [3]float64{42, 0, 58}}
	b := &builder{m}

	// ground and perimeter
	m.Slabs = append(m.Slabs, Slab{0, 0, 84, 64, 0, cYard})
	b.wallX(0, 84, 0, 0, 3, cFence)
	b.wallX(0, 84, 64, 0, 3, cFence)
	b.wallZ(0, 64, 0, 0, 3, cFence)
	b.wallZ(0, 64, 84, 0, 3, cFence)
	b.box(83.6, 28.5, 84.2, 31.5, 0, 3, 0xd08a3c, true) // weak fence section

	// ---- B1: cell block (two floors) x4..36, z4..24
	for _, lv := range []struct {
		y float64
		c int
	}{{0, cB1}, {floor, cB1Up}} {
		b.wallX(4, 36, 4, lv.y, wallH, lv.c)
		b.wallZ(4, 24, 4, lv.y, wallH, lv.c)
		b.wallZ(4, 24, 36, lv.y, wallH, lv.c)
		if lv.y == 0 {
			b.wallX(4, 36, 24, 0, wallH, lv.c, gap{16, 2.4})
		} else {
			b.wallX(4, 36, 24, lv.y, wallH, lv.c)
		}
	}
	b.door("d_b1_main", 16, 0, 24, 2.4, "x", false)
	m.Slabs = append(m.Slabs, Slab{8, 4, 36, 24, floor, cFloor}, Slab{4, 22, 8, 24, floor, cFloor})
	b.box(8, 4, 36, 24, floor-0.3, 0.3, cFloor, false)
	b.box(4, 22, 8, 24, floor-0.3, 0.3, cFloor, false)
	m.Ramps = append(m.Ramps, Ramp{X0: 4, Z0: 10, X1: 8, Z1: 22, Y0: 0, Y1: floor, Axis: "z"})
	for k := 1; k <= 3; k++ { // mass under the stairs so nobody walks through them
		z0 := 10 + 3*float64(k)
		b.box(4.2, z0, 7.8, z0+3, 0, float64(k), cDark, true)
	}
	b.box(12, 12, 28, 16, 0.01, 0.02, 0xe6c84a, false) // roll-call marking in the hall
	b.wallZ(4, 18, 8, floor, wallH, cIn)
	for _, x := range []float64{15, 22, 29} {
		b.wallZ(4, 18, x, floor, wallH, cIn)
	}
	var cg []gap
	for i := 0; i < 4; i++ {
		cx := 11.5 + 7*float64(i)
		cg = append(cg, gap{cx, 1.8})
		b.door("d_c"+string(rune('1'+i)), cx, floor, 18, 1.8, "x", false)
		x0 := 8 + 7*float64(i)
		id := "cell" + string(rune('1'+i))
		b.room(id, "Buồng giam "+string(rune('1'+i)), x0, 4, x0+7, 18, 1, 6)
		b.box(x0+0.4, 5, x0+2.2, 7.2, floor, 0.55, cBed, true)
		b.obj("bed"+string(rune('1'+i)), "bed", "Giường", x0+1.3, floor, 6.1)
	}
	b.wallX(8, 36, 18, floor, wallH, cIn, cg...)
	b.room("hall", "Sảnh tầng 1", 4, 4, 36, 24, 0, 24)
	b.room("corridor", "Hành lang tầng 2", 4, 18, 36, 24, 1, 24)

	// ---- B2: canteen + classroom x40..76, z4..24
	b.wallX(40, 76, 4, 0, wallH, cB2)
	b.wallX(40, 76, 24, 0, wallH, cB2, gap{49, 2.4}, gap{67, 2.4})
	b.wallZ(4, 24, 40, 0, wallH, cB2)
	b.wallZ(4, 24, 76, 0, wallH, cB2)
	b.wallZ(4, 24, 58, 0, wallH, cIn, gap{14, 2.0})
	b.door("d_can", 49, 0, 24, 2.4, "x", false)
	b.door("d_cls", 67, 0, 24, 2.4, "x", false)
	b.door("d_mid", 58, 0, 14, 2.0, "z", false)
	b.room("canteen", "Căng tin", 40, 4, 58, 24, 0, 16)
	b.room("classroom", "Phòng học", 58, 4, 76, 24, 0, 12)
	b.box(44.5, 5.5, 53.5, 6.9, 0, 1.1, cWood, true)
	b.obj("counter", "counter", "Quầy căng tin", 49, 0, 8.2)
	b.box(43.7, 14.5, 46.3, 15.5, 0, 0.8, cWood, true)
	b.box(50.7, 14.5, 53.3, 15.5, 0, 0.8, cWood, true)
	for i := 0; i < 4; i++ {
		cx := 61.5 + 4*float64(i)
		b.box(cx-1.2, 9.5, cx+1.2, 10.4, 0, 0.8, cWood, true)
		b.obj("desk"+string(rune('1'+i)), "desk", "Bàn học", cx, 0, 11.4)
	}
	b.box(63, 4.2, 71, 4.4, 1.0, 1.6, 0x2f4a3c, false) // blackboard

	// ---- B3: workshop x4..30, z34..56
	b.wallX(4, 30, 34, 0, wallH, cB3)
	b.wallX(4, 30, 56, 0, wallH, cB3)
	b.wallZ(34, 56, 4, 0, wallH, cB3)
	b.wallZ(34, 56, 30, 0, wallH, cB3, gap{45, 2.4})
	b.door("d_ws", 30, 0, 45, 2.4, "z", false)
	b.room("workshop", "Xưởng", 4, 34, 30, 56, 0, 12)
	for i := 0; i < 3; i++ {
		cx := 10 + 6*float64(i)
		b.box(cx-1.5, 37.2, cx+1.5, 38.6, 0, 0.95, cMetal, true)
		b.obj("bench"+string(rune('1'+i)), "bench", "Bàn làm việc", cx, 0, 39.8)
	}

	// ---- B4: guard block x60..76, z36..52 (office + two isolation cells)
	b.wallX(60, 76, 36, 0, wallH, cB4)
	b.wallX(60, 76, 52, 0, wallH, cB4)
	b.wallZ(36, 52, 60, 0, wallH, cB4, gap{44, 2.0})
	b.wallZ(36, 52, 76, 0, wallH, cB4)
	b.wallZ(36, 52, 68, 0, wallH, cIn4)
	b.wallX(68, 76, 44, 0, wallH, cIn4)
	b.door("d_office", 60, 0, 44, 2.0, "z", true)
	b.room("office", "Văn phòng cai ngục", 60, 36, 68, 52, 0, 0)
	b.room("iso1", "Biệt giam 1", 68, 36, 76, 44, 0, 1)
	b.room("iso2", "Biệt giam 2", 68, 44, 76, 52, 0, 1)
	m.Iso = [2][3]float64{{72, 0, 40}, {72, 0, 48}}

	// ---- yard props
	b.box(48.9, 39.9, 49.1, 40.1, 0, 2.4, cMetal, true)
	b.box(50.9, 39.9, 51.1, 40.1, 0, 2.4, cMetal, true)
	b.box(48.9, 39.9, 51.1, 40.1, 2.3, 0.1, cMetal, false)
	b.obj("gym", "gym", "Xà đơn", 50, 0, 41.2)
	for _, t := range [][2]float64{{36, 30}, {60, 58}, {20, 60}, {76, 28}} {
		b.box(t[0]-0.3, t[1]-0.3, t[0]+0.3, t[1]+0.3, 0, 1.6, cWood, true)
		b.box(t[0]-1.3, t[1]-1.3, t[0]+1.3, t[1]+1.3, 1.6, 2.2, cLeaf, false)
	}
	b.obj("fence", "fence", "Hàng rào yếu", 83.2, 0, 30)

	m.Route = []RN{
		{16, 0, 31, "open:d_b1_main"}, {16, 0, 24, ""}, {12, 0, 14, ""}, {6, 0, 10, ""},
		{6, floor, 22, ""}, {20, floor, 21, ""}, {33, floor, 21, ""}, {12, floor, 21, ""}, {6, floor, 22, ""},
		{6, 0, 10, ""}, {16, 0, 16, ""}, {16, 0, 24, ""}, {16, 0, 31, "close:d_b1_main"},
		{34, 0, 31, ""}, {34, 0, 45, "open:d_ws"}, {20, 0, 45, ""}, {8, 0, 45, ""}, {34, 0, 45, "close:d_ws"},
		{50, 0, 45, ""}, {50, 0, 30, ""}, {67, 0, 30, "open:d_cls"}, {67, 0, 14, ""}, {67, 0, 30, "close:d_cls"},
		{49, 0, 30, "open:d_can"}, {49, 0, 14, ""}, {49, 0, 30, "close:d_can"}, {42, 0, 40, ""},
	}
	m.finalize()
	return m
}
