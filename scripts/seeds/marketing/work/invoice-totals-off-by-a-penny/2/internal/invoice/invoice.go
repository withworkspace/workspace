// Package invoice is an invoice and what it adds up to.
package invoice

import "sort"

// Money is an amount in minor units (pence, cents): never a float.
type Money int64

// Line is one thing an invoice charges for.
type Line struct {
	Description string
	Quantity    int64
	UnitPrice   Money
	// TaxRate is in basis points: 2000 is 20%.
	TaxRate int64
}

// Net is what the line costs before tax.
func (l Line) Net() Money { return Money(l.Quantity) * l.UnitPrice }

// Invoice is what a customer is asked to pay.
type Invoice struct {
	Number   string
	Customer string
	Currency string
	Lines    []Line
}

// Subtotal is the invoice before tax.
func (inv Invoice) Subtotal() Money {
	var sum Money
	for _, l := range inv.Lines {
		sum += l.Net()
	}
	return sum
}

// RateTax is an invoice's tax at one rate: what the PDF shows a row of.
type RateTax struct {
	Rate int64 // basis points
	Net  Money
	Tax  Money
}

// TaxByRate is the tax at each rate the invoice's lines have, lowest
// first: each rate's tax is worked out on what its lines come to
// together, and rounded once.
func (inv Invoice) TaxByRate() []RateTax {
	net := map[int64]Money{}
	for _, l := range inv.Lines {
		net[l.TaxRate] += l.Net()
	}
	out := make([]RateTax, 0, len(net))
	for rate, n := range net {
		out = append(out, RateTax{Rate: rate, Net: n, Tax: roundDiv(int64(n)*rate, 10000)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Rate < out[j].Rate })
	return out
}

// Tax is the invoice's tax: the sum of what the PDF shows for each rate,
// so the total is what a customer adds up. Rounding each line's tax and
// adding those up drifts from it, a penny at a time.
func (inv Invoice) Tax() Money {
	var sum Money
	for _, t := range inv.TaxByRate() {
		sum += t.Tax
	}
	return sum
}

// Total is what's to pay.
func (inv Invoice) Total() Money { return inv.Subtotal() + inv.Tax() }

// roundDiv is n/d, rounded half away from zero.
func roundDiv(n, d int64) Money {
	if n < 0 {
		return Money(-((-n + d/2) / d))
	}
	return Money((n + d/2) / d)
}
