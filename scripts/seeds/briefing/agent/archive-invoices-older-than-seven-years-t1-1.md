> read repos/api/.ground/internal/invoice/invoice.go
> read repos/api/.ground/internal/httpapi/invoices.go
> read repos/web/.ground/src/invoices/InvoiceList.tsx
> read repos/records/.ground/adr/0003-an-invoice-freezes-when-it-is-sent.md
> run ws find archive
Nothing archives an invoice today. `GET /invoices` (`api/internal/httpapi/invoices.go`) lists every invoice a studio has sent, and the dashboard's list (`web/src/invoices/InvoiceList.tsx`) shows them all. A sent invoice is frozen (ADR 0003), so archiving can't change one. It can only say where the invoice is listed.

I'd build it in three parts:

1. **An archived state.** An invoice gets an `archived_at`, set by a daily job once it's seven years old. Nothing else about it changes.
2. **The list.** `GET /invoices` leaves archived invoices out unless it's asked for them, and the dashboard gets an Archived view.
3. **The PDF stays.** An archived invoice keeps its number, and its PDF is still served, because a studio can be asked for one years later.

A few things to settle:

- **Seven years from what?** The issue date, or the date it was paid? I'd say the issue date. It's on the invoice, and it never changes.
- **Invoices still owed.** Should one that hasn't been paid be archived? I'd leave those in the list, whatever their age.
- **Deleting.** Is archived the end, or are they deleted later? I'd keep to archiving. Deleting is a decision about retention, and worth an ADR of its own.

It changes what studios see, so the process asks for a proposal in records before anything is built. If you say "all three, as recommended", I'll propose the brief.
