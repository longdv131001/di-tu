package main

import (
	"testing"
	"time"
)

func TestLimiterEightPerSecond(t *testing.T) {
	var l limiter
	now := time.Now()
	for i := 0; i < 8; i++ {
		if !l.allowAt(now.Add(time.Duration(i) * 10 * time.Millisecond)) {
			t.Fatalf("request %d unexpectedly blocked", i+1)
		}
	}
	if l.allowAt(now.Add(90 * time.Millisecond)) {
		t.Fatal("ninth request should be blocked")
	}
	if !l.allowAt(now.Add(time.Second)) {
		t.Fatal("limiter should release after one second")
	}
}
