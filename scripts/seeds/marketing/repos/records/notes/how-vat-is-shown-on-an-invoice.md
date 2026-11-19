# How VAT is shown on an invoice

An invoice's PDF shows one VAT figure for each rate: the net of the lines at that rate, and the tax on it.

| Rate | Net | VAT |
|---|---|---|
| 20% | £912.00 | £182.40 |
| 5% | £40.00 | £2.00 |

A customer checks the total by adding these up. So does their accountant.

Lines don't show their own tax. The api can work it out for a line, but nothing a customer sees uses it.
