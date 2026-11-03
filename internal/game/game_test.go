package game

import (
	"testing"
	"time"
)

func TestEconomyAndWork(t *testing.T) {
	p := NewPlayer("x", "Bắp Rang", "#f80", "short", time.Now())
	if !Buy(p, "meal") || p.Money != 88 || p.Stamina != 100 {
		t.Fatal("meal economy")
	}
	if !ApplyWork(p, 80, 3*time.Second) || p.Money != 101 {
		t.Fatal("work reward")
	}
	if _, ok := WorkReward(101, 3*time.Second); ok {
		t.Fatal("score exploit")
	}
	if _, ok := WorkReward(50, time.Second); ok {
		t.Fatal("time exploit")
	}
}
func TestSentenceReduction(t *testing.T) {
	p := NewPlayer("x", "Bắp Rang", "#f80", "short", time.Now())
	p.Sentence = 10
	p.Reform = 19
	if ReduceSentence(p) != 0 {
		t.Fatal()
	}
	p.Reform++
	if ReduceSentence(p) != 1 || p.Sentence != 9 {
		t.Fatal("sentence")
	}
}
func TestEscapeChance(t *testing.T) {
	if EscapeChance(3, 3, true, 0) <= EscapeChance(3, 3, false, 0) {
		t.Fatal("night bonus")
	}
	if EscapeChance(3, 3, true, 100) < 1 {
		t.Fatal("lower bound")
	}
}
