package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"example.com/api/internal/invoice"
)

type memory []invoice.Invoice

func (m memory) Invoices(context.Context) ([]invoice.Invoice, error) { return m, nil }

func TestListInvoices(t *testing.T) {
	store := memory{{
		Number:   "INV-0042",
		Customer: "Marlow Studio",
		Currency: "GBP",
		Lines:    []invoice.Line{{Description: "Design", Quantity: 2, UnitPrice: 45000, TaxRate: 2000}},
	}}
	rec := httptest.NewRecorder()
	ListInvoices(store)(rec, httptest.NewRequest("GET", "/invoices", nil))

	var got struct {
		Invoices []invoiceJSON `json:"invoices"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Invoices) != 1 || got.Invoices[0].Total != 108000 {
		t.Errorf("GET /invoices = %+v, want INV-0042 at 108000", got.Invoices)
	}
}
