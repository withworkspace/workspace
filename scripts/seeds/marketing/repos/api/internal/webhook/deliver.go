// Package webhook tells a customer's endpoint what happened to an invoice.
package webhook

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
)

// Event is something that happened: invoice.sent, invoice.paid.
type Event struct {
	ID   string
	Type string
	Body []byte
}

// Deliverer posts events to an endpoint.
type Deliverer struct {
	Client *http.Client
}

// Deliver posts the event once. An answer that isn't 2xx is an error:
// nothing tries again yet.
func (d Deliverer) Deliver(ctx context.Context, url string, e Event) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(e.Body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Sorrel-Event", e.Type)
	req.Header.Set("Sorrel-Delivery", e.ID)
	client := d.Client
	if client == nil {
		client = http.DefaultClient
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return fmt.Errorf("webhook %s: %s", e.ID, res.Status)
	}
	return nil
}
