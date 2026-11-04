package api

import (
	"compress/gzip"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"dituonline/internal/auth"
	"dituonline/internal/game"
	"dituonline/internal/store"
)

var userRe = regexp.MustCompile(`^[a-z0-9_]{3,20}$`)

type Server struct {
	hub     *Hub
	st      store.Store
	signer  *auth.Signer
	cfg     Config
	mapJSON []byte
	static  fs.FS
	rl      *ipLimiter
	dummy   string
}

func NewServer(h *Hub, st store.Store, signer *auth.Signer, cfg Config, static fs.FS) *Server {
	mj, _ := json.Marshal(h.m.Wire())
	return &Server{hub: h, st: st, signer: signer, cfg: cfg, mapJSON: mj, static: static, rl: &ipLimiter{hits: map[string][]time.Time{}, off: cfg.DisableRateLimits}, dummy: auth.HashPassword("dummy")}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/register", s.register)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("GET /api/map", s.serveMap)
	mux.HandleFunc("GET /api/leaderboard", s.leaderboard)
	mux.HandleFunc("GET /healthz", s.health)
	mux.HandleFunc("GET /ws", s.ws)
	mux.HandleFunc("POST /api/admin/ban", s.admin(s.adminBan))
	mux.HandleFunc("POST /api/admin/mute", s.admin(s.adminMute))
	mux.HandleFunc("GET /api/admin/reports", s.admin(s.adminReports))
	mux.Handle("/", s.staticHandler())
	return secure(mux)
}

func secure(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' ws: wss:; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; frame-ancestors 'none'")
		next.ServeHTTP(w, r)
	})
}

// ---- helpers ----

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		fail(w, 400, "Dữ liệu không hợp lệ")
		return false
	}
	return true
}

