package httpapi

import (
	"net/http"
	"strconv"
	"sync"
	"time"
)

// RateLimit lets each API key make limit requests a minute, and answers
// 429 with Retry-After once it's made them.
func RateLimit(limit int, next http.Handler) http.Handler {
	var mu sync.Mutex
	type window struct {
		start time.Time
		count int
	}
	windows := map[string]*window{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Authorization")
		now := time.Now()
		mu.Lock()
		win := windows[key]
		if win == nil || now.Sub(win.start) >= time.Minute {
			win = &window{start: now}
			windows[key] = win
		}
		win.count++
		over := win.count > limit
		wait := time.Minute - now.Sub(win.start)
		mu.Unlock()
		if over {
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
			http.Error(w, "too many requests", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
