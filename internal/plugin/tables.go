package plugin

import (
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/config"
)

// A plugin's config in tables: a key of type table holds named rows, each
// with the fields the plugin declared ([plugin.jira.queries.bugs]). ws
// reads them by what's declared: each row checked against the fields,
// yours laid over the team's (config.Load does that, table by table), and
// the rows in the order they were written. What's wrong is said, with
// where, and left out of what the plugin's given.

// RowEnabled is the one field every row has, whatever its table declares:
// enabled = false leaves the row out of what the plugin's given, which is
// how one of the team's is taken away in your own file.
const RowEnabled = "enabled"

// ConfigProblem is something wrong in a plugin's config: where it is, as its
// path (spaces.ram.lable), and what's wrong, in words.
type ConfigProblem struct {
	Path string
	Said string
}

func (p ConfigProblem) String() string { return p.Path + " " + p.Said }

// Row is one row of a table, as it's set: its name, and its fields'
// values, each as its field's type (a table's as its rows). It marshals
// as the protocol's ConfigRow.
type Row struct {
	Name   string         `json:"name"`
	Values map[string]any `json:"values"`
	// Off is a row that says enabled = false, and Left why one's left
	// out for being wrong (a field it needs isn't set): neither is given
	// to the plugin.
	Off  bool   `json:"-"`
	Left string `json:"-"`
}

// Given is whether the row is given to the plugin.
func (r Row) Given() bool { return !r.Off && r.Left == "" }

// Order is the names of the rows under a path of a plugin's config
// (queries; spaces, ram, custom_fields), in the order they were written.
type Order func(path ...string) []string

// OrderOf is the order a plugin's rows were written in, from the keys the
// config's files have under [plugin] (config.Config.PluginOrder): the
// team's first, then those only yours.
func OrderOf(keys [][]string, name string) Order {
	return func(path ...string) []string {
		prefix := append([]string{name}, path...)
		var out []string
		for _, k := range keys {
			if len(k) <= len(prefix) || !slices.Equal(k[:len(prefix)], prefix) {
				continue
			}
			if row := k[len(prefix)]; !slices.Contains(out, row) {
				out = append(out, row)
			}
		}
		return out
	}
}

// Shaped is a plugin's config as its registration reads it, with its
// secrets: Shape of what the files say.
func Shaped(root, name string, keys []ConfigKey) (map[string]any, []ConfigProblem, error) {
	cfg, _, err := config.Load(root)
	if err != nil {
		return nil, nil, err
	}
	out, problems := Shape(keys, cfg.Plugin[name], OrderOf(cfg.PluginOrder, name))
	for k, v := range readSecrets(root, name) {
		out[k] = v
	}
	return out, problems, nil
}

// Shape is a plugin's config (raw, as the files have it) as its keys
// declare it: a table as its rows ([]Row) in the order written, a map as
// text by text, and anything else as it's sent. What's wrong in a table
// or a map is said and left out: a field that isn't declared, a value of
// the wrong type, and a row without a field it needs, which is kept, and
// marked as left out. A key that isn't declared is passed as it is: the
// plugin may be from before it declared it.
func Shape(keys []ConfigKey, raw map[string]any, order Order) (map[string]any, []ConfigProblem) {
	out := map[string]any{}
	var problems []ConfigProblem
	declared := map[string]ConfigKey{}
	for _, k := range keys {
		declared[k.Key] = k
	}
	for name, v := range raw {
		k, ok := declared[name]
		switch {
		case ok && k.Type == protocol.TypeTable:
			rows, ps := shapeRows(k, v, []string{name}, order)
			problems = append(problems, ps...)
			out[name] = rows
		case ok && k.Type == protocol.TypeMap:
			m, said := mapValue(v)
			if said != "" {
				problems = append(problems, ConfigProblem{Path: name, Said: said})
				continue
			}
			out[name] = m
		default:
			out[name] = jsonValue(v)
		}
	}
	slices.SortStableFunc(problems, func(a, b ConfigProblem) int { return strings.Compare(a.Path, b.Path) })
	return out, problems
}

