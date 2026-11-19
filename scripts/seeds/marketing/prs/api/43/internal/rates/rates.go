// Package rates is what one currency is worth in another.
package rates

import (
	"sync"
	"time"
)

// Source is where a rate comes from: the bank's API.
type Source func(from, to string) (float64, error)

// Cache keeps each rate for an hour, so listing invoices doesn't ask the
// bank once a row.
type Cache struct {
	Source Source
	Now    func() time.Time

	mu    sync.Mutex
	rates map[string]entry
}

type entry struct {
	rate float64
	at   time.Time
}

// Rate is what one unit of from is worth in to, an hour old at most.
func (c *Cache) Rate(from, to string) (float64, error) {
	now := time.Now
	if c.Now != nil {
		now = c.Now
	}
	key := from + ">" + to
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.rates[key]; ok && now().Sub(e.at) < time.Hour {
		return e.rate, nil
	}
	rate, err := c.Source(from, to)
	if err != nil {
		return 0, err
	}
	if c.rates == nil {
		c.rates = map[string]entry{}
	}
	c.rates[key] = entry{rate: rate, at: now()}
	return rate, nil
}
