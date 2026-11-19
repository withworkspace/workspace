// Package httpapi is the API's HTTP surface.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"example.com/api/internal/invoice"
)

// Store is where invoices are kept.
type Store interface {
	Invoices(ctx context.Context) ([]invoice.Invoice, error)
}

// invoiceJSON is an invoice as the API gives it: amounts in minor units.
type invoiceJSON struct {
	Number   string `json:"number"`
	Customer string `json:"customer"`
	Currency string `json:"currency"`
	Subtotal int64  `json:"subtotal"`
	Tax      int64  `json:"tax"`
	Total    int64  `json:"total"`
}

func toJSON(inv invoice.Invoice) invoiceJSON {
	return invoiceJSON{
		Number:   inv.Number,
		Customer: inv.Customer,
		Currency: inv.Currency,
		Subtotal: int64(inv.Subtotal()),
		Tax:      int64(inv.Tax()),
		Total:    int64(inv.Total()),
	}
}

// ListInvoices answers GET /invoices a page at a time: ?limit= invoices
// after the one numbered ?after=, with the number to ask after next.
func ListInvoices(store Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invoices, err := store.Invoices(r.Context())
		if err != nil {
			http.Error(w, "couldn't read the invoices", http.StatusInternalServerError)
			return
		}
		invoices, next := page(invoices, r.URL.Query())
		out := make([]invoiceJSON, 0, len(invoices))
		for _, inv := range invoices {
			out = append(out, toJSON(inv))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"invoices": out, "next": next})
	}
}
