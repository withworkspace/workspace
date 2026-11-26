package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/mission"
)

// Each orchestrator run is kept with its mission: what it looked at, the
// prompt it was given, what it said, and each call it made, refused ones
// marked. When a run goes wrong, this is what says why.

// Report is what a run printed: its final message, and the calls it made
// when its harness reports them.
type Report struct {
	Output string
	Calls  []Call
}

// Call is one tool call a run made, and how it went.
type Call = harness.ToolCall

// RunRecord is one run, as kept.
type RunRecord struct {
	ID      string    `json:"id"`
	Started time.Time `json:"started"`
	Ended   time.Time `json:"ended"`
	For     string    `json:"for"` // what it looked at: "c2 (done)"
	Trigger string    `json:"trigger"`
	Crew    string    `json:"crew,omitempty"`
	Harness string    `json:"harness"`
	Model   string    `json:"model,omitempty"`
	// Retry: it ran again because the run before ran no ws mission
	// command.
	Retry  bool   `json:"retry,omitempty"`
	Prompt string `json:"prompt"`
	Output string `json:"output"`
	Calls  []Call `json:"calls,omitempty"`
	// Acted: it ran a ws mission command (its note, or something that
	// placed the ball).
	Acted bool   `json:"acted"`
	Error string `json:"error,omitempty"`
}

// keepRuns is how many runs a mission keeps; older ones go.
const keepRuns = 50

func runsDir(store mission.Store, slug string) string {
	return filepath.Join(store.Dir(), slug, "orchestrator")
}

// NewRunID is an id for a run starting at, from when it started, that no
// run kept has.
func NewRunID(store mission.Store, slug string, at time.Time) string {
	dir := runsDir(store, slug)
	base := at.UTC().Format("20060102-150405")
	id := base
	for n := 2; ; n++ {
		_, errJ := os.Stat(filepath.Join(dir, id+".json"))
		_, errP := os.Stat(filepath.Join(dir, id+".prompt.md"))
		if os.IsNotExist(errJ) && os.IsNotExist(errP) {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, n)
	}
}

// SaveRun keeps a run (one with no id is given one from when it started),
// and lets the oldest go past keepRuns.
func SaveRun(store mission.Store, slug string, r *RunRecord) error {
	dir := runsDir(store, slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if r.ID == "" {
		r.ID = NewRunID(store, slug, r.Started)
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(dir, r.ID+".json"), data, 0o644); err != nil {
		return err
	}
	ids := runIDs(dir)
	for len(ids) > keepRuns {
		os.Remove(filepath.Join(dir, ids[0]+".json"))
		os.Remove(filepath.Join(dir, ids[0]+".prompt.md"))
		ids = ids[1:]
	}
	return nil
}

// runIDs are a mission's kept runs, oldest first.
func runIDs(dir string) []string {
	entries, _ := os.ReadDir(dir)
	var ids []string
	for _, e := range entries {
		if id, ok := strings.CutSuffix(e.Name(), ".json"); ok {
			ids = append(ids, id)
		}
	}
	sort.Strings(ids)
	return ids
}

// Runs are a mission's kept runs, newest first, without their prompts.
func Runs(store mission.Store, slug string) []RunRecord {
	dir := runsDir(store, slug)
	ids := runIDs(dir)
	out := make([]RunRecord, 0, len(ids))
	for _, id := range slices.Backward(ids) {
		if r, err := LoadRun(store, slug, id); err == nil {
			r.Prompt = ""
			out = append(out, *r)
		}
	}
	return out
}

// LoadRun is one kept run, in full.
func LoadRun(store mission.Store, slug, id string) (*RunRecord, error) {
	if strings.ContainsAny(id, `/\`) || strings.HasPrefix(id, ".") {
		return nil, os.ErrNotExist
	}
	data, err := os.ReadFile(filepath.Join(runsDir(store, slug), id+".json"))
	if err != nil {
		return nil, err
	}
	var r RunRecord
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}
