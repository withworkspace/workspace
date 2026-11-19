# Export invoices as CSV

## Problem

At the end of each quarter, studios copy their invoices into a spreadsheet for their accountant, a row at a time. It's the request support hears most.

## Proposal

An **Export** button on the invoices page downloads what the page is showing, filter included, as a CSV: one row an invoice, with its number, customer, currency, subtotal, tax and total.

Amounts are written as the customer reads them (`1094.40`), not as minor units: the file is for a spreadsheet, not for a program.

## What changes

- **api:** `GET /invoices.csv`, which takes the filter `GET /invoices` does.
- **web:** the button, in the invoices toolbar, disabled while there's nothing to export.

## Not in this

- Other formats. A PDF bundle is a different request.
- Line items. One row an invoice is what accountants asked for.
