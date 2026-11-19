package httpapi

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strings"

	"example.com/api/internal/invoice"
)

// ExportInvoices answers GET /invoices.csv with a row for each invoice
// whose customer matches ?customer=, as the dashboard's filter does. It's
// for a spreadsheet, so amounts are written as a customer reads them
// (1094.40), not as minor units.
func ExportInvoices(store Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		invoices, err := store.Invoices(r.Context())
		if err != nil {
			http.Error(w, "couldn't read the invoices", http.StatusInternalServerError)
			return
		}
		filter := strings.ToLower(r.URL.Query().Get("customer"))
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="invoices.csv"`)
		out := csv.NewWriter(w)
		out.Write([]string{"number", "customer", "currency", "subtotal", "tax", "total"})
		for _, inv := range invoices {
			if !strings.Contains(strings.ToLower(inv.Customer), filter) {
				continue
			}
			out.Write([]string{inv.Number, inv.Customer, inv.Currency, decimal(inv.Subtotal()), decimal(inv.Tax()), decimal(inv.Total())})
		}
		out.Flush()
	}
}

// decimal is an amount as a spreadsheet reads it: 109440 is 1094.40.
func decimal(m invoice.Money) string {
	sign := ""
	if m < 0 {
		sign, m = "-", -m
	}
	return fmt.Sprintf("%s%d.%02d", sign, m/100, m%100)
}
