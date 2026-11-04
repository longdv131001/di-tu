package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"dituonline/internal/auth"
	"dituonline/internal/store"
	"dituonline/internal/world"
	"dituonline/web"
)

type env struct {
	t   *testing.T
	st  store.Store
	hub *Hub
	ts  *httptest.Server
}

func newEnv(t *testing.T, st store.Store) *env {
	auth.Iterations = 1000
	cfg := Config{MaxPlayers: 24, DaySeconds: 600, AdminKey: "adm"}
	hub := NewHub(cfg, st, world.Build())
	srv := NewServer(hub, st, auth.NewSigner("test-secret"), cfg, web.Static())
	return &env{t: t, st: st, hub: hub, ts: httptest.NewServer(srv.Handler())}
}

func (e *env) close() { e.ts.Close(); e.hub.Shutdown() }

func (e *env) post(path string, body any, hdr ...string) (int, map[string]any) {
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", e.ts.URL+path, bytes.NewReader(b))
	for k := 0; k+1 < len(hdr); k += 2 {
		req.Header.Set(hdr[k], hdr[k+1])
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (e *env) register(user, name string) string {
	code, out := e.post("/api/register", map[string]string{"username": user, "password": "password123", "name": name})
	if code != 200 {
		e.t.Fatalf("register %s: %d %v", user, code, out)
	}
	return out["token"].(string)
}

type wsc struct {
	c *websocket.Conn
	t *testing.T
}

func (e *env) dial(token string) *wsc {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.ts.URL, "http")+"/ws", nil)
	if err != nil {
		e.t.Fatal(err)
	}
	w := &wsc{c, e.t}
	w.send(map[string]any{"t": "hello", "token": token})
	return w
}

func (w *wsc) send(v any) {
	b, _ := json.Marshal(v)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := w.c.Write(ctx, websocket.MessageText, b); err != nil {
		w.t.Fatal(err)
	}
}

// until reads messages until pred matches or the timeout passes.
func (w *wsc) until(d time.Duration, pred func(map[string]any) bool) map[string]any {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	for {
		_, data, err := w.c.Read(ctx)
		if err != nil {
			return nil
		}
		var m map[string]any
		json.Unmarshal(data, &m)
		if pred(m) {
			return m
		}
	}
}

func typ(t string) func(map[string]any) bool {
	return func(m map[string]any) bool { return m["t"] == t }
}

func TestRegisterLoginValidation(t *testing.T) {
	e := newEnv(t, store.NewMemory())
	defer e.close()
	for _, c := range []map[string]string{
		{"username": "ab", "password": "password123", "name": "Hung"},
		{"username": "valid_user", "password": "short", "name": "Hung"},
		{"username": "valid_user", "password": "password123", "name": "<b>"},
	} {
		if code, _ := e.post("/api/register", c); code != 400 {
			t.Fatalf("expected 400 for %v, got %d", c, code)
		}
	}
	e.register("hung", "Hung")
	if code, _ := e.post("/api/register", map[string]string{"username": "HUNG", "password": "password123", "name": "Other"}); code != 409 {
		t.Fatalf("duplicate username: %d", code)
	}
	if code, _ := e.post("/api/register", map[string]string{"username": "hung2", "password": "password123", "name": "hung"}); code != 409 {
		t.Fatalf("duplicate character name: %d", code)
	}
	if code, out := e.post("/api/login", map[string]string{"username": "hung", "password": "password123"}); code != 200 || out["token"] == nil {
		t.Fatalf("login: %d %v", code, out)
	}
	if code, _ := e.post("/api/login", map[string]string{"username": "hung", "password": "nope-nope"}); code != 401 {
		t.Fatalf("bad password: %d", code)
	}
	if code, _ := e.post("/api/login", map[string]string{"username": "ghost", "password": "password123"}); code != 401 {
		t.Fatalf("unknown user: %d", code)
	}
}

func TestRegisterRateLimit(t *testing.T) {
	e := newEnv(t, store.NewMemory())
	defer e.close()
	got429 := false
	for k := 0; k < 14; k++ {
		code, _ := e.post("/api/register", map[string]string{"username": "u" + string(rune('a'+k)) + "xx", "password": "password123", "name": "N" + string(rune('a'+k)) + "x"})
		if code == 429 {
			got429 = true
		}
	}
	if !got429 {
		t.Fatal("registration is not rate limited")
	}
}

