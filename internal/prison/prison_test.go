package prison

import (
	"dituonline/internal/proto"
	"testing"
	"time"
)

func TestJoinMoveAndChat(t *testing.T) {
	p := New("test", 6, 90, 30)
	out := make(chan []byte, 32)
	v := p.Call(Request{Token: "token-a", Msg: proto.Message{Type: "join", Name: "Bắp Rang", Color: "#f80"}, Out: out})
	snap, ok := v.(map[string]any)
	if !ok || snap["t"] != "welcome" {
		t.Fatalf("unexpected welcome: %#v", v)
	}
	p.Call(Request{Token: "token-b", Msg: proto.Message{Type: "join", Name: "Mì Gói", Color: "#0a0"}, Out: make(chan []byte, 32)})
	p.Call(Request{Token: "token-a", Msg: proto.Message{Type: "move", Zone: "yard"}})
	time.Sleep(130 * time.Millisecond)
	p.Call(Request{Token: "token-a", Msg: proto.Message{Type: "chat", Channel: "prison", Text: "Xin chào cả trại"}})
	found := false
	deadline := time.After(time.Second)
	for !found {
		select {
		case b := <-out:
			if len(b) > 0 && contains(string(b), "Xin chào cả trại") {
				found = true
			}
		case <-deadline:
			t.Fatal("chat message not broadcast")
		}
	}
	if p.Count() != 2 {
		t.Fatalf("expected 2 players, got %d", p.Count())
	}
}
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
func TestDisconnectGrace(t *testing.T) {
	p := New("test", 6, 90, 30)
	out := make(chan []byte, 4)
	p.Call(Request{Token: "resume-me", Msg: proto.Message{Type: "join", Name: "Tù nhân"}, Out: out})
	p.Call(Request{Token: "resume-me", Msg: proto.Message{Type: "disconnect"}})
	v := p.Call(Request{Token: "resume-me", Msg: proto.Message{Type: "join", Name: "Tù nhân"}, Out: out})
	if v == nil || p.Count() != 1 {
		t.Fatal("rejoin within grace should preserve player")
	}
}

func TestSocialFriendCrewAndMarket(t *testing.T) {
	p := New("social-test", 10, 90, 30)
	outA, outB := make(chan []byte, 64), make(chan []byte, 64)
	p.Call(Request{Token: "social-a", Msg: proto.Message{Type: "join", Name: "Bap Rang"}, Out: outA})
	p.Call(Request{Token: "social-b", Msg: proto.Message{Type: "join", Name: "Mi Goi"}, Out: outB})
	p.Call(Request{Token: "social-a", Msg: proto.Message{Type: "friend_request", Target: "Mi Goi"}, Out: outA})
	time.Sleep(130 * time.Millisecond)
	p.Call(Request{Token: "social-b", Msg: proto.Message{Type: "friend_reply", Target: "Bap Rang", Choice: "accept"}, Out: outB})
	time.Sleep(130 * time.Millisecond)
	p.Call(Request{Token: "social-a", Msg: proto.Message{Type: "crew_create", Name: "Hoi Mi Tom"}, Out: outA})
	v := p.Call(Request{Token: "social-a", Msg: proto.Message{Type: "leaderboard"}, Out: outA})
	_ = v
	// Social state is exposed in the welcome snapshot and refreshed through the output channel.
	foundFriend, foundCrew := false, false
	deadline := time.After(time.Second)
	for !(foundFriend && foundCrew) {
		select {
		case b := <-outA:
			if contains(string(b), `"name":"Mi Goi"`) && contains(string(b), `"friends"`) {
				foundFriend = true
			}
			if contains(string(b), `"name":"Hoi Mi Tom"`) {
				foundCrew = true
			}
		case <-deadline:
			t.Fatalf("social updates missing: friend=%v crew=%v", foundFriend, foundCrew)
		}
	}
}
