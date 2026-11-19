package httpapi

import (
	"net/url"
	"testing"

	"example.com/api/internal/invoice"
)

func TestPage(t *testing.T) {
	all := []invoice.Invoice{{Number: "INV-1"}, {Number: "INV-2"}, {Number: "INV-3"}}

	first, next := page(all, url.Values{"limit": {"2"}})
	if len(first) != 2 || next != "INV-2" {
		t.Errorf("the first page = %d invoices, next %q; want 2, INV-2", len(first), next)
	}
	last, next := page(all, url.Values{"limit": {"2"}, "after": {"INV-2"}})
	if len(last) != 1 || next != "" {
		t.Errorf("the last page = %d invoices, next %q; want 1 and no next", len(last), next)
	}
}
