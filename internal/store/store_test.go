package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func suite(t *testing.T, s Store) {
	ctx := context.Background()
	a, err := s.CreateAccount(ctx, "Alice", "hash")
	if err != nil || a.ID == 0 {
		t.Fatal(err)
	}
	if _, err := s.CreateAccount(ctx, "alice", "x"); err != ErrExists {
		t.Fatalf("want ErrExists, got %v", err)
	}
	got, err := s.AccountByName(ctx, "ALICE")
	if err != nil || got.ID != a.ID || got.PassHash != "hash" {
		t.Fatalf("lookup: %+v %v", got, err)
	}
	c := &Character{AccountID: a.ID, Name: "Hung", Outfit: Outfit{Shirt: 2}, Money: 50,
		Data: CharData{Inv: map[string]int{"wire": 2}, Stamina: 80, Sentence: 30, Quests: Quests{DayKey: "d", Daily: []QState{{ID: "work", Prog: 1}}}}}
	if err := s.CreateCharacter(ctx, c); err != nil || c.ID == 0 {
		t.Fatal(err)
	}
	b, _ := s.CreateAccount(ctx, "bob", "h")
	if err := s.CreateCharacter(ctx, &Character{AccountID: b.ID, Name: "hung"}); err != ErrExists {
		t.Fatalf("duplicate name should fail, got %v", err)
	}
	c.Money, c.Rep, c.Data.X = 120, 7, 12.5
	c.Data.Inv["pliers"] = 1
	if err := s.SaveCharacter(ctx, c); err != nil {
		t.Fatal(err)
	}
	r, err := s.CharacterByAccount(ctx, a.ID)
	if err != nil || r.Money != 120 || r.Data.X != 12.5 || r.Data.Inv["pliers"] != 1 || r.Data.Inv["wire"] != 2 || r.Data.Quests.Daily[0].Prog != 1 || r.Outfit.Shirt != 2 {
		t.Fatalf("roundtrip: %+v %v", r, err)
	}
	if _, err := s.CharacterByName(ctx, "HUNG"); err != nil {
		t.Fatal(err)
	}
	top, err := s.Top(ctx, "money", 5)
	if err != nil || len(top) != 1 || top[0].Money != 120 {
		t.Fatalf("top: %+v %v", top, err)
	}
	until := time.Now().Add(time.Hour)
	if err := s.SetBan(ctx, "alice", until, "spam"); err != nil {
		t.Fatal(err)
	}
	if g, _ := s.AccountByName(ctx, "alice"); g.BannedUntil.Before(time.Now()) || g.BanReason != "spam" {
		t.Fatalf("ban not stored: %+v", g)
	}
	if err := s.SetBan(ctx, "alice", time.Time{}, ""); err != nil {
		t.Fatal(err)
	}
	if g, _ := s.AccountByName(ctx, "alice"); !g.BannedUntil.IsZero() {
		t.Fatal("unban failed")
	}
	if err := s.SetMute(ctx, "nobody", until); err != ErrNotFound {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
	if err := s.AddReport(ctx, c.ID, c.ID, "test"); err != nil {
		t.Fatal(err)
	}
	if rs, err := s.Reports(ctx, 5); err != nil || len(rs) != 1 || rs[0].Reason != "test" {
		t.Fatalf("reports: %+v %v", rs, err)
	}
}

func TestMemory(t *testing.T) { suite(t, NewMemory()) }

// Runs only when TEST_DATABASE_URL points at an empty scratch database.
func TestPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	p, err := OpenPostgres(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	if _, err := p.db.ExecContext(ctx, `TRUNCATE accounts, characters, reports RESTART IDENTITY CASCADE`); err != nil {
		t.Fatal(err)
	}
	suite(t, p)
	p2, err := OpenPostgres(ctx, url) // second open must not re-run migrations
	if err != nil {
		t.Fatal(err)
	}
	p2.Close()
}
