package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIdempotentAnswersTheSameKeyOnce(t *testing.T) {
	calls := 0
	h := Idempotent(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusCreated)
	}))
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/invoices", nil)
		req.Header.Set("Idempotency-Key", "k1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusCreated {
			t.Errorf("try %d: %d, want 201", i+1, rec.Code)
		}
	}
	if calls != 1 {
		t.Errorf("the handler ran %d times, want 1", calls)
	}
}
