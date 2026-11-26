package cost

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Env is what asks for costs to be measured, set where the daemon starts.
const Env = "WS_MEASURE"

// TagHeader tags a request's line, for whoever's measuring to find it.
const TagHeader = "X-Ws-Measure"

const (
	sampleAfter = 20 * time.Millisecond // a request running past this has its stack sampled
	sampleEvery = 5 * time.Millisecond
	window      = 10 * time.Second // what was done behind requests, said this often
)

var (
	outMu sync.Mutex
	out   *os.File
)

// Start measures from now on if Env is set, writing to root's
// .ws/measure.jsonl, and says whether it is. Each Start after the first
// does nothing more.
func Start(root string) bool {
	if os.Getenv(Env) == "" {
		return false
	}
	outMu.Lock()
	defer outMu.Unlock()
	if out != nil {
		return true
	}
	f, err := os.OpenFile(filepath.Join(root, ".ws", "measure.jsonl"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return false
	}
	out = f
	http.DefaultTransport = transport{http.DefaultTransport}
	on.Store(true)
	go sample()
	go behind()
	return true
}

func write(v any) {
	if l, ok := v.(Line); ok {
		outMu.Lock()
		each := observer
		outMu.Unlock()
		if each != nil {
			each(l)
		}
	}
	outMu.Lock()
	defer outMu.Unlock()
	if out == nil {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	out.Write(append(b, '\n'))
}

// observer, if set, is told each request's line (Observe).
var observer func(Line)

// Observe measures each request from now on, telling each its line, with
// or without Env: what a test asks of every route. Handlers made before
// it (Wrap) aren't measured. stop stops it, and measuring with it, unless
// Env had it measured anyway.
func Observe(each func(Line)) (stop func()) {
	outMu.Lock()
	observer = each
	byEnv := out != nil
	outMu.Unlock()
	on.Store(true)
	return func() {
		outMu.Lock()
		observer = nil
		outMu.Unlock()
		if !byEnv {
			on.Store(false)
		}
	}
}

// Tally is one kind's count and time, as written.
type Tally struct {
	N  int64   `json:"n"`
	Ms float64 `json:"ms"`
}

func tallies(c counts) map[string]Tally {
	m := map[string]Tally{}
	for k := range kinds {
		if c.t[k].N > 0 {
			m[Kind(k).String()] = Tally{c.t[k].N, ms(c.t[k].Dur)}
		}
	}
	return m
}

// whom is the costs by who asked, most time first, the first few.
func whom(c counts, most int) []By {
	var bs []By
	for who, t := range c.by {
		bs = append(bs, By{who, t.N, ms(t.Dur)})
	}
	sort.Slice(bs, func(i, j int) bool { return bs[i].Ms > bs[j].Ms })
	if len(bs) > most {
		bs = bs[:most]
	}
	return bs
}

// By is what one asker cost: "kind what · function", how many, how long.
type By struct {
	Who string  `json:"who"`
	N   int64   `json:"n"`
	Ms  float64 `json:"ms"`
}

func ms(d time.Duration) float64 { return float64(d.Microseconds()) / 1000 }

// Line is one request's cost.
type Line struct {
	At      time.Time        `json:"at"`
	Method  string           `json:"method"`
	Pattern string           `json:"pattern"`
	Path    string           `json:"path"`
	Status  int              `json:"status"`
	Bytes   int64            `json:"bytes"`
	Ms      float64          `json:"ms"`
	FirstMs float64          `json:"first_ms"` // to the first byte (or the head) written
	Stream  bool             `json:"stream,omitempty"`
	Costs   map[string]Tally `json:"costs,omitempty"`
	Stmts   int64            `json:"stmts,omitempty"`
	// Rest is the time not spent on any of Costs: the handler's own
	// work, and waiting on other goroutines (Waits says which).
	RestMs float64 `json:"rest_ms"`
	// Waits is what its goroutine was seen doing, for a request that ran
	// past sampleAfter: "state · function" → milliseconds, about.
	Waits map[string]float64 `json:"waits,omitempty"`
	// Tag is what the request was tagged with by whoever's measuring
	// (TagHeader): the page or the round it was asked for in.
	Tag string `json:"tag,omitempty"`
	// By is who asked for its costs, the most time first.
	By []By `json:"by,omitempty"`
	// Gits is every git process started while it ran, by anyone.
	Gits int64 `json:"gits_anyone,omitempty"`
}

// Wrap measures each request mux serves, when measuring. The pattern
// that served it is mux's.
func Wrap(mux *http.ServeMux) http.Handler {
	if !On() {
		return mux
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "" { // a socket: it lasts as long as the page
			mux.ServeHTTP(w, r)
			return
		}
		_, pattern := mux.Handler(r)
		id := goid()
		q := &req{start: time.Now(), waits: map[string]int{}}
		gits := gitsStarted()
		mu.Lock()
		live[id] = q
		mu.Unlock()
		rw := &writer{ResponseWriter: w, start: q.start}
		defer func() {
			mu.Lock()
			delete(live, id)
			mu.Unlock()
			took := time.Since(q.start)
			q.mu.Lock()
			defer q.mu.Unlock()
			line := Line{
				At: q.start, Method: r.Method, Pattern: pattern, Path: r.URL.RequestURI(),
				Status: rw.status, Bytes: rw.bytes, Ms: ms(took), FirstMs: ms(rw.first), Stream: rw.flushed,
				Costs: tallies(q.c), Stmts: q.c.stmts, Gits: gitsStarted() - gits, Tag: r.Header.Get(TagHeader), By: whom(q.c, 8),
			}
			if line.Status == 0 {
				line.Status = http.StatusOK
			}
			rest := took
			for k := range kinds {
				rest -= q.c.t[k].Dur
			}
			line.RestMs = ms(max(rest, 0))
			if q.samples > 0 {
				line.Waits = map[string]float64{}
				for what, n := range q.waits {
					line.Waits[what] = ms(time.Duration(n) * sampleEvery)
				}
			}
			write(line)
		}()
		mux.ServeHTTP(rw, r)
	})
}

// gitsStarted is set by package git, which this one can't import.
var gitsStarted = func() int64 { return 0 }

// CountGits says where to read how many git processes have started.
func CountGits(f func() int64) { gitsStarted = f }

// writer notes what was written and when it began.
type writer struct {
	http.ResponseWriter
	start   time.Time
	first   time.Duration
	status  int
	bytes   int64
	flushed bool
}

func (w *writer) began() {
	if w.first == 0 {
		w.first = time.Since(w.start)
	}
}

func (w *writer) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.began()
	w.ResponseWriter.WriteHeader(code)
}

