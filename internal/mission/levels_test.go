package mission

import (
	"strings"
	"testing"
)

const levelsPlan = `## Z
- Done.

## Stops early
- None.

## Steps
1. Work: **Bump.** Bump the version.
   - in: ` + "`api`" + `
   - level: rote
   - check: ` + "`go test ./...`" + ` passes
2. Work: **Find it.** Find the cause.
   - in: ` + "`api`" + `
   - level: hard
   - check: the cause is named
3. Work: **Fix.** Fix it.
   - in: ` + "`api`" + `
   - on: opus
   - check: it's fixed
4. Review: **Look.** Look at it.
`

func TestPlanLevels(t *testing.T) {
	p, err := ParsePlan(levelsPlan, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{LevelRote, LevelHard, LevelNormal} {
		if got := p.Steps[i].LevelOf(); got != want {
			t.Errorf("step %d level = %q, want %q", i+1, got, want)
		}
	}
	if got := p.Steps[0].WhoSays(); got != "crew, in `api`, rote" {
		t.Errorf("who says = %q", got)
	}
}

func TestPlanLevelsRefused(t *testing.T) {
	for name, c := range map[string]struct{ plan, says string }{
		"rote with no command": {strings.Replace(levelsPlan, "   - level: hard", "   - level: rote", 1), "step 2 is rote but has no check ws runs"},
		"level and on":         {strings.Replace(levelsPlan, "   - on: opus", "   - on: opus\n   - level: hard", 1), "step 3 has both level: and on:"},
		"not a level":          {strings.Replace(levelsPlan, "level: hard", "level: tricky", 1), `level: "tricky" is not a level`},
		"not on a review":      {levelsPlan + "   - level: rote\n", "level: does not apply to a Review step"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ParsePlan(c.plan, nil)
			if err == nil || !strings.Contains(err.Error(), c.says) {
				t.Fatalf("err = %v, want it to say %q", err, c.says)
			}
		})
	}
}

func TestAbove(t *testing.T) {
	if Above(LevelRote) != LevelNormal || Above(LevelNormal) != LevelHard || Above(LevelHard) != "" || Above("x") != "" {
		t.Error("levels out of order")
	}
}

func TestWithLevel(t *testing.T) {
	t.Run("replaces a level", func(t *testing.T) {
		got, err := WithLevel(levelsPlan, 2, LevelNormal)
		if err != nil {
			t.Fatal(err)
		}
		if want := strings.Replace(levelsPlan, "level: hard", "level: normal", 1); got != want {
			t.Errorf("got\n%s\nwant\n%s", got, want)
		}
	})
	t.Run("adds one, taking out on:", func(t *testing.T) {
		got, err := WithLevel(levelsPlan, 3, LevelHard)
		if err != nil {
			t.Fatal(err)
		}
		want := strings.Replace(levelsPlan, "   - on: opus\n   - check: it's fixed\n", "   - check: it's fixed\n   - level: hard\n", 1)
		if got != want {
			t.Errorf("got\n%s\nwant\n%s", got, want)
		}
		p, _ := ParsePlan(got, nil)
		if s := p.Steps[2]; s.Level != LevelHard || s.Model != "" {
			t.Errorf("step 3 = level %q, model %q", s.Level, s.Model)
		}
	})
	t.Run("refuses rote with no command check", func(t *testing.T) {
		if _, err := WithLevel(levelsPlan, 2, LevelRote); err == nil || !strings.Contains(err.Error(), "no check ws runs") {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("refuses a step that isn't Work", func(t *testing.T) {
		if _, err := WithLevel(levelsPlan, 4, LevelHard); err == nil {
			t.Error("want refused")
		}
	})
}
