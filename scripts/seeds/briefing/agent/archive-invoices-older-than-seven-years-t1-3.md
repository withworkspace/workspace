> read repos/records/.ground/proposals/overdue-reminders.md
> run ws mission brief --propose --revises 1 --title "Archive old invoices" --set -
| ## Goal
| An invoice more than seven years old is archived: out of a studio's list of invoices, and still there to open when it's asked for. A studio can bring one back.
|
| ## Context
| - Nothing archives an invoice today. `GET /invoices` (`api/internal/httpapi/invoices.go`) lists every invoice a studio has sent, and the dashboard's list (`web/src/invoices/InvoiceList.tsx`) shows them all.
| - A sent invoice is frozen (ADR 0003, `records/adr/0003-an-invoice-freezes-when-it-is-sent.md`), so archiving can't change one. It says where the invoice is listed, and nothing else.
| - Two studios have more than 4,000 invoices each, most of them years old. The list is what they've asked about.
| - A studio's accountant counts in tax years, so the seven years run from the end of the tax year an invoice was issued in.
| - It changes what studios see, so the process asks for a proposal in records before it's built.
|
| ## Constraints
| - Seven years is counted from the end of the tax year the invoice was issued in, not from its issue date.
| - An invoice that's still owed isn't archived, whatever its age.
| - Nothing is deleted. Whether archived invoices are ever deleted is a decision of its own.
| - An archived invoice keeps its number, and its PDF is still served.
| - Unarchiving is the studio's: the job doesn't archive again an invoice a studio has brought back.
|
| ## Acceptance
| - A daily job archives every paid invoice whose tax year ended more than seven years ago, and logs how many.
| - `GET /invoices` leaves archived invoices out unless it's asked for them (`?archived=true`).
| - The dashboard has an Archived view, an archived invoice opens as it always did, and Unarchive puts it back in the list.
| - Tests cover the job, the endpoint, unarchiving and the tax-year arithmetic; `go test ./...` passes.
> run ws mission plan --propose --revises 2 --set -
| ## Z
| - Paid invoices whose tax year ended more than seven years ago are archived each day: out of the list, and still there to open.
| - A studio sees its archived invoices in the dashboard, and can bring one back.
| - The proposal is accepted in records, and the change has been seen working in a browser.
| - Merged, with its tests.
|
| ## Stops early
| - The proposal isn't accepted: that's the answer, and nothing is built.
|
| ## Steps
| 1. Work: **Write the proposal.** What's archived and when, what a studio sees, and how an invoice is brought back. "Anything that changes what customers see is proposed first"
|    - in: `records`
|    - makes: a proposal, `proposals/archive-old-invoices.md`
|    - check: it says when an invoice is archived, in tax years, with an example
|    - check: it says what the dashboard shows, and how an invoice is unarchived
| 2. Review: **Accept the proposal.** Read it, and say whether it's built. "Once it's accepted, it can be built"
| 3. Land: **Land the proposal.**
|    - in: `records`
| 4. Work: **Archive in the api.** The `archived_at` field, the daily job, `?archived=true` on `GET /invoices`, and unarchiving.
|    - in: `api`
|    - check: `go test ./...` passes
|    - check: an invoice still owed is never archived
|    - check: an invoice a studio unarchived isn't archived again
|    - check: tests cover the tax-year arithmetic at both ends of a year
| 5. Work: **The Archived view.** The dashboard's list leaves archived invoices out, with an Archived view and Unarchive.
|    - in: `web`
|    - check: an archived invoice opens as it always did
|    - check: Unarchive puts an invoice back in the list without a reload
| 6. Verify: **Browser check.** See the Archived view working in a browser, by someone who didn't build it. "A change to the dashboard is shown working in a browser"
|    - in: `web`
|    - check: archived invoices are out of the list, and in the Archived view
|    - check: Unarchive puts one back in the list
|    - yours: the Archived view reads well beside the list
| 7. Review: **Review the change.** The diffs in `api` and `web`, and the verification bundle. "I review every change before its pull request is opened"
| 8. Open Pull Request
|    - in: `api`
| 9. Open Pull Request
|    - in: `web`
I've proposed the brief again with its plan, and confirming it starts the mission.

**What changed in the brief.** The seven years now run from the end of the tax year an invoice was issued in, not from its issue date. A studio can unarchive an invoice, and the job leaves one it brought back alone. The tests name unarchiving and the tax-year arithmetic.

**Plan:**
1. **Write the proposal** (Work, in `records`). It changes what studios see, so it's proposed first.
2. **Accept the proposal** (Review). Your checkpoint: nothing is built until you've read it.
3. **Land the proposal** (`records`).
4. **Archive in the api** (Work). The field, the daily job, the endpoint and unarchiving.
5. **The Archived view** (Work, in `web`).
6. **Browser check** (Verify). Someone who didn't build it shows it working. Whether the view reads well is left as your own check.
7. **Review the change** (Review), then a pull request in `api` and one in `web`.

I left the tax year's dates out of the brief, because the proposal is where they're settled.
