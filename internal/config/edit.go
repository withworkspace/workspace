package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

// Writing ws.local.toml so what's in it stays as it was written: a change
// edits the lines of the keys whose values changed, and nothing else, so
// your comments, order and spacing survive ws's own writes (a lift boards
// a capsule) and the settings page's.

// ErrHandEdit is a change the file is written in a way ws can't make
// safely (a key in an inline table, or as a dotted key where ws would add
// a table): it's left as it is, to be edited by hand.
var ErrHandEdit = errors.New("ws.local.toml is written in a way ws can't change safely: edit it by hand")

// Turn runs fn, a write of root's ws.local.toml, in its turn. ws writes
// the file from more than one process (a lift, ws silo point, the Settings
// page in the daemon), and the workspace's daemon has them take turns: cli
// sets this to a turn of the records' writer, which every process's writes
// wait for in the order they asked. Left as it is, writes take turns in
// this process only.
var Turn = func(root string, fn func() error) error {
	localTurn.Lock()
	defer localTurn.Unlock()
	return fn()
}

var localTurn sync.Mutex

// Layers are what each file says, apart: the team's ws.toml, and your
// ws.local.toml (empty where there's none). Load merges them; the
// settings page shows where each value comes from.
func Layers(root string) (*Config, LocalConfig, error) {
	team, err := Parse(filepath.Join(root, FileName))
	if err != nil {
		return nil, LocalConfig{}, err
	}
	local, err := ReadLocal(root)
	if err != nil {
		return nil, LocalConfig{}, err
	}
	return team, local, nil
}

// UpdateLocal changes ws.local.toml: fn is given what it says, and what fn
// changes is written back, key by key, keeping the rest of the file as it
// was: comments, order, spacing, and what ws doesn't read. Where the file
// is written in a way that can't be edited key by key (ErrHandEdit), it's
// written whole from what was read, as before, which loses its comments:
// ws's own writes (a lift) mustn't fail over how the file's laid out.
func UpdateLocal(root string, fn func(*LocalConfig)) error {
	return Turn(root, func() error { return updateLocal(root, fn, true) })
}

// StartLocal is UpdateLocal for a workspace being made (ws init): nothing
// else writes its ws.local.toml yet, and it has no daemon to take turns
// in.
func StartLocal(root string, fn func(*LocalConfig)) error {
	return updateLocal(root, fn, true)
}

// EditLocal is UpdateLocal for a change someone asked for (the settings
// page): one that can't be made key by key fails with ErrHandEdit, and
// the file is left as it was.
func EditLocal(root string, fn func(*LocalConfig)) error {
	return Turn(root, func() error { return updateLocal(root, fn, false) })
}

func updateLocal(root string, fn func(*LocalConfig), rewrite bool) error {
	path := filepath.Join(root, LocalFileName)
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	text := string(data)
	var local LocalConfig
	md, err := toml.Decode(text, &local)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", LocalFileName, err)
	}
	before, err := leaves(local)
	if err != nil {
		return err
	}
	fn(&local)
	after, err := leaves(local)
	if err != nil {
		return err
	}

	out, err := editLeaves(text, before, after)
	if err == nil {
		err = sameLeaves(out, after)
	}
	if err != nil {
		if !rewrite {
			return err
		}
		if out, err = rewriteLocal(text, md, local); err != nil {
			return err
		}
	}
	if out == text {
		return nil
	}
	// Written in place, not renamed over: the silo watcher watches the file
	// itself (workspace.SiloWatcher), and a rename would lose it.
	return os.WriteFile(path, []byte(out), 0o644)
}

