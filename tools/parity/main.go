// Command parity prints the map and a reference trajectory so the JavaScript physics
// (web/static/js/physics.js) can be checked against the Go implementation:
//
//	go run ./tools/parity > /tmp/parity.json && node tools/parity/check.mjs /tmp/parity.json
package main

import (
	"encoding/json"
	"os"

	"dituonline/internal/world"
)

type run struct {
	Start   [3]float64   `json:"start"`
	Open    bool         `json:"open"`
	Samples [][3]float64 `json:"samples"`
}

func main() {
	m := world.Build()
	starts := [][3]float64{{42, 0, 58}, {20, 0, 10}, {6, 0, 8}, {11.5, 4, 10}, {50, 0, 20}, {16, 0, 28}, {10, 0, 45}}
	var runs []run
	for si, st := range starts {
		for _, open := range []bool{false, true} {
			state := uint32(12345 + si*7)
			rnd := func() uint32 { state = state*1664525 + 1013904223; return state }
			b := &world.Body{X: st[0], Y: st[1], Z: st[2], Grounded: true}
			in := world.Input{}
			r := run{Start: st, Open: open}
			for step := 0; step < 4000; step++ {
				if step%15 == 0 {
					in.MX = float64(int(rnd()>>16)%3 - 1)
					in.MZ = float64(int(rnd()>>16)%3 - 1)
					in.Yaw += float64(int(rnd()>>16)%100-50) / 40
					in.Run = rnd()>>16&1 == 1
				}
				in.Jump = step%37 == 0
				m.StepBody(b, in, func(string) bool { return open })
				if step%50 == 0 {
					r.Samples = append(r.Samples, [3]float64{b.X, b.Y, b.Z})
				}
			}
			runs = append(runs, r)
		}
	}
	json.NewEncoder(os.Stdout).Encode(map[string]any{"map": m.Wire(), "runs": runs})
}
