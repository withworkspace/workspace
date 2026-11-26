package automation

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"go.starlark.net/starlark"
	"go.starlark.net/starlarkstruct"

	"github.com/withworkspace/workspace/internal/store"
)

// What an automation keeps, and what it shows. keep(series, value) keeps
// a number as of now in a series of its own, and kept(series) gives them
// back: a run starts with nothing, so what's over time (coverage's trend,
// how many merged a week) is kept so. output(markdown) is what it shows
// you: one page of its own, markdown or (html = …) a page in a frame,
// written whole by each run, read-only, its last versions kept. A chart
// is markdown's ```chart block, which chart(points) writes: ws draws it.

// MayOutput is writing its output.
const MayOutput = "output"

// Formats an output's in.
const (
	OutputMarkdown = "markdown"
	OutputHTML     = "html"
)

// outputsKept is how many versions of an output are kept; outputMost the
// most an output can be.
const (
	outputsKept = 52
	outputMost  = 1 << 20
)

func init() {
	store.Define("automation-output", store.SQL(`
		-- A number an automation kept (keep), in a series of its own.
		CREATE TABLE automation_series (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			series TEXT NOT NULL,
			at TEXT NOT NULL,
			event INTEGER NOT NULL,
			value REAL NOT NULL
		);
		CREATE INDEX automation_series_at ON automation_series (name, series, at);
		-- What an automation shows (output): each version, its last kept.
		CREATE TABLE automation_outputs (
			name TEXT NOT NULL,
			version INTEGER NOT NULL,
			at TEXT NOT NULL,
			event INTEGER NOT NULL,
			format TEXT NOT NULL,
			body TEXT NOT NULL,
			PRIMARY KEY (name, version)
		) WITHOUT ROWID;
	`))
}

// Point is a number kept in a series, and when.
type Point struct {
	At    time.Time `json:"at"`
	Value float64   `json:"value"`
}

// Output is a version of what an automation shows.
type Output struct {
	Name    string    `json:"name"`
	Version int       `json:"version"`
	At      time.Time `json:"at"`
	Event   int64     `json:"event"`
	Format  string    `json:"format"`
	Body    string    `json:"body"`
}

func keepPoint(root, name, series string, event int64, at time.Time, v float64) error {
	return write(root, func(tx *store.Tx) error {
		_, err := tx.Exec(`INSERT INTO automation_series (name, series, at, event, value) VALUES (?, ?, ?, ?, ?)`, name, series, stampOf(at), event, v)
		return err
	})
}

// Series is name's series, its points since then, oldest first.
func Series(root, name, series string, since time.Time) ([]Point, error) {
	out := []Point{}
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT at, value FROM automation_series WHERE name = ? AND series = ? AND at >= ? ORDER BY at, id`, name, series, stampOf(since))
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var at string
			var p Point
			if err := rows.Scan(&at, &p.Value); err != nil {
				return err
			}
			p.At = timeOf(at)
			out = append(out, p)
		}
		return rows.Err()
	})
	return out, err
}

func writeOutput(root string, o Output) error {
	return write(root, func(tx *store.Tx) error {
		var latest sql.NullInt64
		if err := tx.QueryRow(`SELECT max(version) FROM automation_outputs WHERE name = ?`, o.Name).Scan(&latest); err != nil {
			return err
		}
		o.Version = int(latest.Int64) + 1
		if _, err := tx.Exec(`INSERT INTO automation_outputs (name, version, at, event, format, body) VALUES (?, ?, ?, ?, ?, ?)`, o.Name, o.Version, stampOf(o.At), o.Event, o.Format, o.Body); err != nil {
			return err
		}
		_, err := tx.Exec(`DELETE FROM automation_outputs WHERE name = ? AND version <= ?`, o.Name, o.Version-outputsKept)
		return err
	})
}

// Outputs are name's kept versions of its output, the latest first, with
// their bodies only for the one asked for (version, 0 for the latest).
func Outputs(root, name string, version int) (Output, []Output, error) {
	var shown Output
	var all []Output
	err := read(root, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT version, at, event, format FROM automation_outputs WHERE name = ? ORDER BY version DESC`, name)
		if err != nil {
			return err
		}
		for rows.Next() {
			o := Output{Name: name}
			var at string
			if err := rows.Scan(&o.Version, &at, &o.Event, &o.Format); err != nil {
				rows.Close()
				return err
			}
			o.At = timeOf(at)
			all = append(all, o)
		}
		rows.Close()
		if err := rows.Err(); err != nil || len(all) == 0 {
			return err
		}
		if version == 0 {
			version = all[0].Version
		}
		shown = Output{Name: name, Version: version}
		var at string
		err = tx.QueryRow(`SELECT at, event, format, body FROM automation_outputs WHERE name = ? AND version = ?`, name, version).Scan(&at, &shown.Event, &shown.Format, &shown.Body)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNoOutput
		}
		shown.At = timeOf(at)
		return err
	})
	return shown, all, err
}

// ErrNoOutput is the answer for a version of an output there isn't.
var ErrNoOutput = errors.New("there's no such version of its output: the last 52 are kept")

