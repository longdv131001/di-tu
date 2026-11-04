// Command loadtest connects many simulated players and reports latency and throughput.
//
//	DISABLE_RATE_LIMITS=1 go run ./cmd/server &
//	go run ./cmd/loadtest -n 500 -pid $(pgrep -f 'cmd/server|/server$')
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

var (
	base     = flag.String("url", "http://localhost:8080", "server base URL")
	n        = flag.Int("n", 200, "number of simulated players")
	dur      = flag.Duration("d", 30*time.Second, "how long to stay connected")
	ramp     = flag.Duration("ramp", 10*time.Second, "time over which players connect")
	pid      = flag.Int("pid", 0, "server process id (Linux) to report its memory")
	snaps    atomic.Int64
	bytesIn  atomic.Int64
	connects atomic.Int64
	failures atomic.Int64
	mu       sync.Mutex
	rtts     []float64
)

func rss(pid int) string {
	if pid == 0 {
		return "n/a"
	}
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return "n/a"
	}
	for _, l := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(l, "VmRSS:") {
			return strings.TrimSpace(strings.TrimPrefix(l, "VmRSS:"))
		}
	}
	return "n/a"
}

func register(i int, run string) (string, error) {
	body, _ := json.Marshal(map[string]string{"username": fmt.Sprintf("lt%s_%d", run, i), "password": "loadtest-pass", "name": fmt.Sprintf("Bot %s%d", run, i)})
	resp, err := http.Post(*base+"/api/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct{ Token, Error string }
	json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("register: %d %s", resp.StatusCode, out.Error)
	}
	return out.Token, nil
}

func bot(ctx context.Context, i int, run string, wg *sync.WaitGroup) {
	defer wg.Done()
	tok, err := register(i, run)
	if err != nil {
		failures.Add(1)
		if failures.Load() < 4 {
			fmt.Println(err)
		}
		return
	}
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(*base, "http")+"/ws", nil)
	if err != nil {
		failures.Add(1)
		return
	}
	defer c.CloseNow()
	c.SetReadLimit(1 << 20)
	hello, _ := json.Marshal(map[string]string{"t": "hello", "token": tok})
	if c.Write(ctx, websocket.MessageText, hello) != nil {
		failures.Add(1)
		return
	}
	connects.Add(1)
	go func() {
		for {
			_, data, err := c.Read(ctx)
			if err != nil {
				return
			}
			bytesIn.Add(int64(len(data)))
			if bytes.Contains(data, []byte(`"t":"snap"`)) {
				snaps.Add(1)
			} else if bytes.Contains(data, []byte(`"t":"pong"`)) {
				var p struct{ TS int64 }
				json.Unmarshal(data, &p)
				mu.Lock()
				rtts = append(rtts, float64(time.Now().UnixMilli()-p.TS))
				mu.Unlock()
			}
		}
	}()
	rng := rand.New(rand.NewSource(int64(i)))
	yaw, seq := rng.Float64()*6, 0
	tk := time.NewTicker(50 * time.Millisecond)
	defer tk.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			seq++
			if seq%40 == 0 {
				yaw += rng.Float64() - 0.5
			}
			m := map[string]any{"t": "in", "s": seq, "mz": 1, "yaw": yaw, "r": seq%100 < 50}
			if seq%200 == 0 {
				m["j"] = true
			}
			b, _ := json.Marshal(m)
			if c.Write(ctx, websocket.MessageText, b) != nil {
				return
			}
			if seq%20 == 0 {
				p, _ := json.Marshal(map[string]any{"t": "ping", "ts": time.Now().UnixMilli()})
				c.Write(ctx, websocket.MessageText, p)
			}
			if seq%100 == 0 {
				ch, _ := json.Marshal(map[string]any{"t": "chat", "ch": "global", "text": "xin chào cả trại"})
				c.Write(ctx, websocket.MessageText, ch)
			}
		}
	}
}

func main() {
	flag.Parse()
	run := fmt.Sprintf("%x", time.Now().UnixNano()%0xfffff)
	ctx, cancel := context.WithTimeout(context.Background(), *ramp+*dur)
	defer cancel()
	var wg sync.WaitGroup
	fmt.Printf("starting %d bots (run %s), memory before: %s\n", *n, run, rss(*pid))
	start := time.Now()
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go bot(ctx, i, run, &wg)
		time.Sleep(*ramp / time.Duration(*n))
	}
	time.Sleep(*dur / 2)
	fmt.Printf("mid-run: connected=%d memory=%s\n", connects.Load(), rss(*pid))
	wg.Wait()
	el := time.Since(start).Seconds()
	mu.Lock()
	sort.Float64s(rtts)
	pct := func(p float64) float64 {
		if len(rtts) == 0 {
			return math.NaN()
		}
		return rtts[int(float64(len(rtts)-1)*p)]
	}
	fmt.Printf("connected=%d failed=%d snapshots=%d (%.0f/s) inbound=%.1f MB (%.1f KB/s per bot)\n",
		connects.Load(), failures.Load(), snaps.Load(), float64(snaps.Load())/el, float64(bytesIn.Load())/1e6,
		float64(bytesIn.Load())/1e3/el/math.Max(1, float64(connects.Load())))
	fmt.Printf("ping rtt ms: p50=%.0f p95=%.0f p99=%.0f over %d samples; server memory after: %s\n", pct(.5), pct(.95), pct(.99), len(rtts), rss(*pid))
	mu.Unlock()
}
