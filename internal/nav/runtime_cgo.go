//go:build cgo

package nav

import (
	"fmt"
	"sync"
	"time"
	"unsafe"

	ts "github.com/tree-sitter/go-tree-sitter"
	tscs "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tspy "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tsts "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
	"github.com/withworkspace/workspace/internal/nav/grammars/typespec"
)

// grammars are the parsers this build has, by name.
var grammars = map[string]func() unsafe.Pointer{
	"c_sharp":    tscs.Language,
	"go":         tsgo.Language,
	"python":     tspy.Language,
	"typespec":   typespec.Language,
	"typescript": tsts.LanguageTypescript,
	"tsx":        tsts.LanguageTSX,
}

// parseTimeout is the most a parse takes before what it has is used.
const parseTimeout = 2 * time.Second

// Parsers is the official tree-sitter binding. Its parsers are pooled
// (one isn't safe to share), and its queries made once each.
type Parsers struct {
	mu      sync.Mutex
	langs   map[string]*ts.Language
	pools   map[string]*sync.Pool
	queries map[string]*ts.Query // by grammar and query text
}

// Builtin is the runtime this build has.
func Builtin() Runtime { return builtin }

var builtin = &Parsers{langs: map[string]*ts.Language{}, pools: map[string]*sync.Pool{}, queries: map[string]*ts.Query{}}

func (p *Parsers) Has(grammar string) bool { return grammars[grammar] != nil }

func (p *Parsers) lang(grammar string) (*ts.Language, *sync.Pool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if l := p.langs[grammar]; l != nil {
		return l, p.pools[grammar], nil
	}
	fn := grammars[grammar]
	if fn == nil {
		return nil, nil, fmt.Errorf("no grammar %q", grammar)
	}
	l := ts.NewLanguage(fn())
	pool := &sync.Pool{New: func() any {
		parser := ts.NewParser()
		_ = parser.SetLanguage(l)
		return parser
	}}
	p.langs[grammar], p.pools[grammar] = l, pool
	return l, pool, nil
}

func (p *Parsers) query(grammar, text string, l *ts.Language) (*ts.Query, error) {
	key := grammar + "\x00" + text
	p.mu.Lock()
	defer p.mu.Unlock()
	if q := p.queries[key]; q != nil {
		return q, nil
	}
	q, qerr := ts.NewQuery(l, text)
	if qerr != nil {
		return nil, fmt.Errorf("%s query: %s", grammar, qerr.Error())
	}
	p.queries[key] = q
	return q, nil
}

func (p *Parsers) Captures(grammar, query string, src []byte) ([]Match, bool, error) {
	l, pool, err := p.lang(grammar)
	if err != nil {
		return nil, false, err
	}
	q, err := p.query(grammar, query, l)
	if err != nil {
		return nil, false, err
	}
	parser := pool.Get().(*ts.Parser)
	defer pool.Put(parser)
	deadline := time.Now().Add(parseTimeout)
	tree := parser.ParseWithOptions(func(i int, _ ts.Point) []byte {
		if i >= len(src) {
			return nil
		}
		return src[i:]
	}, nil, &ts.ParseOptions{ProgressCallback: func(ts.ParseState) bool { return time.Now().After(deadline) }})
	if tree == nil {
		parser.Reset()
		return nil, false, fmt.Errorf("parsing took longer than %s", parseTimeout)
	}
	defer tree.Close()
	root := tree.RootNode()

	cursor := ts.NewQueryCursor()
	defer cursor.Close()
	names := q.CaptureNames()
	var out []Match
	matches := cursor.Matches(q, root, src)
	for m := matches.Next(); m != nil; m = matches.Next() {
		match := Match{Pattern: int(m.PatternIndex), Captures: make([]Capture, len(m.Captures))}
		for i, c := range m.Captures {
			match.Captures[i] = Capture{Name: names[c.Index], Kind: c.Node.Kind(), Start: int(c.Node.StartByte()), End: int(c.Node.EndByte())}
		}
		out = append(out, match)
	}
	return out, root.HasError(), nil
}
