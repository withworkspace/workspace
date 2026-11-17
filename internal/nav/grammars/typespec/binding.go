//go:build cgo

// Package typespec is TypeSpec's tree-sitter grammar, brought in from
// github.com/happenslol/tree-sitter-typespec (MIT, LICENSE beside it) at
// 395bef1e1eb4dd18365401642beb534e8a244056: its own Go binding includes
// a file outside its module, so it can't be fetched as one. The parser
// is kept under src/, where Go doesn't compile C by itself (a build
// without cgo would refuse to).
package typespec

// #cgo CFLAGS: -std=c11 -fPIC -I${SRCDIR}/src
// #include "src/parser.c"
import "C"

import "unsafe"

// Language is the grammar.
func Language() unsafe.Pointer {
	return unsafe.Pointer(C.tree_sitter_typespec())
}
