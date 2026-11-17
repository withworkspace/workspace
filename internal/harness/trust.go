package harness

import (
	"regexp"
	"strings"

	uv "github.com/charmbracelet/ultraviolet"
)

// Claude Code's folder-trust dialog: the first thing it shows in a folder
// (a git repo, for every capsule of it) it hasn't been trusted in. Its
// menu isn't numbered, so ReadPrompt doesn't see it, and nothing it does
// reaches ws's hooks until it's answered: crew on it would look as if
// they were working.
//
// VERIFIED (Claude Code 2.1.326; testdata/claude-trust-screen.txt):
//
//	Accessing workspace:
//	/the/folder
//	Quick safety check: Is this a project you created or one you trust? …
//	❯ No, exit
//	  Yes, I trust this folder
//	Enter to confirm · Esc to cancel

// TrustDialog is Claude Code's trust dialog on screen: the folder it asks
// about, and how many lines the cursor is from "Yes" (negative: above it).
type TrustDialog struct {
	Folder string
	ToYes  int
}

var (
	trustYes    = regexp.MustCompile(`(?i)^yes\b.*\btrust\b.*\bfolder\b`)
	trustCursor = regexp.MustCompile(`^[❯›>▶→]\s*`)
)

// ReadTrustDialog finds Claude Code's trust dialog on a screen.
func ReadTrustDialog(screen string) (TrustDialog, bool) {
	lines := strings.Split(PlainScreen(screen), "\n")
	asking, folder, yes, cursor := false, "", -1, -1
	for i, l := range lines {
		t := strings.TrimSpace(strings.Trim(l, " │┃|"))
		switch {
		case strings.HasPrefix(t, "Accessing workspace:"):
			if f := strings.TrimSpace(strings.TrimPrefix(t, "Accessing workspace:")); f != "" {
				folder = f
			} else {
				// On the next line with anything on it.
				for _, n := range lines[i+1:] {
					if folder = strings.TrimSpace(strings.Trim(n, " │┃|")); folder != "" {
						break
					}
				}
			}
		case strings.Contains(t, "one you trust?"), strings.Contains(t, "trust the files in this folder"):
			asking = true
		}
		opt := t
		if trustCursor.MatchString(t) {
			opt = trustCursor.ReplaceAllString(t, "")
			cursor = i
		}
		if trustYes.MatchString(opt) {
			yes = i
		}
	}
	if !asking || yes < 0 || cursor < 0 {
		return TrustDialog{}, false
	}
	return TrustDialog{Folder: folder, ToYes: yes - cursor}, true
}

// TrustKeys are the keys that answer a trust dialog yes: down (or up) to
// "Yes", then Enter.
func (d TrustDialog) TrustKeys() []uv.KeyPressEvent {
	var keys []uv.KeyPressEvent
	for i := 0; i < d.ToYes; i++ {
		keys = append(keys, uv.KeyPressEvent{Code: uv.KeyDown})
	}
	for i := 0; i > d.ToYes; i-- {
		keys = append(keys, uv.KeyPressEvent{Code: uv.KeyUp})
	}
	return append(keys, uv.KeyPressEvent{Code: uv.KeyEnter})
}
