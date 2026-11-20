> run ws mission brief --propose --title "Archive old invoices" --set -
| ## Goal
| An invoice more than seven years old is archived: out of a studio's list of invoices, and still there to open when it's asked for.
|
| ## Context
| - Nothing archives an invoice today. `GET /invoices` (`api/internal/httpapi/invoices.go`) lists every invoice a studio has sent, and the dashboard's list (`web/src/invoices/InvoiceList.tsx`) shows them all.
| - A sent invoice is frozen (ADR 0003, `records/adr/0003-an-invoice-freezes-when-it-is-sent.md`), so archiving can't change one. It says where the invoice is listed, and nothing else.
| - Two studios have more than 4,000 invoices each, most of them years old. The list is what they've asked about.
| - It changes what studios see, so the process asks for a proposal in records before it's built.
|
| ## Constraints
| - Seven years is counted from the invoice's issue date.
| - An invoice that's still owed isn't archived, whatever its age.
| - Nothing is deleted. Whether archived invoices are ever deleted is a decision of its own.
| - An archived invoice keeps its number, and its PDF is still served.
|
| ## Acceptance
| - A daily job archives every paid invoice issued more than seven years ago, and logs how many.
| - `GET /invoices` leaves archived invoices out unless it's asked for them (`?archived=true`).
| - The dashboard has an Archived view, and an archived invoice opens as it always did.
| - Tests cover the job, the endpoint and the date arithmetic; `go test ./...` passes.
I've proposed the brief (proposal 1), with all three as recommended: seven years from the issue date, invoices still owed left alone, and nothing deleted.

Read it before I write the plan. The plan will start with the proposal in records, since this changes what studios see, and I'd rather the brief was right before the steps are built on it.
