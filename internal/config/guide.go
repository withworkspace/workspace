package config

import (
	_ "embed"
	"os"
	"slices"
	"strings"

	"github.com/BurntSushi/toml"
)

// The guide to configuring ws, for an agent asked to help set a
// workspace up (ws show config): what each setting is for, the signs in a
// repo that one's worth setting, and every key ws reads. It's held to
// Keys by its test, so a key added here is a key added to the guide.

//go:embed guide.md
var guide string

// Guide is the guide to configuring ws, in markdown.
func Guide() string { return guide }

// Said is what a file says, as dotted paths of its tables and keys, in
// order, read from the file: nil where there's no such file. A table
// that's only a parent of others ([run], [run.service]) isn't said on its
// own.
func Said(path string) ([]string, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, nil
	}
	var v map[string]any
	md, err := toml.DecodeFile(path, &v)
	if err != nil {
		return nil, err
	}
	out := []string{}
	for _, k := range md.Keys() {
		out = append(out, k.String())
	}
	return out, nil
}

// Tops is the top-level tables and keys of what a file says (as Said has
// it), each once, in the order they're first said.
func Tops(said []string) []string {
	var out []string
	for _, k := range said {
		top, _, _ := strings.Cut(k, ".")
		if !slices.Contains(out, top) {
			out = append(out, top)
		}
	}
	return out
}
