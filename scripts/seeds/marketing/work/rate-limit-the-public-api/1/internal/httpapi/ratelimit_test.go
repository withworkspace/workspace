package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRateLimitRefusesTheRequestAfterTheLast(t *testing.T) {
	h := RateLimit(2, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	codes := []int{}
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/invoices", nil)
		req.Header.Set("Authorization", "Bearer sk_test_1")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Errorf("three requests at a limit of two = %v, want 200 200 429", codes)
	}
}
