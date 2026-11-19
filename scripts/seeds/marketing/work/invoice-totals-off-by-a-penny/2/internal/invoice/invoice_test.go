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

// Support's case, INV-0388: three lines at 99p, each at 20% VAT. The
// lines come to £2.97 and the VAT on that is 59p, so the customer adds it
// up to £3.56. The invoice said £3.57.
func TestTaxRoundsOnce(t *testing.T) {
	inv := Invoice{Lines: []Line{
		{Description: "Sticker sheet", Quantity: 1, UnitPrice: 99, TaxRate: 2000},
		{Description: "Sticker sheet", Quantity: 1, UnitPrice: 99, TaxRate: 2000},
		{Description: "Sticker sheet", Quantity: 1, UnitPrice: 99, TaxRate: 2000},
	}}
	if got, want := inv.Tax(), Money(59); got != want {
		t.Errorf("Tax() = %d, want %d: each line's tax is rounded, then they're added up", got, want)
	}
	if got, want := inv.Total(), Money(356); got != want {
		t.Errorf("Total() = %d, want %d", got, want)
	}
}

func TestTaxByRate(t *testing.T) {
	inv := Invoice{Lines: []Line{
		{Description: "Design", Quantity: 2, UnitPrice: 45600, TaxRate: 2000},
		{Description: "Printed guide", Quantity: 4, UnitPrice: 1000, TaxRate: 500},
		{Description: "Hosting", Quantity: 1, UnitPrice: 0, TaxRate: 2000},
	}}
	got := inv.TaxByRate()
	want := []RateTax{
		{Rate: 500, Net: 4000, Tax: 200},
		{Rate: 2000, Net: 91200, Tax: 18240},
	}
	if len(got) != len(want) {
		t.Fatalf("TaxByRate() = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("TaxByRate()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}
