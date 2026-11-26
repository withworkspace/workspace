package report

import (
	"os"
	"path/filepath"
	"testing"
)

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Each format's totals: coverage as a percent of what's covered, a block
// listed twice counted once; tests as passes and failures.
func TestSummary(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "coverage.out", "mode: set\na.go:1.1,2.1 3 1\na.go:3.1,4.1 1 0\nb.go:1.1,2.1 4 0\nb.go:1.1,2.1 4 2\n")
	got, err := Summary("gocover", dir)
	if err != nil || got["total"] != 87.5 || got["covered"] != 7 || got["lines"] != 8 {
		t.Errorf("gocover: %v %v", got, err)
	}

	dir = t.TempDir()
	write(t, dir, "lcov.info", "SF:a.js\nLF:10\nLH:7\nend_of_record\nSF:b.js\nLF:10\nLH:3\nend_of_record\n")
	if got, _ := Summary("lcov", dir); got["total"] != 50.0 {
		t.Errorf("lcov: %v", got)
	}

	dir = t.TempDir()
	write(t, dir, "cobertura.xml", `<?xml version="1.0"?><coverage line-rate="0.8" lines-covered="40" lines-valid="50"></coverage>`)
	if got, _ := Summary("cobertura", dir); got["total"] != 80.0 {
		t.Errorf("cobertura: %v", got)
	}

	dir = t.TempDir()
	write(t, dir, "a.xml", `<testsuites><testsuite tests="5" failures="1" errors="0" skipped="1"/><testsuite tests="3" failures="0"/></testsuites>`)
	if got, _ := Summary("junit", dir); got["tests"] != 8 || got["failures"] != 1 || got["passed"] != 6 {
		t.Errorf("junit: %v", got)
	}

	if _, err := Summary("gocover", filepath.Join(t.TempDir(), "none")); err == nil {
		t.Error("no file, no error")
	}
	if _, err := Summary("sarif", dir); err == nil {
		t.Error("a format it doesn't read")
	}
}
