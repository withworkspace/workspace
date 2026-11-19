# ADR 0001: Money is whole minor units

## Context

An invoice is a promise about an exact amount. A float can't hold £0.10 exactly, and the error shows up as a penny on someone's invoice.

## Decision

Every amount is a whole number of minor units (pence, cents) with its currency beside it: `invoice.Money` in the api, a `number` of minor units over the wire, and formatted only where it's shown, in `web/src/invoices/money.ts`.

## Consequences

- A rate is basis points, so 20% is `2000`, and tax is one integer division.
- Where a division has a remainder, the rounding is a decision, and it's written down where it's made.
- The api never sends a formatted amount.
