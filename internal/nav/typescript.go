package nav

import (
	"encoding/json"
	"path"
	"sort"
	"strings"
)

// typescript is TypeScript and JavaScript: .ts, .mts and .cts with the
// TypeScript grammar, .tsx and the JavaScript files with the TSX one
// (JavaScript is TypeScript without types, and JSX parses only there).
type typescript struct{ noNamespaces }

var (
	tsQuery  = mustQuery("typescript")
	tsxQuery = tsQuery + "\n" + mustQuery("tsx")
)

func (typescript) Name() string { return "typescript" }

func (typescript) Suffixes() []string {
	return []string{".ts", ".tsx", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}
}

func (typescript) Grammar(p string) string {
	switch path.Ext(p) {
	case ".ts", ".mts", ".cts":
		return "typescript"
	}
	return "tsx"
}

func (typescript) Query(grammar string) string {
	if grammar == "tsx" {
		return tsxQuery
	}
	return tsQuery
}

func (typescript) BodyOpen() string      { return "{" }
func (typescript) ImportsExported() bool { return false }

// Literal: see Lang.
func (typescript) Literal(text string) string {
	switch {
	case strings.HasPrefix(text, `"`), strings.HasPrefix(text, "'"), strings.HasPrefix(text, "`"):
		return "String"
	case strings.HasPrefix(text, "["):
		return "Array"
	case strings.HasPrefix(text, "/"):
		return "RegExp"
	}
	return ""
}

// ElemType: see Lang.
func (typescript) ElemType(written string) string {
	return genericArg(written, '<', '>')
}

func (typescript) SeesClass() bool { return true }

func (typescript) Submodule(Env, Where, string) (Where, bool) { return Where{}, false }

func (typescript) Self() []string { return []string{"this"} }

// Hoists: a function declared anywhere in its scope can be called before
// it.
func (typescript) Hoists(scope string, d Def) bool { return d.Kind == "func" }

func (typescript) Exported(d Def) bool { return d.Exported || d.Default }

func (typescript) Shared(Env, string, *Facts) []string { return nil }

func (typescript) ImportName(Env, string, Import) string { return "" }

var tsBuiltins = map[string]bool{}

// tsUtilityTypes are TypeScript's own types.
var tsUtilityTypes = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`Partial Required Readonly Record Pick Omit Exclude Extract NonNullable ReturnType Parameters InstanceType Awaited Uppercase Lowercase ThisType`) {
		tsUtilityTypes[n] = true
	}
}

func init() {
	for _, n := range strings.Fields(`undefined NaN Infinity globalThis window document console navigator location
		Object Array String Number Boolean Symbol BigInt Function Date RegExp Error TypeError RangeError SyntaxError
		Map Set WeakMap WeakSet Promise Proxy Reflect JSON Math Intl ArrayBuffer DataView Uint8Array Int32Array Float64Array
		URL URLSearchParams Headers Request Response fetch AbortController Event EventTarget CustomEvent
		HTMLElement Element Node NodeFilter Document Range Selection KeyboardEvent MouseEvent PointerEvent DOMRect
		HTMLDivElement HTMLInputElement HTMLTextAreaElement HTMLButtonElement HTMLAnchorElement HTMLFormElement HTMLCanvasElement
		Text Window Storage FocusEvent WheelEvent DragEvent ClipboardEvent InputEvent TouchEvent UIEvent MessageEvent
		ResizeObserver IntersectionObserver MutationObserver EventSource WebSocket Blob File FileReader FormData
		TextEncoder TextDecoder AbortSignal DOMParser CSS CSSStyleDeclaration DOMTokenList ShadowRoot localStorage sessionStorage history
		setTimeout clearTimeout setInterval clearInterval requestAnimationFrame cancelAnimationFrame queueMicrotask structuredClone
		parseInt parseFloat isNaN isFinite encodeURIComponent decodeURIComponent encodeURI decodeURI atob btoa
		require module exports process Buffer __dirname __filename
		Partial Required Readonly Record Pick Omit Exclude Extract NonNullable ReturnType Parameters InstanceType Awaited
		Uppercase Lowercase ThisType Iterable Iterator AsyncIterable PromiseLike ArrayLike ReadonlyArray`) {
		tsBuiltins[n] = true
	}
}

func (typescript) Builtin(name string) bool { return tsBuiltins[name] }

