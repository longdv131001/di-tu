package game

import (
	"context"

	"dituonline/internal/world"
)

func contextBG() context.Context { return context.Background() }

func worldInput(m Msg) world.Input {
	return world.Input{MX: m.MX, MZ: m.MZ, Yaw: m.Yaw, Jump: m.J, Run: m.R}
}

// RouteDMHook lets tests (and the hub) set the direct-message router after construction.
func (i *Instance) RouteDMHook(f func(from, to, text string) bool) { i.cfg.RouteDM = f }
