package web

import (
	"encoding/json"
	"errors"
	"hash/fnv"
	"net/http"
	"strconv"

	"github.com/withworkspace/workspace/internal/automation"
	"github.com/withworkspace/workspace/internal/events"
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
	RunAutomationNow(name string) (events.Event, error)
	AutomationOutput(name string, version int) (automation.Output, []automation.Output, error)
	DryRunAutomation(name string, show func(ref string) (map[string]any, error)) (automation.DryRun, error)
	// AutomationsStamp names where they stand, cheaply: it moves with each
	// write to what's kept of them.
	AutomationsStamp() string
	AutomationActions(on string) ([]ops.AutomationAction, error)
	SetAutomationGrant(name, kind string, on bool) error
	RunAutomationAction(name, title, on string, input map[string]any) (ops.AutomationActionRun, error)
}

// ActionsInfo is what GET /api/actions serves: your automations' actions
// on what ?on= names (a ref), as it stands, or on nothing in particular
// without one.
type ActionsInfo struct {
	On      string                 `json:"on,omitempty"`
	Actions []ops.AutomationAction `json:"actions"`
}

// ActionRun is what POST /api/actions/run takes: which automation's
// action, on what (a ref, "" for nothing), with what it asks.
type ActionRun struct {
	Automation string         `json:"automation"`
	Title      string         `json:"title"`
	On         string         `json:"on,omitempty"`
	Input      map[string]any `json:"input,omitempty"`
}

// ActionRan is what POST /api/actions/run answers: how the run went by
// the time it's said.
type ActionRan struct {
	ops.AutomationActionRun
}

// actions is GET /api/actions?on=ref.
func (s *Server) actions(w http.ResponseWriter, r *http.Request) {
	on := r.URL.Query().Get("on")
	list, err := s.App.AutomationActions(on)
	if err != nil {
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ActionsInfo{On: on, Actions: list})
}

// actionRun is POST /api/actions/run: one of your automations' actions,
// run by you, and how it went once it's ended or parked (or still
// running, after a few seconds).
func (s *Server) actionRun(w http.ResponseWriter, r *http.Request) {
	var b ActionRun
	if !readBody(w, r, &b) {
		return
	}
	got, err := s.App.RunAutomationAction(b.Automation, b.Title, b.On, b.Input)
	if err != nil {
		writeError(w, automationErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, ActionRan{got})
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
	// Outline is its script, drawn, with how each step's gone over its
	// runs (Steps) and where they failed (Failures: line:col, how many).
	Outline  *automation.Outline    `json:"outline,omitempty"`
	Steps    []automation.StepCount `json:"steps"`
	Failures map[string]int         `json:"failures,omitempty"`
}

// AutomationOutputInfo is what GET /api/automations/{name}/output serves:
// a version of what it shows (?version=, the latest without), and every
// version kept, the latest first, without their bodies. An HTML one's
// body isn't sent: it's shown from its frame
// (/frame/automations/{name}/output/{version}).
type AutomationOutputInfo struct {
	Output   *automation.Output  `json:"output,omitempty"`
	Versions []automation.Output `json:"versions"`
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
	writeJSON(w, http.StatusOK, AutomationPage{Automation: d.Automation, Source: d.Source, Runs: d.Runs, Outline: d.Outline, Steps: d.Steps, Failures: d.Failures})
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

// AutomationGrant is what POST /api/automations/{name}/grants takes: a
// plugin's kind of proposal, and whether the automation may do it without
// asking.
type AutomationGrant struct {
	Kind string `json:"kind"`
	On   bool   `json:"on"`
}

// automationGrant is POST /api/automations/{name}/grants.
func (s *Server) automationGrant(w http.ResponseWriter, r *http.Request) {
	var b AutomationGrant
	if !readBody(w, r, &b) {
		return
	}
	if err := s.App.SetAutomationGrant(r.PathValue("name"), b.Kind, b.On); err != nil {
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

// automationRunNow is POST /api/automations/{name}/run: one on a schedule
// run now, as if its time had come.
func (s *Server) automationRunNow(w http.ResponseWriter, r *http.Request) {
	ev, err := s.App.RunAutomationNow(r.PathValue("name"))
	if err != nil {
		writeError(w, automationErr(err), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"event": ev.Ref()})
}

// automationOutput is GET /api/automations/{name}/output.
func (s *Server) automationOutput(w http.ResponseWriter, r *http.Request) {
	version, _ := strconv.Atoi(r.URL.Query().Get("version"))
	shown, all, err := s.App.AutomationOutput(r.PathValue("name"), version)
	if err != nil {
		writeError(w, automationErr(err), http.StatusInternalServerError)
		return
	}
	info := AutomationOutputInfo{Versions: all}
	if shown.Version > 0 {
		if shown.Format == automation.OutputHTML {
			shown.Body = ""
		}
		info.Output = &shown
	}
	writeJSON(w, http.StatusOK, info)
}

// automationOutputFrame is GET /frame/automations/{name}/output/{version}:
// an HTML output, as a frame's page, under the frame's policy (nothing
// from the network, nothing of the app's).
func (s *Server) automationOutputFrame(w http.ResponseWriter, r *http.Request) {
	frameHeaders(w, "")
	version, err := strconv.Atoi(r.PathValue("version"))
	shown, _, oerr := s.App.AutomationOutput(r.PathValue("name"), version)
	if err != nil || version < 1 || oerr != nil || shown.Format != automation.OutputHTML {
		framePage(w, http.StatusNotFound, "Could not show output", "Output not found: addresses look like /frame/automations/coverage/output/3.")
		return
	}
	s.serveFrameFile(w, r, r.URL.Path, "index.html", []byte(shown.Body), "", "")
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
