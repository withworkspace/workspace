package web

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func getProcess(t *testing.T, s *Server) ProcessInfo {
	t.Helper()
	var p ProcessInfo
	json.NewDecoder(do(t, s, "GET", "/api/process", true, nil).Body).Decode(&p)
	return p
}

// The Settings page shows the process as it's written, and where it is.
// There's nothing to read back or confirm: the planner and the
// orchestrator read it as it is.
func TestProcessAPI(t *testing.T) {
	s, _ := docServer(t)
	if p := getProcess(t, s); p.Exists || p.Text != "" || p.Name != "PROCESS.md" {
		t.Errorf("no process: %+v", p)
	}
	text := "Bugs are reproduced first, then fixed.\n"
	os.WriteFile(filepath.Join(s.Root, "PROCESS.md"), []byte(text), 0o644)
	if p := getProcess(t, s); !p.Exists || p.Text != text || p.Name != "PROCESS.md" || !p.Uncommitted {
		t.Errorf("a process, not committed: %+v", p)
	}
	for _, path := range []string{"/api/process/read-back", "/api/process/confirm", "/api/process/answer"} {
		if res := do(t, s, "POST", path, true, nil); res.StatusCode == http.StatusOK {
			t.Errorf("%s is still there", path)
		}
	}
}