// nodeBuiltins are Node's own modules, documented at nodejs.org.
var nodeBuiltins = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`assert buffer child_process cluster crypto dgram dns events fs http http2 https net os path
		perf_hooks process querystring readline stream string_decoder timers tls tty url util v8 vm worker_threads zlib test`) {
		nodeBuiltins[n] = true
	}
}

func (typescript) Docs(module, name string) string {
	if module == "" {
		if name == "" {
			return ""
		}
		parts := strings.Split(name, ".")
		if tsGlobalObjects[parts[0]] {
			return "https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/" + strings.Join(parts, "/")
		}
		if tsUtilityTypes[parts[0]] {
			return "https://www.typescriptlang.org/docs/handbook/utility-types.html"
		}
		return "https://developer.mozilla.org/en-US/docs/Web/API/" + strings.Join(parts, "/")
	}
	m := strings.TrimPrefix(module, "node:")
	if base := strings.SplitN(m, "/", 2)[0]; nodeBuiltins[base] || strings.HasPrefix(module, "node:") {
		return "https://nodejs.org/api/" + base + ".html"
	}
	return "https://www.npmjs.com/package/" + tsPackageName(module)
}

// tsPackageName is the package a bare import is of: @scope/name, or name.
func tsPackageName(spec string) string {
	parts := strings.Split(spec, "/")
	if strings.HasPrefix(spec, "@") && len(parts) >= 2 {
		return parts[0] + "/" + parts[1]
	}
	return parts[0]
}

// TypeName: Foo<T> is Foo, Foo | null is Foo, NS.Foo is NS, Foo; an array,
// a tuple, an object or a function type is no single type.
func (typescript) TypeName(written string) []string {
	t := strings.TrimSpace(written)
	if strings.Contains(t, "|") {
		var kept []string
		for _, part := range splitTop(t, '|') {
			p := strings.TrimSpace(part)
			if p != "null" && p != "undefined" && p != "" {
				kept = append(kept, p)
			}
		}
		if len(kept) != 1 {
			return nil
		}
		t = kept[0]
	}
	if strings.HasSuffix(t, "[]") {
		return []string{"Array"}
	}
	// Partial<Foo> has Foo's members.
	for _, wrap := range []string{"Partial<", "Required<", "Readonly<", "NonNullable<"} {
		if strings.HasPrefix(t, wrap) && strings.HasSuffix(t, ">") {
			return typescript{}.TypeName(t[len(wrap) : len(t)-1])
		}
	}
	if i := strings.IndexByte(t, '<'); i > 0 {
		t = t[:i]
	}
	if t == "" || strings.ContainsAny(t, "[]{}()=> ,&|'\"") || t == "any" || t == "unknown" || t == "object" {
		return nil
	}
	if w, ok := tsWrappers[t]; ok {
		return []string{w}
	}
	return strings.Split(t, ".")
}

// tsWrappers are the types a primitive's members are of.
var tsWrappers = map[string]string{"string": "String", "number": "Number", "boolean": "Boolean", "bigint": "BigInt", "symbol": "Symbol"}

// tsGlobalObjects are JavaScript's own (MDN's Global_Objects); the rest
// of the builtins are the browser's (MDN's Web API).
var tsGlobalObjects = map[string]bool{}

func init() {
	for _, n := range strings.Fields(`Object Array String Number Boolean Symbol BigInt Function Date RegExp Error TypeError RangeError SyntaxError
		Map Set WeakMap WeakSet Promise Proxy Reflect JSON Math Intl ArrayBuffer DataView Uint8Array Int32Array Float64Array
		globalThis undefined NaN Infinity parseInt parseFloat isNaN isFinite encodeURIComponent decodeURIComponent encodeURI decodeURI`) {
		tsGlobalObjects[n] = true
	}
}

// tsExts are what an import's path is tried with, in order.
var tsExts = []string{".ts", ".tsx", ".d.ts", ".mts", ".cts", ".js", ".jsx", ".mjs", ".cjs"}

