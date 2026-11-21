package github

import (
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/ref"
)

func TestHeadsQuery_OneAliasPerRepo_EachByItsOwner(t *testing.T) {
	q := headsQuery([]ref.Remote{{Owner: "acme", Name: "api"}, {Owner: "other", Name: "web\"x"}}, "main")
	for _, want := range []string{
		`r0: repository(owner: "acme", name: "api") { ref(qualifiedName: "refs/heads/main")`,
		`r1: repository(owner: "other", name: "web\"x")`,
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query %s\nwants %s", q, want)
		}
	}
}

func TestParseHeads_KeepsWhatAnsweredBesideAnError(t *testing.T) {
	data := []byte(`{"data":{"r0":null,"r1":{"ref":{"target":{"oid":"abc"}}},"r2":{"ref":null}},
		"errors":[{"type":"NOT_FOUND","path":["r0"],"message":"Could not resolve"}]}`)
	got, err := parseHeads(data, []ref.Remote{{Owner: "acme", Name: "gone"}, {Owner: "acme", Name: "api"}, {Owner: "acme", Name: "nobranch"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[ref.Remote{Owner: "acme", Name: "api"}] != "abc" {
		t.Errorf("heads = %v, want only api at abc", got)
	}
}

func TestParseHeads_NoDataIsAnError(t *testing.T) {
	if _, err := parseHeads([]byte(`{"message":"Bad credentials"}`), []ref.Remote{{Owner: "acme", Name: "api"}}); err == nil {
		t.Error("an answer without data should be an error")
	}
	if _, err := parseHeads(nil, []ref.Remote{{Owner: "acme", Name: "api"}}); err == nil {
		t.Error("no answer should be an error")
	}
}
