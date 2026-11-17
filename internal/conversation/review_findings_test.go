package conversation

import (
	"reflect"
	"testing"
)

// A review's pass's reply is read as a page's, with each finding's place
// where a page's has its quote.
func TestParseReviewFindings(t *testing.T) {
	reply := "The token check is sound; the cache key isn't.\n\n" +
		"## blocking\n@ internal/cache/key.go:41-44\nThe key leaves the tenant out.\n\n```suggestion\n\tkey := tenant + \":\" + token\n```\n\n" +
		"## nit\n\n@ `old.go:12` old\nGone, but its comment isn't.\n\n" +
		"## flag\nThis fixes the symptom in the cache. The session store has the same bug.\n\n" +
		"## question\n@ a b/c.go:7\n> not a quote here\nWhy seven?\n\n" +
		"## nit\n@ 1 old-main.go-line-deleted-root\nThe root package is gone.\n"
	summary, fs := ParseReviewFindings(reply)
	if summary != "The token check is sound; the cache key isn't." || len(fs) != 5 {
		t.Fatalf("summary %q, %d findings: %+v", summary, len(fs), fs)
	}
	want := []Finding{
		{Kind: "blocking", Path: "internal/cache/key.go", Side: "new", Line: 41, EndLine: 44, Text: "The key leaves the tenant out.", Suggest: "\tkey := tenant + \":\" + token"},
		{Kind: "nit", Path: "old.go", Side: "old", Line: 12, EndLine: 12, Text: "Gone, but its comment isn't."},
		{Kind: "flag", Text: "This fixes the symptom in the cache. The session store has the same bug."},
		{Kind: "question", Path: "a b/c.go", Side: "new", Line: 7, EndLine: 7, Text: "> not a quote here\nWhy seven?"},
		// A place that can't be read is no place.
		{Kind: "nit", Text: "The root package is gone."},
	}
	if !reflect.DeepEqual(fs, want) {
		t.Errorf("findings:\n got %+v\nwant %+v", fs, want)
	}
	if fs[0].Placed() != true || fs[2].Placed() {
		t.Errorf("placed: %v %v", fs[0].Placed(), fs[2].Placed())
	}
	// A page's reply has no places: an @ line there is what it says.
	if _, ps := ParseFindings("x\n\n## question\n@ a.go:1\nWhy?\n"); len(ps) != 1 || ps[0].Path != "" || ps[0].Text != "@ a.go:1\nWhy?" {
		t.Errorf("a page's: %+v", ps)
	}
}

// A review's finding is the same finding wherever its lines move to, and
// however it's worded: it's known by its kind, its file and how its lines
// read.
func TestKeyFindings(t *testing.T) {
	lines := []string{"\tkey := token", "\treturn key"}
	a := []Finding{
		{Kind: "blocking", Path: "key.go", Side: "new", Line: 41, EndLine: 42, Excerpt: lines, Text: "No tenant."},
		{Kind: "flag", Text: "Split it."},
		{Kind: "blocking", Path: "key.go", Side: "new", Line: 90, EndLine: 91, Excerpt: lines, Text: "And again here."},
	}
	b := []Finding{
		{Kind: "flag", Text: "split it."},
		{Kind: "blocking", Path: "key.go", Side: "new", Line: 57, EndLine: 58, Excerpt: []string{"    key := token", "    return key"}, Text: "The tenant's left out."},
		{Kind: "nit", Path: "key.go", Side: "new", Line: 57, EndLine: 58, Excerpt: lines, Text: "x"},
		{Kind: "blocking", Path: "other.go", Side: "new", Line: 41, EndLine: 42, Excerpt: lines, Text: "x"},
	}
	KeyFindings(a)
	KeyFindings(b)
	if a[0].Key == "" || a[0].Key != b[1].Key {
		t.Errorf("moved and reworded: %q, %q", a[0].Key, b[1].Key)
	}
	if a[1].Key != b[0].Key {
		t.Errorf("on no lines: %q, %q", a[1].Key, b[0].Key)
	}
	if a[2].Key != a[0].Key+"-2" {
		t.Errorf("the same lines twice: %q after %q", a[2].Key, a[0].Key)
	}
	if b[2].Key == b[1].Key || b[3].Key == b[1].Key {
		t.Errorf("another kind, another file: %q %q %q", b[1].Key, b[2].Key, b[3].Key)
	}
}

func TestOpenReviewPass(t *testing.T) {
	s := ForWorkspace(owned(t))
	c, err := s.OpenReviewPass("api#89", "security", at)
	if err != nil {
		t.Fatal(err)
	}
	if c.What != WhatReviewPass || c.Kind != Pass || c.Anchor.PR != "api#89" || !c.NeedsAnswer() {
		t.Errorf("opened: %+v", c)
	}
	if got := s.ReviewPassOn("api#89", "security"); got == nil || got.ID != c.ID {
		t.Errorf("found: %+v", got)
	}
	if s.ReviewPassOn("api#90", "security") != nil || s.ReviewPassOn("api#89", "other") != nil {
		t.Error("found one that isn't there")
	}
	if _, err := s.OpenReviewPass("", "security", at); err == nil {
		t.Error("a pass over no pull request was opened")
	}
}
