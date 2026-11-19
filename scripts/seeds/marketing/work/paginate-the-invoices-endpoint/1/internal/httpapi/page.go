package httpapi

import (
	"net/url"
	"strconv"

	"example.com/api/internal/invoice"
)

// pageSize is how many invoices GET /invoices gives unless ?limit= asks
// for fewer, and the most it gives.
const pageSize = 50

// page is the invoices after the one numbered ?after=, ?limit= of them,
// and the number to ask after for the next page ("" on the last).
func page(invoices []invoice.Invoice, q url.Values) ([]invoice.Invoice, string) {
	limit, err := strconv.Atoi(q.Get("limit"))
	if err != nil || limit < 1 || limit > pageSize {
		limit = pageSize
	}
	from := 0
	if after := q.Get("after"); after != "" {
		for i, inv := range invoices {
			if inv.Number == after {
				from = i + 1
				break
			}
		}
	}
	rest := invoices[from:]
	if len(rest) <= limit {
		return rest, ""
	}
	return rest[:limit], rest[limit-1].Number
}
