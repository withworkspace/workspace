package harness

import (
	"encoding/json"
	"slices"
	"strings"
	"time"
)

// What an agent spent: the tokens it was sent and wrote, by model, as its
// harness reports them. Tokens are what's kept, not money: a price is put
// on them when they're read, so a price that changes doesn't rewrite what
// was spent. Where the harness gives its own figure for what a run cost
// (Claude Code's, at list price), that's kept beside them.
//
// The four kinds are kept apart because they're priced apart, and far
// apart: a crew's session reads its cache many hundred times over for
// every token it writes, so a sum of them says nothing of the cost.

// Usage is what was spent on one model.
type Usage struct {
	Model      string
	Input      int64 // tokens sent that weren't cached
	Output     int64 // tokens written, thinking among them
	CacheWrite int64 // tokens sent that were cached as they went
	CacheRead  int64 // tokens read back from the cache
	// Cost is the harness's own figure for it, in US dollars: 0 where it
	// gives none.
	Cost float64
}

// Add adds what o spent to u's.
func (u *Usage) Add(o Usage) {
	u.Input += o.Input
	u.Output += o.Output
	u.CacheWrite += o.CacheWrite
	u.CacheRead += o.CacheRead
	u.Cost += o.Cost
}

// Zero is whether nothing was spent.
func (u Usage) Zero() bool {
	return u.Input == 0 && u.Output == 0 && u.CacheWrite == 0 && u.CacheRead == 0 && u.Cost == 0
}

// Spends is what a harness says of what its agents spend: in a headless
// run's output (Runs), and in a crew's session file (Crew).
type Spends struct {
	Runs, Crew bool
}

// spender is a harness that says what its agents spend.
type spender interface {
	spends() Spends
}

func (c *Claude) spends() Spends { return Spends{Runs: true, Crew: true} }

// claudeResultUsage is what a Claude Code run's result line says it spent:
// by model where it says (modelUsage, with its cost), else the run's whole
// usage on no model in particular.
func claudeResultUsage(line []byte) []Usage {
	var e struct {
		Cost  float64 `json:"total_cost_usd"`
		Usage *struct {
			Input      int64 `json:"input_tokens"`
			Output     int64 `json:"output_tokens"`
			CacheWrite int64 `json:"cache_creation_input_tokens"`
			CacheRead  int64 `json:"cache_read_input_tokens"`
		} `json:"usage"`
		Models map[string]struct {
			Input      int64   `json:"inputTokens"`
			Output     int64   `json:"outputTokens"`
			CacheWrite int64   `json:"cacheCreationInputTokens"`
			CacheRead  int64   `json:"cacheReadInputTokens"`
			Cost       float64 `json:"costUSD"`
		} `json:"modelUsage"`
	}
	if json.Unmarshal(line, &e) != nil {
		return nil
	}
	var out []Usage
	for model, m := range e.Models {
		u := Usage{Model: model, Input: m.Input, Output: m.Output, CacheWrite: m.CacheWrite, CacheRead: m.CacheRead, Cost: m.Cost}
		if !u.Zero() {
			out = append(out, u)
		}
	}
	if len(out) == 0 && e.Usage != nil {
		u := Usage{Input: e.Usage.Input, Output: e.Usage.Output, CacheWrite: e.Usage.CacheWrite, CacheRead: e.Usage.CacheRead, Cost: e.Cost}
		if !u.Zero() {
			out = append(out, u)
		}
	}
	slices.SortFunc(out, func(a, b Usage) int { return strings.Compare(a.Model, b.Model) })
	return out
}

// Spent is what's spent on one model on one day (UTC), in a session.
type Spent struct {
	Day string // 2006-01-02
	Usage
}

// SessionSpend reads what a crew's session spent in lines of its session
// file, by model and day. Claude Code writes a message's usage on each of
// the lines it writes the message in (one a block), the same on each and
// one after another: a message is counted once, at its first line. last
// is the message read last before these lines (one whose lines may carry
// on into them), and the message read last in them is returned, for the
// next read to carry on from.
func SessionSpend(harnessName string, lines [][]byte, last string) ([]Spent, string) {
	if harnessName != "claude" {
		return nil, last
	}
	at := map[[2]string]int{} // model, day: index in out
	var out []Spent
	for _, line := range lines {
		// Only an agent's message says what it spent; the rest (what
		// you typed, tool results, bookkeeping) needn't be read.
		if !strings.Contains(string(line), `"assistant"`) {
			continue
		}
		var e struct {
			Type      string    `json:"type"`
			Timestamp time.Time `json:"timestamp"`
			Message   struct {
				ID    string `json:"id"`
				Model string `json:"model"`
				Usage *struct {
					Input      int64 `json:"input_tokens"`
					Output     int64 `json:"output_tokens"`
					CacheWrite int64 `json:"cache_creation_input_tokens"`
					CacheRead  int64 `json:"cache_read_input_tokens"`
				} `json:"usage"`
			} `json:"message"`
		}
		if json.Unmarshal(line, &e) != nil || e.Type != "assistant" || e.Message.Usage == nil {
			continue
		}
		if e.Message.ID != "" && e.Message.ID == last {
			continue
		}
		last = e.Message.ID
		// What Claude Code writes itself (an API error it reports, a
		// command's output) is on no model, and spent nothing.
		if e.Message.Model == "" || e.Message.Model == "<synthetic>" {
			continue
		}
		u := Usage{Model: e.Message.Model, Input: e.Message.Usage.Input, Output: e.Message.Usage.Output,
			CacheWrite: e.Message.Usage.CacheWrite, CacheRead: e.Message.Usage.CacheRead}
		if u.Zero() {
			continue
		}
		day := e.Timestamp.UTC().Format(time.DateOnly)
		k := [2]string{u.Model, day}
		i, ok := at[k]
		if !ok {
			i = len(out)
			at[k] = i
			out = append(out, Spent{Day: day, Usage: Usage{Model: u.Model}})
		}
		out[i].Add(u)
	}
	return out, last
}
