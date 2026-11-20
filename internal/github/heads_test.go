package github

import (
	"strings"
	"testing"
)

func TestHeadsQuery_OneAliasPerRepo(t *testing.T) {
	q := headsQuery("acme", []string{"api", "web\"x"}, "main")
	for _, want := range []string{
		`r0: repository(owner: "acme", name: "api") { ref(qualifiedName: "refs/heads/main")`,
		`r1: repository(owner: "acme", name: "web\"x")`,
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query %s\nwants %s", q, want)
		}
	}
}

func TestParseHeads_KeepsWhatAnsweredBesideAnError(t *testing.T) {
	data := []byte(`{"data":{"r0":null,"r1":{"ref":{"target":{"oid":"abc"}}},"r2":{"ref":null}},
		"errors":[{"type":"NOT_FOUND","path":["r0"],"message":"Could not resolve"}]}`)
	got, err := parseHeads(data, []string{"gone", "api", "nobranch"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got["api"] != "abc" {
		t.Errorf("heads = %v, want only api at abc", got)
	}
}

func TestParseHeads_NoDataIsAnError(t *testing.T) {
	if _, err := parseHeads([]byte(`{"message":"Bad credentials"}`), []string{"api"}); err == nil {
		t.Error("an answer without data should be an error")
	}
	if _, err := parseHeads(nil, []string{"api"}); err == nil {
		t.Error("no answer should be an error")
	}
}
