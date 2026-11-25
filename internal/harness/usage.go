package harness

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/acp"
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

// Which say: Claude Code and pi, in a run's output and their session
// files; Cursor's CLI in a run's output (its chats are in a database ws
// can't read); codex-acp and opencode in each prompt's response, which ws
// keeps in its record of their session. cursor-agent's ACP says nothing.
func (c *Claude) spends() Spends   { return Spends{Runs: true, Crew: true} }
func (p *Pi) spends() Spends       { return Spends{Runs: true, Crew: true} }
func (c *Cursor) spends() Spends   { return Spends{Runs: true} }
func (c *Codex) spends() Spends    { return Spends{Runs: true, Crew: true} }
func (o *OpenCode) spends() Spends { return Spends{Runs: true, Crew: true} }

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

// spentBy adds what's spent, by model and day, keeping the order each was
// first spent in.
type spentBy struct {
	at  map[[2]string]int // model, day: index in out
	out []Spent
}

func (s *spentBy) add(at time.Time, u Usage) {
	if u.Zero() {
		return
	}
	day := at.UTC().Format(time.DateOnly)
	k := [2]string{u.Model, day}
	i, ok := s.at[k]
	if !ok {
		if s.at == nil {
			s.at = map[[2]string]int{}
		}
		i = len(s.out)
		s.at[k] = i
		s.out = append(s.out, Spent{Day: day, Usage: Usage{Model: u.Model}})
	}
	s.out[i].Add(u)
}

// SessionSpend reads what a crew's session spent in lines of its session
// file (an ACP crew's: ws's record of it), by model and day. model is the
// one it's on where the harness doesn't say (an ACP agent may not). last
// is the message read last before these lines, and the message read last
// in them is returned, for the next read to carry on from: Claude Code
// writes a message's usage on each of the lines it writes the message in
// (one a block), the same on each and one after another, so a message is
// counted once, at its first line, even where a read ends among them.
func SessionSpend(harnessName, model string, lines [][]byte, last string) ([]Spent, string) {
	var s spentBy
	acpRecord := CapsOf(harnessName).Transcript == TranscriptACP
	for _, line := range lines {
		switch {
		case acpRecord:
			// ws's record: what a turn spent is on the line it stopped.
			if !bytes.Contains(line, []byte(`"usage"`)) {
				continue
			}
			var e acp.Event
			if json.Unmarshal(line, &e) != nil || e.Usage == nil {
				continue
			}
			s.add(e.At, acpUsage(*e.Usage, model))
		case harnessName == "claude":
			// Only an agent's message says what it spent; the rest (what
			// you typed, tool results, bookkeeping) needn't be read.
			if !bytes.Contains(line, []byte(`"assistant"`)) {
				continue
			}
			var e struct {
				Type      string    `json:"type"`
				Timestamp time.Time `json:"timestamp"`
				Message   struct {
					ID    string      `json:"id"`
					Model string      `json:"model"`
					Usage *claudeUsed `json:"usage"`
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
			s.add(e.Timestamp, e.Message.Usage.usage(e.Message.Model))
		case harnessName == "pi":
			// A message is one line: {"type":"message", "message": …}.
			if !bytes.Contains(line, []byte(`"usage"`)) {
				continue
			}
			var e struct {
				Type      string    `json:"type"`
				Timestamp time.Time `json:"timestamp"`
				Message   piMessage `json:"message"`
			}
			if json.Unmarshal(line, &e) != nil || e.Type != "message" {
				continue
			}
			if u, ok := e.Message.usage(); ok {
				s.add(e.Timestamp, u)
			}
		}
	}
	return s.out, last
}

// claudeUsed is a Claude Code message's usage.
type claudeUsed struct {
	Input      int64 `json:"input_tokens"`
	Output     int64 `json:"output_tokens"`
	CacheWrite int64 `json:"cache_creation_input_tokens"`
	CacheRead  int64 `json:"cache_read_input_tokens"`
}

func (c claudeUsed) usage(model string) Usage {
	return Usage{Model: model, Input: c.Input, Output: c.Output, CacheWrite: c.CacheWrite, CacheRead: c.CacheRead}
}

// piMessage is a message as pi writes it, in its session file and as a
// message_end in --mode json: an assistant's says what it spent, with its
// cost.
type piMessage struct {
	Role  string `json:"role"`
	Model string `json:"model"`
	Usage *struct {
		Input      int64 `json:"input"`
		Output     int64 `json:"output"`
		CacheRead  int64 `json:"cacheRead"`
		CacheWrite int64 `json:"cacheWrite"`
		Cost       struct {
			Total float64 `json:"total"`
		} `json:"cost"`
	} `json:"usage"`
}

func (m piMessage) usage() (Usage, bool) {
	if m.Role != "assistant" || m.Usage == nil {
		return Usage{}, false
	}
	u := m.Usage
	return Usage{Model: m.Model, Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite, Cost: u.Cost.Total}, true
}

// acpUsage is what an ACP agent said a turn spent, on model where it
// didn't say which.
func acpUsage(u acp.Usage, model string) Usage {
	if u.Model != "" {
		model = u.Model
	}
	return Usage{Model: model, Input: u.Input, Output: u.Output, CacheRead: u.CacheRead, CacheWrite: u.CacheWrite}
}

// addUsage adds u to us, on its model's.
func addUsage(us []Usage, u Usage) []Usage {
	if u.Zero() {
		return us
	}
	for i := range us {
		if us[i].Model == u.Model {
			us[i].Add(u)
			return us
		}
	}
	return append(us, u)
}