// keepBuiltin is keep(series, value): a number kept as of now.
func (r *run) keepBuiltin(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var series string
	var value starlark.Value
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "series", &series, "value", &value); err != nil {
		return nil, err
	}
	v, ok := starlark.AsFloat(value)
	if !ok || math.IsNaN(v) || math.IsInf(v, 0) {
		return nil, fmt.Errorf("keep: %s is a %s: it keeps numbers", series, value.Type())
	}
	if series == "" {
		return nil, fmt.Errorf("keep: say which series, as keep(\"coverage\", 71.4)")
	}
	_, err := r.step("keep", func() (string, error) {
		if r.dry != nil {
			r.dry.kept[series] = append(r.dry.kept[series], Point{At: r.ev.At, Value: v})
			return "", nil
		}
		return "", keepPoint(r.e.Root, r.a.Name, series, r.ev.N, r.e.now(), v)
	})
	return starlark.None, err
}

// keptBuiltin is kept(series, since = "90d"): its points, oldest first,
// each .at (as 2026-11-26T03:00:00Z) and .value.
func (r *run) keptBuiltin(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var series string
	since := "90d"
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "series", &series, "since?", &since); err != nil {
		return nil, err
	}
	d, err := lengthOf(since)
	if err != nil {
		return nil, fmt.Errorf("kept: since: %v", err)
	}
	got, err := r.read("kept", func() (any, error) {
		if r.dry != nil {
			// A dry run's are those it kept itself, as it went.
			var out []Point
			for _, p := range r.dry.kept[series] {
				if !p.At.Before(r.ev.At.Add(-d)) {
					out = append(out, p)
				}
			}
			return out, nil
		}
		return Series(r.e.Root, r.a.Name, series, r.e.now().Add(-d))
	})
	if err != nil {
		return nil, err
	}
	if got == nil {
		return starlark.NewList(nil), nil
	}
	return valueOf(got), nil
}

// outputBuiltin is output(markdown) or output(html = …): what it shows,
// written whole.
func (r *run) outputBuiltin(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	if !r.a.Allows(MayOutput) {
		return nil, fmt.Errorf("output: it may not (may = [\"output\"] says it may)")
	}
	var markdown, html string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "markdown?", &markdown, "html?", &html); err != nil {
		return nil, err
	}
	o := Output{Name: r.a.Name, Event: r.ev.N, At: r.e.now(), Format: OutputMarkdown, Body: markdown}
	if html != "" {
		if markdown != "" {
			return nil, fmt.Errorf("output: markdown or html, not both")
		}
		o.Format, o.Body = OutputHTML, html
	}
	if len(o.Body) > outputMost {
		return nil, fmt.Errorf("output: it's %d KB, and can be at most %d KB", len(o.Body)>>10, outputMost>>10)
	}
	_, err := r.step("output", func() (string, error) {
		if r.dry != nil {
			o.At = r.ev.At
			r.dry.output = &o
			r.dry.did = append(r.dry.did, "output: its page, as "+o.Format)
			return "", nil
		}
		return "", writeOutput(r.e.Root, o)
	})
	return starlark.None, err
}

// chartBuiltin is chart(points, title = …, unit = …): markdown's ```chart
// block of them, for output. points is kept()'s, or a dict of series
// names to theirs, each a line.
func chartBuiltin(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var points starlark.Value
	var title, unit string
	if err := starlark.UnpackArgs(b.Name(), args, kwargs, "points", &points, "title?", &title, "unit?", &unit); err != nil {
		return nil, err
	}
	type line struct {
		Name   string   `json:"name"`
		Points [][2]any `json:"points"`
	}
	var lines []line
	add := func(name string, v starlark.Value) error {
		ps, err := pointsOf(v)
		if err != nil {
			return err
		}
		lines = append(lines, line{Name: name, Points: ps})
		return nil
	}
	if d, ok := points.(*starlark.Dict); ok {
		for _, item := range d.Items() {
			name, _ := starlark.AsString(item[0])
			if err := add(name, item[1]); err != nil {
				return nil, err
			}
		}
	} else if err := add(title, points); err != nil {
		return nil, err
	}
	block, _ := json.Marshal(map[string]any{"title": title, "unit": unit, "series": lines})
	return starlark.String("```chart\n" + string(block) + "\n```\n"), nil
}

// pointsOf is a list of kept()'s points (each .at and .value), or of
// (at, value) pairs, as a chart's.
func pointsOf(v starlark.Value) ([][2]any, error) {
	iterable, ok := v.(starlark.Iterable)
	if !ok {
		return nil, fmt.Errorf("chart: points is a %s: kept()'s, or a list of (at, value)", v.Type())
	}
	var out [][2]any
	iter := iterable.Iterate()
	defer iter.Done()
	var x starlark.Value
	for iter.Next(&x) {
		var at, value starlark.Value
		switch p := x.(type) {
		case *starlarkstruct.Struct:
			at, _ = p.Attr("at")
			value, _ = p.Attr("value")
		case starlark.Tuple:
			if len(p) == 2 {
				at, value = p[0], p[1]
			}
		}
		when, ok1 := starlark.AsString(at)
		n, ok2 := starlark.AsFloat(value)
		if !ok1 || !ok2 {
			return nil, fmt.Errorf("chart: a point is %s: each is kept()'s (.at, .value), or (at, value)", x.String())
		}
		out = append(out, [2]any{when, n})
	}
	return out, nil
}
