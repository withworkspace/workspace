package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDeliver(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Get("Sorrel-Event")
	}))
	defer srv.Close()

	err := Deliverer{}.Deliver(context.Background(), srv.URL, Event{ID: "evt_1", Type: "invoice.paid", Body: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if got != "invoice.paid" {
		t.Errorf("Sorrel-Event = %q, want invoice.paid", got)
	}
}

func TestDeliverSaysWhenItsRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if err := (Deliverer{}).Deliver(context.Background(), srv.URL, Event{ID: "evt_2", Type: "invoice.sent"}); err == nil {
		t.Error("Deliver() of a 503 = nil, want an error")
	}
}
