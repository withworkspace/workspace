package nav

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// References is GET …/nav/references: where a definition is used.
type References struct {
	Name string `json:"name"`
	// Def is the definition they're uses of.
	Def  *Target `json:"def,omitempty"`
	Uses []Ref   `json:"uses"`
	// Partial: it stopped before reading every file that mentions the
	// name, or resolving every use.
	Partial bool `json:"partial,omitempty"`
}

// Ref is one use.
type Ref struct {
	Path string `json:"path"`
	Line int    `json:"line"`
	Col  int    `json:"col"` // UTF-16 units from 0
	Text string `json:"text"`
	// Maybe: it wasn't followed to the definition, but could be it (one
	// of several by name, or an interface's method it may satisfy).
	Maybe bool `json:"maybe,omitempty"`
}

// Limits on who uses a name.
const (
	maxUses        = 300
	referencesTime = 5 * time.Second
)

// References is where the definition at a place (or what the name at it
// leads to) is used, in the same commit.
func (s *Service) References(dir, rev, p string, line, col int) (*References, error) {
	key := strings.Join([]string{"refs", dir, rev, p, fmt.Sprint(line), fmt.Sprint(col)}, "\x00")
	if r, ok := s.refs.get(key); ok {
		return r, nil
	}
	t, err := s.tree(dir, rev)
	if err != nil {
		return nil, err
	}
	f, err := s.Facts(t, p)
	if err != nil {
		return nil, err
	}
	src, err := t.Read(p)
	if err != nil {
		return nil, err
	}
	pos, ok := byteAt(src, f, line, col)
	if !ok {
		return nil, ErrNoName
	}
	w := s.walker(t, LangOf(p))
	w.deadline = time.Now().Add(referencesTime)
	r, name := w.at(p, f, pos)
	if name == "" {
		return nil, ErrNoName
	}
	out := &References{Name: name, Uses: []Ref{}}
	def, ok := r.site()
	if !ok || r.kind != "def" {
		// Not a definition of the repo's: what's used of it is still by
		// its name, but there's nothing to follow them to.
		s.refs.put(key, out)
		return out, nil
	}
	d := def.def()
	out.Name = d.Name
	tg := w.target(def)
	out.Def = &tg
	w.uses(def, out)
	if !out.Partial {
		s.refs.put(key, out)
	}
	return out, nil
}

// uses fills in where a definition's name is used and leads to it.
func (w *walker) uses(def site, out *References) {
	d := def.def()
	hits, err := w.t.Grep(d.Name, w.lang.Suffixes()...)
	if err != nil {
		out.Partial = true
		return
	}
	var files []string
	seen := map[string]bool{}
	for _, h := range hits {
		if !seen[h.Path] {
			seen[h.Path] = true
			files = append(files, h.Path)
		}
	}
	// The definition's own file first, then its folder's, then the rest.
	sort.SliceStable(files, func(i, j int) bool { return closeness(def.path, files[i]) > closeness(def.path, files[j]) })
	if len(files) > maxGrepFiles {
		files, out.Partial = files[:maxGrepFiles], true
	}
	resolved := 0
	for _, p := range files {
		f, err := w.Facts(p)
		if err != nil {
			continue
		}
		src := w.src(p)
		for _, u := range f.Uses {
			if u.Name != d.Name {
				continue
			}
			if resolved >= maxUses || w.late() {
				out.Partial = true
				return
			}
			resolved++
			r := w.use(p, f, u, 0, false)
			maybe := false
			switch {
			case r.kind == "def" && has(r.defs, def):
			case r.kind == "candidates" && has(r.defs, def):
				maybe = true
			case r.kind == "def" && d.Kind == "method" && w.interfaceMethod(r.defs):
				maybe = true // an interface's method it may satisfy
			default:
				continue
			}
			line, start := f.Line(u.At.Start)
			end := len(src)
			if next, ok := f.Offset(line + 1); ok {
				end = int(next) - 1
			}
			out.Uses = append(out.Uses, Ref{
				Path:  p,
				Line:  line,
				Col:   utf16Len(src[start:u.At.Start]),
				Text:  cut(strings.TrimSpace(string(src[start:end])), 200),
				Maybe: maybe,
			})
		}
	}
}

func has(sites []site, s site) bool {
	for _, x := range sites {
		if x.path == s.path && x.i == s.i {
			return true
		}
	}
	return false
}

// interfaceMethod is whether a use led to an interface's method.
func (w *walker) interfaceMethod(sites []site) bool {
	for _, s := range sites {
		d := s.def()
		if d.Kind == "method" && d.Container >= 0 && s.f.Defs[d.Container].Kind == "interface" {
			return true
		}
	}
	return false
}
