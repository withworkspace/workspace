package automation

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"go.starlark.net/starlark"

	"github.com/withworkspace/workspace/internal/events"
	"github.com/withworkspace/workspace/internal/store"
)

// Acting: act("jira:changes", [...]) does something of a plugin's, as
// propose would ask for it, but done there and then where it may be:
//
//   - In a run on what happened (an event, the clock), with nobody looking,
//     it's accepted at once only where you let this automation do that
//     kind without asking (a grant, yours, kept per automation and kind),
//     and its plugin says it's safe done so (Proposes(...).Unattended()).
//     Otherwise it's proposed, for you to accept, as propose's is.
//   - In a run you started (one of its actions), it's put in front of you
//     to confirm, where you started it: you're there, and see exactly what
//     it'll do. No grant's needed.
//
// Either way it's a proposal in the automation's conversation, so what was
// done, and by whom, is kept as any card's is; act gives the same handle
// propose does, for a wait.
//
// may names the kinds it may act on (may = ["jira:changes"]): the script
// asks; the grant is yours, on its card or its page, never the script's.

// pluginKind is what a plugin proposes, as may and act name it.
var pluginKind = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*:[a-z0-9][a-z0-9-]*$`)

// IsPluginKind is whether a may is a plugin's kind (jira:changes).
func IsPluginKind(s string) bool { return pluginKind.MatchString(s) }

// PluginKinds are the plugin kinds a may act on, in order.
func (a *Automation) PluginKinds() []string {
	var out []string
	for _, m := range a.May {
		if IsPluginKind(m) {
			out = append(out, m)
		}
	}
	return out
}

func init() {
	store.Define("automation-grants", store.SQL(`
		-- What you let an automation do without asking: a plugin's kind of
		-- proposal, accepted as it runs on what happens.
		CREATE TABLE automation_grants (
			name TEXT NOT NULL,
			kind TEXT NOT NULL,                 -- jira:changes
			at TEXT NOT NULL,
			PRIMARY KEY (name, kind)
		) WITHOUT ROWID;
	`))
}

// Grants are the kinds you let name do without asking, in order.
func Grants(root, name string) ([]string, error) {
	var out []string
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT kind FROM automation_grants WHERE name = ? ORDER BY kind`, name)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var k string
			if err := rows.Scan(&k); err != nil {
				return err
			}
			out = append(out, k)
		}
		return rows.Err()
	})
	return out, err
}

// Grant lets name do kind without asking (on), or no longer (off). A kind
// it may not act on (its may) can't be granted.
func Grant(root, name, kind string, on bool, at time.Time) error {
	a, err := Load(root, name)
	if err != nil {
		return ErrNotFound
	}
	if on && !a.Allows(kind) {
		return fmt.Errorf("%s may not act on %s: its may says what it may (%s)", name, kind, strings.Join(a.PluginKinds(), ", "))
	}
	return write(root, func(tx *store.Tx) error {
		if !on {
			_, err := tx.Exec(`DELETE FROM automation_grants WHERE name = ? AND kind = ?`, name, kind)
			return err
		}
		_, err := tx.Exec(`INSERT INTO automation_grants (name, kind, at) VALUES (?, ?, ?) ON CONFLICT (name, kind) DO NOTHING`, name, kind, stampOf(at))
		return err
	})
}

// keepGrants drops what name was let do that its may no longer asks for:
// a revised script asks again for anything new.
func keepGrants(tx *store.Tx, name string, may []string) error {
	rows, err := tx.Query(`SELECT kind FROM automation_grants WHERE name = ?`, name)
	if err != nil {
		return err
	}
	var gone []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			rows.Close()
			return err
		}
		found := false
		for _, m := range may {
			found = found || m == k
		}
		if !found {
			gone = append(gone, k)
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, k := range gone {
		if _, err := tx.Exec(`DELETE FROM automation_grants WHERE name = ? AND kind = ?`, name, k); err != nil {
			return err
		}
	}
	return nil
}

// Acting is how an act's proposal is to be settled as it's made: Accept,
// with nobody looking (granted, and the plugin's to agree); Confirm, by
// you, where you started the run; or neither, left for you to accept.
type Acting struct {
	Accept  bool
	Confirm bool
}

// act is act(kind, input, why = "…"): a plugin's kind of proposal, done as
// the run may (see above): its handle at once.
func (r *run) act(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var kind, why string
	var input starlark.Value
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "kind", &kind, "input", &input, "why?", &why); err != nil {
		return nil, err
	}
	kind = strings.TrimSpace(kind)
	if !IsPluginKind(kind) {
		return nil, fmt.Errorf("act: %q isn't a plugin's kind of proposal: it's the plugin's name, a colon and what (jira:changes). For a mission or a decision, propose", kind)
	}
	if !r.a.Allows(kind) {
		return nil, fmt.Errorf("act: it may not act on %s (may = [%q] says it may)", kind, kind)
	}
	v, err := goOf(input)
	if err != nil {
		return nil, fmt.Errorf("act: %w", err)
	}
	p := Proposing{Kind: kind, Input: v, Why: strings.TrimSpace(why), On: r.ev.Ref() + ": " + r.ev.Says()}
	how := Acting{Confirm: r.ev.Kind == events.ActionRun && r.ev.By == events.ByYou}
	granted := false
	if !how.Confirm {
		grants, _ := Grants(r.e.Root, r.a.Name)
		for _, g := range grants {
			granted = granted || g == kind
		}
		how.Accept = granted
	}
	do := func() (string, error) {
		if r.e.Hooks.Act == nil {
			return "", fmt.Errorf("act: there's nowhere to do it here")
		}
		return r.e.Hooks.Act(r.a, p, how, r.conv, ChainOf(r.a.Name, r.ev, r.batch))
	}
	if r.dry != nil {
		do = func() (string, error) {
			said := "act " + p.Says()
			switch {
			case how.Confirm:
				said += " (you'd confirm it)"
			case granted:
				said += " (done without asking: you let it)"
			default:
				said += " (proposed: you haven't let it do that without asking)"
			}
			r.dry.did = append(r.dry.did, said)
			return fmt.Sprintf("dry#%d", r.steps), nil
		}
	}
	ref, err := r.step("act", do)
	if err != nil {
		return nil, err
	}
	if c, _, ok := strings.Cut(ref, "#"); ok && r.dry == nil {
		r.conv = c
	}
	return proposalHandle{ref: ref, kind: kind}, nil
}
