package config

import (
	"reflect"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// What each of the three files can say, as dotted paths: what the guide
// (guide.md) is held to. A table keyed by a name the file chooses
// ([repos.frontend], [run.service.web]) is <name> in its path.

// KeyFile is one of the files a key can be in.
type KeyFile string

const (
	InTeam  KeyFile = FileName      // ws.toml
	InLocal KeyFile = LocalFileName // ws.local.toml
	InRepo  KeyFile = RepoFileName  // ws.repo.toml
)

// Keys is every key ws reads, each with the files it's read from, by its
// dotted path (crew.levels.rote, repos.<name>.aliases).
func Keys() map[string][]KeyFile {
	out := map[string][]KeyFile{}
	for _, f := range []struct {
		file KeyFile
		t    reflect.Type
	}{
		{InTeam, reflect.TypeOf(Config{})},
		{InLocal, reflect.TypeOf(LocalConfig{})},
		{InRepo, reflect.TypeOf(RepoFileConfig{})},
	} {
		walkKeys(f.t, "", func(path string) { out[path] = append(out[path], f.file) })
	}
	return out
}

// SortedKeys is Keys' paths in order.
func SortedKeys() []string {
	keys := Keys()
	out := make([]string, 0, len(keys))
	for k := range keys {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

var unmarshaler = reflect.TypeOf((*toml.Unmarshaler)(nil)).Elem()

func walkKeys(t reflect.Type, prefix string, leaf func(string)) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	for i := range t.NumField() {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag, _, _ := strings.Cut(f.Tag.Get("toml"), ",")
		if tag == "-" {
			continue
		}
		if f.Anonymous && tag == "" { // [conversation]'s AgentConfig: its keys are the table's
			walkKeys(f.Type, prefix, leaf)
			continue
		}
		name := tag
		if name == "" {
			name = f.Name
		}
		walkValue(f.Type, prefix+name, leaf)
	}
}

func walkValue(t reflect.Type, path string, leaf func(string)) {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if reflect.PointerTo(t).Implements(unmarshaler) {
		leaf(path)
		return
	}
	switch t.Kind() {
	case reflect.Struct:
		walkKeys(t, path+".", leaf)
	case reflect.Map:
		e := t.Elem()
		for e.Kind() == reflect.Pointer {
			e = e.Elem()
		}
		if e.Kind() == reflect.Struct && !reflect.PointerTo(e).Implements(unmarshaler) {
			walkKeys(e, path+".<name>.", leaf)
			return
		}
		leaf(path)
	default:
		leaf(path)
	}
}