// shapeRows is a table's rows, from what the files have under it.
func shapeRows(table ConfigKey, v any, path []string, order Order) ([]Row, []ConfigProblem) {
	at := JoinPath(path)
	byName, ok := v.(map[string]any)
	if !ok {
		return nil, []ConfigProblem{{Path: at, Said: fmt.Sprintf("is a table of rows, each written as [plugin.….%s.<name>]: it's %s", at, typed(v))}}
	}
	names := order(path...)
	names = slices.DeleteFunc(names, func(n string) bool { _, ok := byName[n]; return !ok })
	var rest []string
	for n := range byName {
		if !slices.Contains(names, n) {
			rest = append(rest, n)
		}
	}
	slices.Sort(rest)
	names = append(names, rest...)

	fields := map[string]ConfigKey{}
	for _, f := range table.Fields {
		fields[f.Key] = f
	}
	var rows []Row
	var problems []ConfigProblem
	for _, name := range names {
		rowPath := append(slices.Clone(path), name)
		values, ok := byName[name].(map[string]any)
		if !ok {
			problems = append(problems, ConfigProblem{Path: JoinPath(rowPath), Said: fmt.Sprintf("is a row of %s, written as a table of its fields: it's %s", at, typed(byName[name]))})
			continue
		}
		row := Row{Name: name, Values: map[string]any{}}
		for _, key := range sortedKeys(values) {
			fv := values[key]
			fieldPath := append(slices.Clone(rowPath), key)
			f, ok := fields[key]
			switch {
			case key == RowEnabled && !ok:
				on, isBool := fv.(bool)
				if !isBool {
					problems = append(problems, ConfigProblem{Path: JoinPath(fieldPath), Said: "is true or false: it's " + typed(fv)})
					continue
				}
				row.Off = !on
			case !ok:
				problems = append(problems, ConfigProblem{Path: JoinPath(fieldPath), Said: fmt.Sprintf("isn't a field of %s (it has: %s)", at, strings.Join(fieldNames(table), ", "))})
			case f.Type == protocol.TypeTable:
				inner, ps := shapeRows(f, fv, fieldPath, order)
				problems = append(problems, ps...)
				row.Values[key] = inner
			default:
				value, said := fieldValue(f, fv)
				if said != "" {
					problems = append(problems, ConfigProblem{Path: JoinPath(fieldPath), Said: said})
					continue
				}
				row.Values[key] = value
			}
		}
		for _, f := range table.Fields {
			if _, set := row.Values[f.Key]; f.Required && f.Default == nil && !set && !row.Off {
				row.Left = "it has no " + f.Key
				problems = append(problems, ConfigProblem{Path: JoinPath(rowPath), Said: fmt.Sprintf("has no %s, which a row of %s needs: the row isn't given to the plugin", f.Key, at)})
				break
			}
		}
		rows = append(rows, row)
	}
	return rows, problems
}

// fieldValue is a value the files have for a field, as the field's type:
// or what's wrong with it.
func fieldValue(f ConfigKey, v any) (any, string) {
	switch f.Type {
	case protocol.TypeBool:
		if b, ok := v.(bool); ok {
			return b, ""
		}
		return nil, "is true or false: it's " + typed(v)
	case protocol.TypeNumber:
		switch n := v.(type) {
		case int64:
			return float64(n), ""
		case float64:
			return n, ""
		}
		return nil, "is a number: it's " + typed(v)
	case protocol.TypeList:
		switch l := v.(type) {
		case []string:
			return l, ""
		case []any:
			out := make([]string, 0, len(l))
			for _, e := range l {
				s, ok := e.(string)
				if !ok {
					return nil, "is a list of text: it has " + typed(e) + " in it"
				}
				out = append(out, s)
			}
			return out, ""
		}
		return nil, "is a list of text: it's " + typed(v)
	case protocol.TypeMap:
		return mapValue(v)
	case protocol.TypeEnum:
		s, ok := v.(string)
		if !ok || !slices.Contains(f.Options, s) {
			return nil, fmt.Sprintf("is one of: %s: it's %s", strings.Join(f.Options, ", "), typed(v))
		}
		return s, ""
	case protocol.TypeURL:
		s, ok := v.(string)
		if u, err := url.Parse(s); !ok || err != nil || u.Scheme == "" || u.Host == "" {
			return nil, "is an address, such as https://example.com: it's " + typed(v)
		}
		return s, ""
	}
	if s, ok := v.(string); ok {
		return s, ""
	}
	return nil, "is text: it's " + typed(v)
}

