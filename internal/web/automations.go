package web

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"net/http"
	"strconv"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/ops"
)

// The Automations page: your automations (internal/automation), each
// with how it stands and what its runs did; one turned on or off, taken
// away, or dry-run again over the last 30 days. Writing one is a
// conversation's (it proposes one in words): this page has no editor.

// Automating is what the Automations page asks of the workspace.
type Automating interface {
	Automations() ([]ops.AutomationItem, error)
	Automation(name string) (ops.AutomationDetail, error)
	SetAutomationOn(name string, on bool) error
	RemoveAutomation(name string) error
	DryRunAutomation(name string, show func(ref string) (map[string]any, error)) (automation.DryRun, error)
	// AutomationsStamp names where they stand, cheaply: it moves with each
	// write to what's kept of them.
	AutomationsStamp() string
}

// AutomationsInfo is what GET /api/automations serves: each of yours, by
// name.
type AutomationsInfo struct {
	Automations []ops.AutomationItem `json:"automations"`
}

// AutomationPage is what GET /api/automations/{name} serves: one, as
// listed, with its script and latest runs.
type AutomationPage struct {
	Automation ops.AutomationItem  `json:"automation"`
	Source     string              `json:"source"`
	Runs       []ops.AutomationRun `json:"runs"`
}

// AutomationOn is what POST /api/automations/{name}/on takes: on or off.
type AutomationOn struct {
	On bool `json:"on"`
}

func (s *Server) automationsInfo() (AutomationsInfo, error) {
	list, err := s.App.Automations()
	return AutomationsInfo{Automations: list}, err
}

// automations is GET /api/automations.
func (s *Server) automations(w http.ResponseWriter, r *http.Request) {
	info, err := s.automationsInfo()
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// automationsFingerprint says whether what the Automations page shows
// has changed: the list, and each one's latest run (its page's runs move
// with it).
func (s *Server) automationsFingerprint() string {
	info, err := s.automationsInfo()
	if err != nil {
		return "error: " + err.Error()
	}
	data, _ := json.Marshal(info)
	h := fnv.New64a()
	h.Write(data)
	return strconv.FormatUint(h.Sum64(), 16)
}

// automationDetail is GET /api/automations/{name}: one, with its script
// and latest runs.
func (s *Server) automationDetail(w http.ResponseWriter, r *http.Request) {
	d, err := s.App.Automation(r.PathValue("name"))
	if err != nil {
		writeError(w, automationErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, AutomationPage{Automation: d.Automation, Source: d.Source, Runs: d.Runs})
}

// automationOn is POST /api/automations/{name}/on: turned on (from the
// next event) or off.
func (s *Server) automationOn(w http.ResponseWriter, r *http.Request) {
	var b AutomationOn
	if !readBody(w, r, &b) {
		return
	}
	if err := s.App.SetAutomationOn(r.PathValue("name"), b.On); err != nil {
		writeError(w, automationErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// automationRemove is DELETE /api/automations/{name}: its file, and its
// runs, gone.
func (s *Server) automationRemove(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.App.RemoveAutomation(name); err != nil {
		writeError(w, automationErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"removed": name})
}

// automationDryRun is POST /api/automations/{name}/dry-run: what it would
// have done over the last 30 days, done again now with nothing reaching
// outside it.
func (s *Server) automationDryRun(w http.ResponseWriter, r *http.Request) {
	d, err := s.App.DryRunAutomation(r.PathValue("name"), s.Show)
	if err != nil {
		writeError(w, automationErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// automationErr is a "there's none by that name" as a 404, saying so.
func automationErr(err error) error {
	if errors.Is(err, automation.ErrNotFound) {
		return notFound{err}
	}
	return err
}

// notFound is an error that's a 404, said as it is.
type notFound struct{ error }

func (notFound) Is(target error) bool { return target == ErrNotFound }
