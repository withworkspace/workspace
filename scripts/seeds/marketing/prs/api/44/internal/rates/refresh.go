package rates

import (
	"context"
	"time"
)

// Refresh asks the bank again for every rate the cache holds, every half
// hour, so nobody's request is the one that waits for it.
func (c *Cache) Refresh(ctx context.Context) {
	tick := time.NewTicker(30 * time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			c.refreshAll()
		}
	}
}

func (c *Cache) refreshAll() {
	c.mu.Lock()
	keys := make([]string, 0, len(c.rates))
	for k := range c.rates {
		keys = append(keys, k)
	}
	c.mu.Unlock()
	for _, k := range keys {
		from, to := k[:3], k[4:]
		rate, err := c.Source(from, to)
		if err != nil {
			continue
		}
		c.mu.Lock()
		c.rates[k] = entry{rate: rate, at: time.Now()}
		c.mu.Unlock()
	}
}
