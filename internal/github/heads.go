package github

import (
	"encoding/json"
	"fmt"
	"strings"

	gh "github.com/cli/go-gh/v2"
)

// HeadsClient is a Client that can say where many repos' branches are on
// GitHub in one request. The daemon's ground watcher asks it each time it
// looks, and fetches only the repos whose branch has moved; a Client
// without it (the fixture, a stub) has each repo fetched instead.
type HeadsClient interface {
	Heads(org string, repos []string, branch string) (map[string]string, error)
}

// headsPerQuery is how many repos one query asks after. GitHub charges a
// query by the lists it pages through, and this one pages through none,
// so a query costs one point however many repos it names: this only
// keeps a query to a size GitHub's happy to parse.
const headsPerQuery = 100

func (LiveClient) Heads(org string, repos []string, branch string) (map[string]string, error) {
	return Heads(org, repos, branch)
}

// Heads is where each repo's branch is on GitHub: repo → commit. A repo
// GitHub doesn't have, or that hasn't the branch, isn't in it; a query
// that answered for some repos and not others answers with those it did.
//
// VERIFIED (github.com, 2026-11-20): eight repos in one query cost one
// point of the 5,000 an hour and answered in about a second, where `git
// ls-remote` took 7.4s for the same eight one after another. A repo that
// doesn't resolve comes back null with an error beside the others' data,
// and gh exits 1.
func Heads(org string, repos []string, branch string) (map[string]string, error) {
	out := map[string]string{}
	for start := 0; start < len(repos); start += headsPerQuery {
		chunk := repos[start:min(start+headsPerQuery, len(repos))]
		stdOut, stdErr, err := gh.Exec("api", "graphql", "-f", "query="+headsQuery(org, chunk, branch))
		heads, perr := parseHeads(stdOut.Bytes(), chunk)
		if perr != nil {
			if err != nil {
				return out, fmt.Errorf("heads of %s's repos: %s", org, ghError(stdOut.Bytes(), stdErr.String(), err))
			}
			return out, perr
		}
		for repo, oid := range heads {
			out[repo] = oid
		}
	}
	return out, nil
}

// headsQuery asks after each repo's branch under an alias of its place in
// repos (r0, r1, …): a repo's name can't be an alias, but its place can.
func headsQuery(org string, repos []string, branch string) string {
	var b strings.Builder
	b.WriteString("query {")
	for i, repo := range repos {
		fmt.Fprintf(&b, " r%d: repository(owner: %s, name: %s) { ref(qualifiedName: %s) { target { oid } } }",
			i, graphQLString(org), graphQLString(repo), graphQLString("refs/heads/"+branch))
	}
	b.WriteString(" }")
	return b.String()
}

// graphQLString is s as a GraphQL string literal, whose escapes are
// JSON's.
func graphQLString(s string) string {
	q, _ := json.Marshal(s)
	return string(q)
}

// parseHeads reads headsQuery's answer for repos: an error only when
// there's no data in it at all.
func parseHeads(data []byte, repos []string) (map[string]string, error) {
	var res struct {
		Data map[string]*struct {
			Ref *struct {
				Target struct {
					OID string `json:"oid"`
				} `json:"target"`
			} `json:"ref"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &res); err != nil {
		return nil, fmt.Errorf("reading heads: %w", err)
	}
	if res.Data == nil {
		return nil, fmt.Errorf("reading heads: no data")
	}
	out := map[string]string{}
	for i, repo := range repos {
		if r := res.Data[fmt.Sprintf("r%d", i)]; r != nil && r.Ref != nil && r.Ref.Target.OID != "" {
			out[repo] = r.Ref.Target.OID
		}
	}
	return out, nil
}
