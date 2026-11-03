package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"
)

type sample struct{ ms float64 }

func main() {
	n := flag.Int("n", 100, "Số client giả lập")
	rate := flag.Int("rate", 1, "Số ping mỗi client mỗi giây")
	url := flag.String("url", "ws://localhost:8080/ws", "WebSocket URL")
	pid := flag.Int("pid", 0, "PID server để đọc RAM trên Linux")
	flag.Parse()
	if *n < 1 || *rate < 1 || *rate > 8 {
		fmt.Fprintln(os.Stderr, "n và rate phải lớn hơn 0")
		os.Exit(2)
	}
	var connected, failed atomic.Int64
	lat := make(chan sample, *n**rate*10)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	start := time.Now()
	for i := 0; i < *n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c, _, e := websocket.Dial(ctx, *url, &websocket.DialOptions{HTTPClient: &http.Client{Timeout: 10 * time.Second}})
			if e != nil {
				failed.Add(1)
				return
			}
			defer c.Close(websocket.StatusNormalClosure, "")
			connected.Add(1)
			raw := make([]byte, 24)
			_, _ = rand.Read(raw)
			token := hex.EncodeToString(raw)
			join, _ := json.Marshal(map[string]any{"t": "join", "token": token, "name": fmt.Sprintf("Test%04d", i), "color": "#f28c45"})
			_ = c.Write(ctx, websocket.MessageText, join)
			readCtx, stop := context.WithTimeout(ctx, 15*time.Second)
			_, _, e = c.Read(readCtx)
			stop()
			if e != nil {
				failed.Add(1)
				return
			}
			pongCh := make(chan []byte, 128)
			readErr := make(chan error, 1)
			go func() {
				for {
					_, b, err := c.Read(ctx)
					if err != nil {
						readErr <- err
						return
					}
					select {
					case pongCh <- b:
					default:
					}
				}
			}()
			ticker := time.NewTicker(time.Second / time.Duration(*rate))
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case err := <-readErr:
					if ctx.Err() == nil && err != nil {
						failed.Add(1)
					}
					return
				case <-ticker.C:
					sent := time.Now().UnixNano()
					ping, _ := json.Marshal(map[string]any{"t": "ping", "sentAt": sent})
					if err := c.Write(ctx, websocket.MessageText, ping); err != nil {
						if ctx.Err() == nil {
							failed.Add(1)
						}
						return
					}
				case b := <-pongCh:
					var msg struct {
						Type   string `json:"t"`
						SentAt int64  `json:"sentAt"`
					}
					if json.Unmarshal(b, &msg) == nil && msg.Type == "pong" && msg.SentAt > 0 {
						lat <- sample{float64(time.Now().UnixNano()-msg.SentAt) / 1e6}
					}
				}
			}
		}(i)
		time.Sleep(time.Millisecond)
	}
	time.Sleep(10 * time.Second)
	cancel()
	wg.Wait()
	close(lat)
	vals := []float64{}
	sum := 0.0
	for x := range lat {
		vals = append(vals, x.ms)
		sum += x.ms
	}
	sort.Float64s(vals)
	avg := 0.0
	if len(vals) > 0 {
		avg = sum / float64(len(vals))
	}
	p95 := 0.0
	if len(vals) > 0 {
		p95 = vals[(len(vals)-1)*95/100]
	}
	fmt.Printf("clients yêu cầu: %d\nKết nối thành công: %d\nLỗi kết nối/đọc: %d\nPing đo được: %d\nĐộ trễ trung bình: %.2f ms\nĐộ trễ p95: %.2f ms\nThời gian: %.1fs\n", *n, connected.Load(), failed.Load(), len(vals), avg, p95, time.Since(start).Seconds())
	if *pid > 0 {
		if b, e := os.ReadFile(fmt.Sprintf("/proc/%d/status", *pid)); e == nil {
			for _, line := range strings.Split(string(b), "\n") {
				if strings.HasPrefix(line, "VmRSS:") {
					fmt.Println("RAM server:", strings.TrimSpace(strings.TrimPrefix(line, "VmRSS:")))
					break
				}
			}
		} else {
			fmt.Println("Không đọc được RAM PID:", e)
		}
	} else {
		fmt.Println("RAM server: dùng -pid PID (Linux) để đọc VmRSS")
	}
}
