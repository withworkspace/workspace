package automation

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const trend = `# Keep a number each day, and show its trend.
when = every("day 03:00")
may = ["output"]

def run(e):
    n = len(kept("coverage")) + 70
    keep("coverage", n)
    points = kept("coverage", since = "30d")
    output(chart(points, title = "Coverage", unit = "%") + "\n\nToday **%d%%**." % n)
`

// A run keeps a number in a series of its own, and the next run's given
// them all; its output is written whole each time, its versions kept, a
// chart a ```chart block of the points.
func TestOutput_KeepsAndShows(t *testing.T) {
	e, _ := engine(t)
	later := clock(e)
	if _, err := e.Add("trend", []byte(trend)); err != nil {
		t.Fatal(err)
	}
	for range 3 {
		later(24 * time.Hour)
		e.RunDue()
	}
	points, _ := Series(e.Root, "trend", "coverage", time.Time{})
	if len(points) != 3 || points[0].Value != 70 || points[2].Value != 72 {
		t.Fatalf("points %+v", points)
	}
	shown, all, err := Outputs(e.Root, "trend", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 || shown.Version != 3 || shown.Format != OutputMarkdown || !strings.HasSuffix(shown.Body, "Today **72%**.") {
		t.Fatalf("output %+v (%d versions)", shown, len(all))
	}
	block := strings.TrimPrefix(strings.SplitN(shown.Body, "\n```\n", 2)[0], "```chart\n")
	var c struct {
		Title  string `json:"title"`
		Unit   string `json:"unit"`
		Series []struct {
			Points [][2]any `json:"points"`
		} `json:"series"`
	}
	if err := json.Unmarshal([]byte(block), &c); err != nil || c.Title != "Coverage" || c.Unit != "%" || len(c.Series) != 1 || len(c.Series[0].Points) != 3 {
		t.Errorf("chart %q: %+v %v", block, c, err)
	}
	if first, _, _ := Outputs(e.Root, "trend", 1); !strings.HasSuffix(first.Body, "Today **70%**.") {
		t.Errorf("version 1: %q", first.Body)
	}
}

// Without may output, it shows nothing; a number's what's kept.
func TestOutput_Refused(t *testing.T) {
	e, _ := engine(t)
	later := clock(e)
	e.Add("quiet", []byte(strings.Replace(trend, `may = ["output"]`, "", 1)))
	e.Add("words", []byte(strings.Replace(trend, `keep("coverage", n)`, `keep("coverage", "lots")`, 1)))
	later(25 * time.Hour)
	e.RunDue()
	if runs, _ := Runs(e.Root, "quiet", 1); len(runs) != 1 || !strings.Contains(runs[0].Error, "output: it may not") {
		t.Errorf("quiet: %+v", runs)
	}
	if runs, _ := Runs(e.Root, "words", 1); len(runs) != 1 || !strings.Contains(runs[0].Error, "keeps numbers") {
		t.Errorf("words: %+v", runs)
	}
}

// A dry run keeps what it keeps across its runs, in memory, and says
// what it'd show now: the card's preview.
func TestOutput_DryRunPreview(t *testing.T) {
	e, _ := engine(t)
	a, err := Check("trend", []byte(trend))
	if err != nil {
		t.Fatal(err)
	}
	dr, err := e.DryRun(a, 5*24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if dr.Output == nil || !strings.Contains(dr.Output.Body, "Today **7") || dr.Ran < 4 {
		t.Fatalf("%+v", dr)
	}
	if !strings.Contains(dr.Output.Body, "Today **"+itoa(70+dr.Ran-1)+"%**") {
		t.Errorf("kept across its runs: %q after %d", dr.Output.Body, dr.Ran)
	}
	if _, all, _ := Outputs(e.Root, "trend", 0); len(all) != 0 {
		t.Errorf("a dry run wrote %d", len(all))
	}
}
