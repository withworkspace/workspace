// Package report reads what a task's report says, as numbers: a
// coverage file's total, a test run's passes and failures. A task's
// recipe names the files its command writes and their format
// (reports.coverage = { path = "coverage.out", format = "gocover" }), and
// ws keeps them with each run (internal/runs); this is what an
// automation's given of them. It reads totals only: the whole of a
// format (each file's lines, each test's output) is for a plugin that
// shows it.
package report

import (
	"bufio"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Formats are the formats a summary's read from.
var Formats = []string{"gocover", "lcov", "cobertura", "junit"}

// ErrFormat is the answer for a format there's no summary of.
var ErrFormat = errors.New("ws reads no summary of that format: it reads " + strings.Join(Formats, ", "))

// Summary is what the files in dir (a run's kept report, every file
// under it) say, by format: for coverage, total (a percent, to one
// place), covered and lines (statements, for gocover); for junit, tests,
// failures, errors, skipped and passed.
func Summary(format, dir string) (map[string]any, error) {
	files, err := filesIn(dir)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("its report isn't there: the run kept no file of it")
	}
	switch format {
	case "gocover", "go":
		return coverage(files, gocover)
	case "lcov":
		return coverage(files, lcov)
	case "cobertura":
		return coverage(files, cobertura)
	case "junit":
		return junit(files)
	}
	return nil, ErrFormat
}

func filesIn(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return out, err
}

// coverage adds up what each file says is covered, of how many.
func coverage(files []string, read func(string) (covered, total int, err error)) (map[string]any, error) {
	var covered, total int
	for _, f := range files {
		c, t, err := read(f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", filepath.Base(f), err)
		}
		covered, total = covered+c, total+t
	}
	pct := 0.0
	if total > 0 {
		pct = math.Round(float64(covered)/float64(total)*1000) / 10
	}
	return map[string]any{"total": pct, "covered": covered, "lines": total}, nil
}

// gocover is a Go cover profile: mode, then file:span statements count.
// A block listed more than once (a package tested by several) is counted
// once, covered if any says so.
func gocover(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	type block struct{ stmts, count int }
	blocks := map[string]*block{}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	first := true
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if first {
			first = false
			if strings.HasPrefix(line, "mode:") {
				continue
			}
		}
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 3 {
			return 0, 0, fmt.Errorf("%q isn't a cover profile's line", line)
		}
		stmts, err1 := strconv.Atoi(fields[1])
		count, err2 := strconv.Atoi(fields[2])
		if err1 != nil || err2 != nil {
			return 0, 0, fmt.Errorf("%q isn't a cover profile's line", line)
		}
		b := blocks[fields[0]]
		if b == nil {
			b = &block{stmts: stmts}
			blocks[fields[0]] = b
		}
		b.count += count
	}
	var covered, total int
	for _, b := range blocks {
		total += b.stmts
		if b.count > 0 {
			covered += b.stmts
		}
	}
	return covered, total, sc.Err()
}

// lcov is an lcov tracefile: each file's LH (lines hit) of LF (found).
func lcov(path string) (int, int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, 0, err
	}
	var hit, found int
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if v, ok := strings.CutPrefix(line, "LH:"); ok {
			n, _ := strconv.Atoi(v)
			hit += n
		} else if v, ok := strings.CutPrefix(line, "LF:"); ok {
			n, _ := strconv.Atoi(v)
			found += n
		}
	}
	return hit, found, nil
}

// cobertura is a Cobertura XML report: its root's lines-covered of
// lines-valid, or its line-rate where those aren't said.
func cobertura(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	var root struct {
		XMLName  xml.Name `xml:"coverage"`
		Covered  int      `xml:"lines-covered,attr"`
		Valid    int      `xml:"lines-valid,attr"`
		LineRate float64  `xml:"line-rate,attr"`
	}
	if err := xml.NewDecoder(f).Decode(&root); err != nil {
		return 0, 0, fmt.Errorf("not a Cobertura report: %w", err)
	}
	if root.Valid > 0 {
		return root.Covered, root.Valid, nil
	}
	return int(math.Round(root.LineRate * 1000)), 1000, nil
}

// junit is JUnit XML: its suites' tests, failures, errors and skipped,
// added up.
func junit(files []string) (map[string]any, error) {
	type suite struct {
		Tests    int     `xml:"tests,attr"`
		Failures int     `xml:"failures,attr"`
		Errors   int     `xml:"errors,attr"`
		Skipped  int     `xml:"skipped,attr"`
		Suites   []suite `xml:"testsuite"`
	}
	var tests, failures, errs, skipped int
	var add func(s suite, top bool)
	add = func(s suite, top bool) {
		if len(s.Suites) > 0 && (top || s.Tests == 0) {
			for _, c := range s.Suites {
				add(c, false)
			}
			return
		}
		tests, failures, errs, skipped = tests+s.Tests, failures+s.Failures, errs+s.Errors, skipped+s.Skipped
	}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		var s suite
		if err := xml.Unmarshal(b, &s); err != nil {
			return nil, fmt.Errorf("%s: not JUnit XML: %w", filepath.Base(f), err)
		}
		add(s, true)
	}
	return map[string]any{"tests": tests, "failures": failures, "errors": errs, "skipped": skipped, "passed": tests - failures - errs - skipped}, nil
}
