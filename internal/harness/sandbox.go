package harness

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// A conversation's shell in a sandbox: what it's kept from writing, which
// commands leave it, and what ws says of one that may not.

// outside are the commands a sandboxed conversation runs outside its
// sandbox, when they're on its list: ws keeps the workspace's records,
// which the sandbox closes, and gh reads your login from the keychain,
// which it can't reach from inside.
var outside = []string{"ws", "workspace", "gh"}

// closedUnder is what under dir a sandboxed shell is kept from writing:
// everything there but the folders in open. Each thing in dir is named,
// but one with something open inside it, whose own things are named in
// its place: a sandbox lets the folder a run starts in be written
// (claudeSandbox), which for a conversation is the whole workspace, and
// what it's told to keep closed stays closed whatever's opened inside
// it. So too a folder in room: what's in it now is closed, each by name,
// and what's made in it afterwards isn't, as nothing new anywhere under
// dir is: a capsule lifted while the run goes on can be written in at
// once, and so can a new file beside what's there. Each turn lists it
// afresh.
func closedUnder(dir string, open, room []string) ([]string, error) {
	if _, err := os.ReadDir(dir); err != nil {
		return nil, fmt.Errorf("reading %s, to close it to a sandboxed shell: %w", dir, err)
	}
	inside := append(append([]string{}, open...), room...)
	var closed []string
	var walk func(dir string)
	walk = func(dir string) {
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			p := filepath.Join(dir, e.Name())
			switch {
			case slices.Contains(open, p):
			case e.IsDir() && (slices.Contains(room, p) || holdsAny(p, inside)):
				walk(p)
			default:
				closed = append(closed, p)
			}
		}
	}
	walk(dir)
	return closed, nil
}

