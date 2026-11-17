package harness

import (
	"regexp"
	"strings"
)

// Prompts on an agent's screen: the menus its TUI draws when it stops to
// ask (a permission, a question), read off the screen so the web UI can
// answer one with the keys a person would press. Reading the screen first
// is what keeps it safe: if what's drawn isn't the menu expected, nothing
// is typed.

// Prompt is a menu on screen: its title (the lines just above), its
// options in order, and which one the cursor's on.
type Prompt struct {
	Title    string
	Options  []string
	Selected int
	// Context is what's drawn above it, back to the last rule across the
	// screen: what it's asking about (the command, the file).
	Context []string
}

var (
	ansi = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)|\x1b[()][A-Za-z0-9]`)
	// "❯ 1. Yes", "  2. No, and tell Claude what to do differently (esc)",
	// inside a box's borders or not.
	optionLine = regexp.MustCompile(`^[\s│┃|]*([❯›>▶→])?\s*(\d{1,2})[.)]\s+(.*?)[\s│┃|]*$`)
	boxOnly    = regexp.MustCompile(`^[\s│┃|─━╭╮╰╯┌┐└┘╌┄·•]*$`)
)

// PlainScreen is a screen's text without its colours and cursor codes.
func PlainScreen(screen string) string { return ansi.ReplaceAllString(screen, "") }

// ReadPrompt finds the last numbered menu on screen (options 1, 2, … on
// lines one after another, or with a line between): the one the agent is
// waiting on.
func ReadPrompt(screen string) (Prompt, bool) {
	lines := strings.Split(PlainScreen(screen), "\n")
	var best Prompt
	found := false
	for i := 0; i < len(lines); i++ {
		m := optionLine.FindStringSubmatch(lines[i])
		if m == nil || m[2] != "1" {
			continue
		}
		p := Prompt{Selected: -1}
		next := 1
		j := i
		for ; j < len(lines); j++ {
			m := optionLine.FindStringSubmatch(lines[j])
			if m == nil {
				// A wrapped option, or a description under one: allowed
				// once, then the menu's over.
				if j+1 < len(lines) && optionLine.MatchString(lines[j+1]) && strings.TrimSpace(lines[j]) != "" {
					continue
				}
				break
			}
			if m[2] != itoa(next) {
				break
			}
			if m[1] != "" {
				p.Selected = next - 1
			}
			p.Options = append(p.Options, strings.TrimSpace(m[3]))
			next++
		}
		if len(p.Options) < 2 {
			continue
		}
		if p.Selected < 0 {
			p.Selected = 0
		}
		// Its title: the nearest lines above with words in them.
		var title []string
		for k := i - 1; k >= 0 && len(title) < 3; k-- {
			t := strings.Trim(lines[k], " │┃|")
			if boxOnly.MatchString(lines[k]) || t == "" {
				if len(title) > 0 {
					break
				}
				continue
			}
			title = append([]string{strings.TrimSpace(t)}, title...)
		}
		p.Title = strings.Join(title, " ")
		// What it's about: the lines above its title, to the rule above.
		for k := i - 1 - len(title); k >= 0 && len(p.Context) < 12; k-- {
			if strings.Contains(lines[k], "────") {
				break
			}
			if t := strings.TrimSpace(strings.Trim(lines[k], " │┃|")); t != "" && !boxOnly.MatchString(lines[k]) {
				p.Context = append([]string{t}, p.Context...)
			}
		}
		best, found = p, true
		i = j
	}
	return best, found
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return string(rune('0'+n/10)) + string(rune('0'+n%10))
}

// IsPermission is whether a prompt asks permission for something: Claude
// Code's "Do you want to proceed?" (or to make an edit, create a file…).
func (p Prompt) IsPermission() bool {
	t := strings.ToLower(p.Title)
	return strings.Contains(t, "do you want to") || strings.Contains(t, "allow") || strings.Contains(t, "permission")
}

// PermissionChoice is which of a permission prompt's options means allow
// once, always allow (this kind of thing, from now on) or deny; -1 if it
// hasn't one.
func (p Prompt) PermissionChoice(want string) int {
	for i, o := range p.Options {
		l := strings.ToLower(o)
		always := strings.Contains(l, "don't ask again") || strings.Contains(l, "always") || strings.Contains(l, "allow all")
		switch want {
		case "allow":
			if strings.HasPrefix(l, "yes") && !always {
				return i
			}
		case "always":
			if strings.HasPrefix(l, "yes") && always {
				return i
			}
		case "deny":
			if strings.HasPrefix(l, "no") {
				return i
			}
		}
	}
	return -1
}

// Option is the index of the option labelled label (as the agent's
// question tool gave it), or -1.
func (p Prompt) Option(label string) int {
	want := strings.ToLower(strings.TrimSpace(label))
	for i, o := range p.Options {
		if l := strings.ToLower(o); l == want || strings.HasPrefix(l, want) {
			return i
		}
	}
	return -1
}

// FreeText is the option that takes your own words ("Type something",
// "Other"), or -1.
func (p Prompt) FreeText() int {
	for i, o := range p.Options {
		l := strings.ToLower(o)
		if strings.HasPrefix(l, "type something") || strings.HasPrefix(l, "other") || strings.HasPrefix(l, "something else") {
			return i
		}
	}
	return -1
}

// Moves are the cursor moves to option i from where it is: down (positive)
// or up (negative) that many times, then Enter.
func (p Prompt) Moves(i int) int { return i - p.Selected }