func TestPlayOverWebSocket(t *testing.T) {
	e := newEnv(t, store.NewMemory())
	defer e.close()
	ta, tb := e.register("anna", "Anna"), e.register("binh", "Binh")
	a, b := e.dial(ta), e.dial(tb)
	defer a.c.CloseNow()
	defer b.c.CloseNow()
	w := a.until(3*time.Second, typ("welcome"))
	if w == nil || w["inst"] == nil {
		t.Fatal("no welcome")
	}
	b.until(3*time.Second, typ("welcome"))
	for s := 1; s <= 8; s++ {
		a.send(map[string]any{"t": "in", "s": s, "mz": -1, "yaw": 0})
		time.Sleep(55 * time.Millisecond)
	}
	snap := a.until(3*time.Second, func(m map[string]any) bool { return m["t"] == "snap" && m["a"].(float64) >= 8 })
	if snap == nil {
		t.Fatal("snapshot never acknowledged inputs")
	}
	if snap["me"].([]any)[2].(float64) > 57 {
		t.Fatalf("player did not move: %v", snap["me"])
	}
	// Binh sees Anna (same spawn area)
	seen := b.until(3*time.Second, func(m map[string]any) bool {
		if m["t"] != "snap" || m["n"] == nil {
			return false
		}
		for _, x := range m["n"].([]any) {
			if x.(map[string]any)["name"] == "Anna" {
				return true
			}
		}
		return false
	})
	if seen == nil {
		t.Fatal("Binh never saw Anna")
	}
	a.send(map[string]any{"t": "chat", "ch": "global", "text": "xin chào"})
	if m := b.until(3*time.Second, func(m map[string]any) bool { return m["t"] == "chat" && m["ch"] == "global" }); m == nil || m["from"] != "Anna" {
		t.Fatalf("global chat: %v", m)
	}
	a.send(map[string]any{"t": "chat", "ch": "dm", "to": "Binh", "text": "riêng tư"})
	if m := b.until(3*time.Second, func(m map[string]any) bool { return m["t"] == "chat" && m["ch"] == "dm" }); m == nil {
		t.Fatal("dm not delivered")
	}
}

