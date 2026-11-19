package invoice

import "testing"

func TestTotal(t *testing.T) {
	inv := Invoice{Lines: []Line{
		{Description: "Design", Quantity: 2, UnitPrice: 45000, TaxRate: 2000},
		{Description: "Hosting", Quantity: 1, UnitPrice: 1200, TaxRate: 2000},
	}}
	if got, want := inv.Subtotal(), Money(91200); got != want {
		t.Errorf("Subtotal() = %d, want %d", got, want)
	}
	if got, want := inv.Tax(), Money(18240); got != want {
		t.Errorf("Tax() = %d, want %d", got, want)
	}
	if got, want := inv.Total(), Money(109440); got != want {
		t.Errorf("Total() = %d, want %d", got, want)
	}
}

func TestTotalWithNoLines(t *testing.T) {
	if got := (Invoice{}).Total(); got != 0 {
		t.Errorf("Total() = %d, want 0", got)
	}
}
