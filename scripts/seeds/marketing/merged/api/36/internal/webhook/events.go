package webhook

// The events a customer's endpoint is told of.
const (
	InvoiceSent = "invoice.sent"
	InvoicePaid = "invoice.paid"
)

// Paid is the event for an invoice whose payment has settled.
func Paid(id string, body []byte) Event {
	return Event{ID: id, Type: InvoicePaid, Body: body}
}
