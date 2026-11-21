package ref

import (
	"fmt"
	"strings"
)

// Remote is where a repo is on GitHub: its owner (an org, or a person) and
// its name there. A workspace's repos needn't share an owner, and a repo's
// name in the workspace needn't be its name on GitHub, so each repo has
// one (workspace.Workspace.RemoteOf).
type Remote struct {
	Owner, Name string
}

// ParseRemote reads owner/name, as ws.toml's github says it.
func ParseRemote(s string) (Remote, bool) {
	owner, name, ok := strings.Cut(strings.TrimSpace(s), "/")
	if !ok || owner == "" || name == "" || strings.ContainsAny(owner+name, "/ \t") {
		return Remote{}, false
	}
	return Remote{Owner: owner, Name: strings.TrimSuffix(name, ".git")}, true
}

// String is owner/name.
func (r Remote) String() string { return r.Owner + "/" + r.Name }

// URL is the repo's page on GitHub.
func (r Remote) URL() string { return "https://github.com/" + r.String() }

// Is is whether owner/name is this repo: GitHub's names aren't case
// sensitive, so an address may say either.
func (r Remote) Is(owner, name string) bool {
	return strings.EqualFold(r.Owner, owner) && strings.EqualFold(r.Name, name)
}

// MarshalText writes it as owner/name, as ws.toml and the web UI say it.
func (r Remote) MarshalText() ([]byte, error) { return []byte(r.String()), nil }

// UnmarshalText reads owner/name.
func (r *Remote) UnmarshalText(b []byte) error {
	got, ok := ParseRemote(string(b))
	if !ok {
		return fmt.Errorf("%q isn't owner/name", b)
	}
	*r = got
	return nil
}