// rewriteLocal is the file written whole from local, with what ws doesn't
// read kept as it was: the way ws.local.toml was written before it was
// edited key by key, for a file that can't be.
func rewriteLocal(text string, md toml.MetaData, local LocalConfig) (string, error) {
	var out bytes.Buffer
	if err := toml.NewEncoder(&out).Encode(local); err != nil {
		return "", err
	}
	if kept := undecoded(text, md); kept != nil {
		var known map[string]any
		if _, err := toml.Decode(out.String(), &known); err != nil {
			return "", err
		}
		addMissing(known, kept)
		out.Reset()
		if err := toml.NewEncoder(&out).Encode(known); err != nil {
			return "", err
		}
	}
	return out.String(), nil
}

// leaf is a value in a TOML document that isn't a table, at its path
// (the tables it's in, then its key).
type leaf struct {
	path  []string
	value any
}

// leaves are local's values, as it would be written, by path
// ("crew.harness"). An empty string, a zero and an empty list are left
// out: in ws's config each means unset (the default), and the encoder
// writes some of them anyway (a field without omitempty, a number).
func leaves(local LocalConfig) (map[string]leaf, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(local); err != nil {
		return nil, err
	}
	var doc map[string]any
	if _, err := toml.Decode(buf.String(), &doc); err != nil {
		return nil, err
	}
	out := map[string]leaf{}
	var walk func(prefix []string, m map[string]any)
	walk = func(prefix []string, m map[string]any) {
		for k, v := range m {
			p := append(append([]string(nil), prefix...), k)
			switch x := v.(type) {
			case map[string]any:
				walk(p, x)
				continue
			case string:
				if x == "" {
					continue
				}
			case int64:
				if x == 0 {
					continue
				}
			case []any:
				if len(x) == 0 {
					continue
				}
			}
			out[pathKey(p)] = leaf{path: p, value: v}
		}
	}
	walk(nil, doc)
	return out, nil
}

// pathKey is a path as one string, for a map key.
func pathKey(p []string) string { return strings.Join(p, "\x00") }