// mapValue is text by text, from an inline table or a table of its own.
func mapValue(v any) (any, string) {
	m, ok := v.(map[string]any)
	if !ok {
		return nil, "is text by text, written { \"a\" = \"b\" }: it's " + typed(v)
	}
	out := make(map[string]string, len(m))
	for k, e := range m {
		s, ok := e.(string)
		if !ok {
			return nil, fmt.Sprintf("is text by text: %s is %s", k, typed(e))
		}
		out[k] = s
	}
	return out, ""
}

// typed is what a value from the files is, for saying it's the wrong
// thing: the value itself where it's short enough to say.
func typed(v any) string {
	switch x := v.(type) {
	case string:
		return fmt.Sprintf("%q", x)
	case bool, int64, float64:
		return fmt.Sprint(x)
	case []any, []string:
		return "a list"
	case map[string]any:
		return "a table"
	case nil:
		return "nothing"
	}
	return fmt.Sprintf("%v", v)
}

func fieldNames(table ConfigKey) []string {
	out := make([]string, 0, len(table.Fields))
	for _, f := range table.Fields {
		out = append(out, f.Key)
	}
	return out
}

func sortedKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

// Wire is a shaped config as it's sent to the plugin: its tables' rows
// that are given, in order, and the rest as it is.
func Wire(shaped map[string]any) map[string]any {
	out := make(map[string]any, len(shaped))
	for k, v := range shaped {
		out[k] = wire(v)
	}
	return out
}

func wire(v any) any {
	rows, ok := v.([]Row)
	if !ok {
		return v
	}
	out := []Row{}
	for _, r := range rows {
		if !r.Given() {
			continue
		}
		values := make(map[string]any, len(r.Values))
		for k, fv := range r.Values {
			values[k] = wire(fv)
		}
		out = append(out, Row{Name: r.Name, Values: values})
	}
	return out
}

// HasTables is whether a plugin declares a table or a map: config ws
// shapes by what's declared, so sends again once it knows.
func HasTables(keys []ConfigKey) bool {
	return slices.ContainsFunc(keys, func(k ConfigKey) bool { return k.Type == protocol.TypeTable || k.Type == protocol.TypeMap })
}

// --- paths ---

// SplitPath is a key's path as it's typed (spaces.ram.label), a part at
// a time: a part with a dot or a space in it is quoted, as TOML has it
// (spaces.ram.custom_fields."Story Points".card).
func SplitPath(s string) ([]string, error) {
	var parts []string
	var part strings.Builder
	quoted, was := false, false
	for _, r := range s {
		switch {
		case r == '"':
			quoted, was = !quoted, true
		case r == '.' && !quoted:
			if part.Len() == 0 && !was {
				return nil, fmt.Errorf("%q has an empty part", s)
			}
			parts = append(parts, part.String())
			part.Reset()
			was = false
		default:
			part.WriteRune(r)
		}
	}
	if quoted {
		return nil, fmt.Errorf("%q has a quote that isn't closed", s)
	}
	if part.Len() == 0 && !was {
		return nil, fmt.Errorf("%q has an empty part", s)
	}
	return append(parts, part.String()), nil
}

// JoinPath is a path as it's typed: each part bare where it can be, and
// quoted where it has a dot, a space or anything else a bare key hasn't.
func JoinPath(parts []string) string {
	out := make([]string, len(parts))
	for i, p := range parts {
		bare := p != ""
		for _, r := range p {
			if !(r == '-' || r == '_' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z') {
				bare = false
			}
		}
		if bare {
			out[i] = p
		} else {
			out[i] = `"` + p + `"`
		}
	}
	return strings.Join(out, ".")
}

// Place is where a path leads in a plugin's config, by what it declares:
// the key or field it sets (Key), or, for a path that names a row and no
// more, the row's table (Table). Entry is the path names one entry of a
// map (the last part is its name).
type Place struct {
	Path  []string
	Key   ConfigKey
	Table ConfigKey
	Row   bool
	Entry bool
}

