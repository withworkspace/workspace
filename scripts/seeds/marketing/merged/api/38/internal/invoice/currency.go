package invoice

import "fmt"

// currencies are the ones Sorrel sends invoices in, with how many minor
// units make one of each.
var currencies = map[string]int{"GBP": 100, "EUR": 100, "USD": 100, "JPY": 1}

// ValidCurrency says whether an invoice can be sent in the currency.
func ValidCurrency(code string) error {
	if _, ok := currencies[code]; !ok {
		return fmt.Errorf("invoice: no such currency %q", code)
	}
	return nil
}
