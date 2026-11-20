// Command wiretypes writes web/src/wire.gen.ts: the TypeScript of what the
// web UI's API sends and takes, from internal/web's own Go types, so the web
// UI can't drift from them. Types of other packages a wire type uses come
// too, named with their package's name in front (review.File: ReviewFile).
//
//	go run ./scripts/wiretypes          # write it
//	go run ./scripts/wiretypes -check   # fail if it isn't what the Go says
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/constant"
	"go/types"
	"os"
	"slices"
	"strings"

	"github.com/coder/guts"
	"github.com/coder/guts/bindings"
	"github.com/coder/guts/config"
)

const (
	web = "github.com/withworkspace/workspace/internal/web"
	out = "web/src/wire.gen.ts"
)

// referenced are the packages wire types use types of. Only the types used
// come, not the whole package. A package missing here shows in the output
// as a field of type unknown, with a comment naming the type.
var referenced = []string{
	"github.com/withworkspace/plugin/protocol",
	"github.com/withworkspace/workspace/internal/artifact",
	"github.com/withworkspace/workspace/internal/conversation",
	"github.com/withworkspace/workspace/internal/harness",
	"github.com/withworkspace/workspace/internal/mission",
	"github.com/withworkspace/workspace/internal/ops",
	"github.com/withworkspace/workspace/internal/page",
	"github.com/withworkspace/workspace/internal/plugin",
	"github.com/withworkspace/workspace/internal/pulls",
	"github.com/withworkspace/workspace/internal/ref",
	"github.com/withworkspace/workspace/internal/review",
	"github.com/withworkspace/workspace/internal/runs",
	"github.com/withworkspace/workspace/internal/team",
	"github.com/withworkspace/workspace/internal/words",
	"github.com/withworkspace/workspace/internal/workspace",
}

// notWire are internal/web's exported names that aren't sent or taken: the
// server, what it asks of ops, and the content security policies it sets.
var notWire = []string{
	"Server", "App",
	"Acting", "Artifacting", "CodeSearching", "Conversing", "Crewing", "Documents", "Files",
	"Focusing", "WhatsNewing", "Missions", "Paging", "Posting", "Processing", "Reviewing", "Running", "Settings",
	"FramePolicy", "PluginPolicy", "ShellPolicy",
}

func main() {
	check := flag.Bool("check", false, "fail if "+out+" isn't what the Go says, rather than writing it")
	flag.Parse()
	ts, err := generate()
	if err != nil {
		fmt.Fprintln(os.Stderr, "wiretypes:", err)
		os.Exit(1)
	}
	if *check {
		have, err := os.ReadFile(out)
		if err != nil || !bytes.Equal(have, []byte(ts)) {
			fmt.Fprintln(os.Stderr, out+" isn't what internal/web's types say: run `just wire` and commit it")
			os.Exit(1)
		}
		return
	}
	if err := os.WriteFile(out, []byte(ts), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, "wiretypes:", err)
		os.Exit(1)
	}
}

func generate() (string, error) {
	g, err := guts.NewGolangParser()
	if err != nil {
		return "", err
	}
	g.PreserveComments()
	if err := g.IncludeGenerate(web); err != nil {
		return "", err
	}
	for _, pkg := range referenced {
		name := pkg[strings.LastIndex(pkg, "/")+1:]
		if err := g.IncludeReference(pkg, strings.ToUpper(name[:1])+name[1:]); err != nil {
			return "", err
		}
	}
	var skip []string
	for _, n := range notWire {
		skip = append(skip, web+"."+n)
	}
	if err := g.ExcludeCustom(skip...); err != nil {
		return "", err
	}
	g.IncludeCustomDeclaration(config.StandardMappings())

	ts, err := g.ToTypescript()
	if err != nil {
		return "", err
	}
	referencedEnums(g, ts)
	ts.ApplyMutations(
		config.ExportTypes,
		config.EnumAsTypes,
		// A field that's left out when empty is optional, not null.
		config.SimplifyOptional,
		// Go would send null for a nil map or a nil in a []*T, but ws
		// doesn't, and the web UI has never had to check.
		config.NotNullMaps,
		config.NullUnionSlices,
	)
	return ts.Serialize()
}

// referencedEnums makes a named string type of a referenced package (such
// as mission.State) the union of its constants. guts brings only the types
// a wire type uses from those packages, not the constants beside them, so
// without this the type would be plain string.
func referencedEnums(g *guts.GoParser, ts *guts.Typescript) {
	for path, pkg := range g.Pkgs {
		if !g.Reference[path] {
			continue
		}
		values := map[*types.TypeName][]string{}
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			c, ok := scope.Lookup(name).(*types.Const)
			if !ok || c.Val().Kind() != constant.String {
				continue
			}
			if named, ok := c.Type().(*types.Named); ok && named.Obj().Pkg() == pkg.Types {
				values[named.Obj()] = append(values[named.Obj()], constant.StringVal(c.Val()))
			}
		}
		for tn, vs := range values {
			key := g.Identifier(tn).Ref()
			node, ok := ts.Node(key)
			if !ok {
				continue // not used by a wire type
			}
			alias, ok := node.(*bindings.Alias)
			if !ok {
				continue
			}
			slices.Sort(vs)
			union := &bindings.UnionType{}
			for _, v := range slices.Compact(vs) {
				union.Types = append(union.Types, &bindings.LiteralType{Value: v})
			}
			alias.Type = union
			ts.ReplaceNode(key, alias)
		}
	}
}
