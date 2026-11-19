package httpapi

import (
	"net/http"
	"sync"
)

// Idempotent answers a POST that carries an Idempotency-Key it has seen
// with what it answered the first time, so a client that tries again
// after a timeout doesn't send an invoice twice.
func Idempotent(next http.Handler) http.Handler {
	var mu sync.Mutex
	seen := map[string]int{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if r.Method != http.MethodPost || key == "" {
			next.ServeHTTP(w, r)
			return
		}
		mu.Lock()
		status, again := seen[key]
		mu.Unlock()
		if again {
			w.Header().Set("Idempotent-Replayed", "true")
			w.WriteHeader(status)
			return
		}
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		mu.Lock()
		seen[key] = rec.status
		mu.Unlock()
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
