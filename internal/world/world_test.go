package world

import (
	"math"
	"testing"
)

func closed(string) bool { return false }
func opened(string) bool { return true }

func run(m *Map, b *Body, in Input, n int, open func(string) bool) {
	for i := 0; i < n; i++ {
		m.StepBody(b, in, open)
	}
}

func TestWallBlocks(t *testing.T) {
	m := Build()
	b := &Body{X: 20, Z: 10, Grounded: true}
	run(m, b, Input{MZ: -1, Yaw: 0, Run: true}, 60, closed) // north towards z=4 wall
	if b.Z < 4.2+R-0.01 {
		t.Fatalf("walked through wall: z=%.2f", b.Z)
	}
}

func TestDoorClosedAndOpen(t *testing.T) {
	m := Build()
	b := &Body{X: 16, Z: 28, Grounded: true}
	run(m, b, Input{MZ: -1}, 50, closed) // towards hall door at z=24
	if b.Z < 24 {
		t.Fatalf("passed closed door: z=%.2f", b.Z)
	}
	b = &Body{X: 16, Z: 28, Grounded: true}
	run(m, b, Input{MZ: -1}, 60, opened)
	if b.Z > 22 {
		t.Fatalf("could not pass open door: z=%.2f", b.Z)
	}
}

func TestStairsReachUpperFloor(t *testing.T) {
	m := Build()
	b := &Body{X: 6, Z: 8, Grounded: true}
	run(m, b, Input{MZ: 1}, 110, opened) // south (+z) up the ramp
	if b.Y < 3.9 || b.Z < 21 {
		t.Fatalf("did not climb: y=%.2f z=%.2f", b.Y, b.Z)
	}
	if got := m.RoomAt(b.X, b.Y, b.Z); got != "corridor" {
		t.Fatalf("room=%s", got)
	}
	// and walk back down
	run(m, b, Input{MZ: -1}, 110, opened)
	if b.Y > 0.1 {
		t.Fatalf("did not descend: y=%.2f", b.Y)
	}
}

func TestCannotWalkUnderStairs(t *testing.T) {
	m := Build()
	b := &Body{X: 6, Z: 8, Grounded: true}
	run(m, b, Input{MZ: 1}, 40, opened) // walks ~6.4m: well onto the ramp
	if b.Y < 1.0 {                      // must be climbing, not passing underneath
		t.Fatalf("expected to be on the ramp, y=%.2f", b.Y)
	}
}

func TestJumpAndLand(t *testing.T) {
	m := Build()
	b := &Body{X: 40, Z: 40, Grounded: true}
	m.StepBody(b, Input{Jump: true}, closed)
	peak := 0.0
	for i := 0; i < 20; i++ {
		m.StepBody(b, Input{}, closed)
		peak = math.Max(peak, b.Y)
	}
	if peak < 0.8 || !b.Grounded || b.Y != 0 {
		t.Fatalf("peak=%.2f grounded=%v y=%.2f", peak, b.Grounded, b.Y)
	}
}

func TestCellDoorsLeadToCorridor(t *testing.T) {
	m := Build()
	b := &Body{X: 11.5, Y: floor, Z: 10, Grounded: true}
	run(m, b, Input{MZ: 1}, 40, closed)
	if b.Z > 18 {
		t.Fatalf("closed cell door ignored z=%.2f", b.Z)
	}
	run(m, b, Input{MZ: 1}, 40, opened)
	if m.RoomAt(b.X, b.Y, b.Z) != "corridor" {
		t.Fatalf("expected corridor, at %.1f,%.1f,%.1f", b.X, b.Y, b.Z)
	}
}

func TestRoomsAndObjects(t *testing.T) {
	m := Build()
	if m.RoomAt(20, 0, 10) != "hall" || m.RoomAt(70, 0, 40) != "iso1" || m.RoomAt(42, 0, 58) != "yard" {
		t.Fatal("room lookup")
	}
	for _, id := range []string{"bed1", "counter", "desk4", "bench3", "gym", "fence"} {
		if _, ok := m.Obj(id); !ok {
			t.Fatalf("missing obj %s", id)
		}
	}
	for _, n := range m.Route {
		if n.Act != "" {
			id := n.Act[len("open:"):]
			if n.Act[:5] == "close" {
				id = n.Act[len("close:"):]
			}
			if _, ok := m.Door(id); !ok {
				t.Fatalf("route references unknown door %s", id)
			}
		}
	}
}
