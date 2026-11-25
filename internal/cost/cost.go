// Package cost measures what the daemon's work costs, when it's asked to
// (WS_MEASURE set where the daemon starts): for each web request, how
// long it took and what it waited on in the meantime (git, SQL, gh, RPC to
// plugins and agents, HTTP out), and every ten seconds what was done
// behind the requests, each a line of .ws/measure.jsonl.
//
// What a request waited on is what its own goroutine waited on: work it
// hands to another goroutine and doesn't wait for isn't its cost, which is
// the point of looking behind a read. Work it waits for on another
// goroutine shows in where it waited (Wrap samples the stacks of requests
// that run long) and in what's left of its time once the rest is counted.
//
// Off, which it is unless asked, each hook is one atomic load.
package cost

import (
	"bytes"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Kind is a kind of thing waited on.
type Kind int

const (
	Git      Kind = iota // a git process, start to exit
	SQLRead              // a read transaction, begin to end
	SQLWrite             // a write transaction, its turn taken to commit
	SQLTurn              // waiting for the turn to write
	GH                   // the gh CLI
	RPC                  // a JSON-RPC call: a plugin, an agent
	HTTP                 // an HTTP request out
	kinds
)

var kindNames = [kinds]string{"git", "sql_read", "sql_write", "sql_turn", "gh", "rpc", "http"}

func (k Kind) String() string { return kindNames[k] }

var on atomic.Bool

// On is whether costs are being measured.
func On() bool { return on.Load() }

// tally is how many and how long.
type tally struct {
	N   int64
	Dur time.Duration
}

// counts is a tally of each kind, and statements run, and of each by
// who asked: "kind what · function" (see caller).
type counts struct {
	t     [kinds]tally
	stmts int64
	by    map[string]*tally
}

func (c *counts) add(k Kind, d time.Duration, who string) {
	c.t[k].N++
	c.t[k].Dur += d
	if who == "" {
		return
	}
	if c.by == nil {
		c.by = map[string]*tally{}
	}
	t := c.by[who]
	if t == nil {
		t = &tally{}
		c.by[who] = t
	}
	t.N++
	t.Dur += d
}

// req is one request being served, while it is.
type req struct {
	start time.Time
	mu    sync.Mutex
	c     counts
	// waits is what its goroutine was seen doing, as samples of
	// "state · function": a sample every sampleEvery while it runs past
	// sampleAfter.
	waits   map[string]int
	samples int
}

var (
	mu   sync.Mutex
	live = map[uint64]*req{} // by goroutine
	// all is everything counted; served is what requests' own goroutines
	// did. The difference is what was done behind them.
	all, served counts
)

// Begin starts timing one k: the func it returns ends it. It's counted
// to the request whose goroutine this is, if it's one.
func Begin(k Kind) func() { return BeginWhat(k, "") }

// BeginWhat is Begin, saying what of k it is (git's subcommand, say).
func BeginWhat(k Kind, what string) func() {
	if !on.Load() {
		return nop
	}
	start := time.Now()
	r := current()
	who := k.String()
	if what != "" {
		who += " " + what
	}
	who += " · " + caller()
	return func() {
		d := time.Since(start)
		mu.Lock()
		all.add(k, d, who)
		if r != nil {
			served.add(k, d, who)
		}
		mu.Unlock()
		if r != nil {
			r.mu.Lock()
			r.c.add(k, d, who)
			r.mu.Unlock()
		}
	}
}

// plumbing is the packages that run what's counted for others: who
// asked is the first function outside them.
var plumbing = []string{"internal/cost.", "internal/git.", "internal/store.", "internal/acp.", "internal/github.ghExec"}

// caller is the function that asked for what's being counted: ws's own
// innermost one outside the plumbing.
func caller() string {
	var pcs [24]uintptr
	frames := runtime.CallersFrames(pcs[:runtime.Callers(3, pcs[:])])
	for {
		f, more := frames.Next()
		if fn, ok := strings.CutPrefix(f.Function, module+"internal/"); ok && !isPlumbing(fn) {
			return fn
		}
		if !more {
			return "?"
		}
	}
}

func isPlumbing(fn string) bool {
	for _, p := range plumbing {
		if strings.HasPrefix(fn, strings.TrimPrefix(p, "internal/")) {
			return true
		}
	}
	return false
}

// Stmt counts one SQL statement.
func Stmt() {
	if !on.Load() {
		return
	}
	r := current()
	mu.Lock()
	all.stmts++
	if r != nil {
		served.stmts++
	}
	mu.Unlock()
	if r != nil {
		r.mu.Lock()
		r.c.stmts++
		r.mu.Unlock()
	}
}

func nop() {}

// current is the request this goroutine is serving, or nil.
func current() *req {
	id := goid()
	mu.Lock()
	r := live[id]
	mu.Unlock()
	return r
}

// goid is this goroutine's id, as its stack's first line says it. Go keeps
// it to itself otherwise; it's only asked for while measuring.
func goid() uint64 {
	var buf [64]byte
	b := buf[:runtime.Stack(buf[:], false)]
	b = bytes.TrimPrefix(b, []byte("goroutine "))
	if i := bytes.IndexByte(b, ' '); i > 0 {
		b = b[:i]
	}
	n, _ := strconv.ParseUint(string(b), 10, 64)
	return n
}