// tsFile is the file a path means: as it is, with an extension, with a
// TypeScript one in place of a JavaScript one, or a folder's index.
func tsFile(t Tree, p string) (string, bool) {
	p = path.Clean(p)
	if _, ok := t.Blob(p); ok && path.Ext(p) != "" {
		return p, true
	}
	switch ext := path.Ext(p); ext {
	case ".js", ".jsx", ".mjs", ".cjs":
		stem := strings.TrimSuffix(p, ext)
		for _, e := range []string{".ts", ".tsx", ".d.ts", ".mts", ".cts"} {
			if _, ok := t.Blob(stem + e); ok {
				return stem + e, true
			}
		}
	}
	for _, e := range tsExts {
		if _, ok := t.Blob(p + e); ok {
			return p + e, true
		}
	}
	for _, e := range tsExts {
		if _, ok := t.Blob(path.Join(p, "index"+e)); ok {
			return path.Join(p, "index"+e), true
		}
	}
	return "", false
}

// jsonc reads JSON with comments and trailing commas, as tsconfig.json is
// written.
func jsonc(src []byte, v any) error {
	var out []byte
	inString, escaped := false, false
	for i := 0; i < len(src); i++ {
		c := src[i]
		switch {
		case inString:
			out = append(out, c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
		case c == '"':
			inString = true
			out = append(out, c)
		case c == '/' && i+1 < len(src) && src[i+1] == '/':
			for i < len(src) && src[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(src) && src[i+1] == '*':
			i += 2
			for i+1 < len(src) && !(src[i] == '*' && src[i+1] == '/') {
				i++
			}
			i++
		case c == ',':
			// A trailing comma: the next thing that isn't space closes.
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\t' || src[j] == '\n' || src[j] == '\r') {
				j++
			}
			if j < len(src) && (src[j] == '}' || src[j] == ']') {
				continue
			}
			out = append(out, c)
		default:
			out = append(out, c)
		}
	}
	return json.Unmarshal(out, v)
}

// tsConfig is what of a tsconfig.json says where imports lead.
type tsConfig struct {
	dir     string // the folder baseUrl and paths are read from
	baseURL string // "" for none
	paths   map[string][]string
}

type tsConfigFile struct {
	Extends         any `json:"extends"`
	CompilerOptions struct {
		BaseURL *string             `json:"baseUrl"`
		Paths   map[string][]string `json:"paths"`
	} `json:"compilerOptions"`
}

// tsConfigFor is the tsconfig.json nearest above a file, with what it
// extends (in the repo) folded in.
func tsConfigFor(e Env, from string) *tsConfig {
	t := e.Tree()
	found := ""
	for dir := path.Dir(from); ; dir = path.Dir(dir) {
		p := path.Join(dir, "tsconfig.json")
		if _, ok := t.Blob(p); ok {
			found = p
			break
		}
		if dir == "." || dir == "/" {
			break
		}
	}
	if found == "" {
		return nil
	}
	return e.Memo("ts.config:"+found, func() any {
		c := &tsConfig{}
		var read func(p string, depth int)
		read = func(p string, depth int) {
			if depth > 6 {
				return
			}
			src, err := t.Read(p)
			if err != nil {
				return
			}
			var f tsConfigFile
			if jsonc(src, &f) != nil {
				return
			}
			// What it extends first: its own settings win.
			var bases []string
			switch x := f.Extends.(type) {
			case string:
				bases = []string{x}
			case []any:
				for _, b := range x {
					if s, ok := b.(string); ok {
						bases = append(bases, s)
					}
				}
			}
			for _, b := range bases {
				if strings.HasPrefix(b, ".") {
					bp := path.Join(path.Dir(p), b)
					if !strings.HasSuffix(bp, ".json") {
						bp += ".json"
					}
					read(bp, depth+1)
				}
			}
			if f.CompilerOptions.BaseURL != nil {
				c.baseURL = path.Join(path.Dir(p), *f.CompilerOptions.BaseURL)
			}
			if f.CompilerOptions.Paths != nil {
				c.paths = f.CompilerOptions.Paths
				c.dir = path.Dir(p)
			}
		}
		read(found, 0)
		if c.paths != nil && c.baseURL != "" {
			c.dir = c.baseURL // paths are read from baseUrl, when there's one
		}
		return c
	}).(*tsConfig)
}

// tsPackages are the repo's own packages, by name: each package.json's,
// outside node_modules.
func tsPackages(e Env) map[string]string {
	return e.Memo("ts.packages", func() any {
		out := map[string]string{}
		for _, p := range e.Tree().Under("", "package.json") {
			if path.Base(p) != "package.json" || strings.Contains(p, "node_modules/") {
				continue
			}
			src, err := e.Tree().Read(p)
			if err != nil {
				continue
			}
			var pj struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(src, &pj) == nil && pj.Name != "" {
				out[pj.Name] = path.Dir(p)
			}
		}
		return out
	}).(map[string]string)
}

// packageEntry is where a package of the repo's import leads: its
// exports for the subpath, or its types, module or main, or its src/index.
func packageEntry(t Tree, dir, sub string) (string, bool) {
	src, err := t.Read(path.Join(dir, "package.json"))
	if err == nil {
		var pj struct {
			Exports any    `json:"exports"`
			Types   string `json:"types"`
			Typings string `json:"typings"`
			Module  string `json:"module"`
			Main    string `json:"main"`
		}
		if json.Unmarshal(src, &pj) == nil {
			key := "."
			if sub != "" {
				key = "./" + sub
			}
			if target := exportTarget(pj.Exports, key); target != "" {
				if p, ok := tsFile(t, path.Join(dir, target)); ok {
					return p, true
				}
			}
			if sub == "" {
				for _, m := range []string{pj.Types, pj.Typings, pj.Module, pj.Main} {
					if m != "" {
						if p, ok := tsFile(t, path.Join(dir, m)); ok {
							return p, true
						}
					}
				}
			}
		}
	}
	if sub != "" {
		if p, ok := tsFile(t, path.Join(dir, sub)); ok {
			return p, true
		}
		return tsFile(t, path.Join(dir, "src", sub))
	}
	if p, ok := tsFile(t, path.Join(dir, "src", "index")); ok {
		return p, true
	}
	return tsFile(t, path.Join(dir, "index"))
}

// exportTarget is what package.json's exports says a subpath is: a path,
// or under conditions (types, then import, then default).
func exportTarget(exports any, key string) string {
	switch x := exports.(type) {
	case string:
		if key == "." {
			return x
		}
	case map[string]any:
		if v, ok := x[key]; ok {
			return condition(v)
		}
		// Conditions at the top: "." alone.
		if key == "." {
			if _, sub := x["."]; !sub {
				return condition(x)
			}
		}
		// A pattern: "./*": "./src/*.ts".
		keys := make([]string, 0, len(x))
		for k := range x {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
		for _, k := range keys {
			if star := strings.IndexByte(k, '*'); star >= 0 && strings.HasPrefix(key, k[:star]) && strings.HasSuffix(key, k[star+1:]) {
				matched := key[star : len(key)-len(k)+star+1]
				return strings.ReplaceAll(condition(x[k]), "*", matched)
			}
		}
	}
	return ""
}

func condition(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]any:
		for _, c := range []string{"types", "import", "module", "default", "require", "node"} {
			if s := condition(x[c]); s != "" {
				return s
			}
		}
	}
	return ""
}

