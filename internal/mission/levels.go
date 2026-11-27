package mission

import (
	"fmt"
	"regexp"
	"strings"
)

// Crew levels: how hard a Work step is, as the planner judges it, so ws
// can run rote work on a smaller model and keep the larger for what needs
// it ([crew.levels] in ws.toml names the model for each). A step says its
// level with - level:, or none, for normal. Rote is held to having a check
// ws runs: a command is what shows a small model did the work, where a
// check in words is judged by a small model too. A rote step whose checks
// come round unmet is started again a level up (ops' stepUp).

// The levels, from least to most.
const (
	LevelRote   = "rote"
	LevelNormal = "normal"
	LevelHard   = "hard"
)

// Levels are the levels in order, least first.
var Levels = []string{LevelRote, LevelNormal, LevelHard}

// LevelOf is a step's level: as it says, or normal.
func (s Step) LevelOf() string {
	if s.Level == "" {
		return LevelNormal
	}
	return s.Level
}

// Above is the level after level ("" for the top one, or one that isn't).
func Above(level string) string {
	for i, l := range Levels {
		if l == level && i+1 < len(Levels) {
			return Levels[i+1]
		}
	}
	return ""
}

// IsLevel reports whether level is one of the levels.
func IsLevel(level string) bool {
	for _, l := range Levels {
		if l == level {
			return true
		}
	}
	return false
}

// RunsACheck reports whether the step has a check ws runs: a command.
func (s Step) RunsACheck() bool {
	for _, c := range s.Checks {
		if c.Run() {
			return true
		}
	}
	return false
}

var paramLevel = StepParam{Key: "level", About: "how hard the work is, which ws runs it on a model for ([crew.levels]): rote, for mechanical work with a command check that shows it's done (a rename, a version bump, snapshots or generated code made again): it runs on a small model, and if its checks come round unmet ws starts it again on the normal one; hard, for work that needs the most thought (finding a cause, a design call, a change across repos, code without tests); normal, or none, for the rest. Not with on:, which names a model itself",
	read: func(st *Step, v string) string {
		l := strings.ToLower(strings.TrimRight(strings.TrimSpace(v), "."))
		if !IsLevel(l) {
			return fmt.Sprintf("level: %q is not a level: write rote, normal or hard", v)
		}
		st.Level = l
		return ""
	}}

// levelsHeld holds each step's level to what it can be: rote only with a
// command check, and a level and an on: not both.
func (p *planParser) levelsHeld() {
	for _, st := range p.steps {
		if st.Level == "" {
			continue
		}
		if st.Model != "" {
			p.problem(st.Line, "step %d has both level: and on: choose one (level: for ws to pick its model, on: to name one)", st.N)
		}
		if st.Level == LevelRote && !st.RunsACheck() {
			p.problem(st.Line, "step %d is rote but has no check ws runs: a rote step runs on a small model, and only a command (`go test ./...` passes) shows it did the work. Add one, or make it normal", st.N)
		}
	}
}

// planFieldLine is one of a step's parameters in a plan, indented, with
// its key.
var planFieldLine = regexp.MustCompile(`^(\s+)[-*+]\s+([A-Za-z]+(?: [A-Za-z]+)*)\s*:`)

// WithLevel is a plan as written with step n at level: its - level: line
// written (or replaced), and any - on: taken out, as a level picks the
// model in its place. The rest is as it was, word for word. It's read
// again, so a level the step can't have (rote, with no check ws runs) is
// refused as ParsePlan would.
func WithLevel(written string, n int, level string) (string, error) {
	if !IsLevel(level) {
		return "", fmt.Errorf("%q is not a level: rote, normal or hard", level)
	}
	p, err := ParsePlan(written, nil)
	if err != nil {
		return "", err
	}
	if n < 1 || n > len(p.Steps) {
		return "", fmt.Errorf("the plan has no step %d", n)
	}
	st := p.Steps[n-1]
	if st.Type != StepWork {
		return "", fmt.Errorf("step %d is a %s step: only Work has a level", n, st.Type)
	}
	lines := strings.Split(strings.ReplaceAll(written, "\r\n", "\n"), "\n")
	// The step runs from its line to the next line that isn't indented.
	start := st.Line - 1
	end := start + 1
	for end < len(lines) {
		l := lines[end]
		if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, " ") && !strings.HasPrefix(l, "\t") {
			break
		}
		end++
	}
	indent := "   "
	last := start // the step's last non-blank line
	var out []string
	out = append(out, lines[:start+1]...)
	replaced := false
	for i := start + 1; i < end; i++ {
		l := lines[i]
		if m := planFieldLine.FindStringSubmatch(l); m != nil {
			indent = m[1]
			switch strings.ToLower(m[2]) {
			case "on":
				continue
			case "level":
				out = append(out, m[1]+"- level: "+level)
				replaced = true
				continue
			}
		}
		out = append(out, l)
	}
	if !replaced {
		// After the step's last non-blank line, so it's among its params.
		for i := len(out) - 1; i > start; i-- {
			if strings.TrimSpace(out[i]) != "" {
				last = i
				break
			}
		}
		out = append(out[:last+1], append([]string{indent + "- level: " + level}, out[last+1:]...)...)
	}
	out = append(out, lines[end:]...)
	text := strings.Join(out, "\n")
	if _, err := ParsePlan(text, nil); err != nil {
		return "", err
	}
	return text, nil
}
