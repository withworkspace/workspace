package nav

import "errors"

// Runtime runs a language's query over a file: all the engine asks of
// tree-sitter. The one built in is the official binding (runtime_cgo.go);
// a build without cgo has none, and navigation says so.
type Runtime interface {
	// Captures parses src with a grammar ("go", "typescript", "tsx"...) and
	// runs query over it: every match, in the order tree-sitter gives
	// them. broken is whether the parse had errors (what's before them
	// still holds).
	Captures(grammar, query string, src []byte) (matches []Match, broken bool, err error)
	// Has is whether the runtime can parse a grammar.
	Has(grammar string) bool
}

// Match is one match of a query: which pattern, and what it captured.
type Match struct {
	Pattern  int
	Captures []Capture
}

// Capture is a node a query named: the capture's name, the node's kind,
// and its bytes.
type Capture struct {
	Name, Kind string
	Start, End int
}

// ErrNoParser is the answer of a build without a parser for a grammar.
var ErrNoParser = errors.New("code navigation not available in this build")
