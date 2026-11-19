// The API's server.
package main

import (
	"context"
	"log"
	"net/http"

	"example.com/api/internal/httpapi"
	"example.com/api/internal/invoice"
)

// sample is what the server has until it's given a database.
type sample struct{}

func (sample) Invoices(context.Context) ([]invoice.Invoice, error) {
	return []invoice.Invoice{
		{Number: "INV-0041", Customer: "Fenwick & Daughters", Currency: "GBP", Lines: []invoice.Line{
			{Description: "Brand refresh", Quantity: 1, UnitPrice: 240000, TaxRate: 2000},
		}},
		{Number: "INV-0042", Customer: "Marlow Studio", Currency: "GBP", Lines: []invoice.Line{
			{Description: "Design", Quantity: 2, UnitPrice: 45000, TaxRate: 2000},
			{Description: "Hosting", Quantity: 1, UnitPrice: 1200, TaxRate: 2000},
		}},
	}, nil
}

func main() {
	mux := http.NewServeMux()
	mux.Handle("GET /invoices", httpapi.ListInvoices(sample{}))
	mux.Handle("GET /invoices.csv", httpapi.ExportInvoices(sample{}))
	log.Println("api: listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
