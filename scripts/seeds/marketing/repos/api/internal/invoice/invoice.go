// Package invoice is an invoice and what it adds up to.
package invoice

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

// Tax is the line's tax, to the nearest minor unit.
func (l Line) Tax() Money { return roundDiv(int64(l.Net())*l.TaxRate, 10000) }

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

// Tax is the invoice's tax.
func (inv Invoice) Tax() Money {
	var sum Money
	for _, l := range inv.Lines {
		sum += l.Tax()
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