func (typescript) Import(e Env, from string, im Import) Where {
	t := e.Tree()
	spec := im.Path
	if spec == "" {
		return Where{}
	}
	one := func(p string) Where { return Where{Files: []string{p}, Dir: path.Dir(p)} }
	if strings.HasPrefix(spec, ".") || strings.HasPrefix(spec, "/") {
		if p, ok := tsFile(t, path.Join(path.Dir(from), spec)); ok {
			return one(p)
		}
		return Where{}
	}
	if c := tsConfigFor(e, from); c != nil {
		// paths, the longest pattern that matches first.
		keys := make([]string, 0, len(c.paths))
		for k := range c.paths {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
		for _, k := range keys {
			star := strings.IndexByte(k, '*')
			matched := ""
			switch {
			case star < 0 && k == spec:
			case star >= 0 && strings.HasPrefix(spec, k[:star]) && strings.HasSuffix(spec, k[star+1:]) && len(spec) >= len(k)-1:
				matched = spec[star : len(spec)-len(k)+star+1]
			default:
				continue
			}
			for _, target := range c.paths[k] {
				if p, ok := tsFile(t, path.Join(c.dir, strings.ReplaceAll(target, "*", matched))); ok {
					return one(p)
				}
			}
		}
		if c.baseURL != "" {
			if p, ok := tsFile(t, path.Join(c.baseURL, spec)); ok {
				return one(p)
			}
		}
	}
	// One of the repo's own packages, by its name.
	name := tsPackageName(spec)
	if dir, ok := tsPackages(e)[name]; ok {
		if p, ok := packageEntry(t, dir, strings.TrimPrefix(strings.TrimPrefix(spec, name), "/")); ok {
			return one(p)
		}
	}
	return Where{External: spec}
}