func TestAuthRequired(t *testing.T) {
	e := newEnv(t, store.NewMemory())
	defer e.close()
	w := e.dial("garbage.token")
	if m := w.until(3*time.Second, typ("err")); m == nil || m["code"] != "auth" {
		t.Fatalf("bad token accepted: %v", m)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	c, _, _ := websocket.Dial(ctx, "ws"+strings.TrimPrefix(e.ts.URL, "http")+"/ws", nil)
	b, _ := json.Marshal(map[string]any{"t": "in", "s": 1})
	c.Write(ctx, websocket.MessageText, b) // not a hello
	_, data, _ := c.Read(ctx)
	if !strings.Contains(string(data), "auth") {
		t.Fatalf("unauthenticated message accepted: %s", data)
	}
}

func TestSecondConnectionReplacesFirstAndReconnectKeepsPlace(t *testing.T) {
	e := newEnv(t, store.NewMemory())
	defer e.close()
	tok := e.register("anna", "Anna")
	a1 := e.dial(tok)
	a1.until(3*time.Second, typ("welcome"))
	for s := 1; s <= 12; s++ {
		a1.send(map[string]any{"t": "in", "s": s, "mx": -1, "yaw": 0})
		time.Sleep(52 * time.Millisecond)
	}
	var last []any
	a1.until(2*time.Second, func(m map[string]any) bool {
		if m["t"] == "snap" {
			last = m["me"].([]any)
		}
		return false
	})
	a2 := e.dial(tok)
	w := a2.until(3*time.Second, typ("welcome"))
	if w == nil {
		t.Fatal("reconnect failed")
	}
	pos := w["pos"].([]any)
	if pos[0].(float64) < 42.5 {
		t.Fatalf("reconnect lost position: %v (last %v)", pos, last)
	}
	if m := a1.until(2*time.Second, typ("err")); m != nil && m["code"] != "other_login" {
		t.Fatalf("unexpected error to old connection: %v", m)
	}
}

func TestPersistenceAcrossRestart(t *testing.T) { persistence(t, store.NewMemory()) }

func TestPersistenceAcrossRestartPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	p, err := store.OpenPostgres(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	persistence(t, p)
}

func persistence(t *testing.T, st store.Store) {
	name := "Persist" + string(rune('A'+time.Now().Nanosecond()%20))
	user := strings.ToLower(name)
	e := newEnv(t, st)
	tok := e.register(user, name)
	a := e.dial(tok)
	a.until(3*time.Second, typ("welcome"))
	cnt, _ := st.Top(context.Background(), "rep", 100)
	_ = cnt
	for s := 1; s <= 10; s++ {
		a.send(map[string]any{"t": "in", "s": s, "mx": 1, "yaw": 0, "r": true})
		time.Sleep(52 * time.Millisecond)
	}
	a.c.CloseNow()
	time.Sleep(200 * time.Millisecond)
	e.close() // graceful shutdown saves everyone

	e2 := newEnv(t, st) // "restart": brand new hub, same database
	defer e2.close()
	code, out := e2.post("/api/login", map[string]string{"username": user, "password": "password123"})
	if code != 200 {
		t.Fatalf("account lost after restart: %d %v", code, out)
	}
	b := e2.dial(out["token"].(string))
	w := b.until(3*time.Second, typ("welcome"))
	if w == nil {
		t.Fatal("no welcome after restart")
	}
	if w["pos"].([]any)[0].(float64) > 41 {
		t.Fatalf("position not restored after restart: %v", w["pos"])
	}
	if w["me"].(map[string]any)["name"] != name {
		t.Fatal("character not restored")
	}
}

func TestAdminBanKicksAndBlocks(t *testing.T) {
	e := newEnv(t, store.NewMemory())
	defer e.close()
	tok := e.register("troll", "Troll")
	a := e.dial(tok)
	a.until(3*time.Second, typ("welcome"))
	if code, _ := e.post("/api/admin/ban", map[string]any{"username": "troll", "minutes": 10, "reason": "spam"}); code != 401 {
		t.Fatalf("admin without key: %d", code)
	}
	if code, _ := e.post("/api/admin/ban", map[string]any{"username": "troll", "minutes": 10, "reason": "spam"}, "X-Admin-Key", "adm"); code != 200 {
		t.Fatalf("ban: %d", code)
	}
	if m := a.until(3*time.Second, typ("err")); m == nil || m["code"] != "kicked" {
		t.Fatalf("banned player not kicked: %v", m)
	}
	if code, _ := e.post("/api/login", map[string]string{"username": "troll", "password": "password123"}); code != 403 {
		t.Fatalf("banned login: %d", code)
	}
	b := e.dial(tok)
	if m := b.until(3*time.Second, typ("err")); m == nil || m["code"] != "banned" {
		t.Fatalf("banned ws: %v", m)
	}
	e.post("/api/admin/ban", map[string]any{"username": "troll", "minutes": 0}, "X-Admin-Key", "adm")
	if code, _ := e.post("/api/login", map[string]string{"username": "troll", "password": "password123"}); code != 200 {
		t.Fatalf("unban: %d", code)
	}
}

func TestMapLeaderboardHealthStatic(t *testing.T) {
	e := newEnv(t, store.NewMemory())
	defer e.close()
	e.register("anna", "Anna")
	for _, p := range []string{"/api/map", "/api/leaderboard?by=money", "/healthz", "/", "/vendor/three.module.min.js"} {
		resp, err := http.Get(e.ts.URL + p)
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("%s: %v %v", p, err, resp)
		}
		resp.Body.Close()
	}
	resp, _ := http.Get(e.ts.URL + "/api/map")
	var m map[string]any
	json.NewDecoder(resp.Body).Decode(&m)
	if len(m["boxes"].([]any)) < 50 || len(m["rooms"].([]any)) < 8 {
		t.Fatalf("map looks empty: %d boxes", len(m["boxes"].([]any)))
	}
	if code, _ := e.post("/api/admin/mute", map[string]any{"username": "x"}); code != 401 {
		t.Fatalf("admin open: %d", code)
	}
}
