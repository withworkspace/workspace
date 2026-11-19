package httpapi

import (
	"net/http/httptest"
	"testing"

	"example.com/api/internal/invoice"
)

func TestExportInvoicesKeepsToTheFilter(t *testing.T) {
	store := memory{
		{Number: "INV-0041", Customer: "Fenwick & Daughters", Currency: "GBP", Lines: []invoice.Line{{Quantity: 1, UnitPrice: 240000, TaxRate: 2000}}},
		{Number: "INV-0042", Customer: "Marlow Studio", Currency: "GBP", Lines: []invoice.Line{{Quantity: 2, UnitPrice: 45600, TaxRate: 2000}}},
	}
	rec := httptest.NewRecorder()
	ExportInvoices(store)(rec, httptest.NewRequest("GET", "/invoices.csv?customer=marlow", nil))

	want := "number,customer,currency,subtotal,tax,total\nINV-0042,Marlow Studio,GBP,912.00,182.40,1094.40\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("GET /invoices.csv?customer=marlow =\n%s\nwant\n%s", got, want)
	}
}