func (w *writer) Write(b []byte) (int, error) {
	w.began()
	n, err := w.ResponseWriter.Write(b)
	w.bytes += int64(n)
	return n, err
}

func (w *writer) Flush() {
	w.flushed = true
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *writer) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("cost: can't hijack")
}

func (w *writer) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// sample looks at the stacks of requests that have run past sampleAfter,
// every sampleEvery, and notes what each was doing.
func sample() {
	buf := make([]byte, 1<<20)
	for range time.Tick(sampleEvery) {
		mu.Lock()
		long := map[uint64]*req{}
		for id, q := range live {
			if time.Since(q.start) > sampleAfter {
				long[id] = q
			}
		}
		mu.Unlock()
		if len(long) == 0 {
			continue
		}
		n := runtime.Stack(buf, true)
		for n == len(buf) {
			buf = make([]byte, 2*len(buf))
			n = runtime.Stack(buf, true)
		}
		for _, g := range bytes.Split(buf[:n], []byte("\n\n")) {
			id, what, ok := doing(g)
			if !ok {
				continue
			}
			if q := long[id]; q != nil {
				q.mu.Lock()
				q.waits[what]++
				q.samples++
				q.mu.Unlock()
			}
		}
	}
}

const module = "github.com/withworkspace/workspace/"

// doing reads one goroutine's stack: its id, and "state · function", the
// function being ws's own innermost one outside this package.
func doing(g []byte) (id uint64, what string, ok bool) {
	head, rest, _ := bytes.Cut(g, []byte("\n"))
	// goroutine 123 [semacquire, 2 minutes]:
	h := strings.TrimPrefix(string(head), "goroutine ")
	sp := strings.IndexByte(h, ' ')
	if sp < 0 {
		return 0, "", false
	}
	id, err := strconv.ParseUint(h[:sp], 10, 64)
	if err != nil {
		return 0, "", false
	}
	state := strings.TrimSuffix(strings.TrimPrefix(h[sp+1:], "["), "]:")
	if i := strings.IndexByte(state, ','); i >= 0 {
		state = state[:i]
	}
	fn := "?"
	for _, line := range strings.Split(string(rest), "\n") {
		if strings.HasPrefix(line, "\t") || !strings.HasPrefix(line, module) || own(line) {
			continue
		}
		fn = strings.TrimPrefix(line, module+"internal/")
		if i := strings.LastIndexByte(fn, '('); i > 0 {
			fn = fn[:i]
		}
		break
	}
	return id, state + " · " + fn, true
}

// own is whether a frame is this package's measuring, not what's measured.
func own(frame string) bool {
	for _, f := range []string{"Begin", "Wrap", "(*writer)", "transport"} {
		if strings.HasPrefix(frame, module+"internal/cost."+f) {
			return true
		}
	}
	return false
}

// behind writes, each window, what was done that no request's goroutine
// did: the daemon's own work, and what requests left to be done behind
// them.
func behind() {
	var was counts
	for range time.Tick(window) {
		mu.Lock()
		now := minus(all, served)
		mu.Unlock()
		d := minus(now, was)
		was = now
		var gs runtime.MemStats
		runtime.ReadMemStats(&gs)
		write(struct {
			At         time.Time        `json:"at"`
			Behind     float64          `json:"behind_s"`
			Costs      map[string]Tally `json:"costs,omitempty"`
			Stmts      int64            `json:"stmts,omitempty"`
			By         []By             `json:"by,omitempty"`
			Goroutines int              `json:"goroutines"`
			HeapMB     float64          `json:"heap_mb"`
		}{time.Now(), window.Seconds(), tallies(d), d.stmts, whom(d, 30), runtime.NumGoroutine(), float64(gs.HeapAlloc) / (1 << 20)})
	}
}

// transport counts each request out that goes by http.DefaultTransport.
type transport struct{ http.RoundTripper }

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) {
	defer Begin(HTTP)()
	return t.RoundTripper.RoundTrip(r)
}

// minus is a's counts less b's.
func minus(a, b counts) counts {
	d := counts{stmts: a.stmts - b.stmts, by: map[string]*tally{}}
	for k := range kinds {
		d.t[k] = tally{a.t[k].N - b.t[k].N, a.t[k].Dur - b.t[k].Dur}
	}
	for who, t := range a.by {
		left := *t
		if o := b.by[who]; o != nil {
			left.N -= o.N
			left.Dur -= o.Dur
		}
		if left.N > 0 {
			d.by[who] = &left
		}
	}
	return d
}
