package hub

import (
	"dituonline/internal/proto"
	"testing"
)

func TestAssignCreatesPrisonWhenFull(t *testing.T) {
	h := New(2, 90, 30)
	for i := 0; i < 3; i++ {
		token := Token()
		v, id := h.Join(token, proto.Message{Name: "Phạm nhân"}, make(chan []byte, 8))
		if v == nil || id == "" {
			t.Fatal("join failed")
		}
		if i < 2 && id != "P-001" {
			t.Fatalf("first prison id = %s", id)
		}
		if i == 2 && id != "P-002" {
			t.Fatalf("expected a second prison, got %s", id)
		}
	}
}
