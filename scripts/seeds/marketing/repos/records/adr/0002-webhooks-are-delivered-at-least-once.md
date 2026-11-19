# ADR 0002: Webhooks are delivered at least once

## Context

A customer's endpoint is sometimes down. An event that's dropped is an invoice their books never mark as paid.

## Decision

An event is delivered until its endpoint answers 2xx, or we give up and say so. Every delivery carries `Sorrel-Delivery`, the event's id, so a receiver can tell one it's seen before.

## Consequences

- Receivers have to be idempotent, and the docs say so first.
- Giving up is visible: the event is marked failed, and the dashboard shows it.
- How long we keep trying isn't decided here.