func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if x := r.Header.Get("X-Forwarded-For"); x != "" {
			return strings.TrimSpace(strings.Split(x, ",")[0])
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

type ipLimiter struct {
	off  bool
	mu   sync.Mutex
	hits map[string][]time.Time
}

func newIPLimiter() *ipLimiter { return &ipLimiter{hits: map[string][]time.Time{}} }

// allow permits n events per window per key.
func (l *ipLimiter) allow(key string, n int, window time.Duration) bool {
	if l.off {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	h := l.hits[key][:0]
	for _, t := range l.hits[key] {
		if now.Sub(t) < window {
			h = append(h, t)
		}
	}
	if len(h) >= n {
		l.hits[key] = h
		return false
	}
	l.hits[key] = append(h, now)
	if len(l.hits) > 20000 { // bound memory under attack
		for k, v := range l.hits {
			if len(v) == 0 || now.Sub(v[len(v)-1]) > window {
				delete(l.hits, k)
			}
		}
	}
	return true
}

// ---- accounts ----

type credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

func (s *Server) register(w http.ResponseWriter, r *http.Request) {
	if !s.rl.allow("reg:"+s.clientIP(r), 10, time.Hour) {
		fail(w, 429, "Bạn đăng ký quá nhiều lần, hãy thử lại sau.")
		return
	}
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	c.Username = strings.ToLower(strings.TrimSpace(c.Username))
	name := game.CleanName(c.Name)
	switch {
	case !userRe.MatchString(c.Username):
		fail(w, 400, "Tên đăng nhập gồm 3-20 ký tự a-z, 0-9 hoặc _")
		return
	case len(c.Password) < 8 || len(c.Password) > 72:
		fail(w, 400, "Mật khẩu cần từ 8 đến 72 ký tự")
		return
	case name == "":
		fail(w, 400, "Tên nhân vật gồm 2-16 chữ cái, số hoặc dấu cách")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	acct, err := s.st.CreateAccount(ctx, c.Username, auth.HashPassword(c.Password))
	if errors.Is(err, store.ErrExists) {
		fail(w, 409, "Tên đăng nhập đã tồn tại")
		return
	} else if err != nil {
		fail(w, 500, "Lỗi máy chủ")
		return
	}
	ch := &store.Character{AccountID: acct.ID, Name: name, Money: 30,
		Outfit: store.Outfit{Shirt: rand.Intn(12), Pants: rand.Intn(8), Skin: rand.Intn(6), Hair: rand.Intn(4), HairC: rand.Intn(8)},
		Data:   store.CharData{Inv: map[string]int{}, Stamina: 100, Discipline: 70}}
	if err := s.st.CreateCharacter(ctx, ch); errors.Is(err, store.ErrExists) {
		fail(w, 409, "Tên nhân vật đã có người dùng")
		return
	} else if err != nil {
		fail(w, 500, "Lỗi máy chủ")
		return
	}
	writeJSON(w, 200, map[string]any{"token": s.signer.Issue(acct.ID, 7*24*time.Hour), "name": ch.Name})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	ip := s.clientIP(r)
	if !s.rl.allow("login:"+ip, 20, 10*time.Minute) {
		fail(w, 429, "Quá nhiều lần thử, hãy đợi vài phút.")
		return
	}
	var c credentials
	if !readJSON(w, r, &c) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	acct, err := s.st.AccountByName(ctx, strings.ToLower(strings.TrimSpace(c.Username)))
	hash := acct.PassHash
	if err != nil {
		hash = s.dummy // same work whether or not the account exists
	}
	if !auth.CheckPassword(c.Password, hash) || err != nil {
		fail(w, 401, "Sai tên đăng nhập hoặc mật khẩu")
		return
	}
	if time.Now().Before(acct.BannedUntil) {
		fail(w, 403, "Tài khoản bị khóa: "+acct.BanReason)
		return
	}
	ch, err := s.st.CharacterByAccount(ctx, acct.ID)
	if err != nil {
		fail(w, 500, "Lỗi máy chủ")
		return
	}
	writeJSON(w, 200, map[string]any{"token": s.signer.Issue(acct.ID, 7*24*time.Hour), "name": ch.Name})
}

func (s *Server) serveMap(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Write(s.mapJSON)
}

func (s *Server) leaderboard(w http.ResponseWriter, r *http.Request) {
	rows, err := s.st.Top(r.Context(), r.URL.Query().Get("by"), 20)
	if err != nil {
		fail(w, 500, "Lỗi máy chủ")
		return
	}
	out := make([]map[string]any, 0, len(rows))
	for k, t := range rows {
		out = append(out, map[string]any{"rank": k + 1, "name": t.Name, "money": t.Money, "rep": t.Rep, "points": t.Points, "escapes": t.Escapes, "releases": t.Releases})
	}
	w.Header().Set("Cache-Control", "public, max-age=10")
	writeJSON(w, 200, out)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	n, p := s.hub.Stats()
	writeJSON(w, 200, map[string]any{"ok": true, "instances": n, "players": p})
}

// ---- admin ----

func (s *Server) admin(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AdminKey == "" {
			http.NotFound(w, r)
			return
		}
		if subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Admin-Key")), []byte(s.cfg.AdminKey)) != 1 {
			if !s.rl.allow("adm:"+s.clientIP(r), 10, time.Minute) {
				fail(w, 429, "Quá nhiều lần thử")
				return
			}
			fail(w, 401, "Sai khóa quản trị")
			return
		}
		next(w, r)
	}
}

type adminReq struct {
	Username string `json:"username"`
	Minutes  int    `json:"minutes"`
	Reason   string `json:"reason"`
}

func (s *Server) charNameOf(ctx context.Context, username string) string {
	a, err := s.st.AccountByName(ctx, username)
	if err != nil {
		return ""
	}
	c, err := s.st.CharacterByAccount(ctx, a.ID)
	if err != nil {
		return ""
	}
	return c.Name
}

func (s *Server) adminBan(w http.ResponseWriter, r *http.Request) {
	var q adminReq
	if !readJSON(w, r, &q) {
		return
	}
	var until time.Time
	if q.Minutes > 0 {
		until = time.Now().Add(time.Duration(q.Minutes) * time.Minute)
	}
	if err := s.st.SetBan(r.Context(), q.Username, until, q.Reason); errors.Is(err, store.ErrNotFound) {
		fail(w, 404, "Không có tài khoản này")
		return
	} else if err != nil {
		fail(w, 500, "Lỗi máy chủ")
		return
	}
	if q.Minutes > 0 {
		if name := s.charNameOf(r.Context(), q.Username); name != "" {
			if inst := s.hub.instanceByName(name); inst != nil {
				inst.Kick(name, "Tài khoản bị khóa: "+q.Reason)
			}
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true, "until": until})
}

func (s *Server) adminMute(w http.ResponseWriter, r *http.Request) {
	var q adminReq
	if !readJSON(w, r, &q) {
		return
	}
	var until time.Time
	if q.Minutes > 0 {
		until = time.Now().Add(time.Duration(q.Minutes) * time.Minute)
	}
	if err := s.st.SetMute(r.Context(), q.Username, until); errors.Is(err, store.ErrNotFound) {
		fail(w, 404, "Không có tài khoản này")
		return
	} else if err != nil {
		fail(w, 500, "Lỗi máy chủ")
		return
	}
	if name := s.charNameOf(r.Context(), q.Username); name != "" {
		if inst := s.hub.instanceByName(name); inst != nil {
			inst.Mute(name, until)
		}
	}
	writeJSON(w, 200, map[string]any{"ok": true, "until": until})
}

func (s *Server) adminReports(w http.ResponseWriter, r *http.Request) {
	n, _ := strconv.Atoi(r.URL.Query().Get("n"))
	if n < 1 || n > 200 {
		n = 50
	}
	rs, err := s.st.Reports(r.Context(), n)
	if err != nil {
		fail(w, 500, "Lỗi máy chủ")
		return
	}
	out := make([]map[string]any, 0, len(rs))
	for _, x := range rs {
		out = append(out, map[string]any{"id": x.ID, "reporter": x.Reporter, "target": x.Target, "reason": x.Reason, "at": x.At})
	}
	writeJSON(w, 200, out)
}

// ---- static files ----

type gzipWriter struct {
	http.ResponseWriter
	io.Writer
}

func (g gzipWriter) Write(b []byte) (int, error) { return g.Writer.Write(b) }
func (g gzipWriter) WriteHeader(code int) {
	g.Header().Del("Content-Length") // length of the uncompressed body no longer applies
	g.ResponseWriter.WriteHeader(code)
}

func (s *Server) staticHandler() http.Handler {
	fsrv := http.FileServer(http.FS(s.static))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/vendor/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		p := r.URL.Path
		compressible := p == "/" || strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".css") || strings.HasSuffix(p, ".html")
		if compressible && strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Add("Vary", "Accept-Encoding")
			r.Header.Del("Range")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			w = gzipWriter{w, gz}
		}
		fsrv.ServeHTTP(w, r)
	})
}