// editLeaves is text with each value that differs between before and after
// set, and each one after hasn't removed.
func editLeaves(text string, before, after map[string]leaf) (string, error) {
	var keys []string
	for k := range after {
		keys = append(keys, k)
	}
	for k := range before {
		if _, ok := after[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	d := parseDoc(text)
	// What's set first, then what's removed: a table that loses a key as
	// it gains another stays where it is, rather than going and coming
	// back at the end.
	for _, k := range keys {
		b, had := before[k]
		a, has := after[k]
		if has && !(had && reflect.DeepEqual(a.value, b.value)) {
			if err := d.set(a.path[:len(a.path)-1], a.path[len(a.path)-1], a.value); err != nil {
				return "", err
			}
		}
	}
	for _, k := range keys {
		if b, ok := before[k]; ok {
			if _, has := after[k]; !has {
				if err := d.unset(b.path[:len(b.path)-1], b.path[len(b.path)-1]); err != nil {
					return "", err
				}
			}
		}
	}
	return d.String(), nil
}

// sameLeaves checks that text says what want does: that the edit came out
// as meant, and nothing it didn't mean to change changed.
func sameLeaves(text string, want map[string]leaf) error {
	var local LocalConfig
	if _, err := toml.Decode(text, &local); err != nil {
		return fmt.Errorf("%w (%v)", ErrHandEdit, err)
	}
	got, err := leaves(local)
	if err != nil {
		return err
	}
	if len(got) != len(want) {
		return ErrHandEdit
	}
	for k, w := range want {
		if g, ok := got[k]; !ok || !reflect.DeepEqual(g.value, w.value) {
			return ErrHandEdit
		}
	}
	return nil
}

// tomlDoc is a TOML document as lines, edited a key at a time.
type tomlDoc struct {
	lines []string
}

func parseDoc(text string) *tomlDoc {
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return &tomlDoc{}
	}
	return &tomlDoc{lines: strings.Split(text, "\n")}
}

func (d *tomlDoc) String() string {
	if len(d.lines) == 0 {
		return ""
	}
	return strings.Join(d.lines, "\n") + "\n"
}

var (
	headerLine = regexp.MustCompile(`^\s*\[\s*([^\[\]]+?)\s*\]\s*(#.*)?$`)
	arrayLine  = regexp.MustCompile(`^\s*\[\[`)
	bareKey    = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

// section is where a table's lines are: its header (-1 for the top, before
// any header), and the line after its last (the next header's, or the end).
type section struct {
	header, end int
}

// sections are the document's tables, by path, in order.
func (d *tomlDoc) sections() (map[string]section, []string, error) {
	out := map[string]section{}
	var order []string
	cur, start := pathKey(nil), -1
	flush := func(end int) {
		out[cur] = section{header: start, end: end}
		order = append(order, cur)
	}
	for i := 0; i < len(d.lines); i++ {
		line := d.lines[i]
		if arrayLine.MatchString(line) {
			// An array of tables: ws writes none, and won't edit one.
			flush(i)
			cur, start = "\x01array", i
			continue
		}
		if m := headerLine.FindStringSubmatch(line); m != nil {
			path, err := splitKey(m[1])
			if err != nil {
				return nil, nil, err
			}
			flush(i)
			cur, start = pathKey(path), i
			continue
		}
		if k, ok := lineKey(line); ok {
			i = valueEnd(d.lines, i, k)
		}
	}
	flush(len(d.lines))
	return out, order, nil
}

// splitKey splits a dotted key ("team.people.ana", `repos."my.repo"`).
func splitKey(s string) ([]string, error) {
	var out []string
	s = strings.TrimSpace(s)
	for s != "" {
		var part string
		switch s[0] {
		case '"':
			end := 1
			for end < len(s) && s[end] != '"' {
				if s[end] == '\\' {
					end++
				}
				end++
			}
			if end >= len(s) {
				return nil, ErrHandEdit
			}
			var v string
			if _, err := toml.Decode("k = "+s[:end+1], &struct {
				K *string `toml:"k"`
			}{&v}); err != nil {
				return nil, ErrHandEdit
			}
			part, s = v, s[end+1:]
		case '\'':
			end := strings.IndexByte(s[1:], '\'')
			if end < 0 {
				return nil, ErrHandEdit
			}
			part, s = s[1:end+1], s[end+2:]
		default:
			end := strings.IndexAny(s, ". \t")
			if end < 0 {
				end = len(s)
			}
			part, s = s[:end], s[end:]
			if !bareKey.MatchString(part) {
				return nil, ErrHandEdit
			}
		}
		out = append(out, part)
		s = strings.TrimSpace(s)
		if s != "" {
			if s[0] != '.' {
				return nil, ErrHandEdit
			}
			s = strings.TrimSpace(s[1:])
		}
	}
	return out, nil
}

// lineKey is the key a key/value line sets, as written ("harness",
// `"my key"`, "people.ana" for a dotted one); ok false for a line that
// isn't one (blank, a comment, a header).
func lineKey(line string) (string, bool) {
	t := strings.TrimSpace(line)
	if t == "" || t[0] == '#' || t[0] == '[' {
		return "", false
	}
	// The key ends at the first = outside quotes.
	inQuote := byte(0)
	for i := 0; i < len(t); i++ {
		c := t[i]
		switch {
		case inQuote != 0:
			if c == '\\' && inQuote == '"' {
				i++
			} else if c == inQuote {
				inQuote = 0
			}
		case c == '"' || c == '\'':
			inQuote = c
		case c == '=':
			return strings.TrimSpace(t[:i]), true
		}
	}
	return "", false
}

// valueEnd is the last line of the value the key/value line at i starts:
// itself, unless its value goes on (an array over several lines, a
// multi-line string).
func valueEnd(lines []string, i int, key string) int {
	line := lines[i]
	eq := strings.Index(line, key) + len(key)
	eq += strings.IndexByte(line[eq:], '=') + 1
	rest := line[eq:]
	depth := 0
	str := "" // the quote a string open across lines began with
	for j := i; j < len(lines); j++ {
		s := rest
		if j > i {
			s = lines[j]
		}
		for k := 0; k < len(s); k++ {
			c := s[k]
			if str != "" {
				switch {
				case len(str) == 3 && strings.HasPrefix(s[k:], str):
					k += 2
					str = ""
				case len(str) == 1 && c == '\\' && str == `"`:
					k++
				case len(str) == 1 && c == str[0]:
					str = ""
				case c == '\\' && str == `"""`:
					k++
				}
				continue
			}
			switch {
			case c == '#':
				k = len(s)
			case strings.HasPrefix(s[k:], `"""`), strings.HasPrefix(s[k:], `'''`):
				str = s[k : k+3]
				k += 2
			case c == '"' || c == '\'':
				str = string(c)
			case c == '[' || c == '{':
				depth++
			case c == ']' || c == '}':
				depth--
			}
		}
		if len(str) == 1 {
			str = "" // a one-line string can't go on
		}
		if depth <= 0 && str == "" {
			return j
		}
	}
	return len(lines) - 1
}

// findKey is the line in sec where key is set, and the last line of its
// value; -1 if it isn't. A dotted key there that starts with key (it sets
// a table inside this one) is ErrHandEdit: ws would add a header for it.
func (d *tomlDoc) findKey(sec section, key string) (int, int, error) {
	for i := sec.header + 1; i < sec.end; i++ {
		k, ok := lineKey(d.lines[i])
		if !ok {
			continue
		}
		end := valueEnd(d.lines, i, k)
		path, err := splitKey(k)
		if err != nil {
			return 0, 0, err
		}
		if len(path) == 1 && path[0] == key {
			return i, end, nil
		}
		if len(path) > 1 && path[0] == key {
			return 0, 0, ErrHandEdit
		}
		i = end
	}
	return -1, -1, nil
}

// set sets key in the table at path to value: on its line where it's set
// already, keeping what's before the value and a comment after it; else
// after the table's last key; else in a new table at the end.
func (d *tomlDoc) set(table []string, key string, value any) error {
	val, err := encodeValue(value)
	if err != nil {
		return err
	}
	secs, _, err := d.sections()
	if err != nil {
		return err
	}
	sec, ok := secs[pathKey(table)]
	if !ok {
		// A table that's set as a dotted key or an inline table somewhere
		// can't be given a header too.
		if d.mentioned(table) {
			return ErrHandEdit
		}
		d.appendTable(table, key+" = "+val)
		return nil
	}
	start, end, err := d.findKey(sec, key)
	if err != nil {
		return err
	}
	if start >= 0 {
		line := d.lines[start]
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		comment := ""
		if start == end {
			comment = trailingComment(line)
		}
		repl := indent + quoteKey(key) + " = " + val + comment
		d.lines = slices.Replace(d.lines, start, end+1, repl)
		return nil
	}
	// After the table's last key, indented as its keys are.
	at, indent := sec.header+1, ""
	for i := sec.header + 1; i < sec.end; i++ {
		if k, ok := lineKey(d.lines[i]); ok {
			line := d.lines[i]
			indent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			i = valueEnd(d.lines, i, k)
			at = i + 1
		}
	}
	if sec.header < 0 && at == 0 {
		// The first key of the top: before any header, after leading
		// comments that aren't a table's.
		d.lines = slices.Insert(d.lines, 0, quoteKey(key)+" = "+val)
		return nil
	}
	d.lines = slices.Insert(d.lines, at, indent+quoteKey(key)+" = "+val)
	return nil
}

// unset removes key from the table at path, and the table's header with
// it if that leaves the table with nothing at all (no keys, no comments).
func (d *tomlDoc) unset(table []string, key string) error {
	secs, _, err := d.sections()
	if err != nil {
		return err
	}
	sec, ok := secs[pathKey(table)]
	if !ok {
		if d.mentioned(table) {
			return ErrHandEdit
		}
		return nil
	}
	start, end, err := d.findKey(sec, key)
	if err != nil || start < 0 {
		return err
	}
	d.lines = slices.Delete(d.lines, start, end+1)
	sec.end -= end + 1 - start
	if sec.header < 0 {
		return nil
	}
	for i := sec.header + 1; i < sec.end; i++ {
		if strings.TrimSpace(d.lines[i]) != "" {
			return nil // something's left: keys, or comments
		}
	}
	// Nothing left: the header goes, with the blank lines after it, and
	// one blank line still parts what was before it from what follows.
	blank := func(i int) bool { return strings.TrimSpace(d.lines[i]) == "" }
	from, to := sec.header, sec.end
	switch {
	case to >= len(d.lines):
		// At the end: the blank line before it goes too.
		if from > 0 && blank(from-1) {
			from--
		}
	case to-1 > from && blank(to-1):
		// The blank line before the next table stays, and so the one
		// before this goes.
		if from > 0 && blank(from-1) {
			from--
		}
		to--
	}
	d.lines = slices.Delete(d.lines, from, to)
	return nil
}

// mentioned is whether the table at path, or one it's in, is set another
// way than by a header: as a dotted key, or an inline table.
func (d *tomlDoc) mentioned(table []string) bool {
	if len(table) == 0 {
		return false
	}
	var doc map[string]any
	if _, err := toml.Decode(d.String(), &doc); err != nil {
		return true
	}
	secs, _, err := d.sections()
	if err != nil {
		return true
	}
	// A path that the document has but no header of it, nor of a table
	// inside it, made.
	var cur any = doc
	for i, part := range table {
		m, ok := cur.(map[string]any)
		if !ok {
			return true
		}
		next, ok := m[part]
		if !ok {
			return false
		}
		prefix := table[:i+1]
		headed := false
		for k := range secs {
			if strings.HasPrefix(k+"\x00", pathKey(prefix)+"\x00") {
				headed = true
				break
			}
		}
		if !headed {
			return true
		}
		cur = next
	}
	return false
}

// appendTable adds a table with one line at the end of the document,
// after a blank line.
func (d *tomlDoc) appendTable(table []string, line string) {
	if n := len(d.lines); n > 0 && strings.TrimSpace(d.lines[n-1]) != "" {
		d.lines = append(d.lines, "")
	}
	d.lines = append(d.lines, "["+joinKey(table)+"]", line)
}

// trailingComment is a comment at the end of a one-line key/value line,
// with the space before it ("  # why"), or "".
func trailingComment(line string) string {
	inQuote := byte(0)
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case inQuote != 0:
			if c == '\\' && inQuote == '"' {
				i++
			} else if c == inQuote {
				inQuote = 0
			}
		case c == '"' || c == '\'':
			inQuote = c
		case c == '#':
			j := i
			for j > 0 && (line[j-1] == ' ' || line[j-1] == '\t') {
				j--
			}
			return line[j:]
		}
	}
	return ""
}

// encodeValue is a value as TOML writes it on the right of a key's =.
func encodeValue(v any) (string, error) {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(map[string]any{"v": v}); err != nil {
		return "", err
	}
	s := strings.TrimSpace(buf.String())
	val, ok := strings.CutPrefix(s, "v = ")
	if !ok || strings.Contains(val, "\n") {
		return "", fmt.Errorf("%w: %v can't be written on one line", ErrHandEdit, v)
	}
	return val, nil
}

// quoteKey is a key as it's written: bare if it can be.
func quoteKey(k string) string {
	if bareKey.MatchString(k) {
		return k
	}
	v, _ := encodeValue(k)
	return v
}

func joinKey(path []string) string {
	parts := make([]string, len(path))
	for i, p := range path {
		parts[i] = quoteKey(p)
	}
	return strings.Join(parts, ".")
}