// holdsAny is whether one of paths is inside dir.
func holdsAny(dir string, paths []string) bool {
	for _, p := range paths {
		if strings.HasPrefix(p, dir+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// looseCommands is a script's commands, each as its words, read loosely:
// where parseShell reads nothing at all of a script that pipes, redirects
// or expands, this still says what it runs, so ws can tell a script that
// runs ws or gh among other things. What's quoted is a word, not
// commands; a heredoc's body and where a redirection goes are neither.
func looseCommands(s string) [][]string {
	var out [][]string
	var words []string
	var pending []heredoc
	end := func() {
		if len(words) > 0 {
			out, words = append(out, words), nil
		}
	}
	for i := 0; i < len(s); {
		c := s[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r':
			i++
		case c == '\\' && i+1 < len(s) && s[i+1] == '\n':
			i += 2 // a line carried on
		case c == '\n':
			i++
			end()
			for _, h := range pending {
				for i < len(s) {
					stop := strings.IndexByte(s[i:], '\n')
					line, next := s[i:], len(s)
					if stop >= 0 {
						line, next = s[i:i+stop], i+stop+1
					}
					if h.tabbed {
						line = strings.TrimLeft(line, "\t")
					}
					i = next
					if line == h.delim {
						break
					}
				}
			}
			pending = nil
		case strings.HasPrefix(s[i:], "<<") && !strings.HasPrefix(s[i:], "<<<"):
			i += 2
			h := heredoc{}
			if i < len(s) && s[i] == '-' {
				h.tabbed = true
				i++
			}
			for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
				i++
			}
			h.delim, i = shellWord(s, i)
			if h.delim != "" {
				pending = append(pending, h)
			}
		case c == '<' || c == '>':
			// A redirection, and where it's to or from: not a command.
			for i < len(s) && strings.IndexByte("<>&|", s[i]) >= 0 {
				i++
			}
			for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
				i++
			}
			_, i = shellWord(s, i)
		case strings.IndexByte(";|&(){}`", c) >= 0:
			i++
			end()
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			i += 2
			end()
		case c == '#' && len(words) == 0:
			for i < len(s) && s[i] != '\n' {
				i++
			}
		default:
			word, next := shellWord(s, i)
			if next == i {
				next++ // a character no word starts with: on past it
			}
			i = next
			if word != "" {
				words = append(words, word)
			}
		}
	}
	end()
	return out
}

// runsFirst are words that run what follows them, or come before it: a
// command after one is as much run as one that starts the line. It's
// longer than it need be: a word wrongly on it only has ws say no where
// the agent's own rules would have.
var runsFirst = []string{"time", "timeout", "nice", "nohup", "stdbuf", "env", "command", "builtin", "exec", "eval", "sudo", "doas", "xargs", "caffeinate", "setsid", "ionice", "chrt", "arch", "watch", "unbuffer",
	"if", "then", "elif", "else", "while", "until", "do", "!"}

// commandStarts is the word each command in a script starts with, past
// the variables set before it.
func commandStarts(s string) []string {
	var out []string
	for _, words := range looseCommands(s) {
		if w := afterVars(words); len(w) > 0 {
			out = append(out, w[0])
		}
	}
	return out
}

// afterVars is a command's words past the variables set before it
// (FOO=bar cmd).
func afterVars(words []string) []string {
	for len(words) > 0 {
		eq := strings.IndexByte(words[0], '=')
		if eq <= 0 || strings.ContainsAny(words[0][:eq], "/-") {
			break
		}
		words = words[1:]
	}
	return words
}

// shellWord reads the word of a script starting at i, its quotes off: it's
// where the script goes on after it. "" where no word starts.
func shellWord(s string, i int) (string, int) {
	var w strings.Builder
	for i < len(s) {
		c := s[i]
		switch {
		case c == '\'':
			j := strings.IndexByte(s[i+1:], '\'')
			if j < 0 {
				return w.String() + s[i+1:], len(s)
			}
			w.WriteString(s[i+1 : i+1+j])
			i += j + 2
		case c == '"':
			i++
			for i < len(s) && s[i] != '"' {
				if s[i] == '\\' && i+1 < len(s) {
					i++
				}
				w.WriteByte(s[i])
				i++
			}
			i++
		case c == '\\' && i+1 < len(s):
			w.WriteByte(s[i+1])
			i += 2
		case strings.IndexByte(" \t\r\n;|&(){}<>`", c) >= 0:
			return w.String(), i
		case c == '$' && i+1 < len(s) && s[i+1] == '(':
			return w.String(), i
		default:
			w.WriteByte(c)
			i++
		}
	}
	return w.String(), i
}

// runsOutside is the first command of a script that only runs outside a
// conversation's sandbox (ws, workspace or gh, by its name), or "". One
// that starts a command, or comes anywhere after a word that runs what
// follows it (timeout 5 ws …, xargs gh …).
//
// It errs towards saying one's there. In a sandboxed run ws allows what
// this says runs neither (GuardAnswer), on Claude Code's word that such a
// command stays in the sandbox: so it must say so of every command Claude
// Code would let out, which it decides from the command's text, less the
// variables and the wrappers (timeout, time, nice, nohup, stdbuf) before
// it.
func runsOutside(cmd string) string {
	for _, words := range looseCommands(cmd) {
		words = afterVars(words)
		if len(words) == 0 {
			continue
		}
		if name := filepath.Base(words[0]); slices.Contains(outside, name) {
			return name
		}
		if slices.Contains(runsFirst, filepath.Base(words[0])) {
			for _, w := range words[1:] {
				if name := filepath.Base(w); slices.Contains(outside, name) {
					return name
				}
			}
		}
	}
	return ""
}

// whyRefused is what ws says to a conversation's agent of a command it
// may not run, which runs ws or gh: why not, and what to do in its place.
// "" for a command that runs neither, which ws has nothing to say of (the
// agent's own rules, or its sandbox, decide it).
//
// A refusal that only says no is where a conversation stops being of use:
// its agent reads Claude Code's own ("Permission to use Bash has been
// denied") as the shell being closed to it, and gives up on what it was
// asked. So each says the way through.
func whyRefused(cmd string, allow []string) string {
	name := runsOutside(cmd)
	if name == "" {
		return ""
	}
	cmds, ok := parseShell(unwrapScript(cmd))
	if !ok || len(cmds) != 1 || filepath.Base(cmds[0][0]) != name {
		return "ws refused this: a " + name + " command runs on its own, as one command: not piped, chained or redirected to a file, not behind another command or a variable set before it (timeout, env, FOO=1), and with nothing a shell would expand ($…, backticks). What it reads, give it with a heredoc whose delimiter is quoted (<<'EOF'). Run the " + name + " command alone, then go on from what it prints."
	}
	words := cmds[0]
	said := strings.Join(words[:min(len(words), 4)], " ")
	if name == "gh" {
		return "ws refused `" + said + "`: of gh, a conversation's agent reads (" + strings.Join(ghReads(allow), ", ") + "). To comment on a pull request, propose the comment (`ws pr comment --propose`). A pull request is opened by a mission: commit what you've done, and propose one (`ws mission new --propose`) whose brief names your branch. To update one that's open, work in a capsule on its branch (`ws dock <repo> <its number>`) and, when the user asks, `git push` there. The rest of your shell works as before."
	}
	if verb := proposable(words, allow); verb != "" {
		return "ws refused `" + said + "`: you're in a conversation, so you propose it, and the user confirms it with a button under your reply. Run it again with --propose straight after `" + verb + "`."
	}
	return "ws refused `" + said + "`: that isn't one of the ws commands a conversation's agent runs (your first message lists them: ws show, ws find, ws status and their kin, and the --propose forms). If the user wants it done, say what to run in your reply, and they run it; if it's work, propose a mission for it (`ws mission new --propose`). The rest of your shell works as before."
}

// whyNotWritten is what ws says to a conversation's agent of a file its
// tools may not write: where they may, and how to get to write in a repo.
func whyNotWritten(path string, dirs []string) string {
	where := "nowhere yet"
	if len(dirs) > 0 {
		where = "in " + strings.Join(dirs, ", ")
	}
	return "ws refused it: " + path + " isn't yours to write. Your tools for writing and editing files write " + where + " (a folder of your own, and the capsules this conversation has lifted). To change a repo's files, lift a capsule of your own (`ws lift <repo> <name>`), dock one on a pull request's branch (`ws dock <repo> <its number>`), or join one that's there and no mission's (`ws lift <repo> <its name>`), and write in it; a repo's .ground and a mission's capsules stay as they are."
}

// within is whether path, as it resolves, is in one of dirs. A file that
// isn't there yet goes by the folder it would be in.
func within(path string, dirs []string) bool {
	path = filepath.Clean(path)
	if !filepath.IsAbs(path) {
		return false
	}
	if r, err := filepath.EvalSymlinks(path); err == nil {
		path = r
	} else if r, err := filepath.EvalSymlinks(filepath.Dir(path)); err == nil {
		path = filepath.Join(r, filepath.Base(path))
	}
	for _, d := range dirs {
		if d != "" && strings.HasPrefix(path, strings.TrimSuffix(d, string(filepath.Separator))+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// ghReads is what of gh the list allows, as it's said.
func ghReads(allow []string) []string {
	var out []string
	for _, a := range allow {
		if strings.HasPrefix(a, "gh ") {
			out = append(out, a)
		}
	}
	return out
}

// proposable is the verb words run without --propose, when the list
// allows it with (ws mission new, of "ws mission new --propose"), or "".
func proposable(words []string, allow []string) string {
	for _, a := range allow {
		verb, ok := strings.CutSuffix(a, " --propose")
		if !ok {
			continue
		}
		want := strings.Fields(verb)
		if len(words) >= len(want) && slices.Equal(words[:len(want)], want) {
			return verb
		}
	}
	return ""
}

// StdinFlag is the flag ws takes its standard input by, from a file: what
// a heredoc gave a command, once ws has moved it there (heredocAside).
const StdinFlag = "--stdin-from"

// asideDir is the folder, beside the guard's allow list, that input set
// aside for a command is kept in.
const asideDir = "stdin"

// SetAside is whether path is input ws set aside for a command: a file
// that's ws's to remove once it's read, as no other given to --stdin-from
// is.
func SetAside(path string) bool {
	dir := filepath.Dir(path)
	return filepath.Base(dir) == asideDir && filepath.Base(filepath.Dir(dir)) == "claude"
}

// heredocAside is a ws command given its input by a heredoc, as the same
// command reading it from a file ws keeps in dir (ws <verb> … --stdin-from
// <file>, which removes it once read), or "" for any other command. A
// sandbox keeps a command with a redirection inside it, a heredoc among
// them, where ws can't write what's proposed; without one, the command
// runs as it would have.
func heredocAside(cmd, dir string) string {
	cmds, bodies, ok := parseScript(unwrapScript(cmd))
	if !ok || len(cmds) != 1 || len(bodies) != 1 || len(cmds[0]) == 0 {
		return ""
	}
	if name := cmds[0][0]; name != "ws" && name != "workspace" {
		return ""
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ""
	}
	sweepOld(dir, time.Hour)
	var id [8]byte
	if _, err := rand.Read(id[:]); err != nil {
		return ""
	}
	file := filepath.Join(dir, hex.EncodeToString(id[:]))
	if err := os.WriteFile(file, []byte(bodies[0]), 0o600); err != nil {
		return ""
	}
	var words []string
	for _, w := range append(append([]string{}, cmds[0]...), StdinFlag, file) {
		words = append(words, shellQuote(w))
	}
	return strings.Join(words, " ")
}

// sweepOld removes the files in dir last written longer ago than age:
// input set aside for a command that never ran.
func sweepOld(dir string, age time.Duration) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if info, err := e.Info(); err == nil && !info.IsDir() && time.Since(info.ModTime()) > age {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
