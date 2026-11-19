# ADR 0003: An invoice freezes when it's sent

## Context

A sent invoice is in someone else's books. Changing it afterwards means two documents with one number.

## Decision

Sending an invoice freezes its lines, rates and totals. A mistake is put right with a credit note and a new invoice, each with its own number.

## Consequences

- A change to how totals are worked out applies to invoices sent after it, never to those already sent.
- Drafts are free to change.
