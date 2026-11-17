// Package process is the workspace's process as plans quote it. The
// process is prose, the team's, in the config repo (PROCESS.md beside
// ws.toml): how work goes here, whose turn each step is, and when it's the
// user's. The planner and the orchestrator read it as it's written; a
// plan's steps may end with its words in double quotes, which have to be
// there.
package process

import (
	"regexp"
	"strings"
)

var elided = regexp.MustCompile(`\s*(?:…|\.\.\.)\s*`)

// Says reports whether the process's text (made plain) has the words
// quoted, as written there, give or take case, spacing, markdown's marks
// and its quote marks; "…" leaves words out, and what's either side of it
// must come in that order.
func Says(plainProcess, quote string) bool {
	parts := elided.Split(plain(quote), -1)
	at, found := 0, false
	for _, part := range parts {
		part = strings.Trim(part, " .,;:")
		if part == "" {
			continue
		}
		i := strings.Index(plainProcess[at:], part)
		if i < 0 {
			return false
		}
		at += i + len(part)
		found = true
	}
	return found
}

// plain is text as quotes are found in it: lower case, its quote marks
// straight, without markdown's marks, its spacing one space.
func plain(s string) string {
	s = strings.NewReplacer("’", "'", "‘", "'", "“", `"`, "”", `"`, "*", "", "_", "", "`", "", "\\", "").Replace(s)
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

// Plain is the process's text as Says finds quotes in it.
func Plain(process string) string { return plain(process) }
