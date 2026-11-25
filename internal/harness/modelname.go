package harness

import (
	"regexp"
	"strings"
)

// ModelEfforts are the efforts a model's name can say, least first.
var ModelEfforts = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

// modelName is what a model's name says of it, for a picker to put it in
// a tree: its maker (vendor), its family and version (one model, the
// picker's row), and the choices this name makes of that one: an effort,
// thinking or not, fast or not.
type modelName struct {
	Vendor, Family, Version, Effort string
	Thinking, Fast                  bool
}

// versionPart is a part of a name that is (part of) its version: "5",
// "4.6", "v41", "k2.7".
var versionPart = regexp.MustCompile(`^(?:\d+(?:\.\d+)*|[vk]\d+(?:\.\d+)*)$`)

// parseModelName reads a name as Cursor's CLI gives them, maker first, its
// choices last: "claude-opus-5-5-high-fast", "claude-4.6-sonnet-medium-
// thinking", "gpt-5.6-sol-xhigh", "cursor-grok-4.6-high". The family can
// come before the version or after it ("claude-opus-4-8", "claude-4.6-
// opus", "gemini-3.8-flash"), and a version in parts is joined with dots
// ("5-5" is 5.5). A name with no version ("auto") isn't one it can read.
func parseModelName(id string) (modelName, bool) {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(id)), "-")
	var n modelName
	if len(parts) > 1 && parts[len(parts)-1] == "fast" {
		n.Fast = true
		parts = parts[:len(parts)-1]
	}
	kept := parts[:0:0]
	for _, p := range parts {
		if p == "thinking" {
			n.Thinking = true
			continue
		}
		kept = append(kept, p)
	}
	parts = kept
	if l := len(parts); l > 2 && parts[l-2] == "extra" && parts[l-1] == "high" {
		parts = append(parts[:l-2], "xhigh")
	}
	if l := len(parts); l > 1 && isModelEffort(parts[l-1]) {
		n.Effort = parts[l-1]
		parts = parts[:l-1]
	}
	if len(parts) > 0 && parts[0] == "cursor" {
		parts = parts[1:] // Cursor's own build of another's ("cursor-grok-4.5")
	}
	if len(parts) < 2 {
		return modelName{}, false
	}
	n.Vendor = parts[0]
	rest := parts[1:]
	at := -1
	for i, p := range rest {
		if versionPart.MatchString(p) {
			at = i
			break
		}
	}
	if at < 0 {
		return modelName{}, false
	}
	end := at
	for end < len(rest) && versionPart.MatchString(rest[end]) && (end == at || !strings.Contains(rest[end], ".")) {
		end++
	}
	n.Version = strings.Join(rest[at:end], ".")
	n.Family = strings.Join(append(append([]string{}, rest[:at]...), rest[end:]...), "-")
	return n, true
}

func isModelEffort(s string) bool {
	for _, e := range ModelEfforts {
		if e == s {
			return true
		}
	}
	return false
}

// choiceWords are the words a model's shown name ends in for its choices
// ("Claude Opus 5.5 1M High Thinking Fast"), last first.
// "1M" (its context) goes too: Cursor's fast ones don't say it, though
// they're the same model.
var choiceWords = regexp.MustCompile(`(?i)\s+(?:no thinking|thinking|fast|extra high|none|minimal|low|medium|high|max|1m)$`)

// modelLabel is a model's shown name without its choices: "Claude Opus
// 5.5", the same for each of its efforts.
func modelLabel(name string) string {
	s := strings.Join(strings.Fields(strings.ReplaceAll(name, "\u200b", "")), " ")
	for {
		t := choiceWords.ReplaceAllString(s, "")
		if t == s || t == "" {
			return s
		}
		s = t
	}
}

// placed is m with where it sits in a tree of models, as its name says,
// where that can be read.
func placed(m Model) Model {
	n, ok := parseModelName(m.ID)
	if !ok {
		return m
	}
	m.Vendor, m.Family, m.Version = n.Vendor, n.Family, n.Version
	m.Effort, m.Thinking, m.Fast = n.Effort, n.Thinking, n.Fast
	if m.Name != "" {
		m.Label = modelLabel(m.Name)
	}
	return m
}
