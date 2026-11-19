# web

Sorrel's dashboard: the invoices a studio has sent, and what's been paid.

    pnpm install
    pnpm dev

It talks to the [api](../api) on `:8080`. Amounts arrive in minor units and are formatted here, in `src/invoices/money.ts`.