// Find is where path leads: a key (site), a row (spaces.ram), a row's
// field (spaces.ram.label), a row's row (spaces.ram.custom_fields.Team),
// its field, or an entry of a map (spaces.ram.columns."In Review").
func Find(keys []ConfigKey, path []string) (Place, error) {
	at := Place{Path: path}
	fields, of := keys, "the plugin's config"
	for i := 0; i < len(path); {
		var k ConfigKey
		found := false
		for _, f := range fields {
			if f.Key == path[i] {
				k, found = f, true
			}
		}
		here := JoinPath(path[:i+1])
		if path[i] == RowEnabled && !found && i > 0 {
			// Every row has it: whether it's given to the plugin.
			if i != len(path)-1 {
				return at, fmt.Errorf("%s is true or false, and has nothing in it", here)
			}
			at.Key = ConfigKey{Key: RowEnabled, Type: protocol.TypeBool, Title: "Given to the plugin"}
			return at, nil
		}
		if !found {
			names := make([]string, 0, len(fields))
			for _, f := range fields {
				names = append(names, f.Key)
			}
			return at, fmt.Errorf("%s isn't in %s (it has: %s)", here, of, strings.Join(names, ", "))
		}
		rest := len(path) - i - 1
		switch {
		case k.Type == protocol.TypeTable && rest == 0:
			return at, fmt.Errorf("%s is a table: name a row and a field of it, as %s.<%s>.<field>", here, here, rowWord(k))
		case k.Type == protocol.TypeTable:
			if path[i+1] == "" {
				return at, fmt.Errorf("%s: a row needs a name", here)
			}
			if rest == 1 {
				at.Table, at.Row = k, true
				return at, nil
			}
			fields, of = k.Fields, JoinPath(path[:i+2])
			i += 2
			continue
		case k.Type == protocol.TypeMap && rest == 1:
			at.Key, at.Entry = k, true
			return at, nil
		case rest > 0:
			return at, fmt.Errorf("%s is %s, and has nothing in it to name", here, typeWords(k.Type))
		}
		at.Key = k
		return at, nil
	}
	return at, errors.New("name a key")
}

func rowWord(table ConfigKey) string {
	if table.Row != "" {
		return strings.ToLower(table.Row)
	}
	return "name"
}

func typeWords(t string) string {
	switch t {
	case protocol.TypeBool:
		return "true or false"
	case protocol.TypeNumber:
		return "a number"
	case protocol.TypeList:
		return "a list"
	case protocol.TypeMap:
		return "text by text"
	}
	return "text"
}

// SetAt sets what a path leads to, in your ws.local.toml: a row's field,
// an entry of a map, or a key (which SetValue does). A nil value unsets
// it, and with a path that names a row, takes the row out of your file.
func SetAt(root, name string, at Place, value any) error {
	if len(at.Path) == 1 && !at.Entry {
		return SetValue(root, name, at.Key, value)
	}
	set := func(local *config.LocalConfig) {
		if value == nil {
			unsetIn(local.Plugin[name], at.Path)
			return
		}
		if local.Plugin == nil {
			local.Plugin = map[string]map[string]any{}
		}
		if local.Plugin[name] == nil {
			local.Plugin[name] = map[string]any{}
		}
		in := local.Plugin[name]
		for _, part := range at.Path[:len(at.Path)-1] {
			next, ok := in[part].(map[string]any)
			if !ok {
				next = map[string]any{}
				in[part] = next
			}
			in = next
		}
		in[at.Path[len(at.Path)-1]] = tomlValue(value)
	}
	err := config.EditLocal(root, set)
	if errors.Is(err, config.ErrHandEdit) {
		err = config.UpdateLocal(root, set)
	}
	return err
}

// unsetIn takes what path leads to out of in, and each table it leaves
// empty on the way back up.
func unsetIn(in map[string]any, path []string) {
	if in == nil || len(path) == 0 {
		return
	}
	if len(path) == 1 {
		delete(in, path[0])
		return
	}
	next, ok := in[path[0]].(map[string]any)
	if !ok {
		return
	}
	unsetIn(next, path[1:])
	if len(next) == 0 {
		delete(in, path[0])
	}
}

// Yours is whether your own ws.local.toml has something at a path of a
// plugin's config: a row that's only the team's isn't yours to take out.
func Yours(root, name string, path []string) bool {
	local, err := config.ReadLocal(root)
	if err != nil {
		return false
	}
	var in any = local.Plugin[name]
	for _, part := range path {
		m, ok := in.(map[string]any)
		if !ok {
			return false
		}
		if in, ok = m[part]; !ok {
			return false
		}
	}
	return true
}
