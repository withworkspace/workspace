# api

Sorrel's API: invoices, and the webhooks that say what happened to them.

    go run ./cmd/api
    go test ./...

- `internal/invoice`: an invoice and what it adds up to. Money is whole minor units, never a float.
- `internal/httpapi`: the HTTP surface.
- `internal/webhook`: telling a customer's endpoint what happened.
