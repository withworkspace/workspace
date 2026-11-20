package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/doc"
	"github.com/withworkspace/workspace/internal/harness"
	"github.com/withworkspace/workspace/internal/ops"
	pages "github.com/withworkspace/workspace/internal/page"
)

// Pages: documents you write in the web UI, with cells an agent fills in
// (>> a table of the existing routes) while you write on. A page is a
// markdown file (.ws/pages/p1.md) the page in the browser saves as you
// type; a cell is one of the workspace's conversations, whose agent's
// latest reply is what the cell holds. The browser writes a cell's reply
// into the page: the server keeps the two, and says when either changes.

// PageInfo is a page in a list: GET /api/workspace has them all.
type PageInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	// Kind is the kind of document it's written as, if it was started as
	// one; Filed, where it was filed as a document, once it was.
	Kind   string        `json:"kind,omitempty"`
	Filed  *pages.Filed  `json:"filed,omitempty"`
	Source *pages.Source `json:"source,omitempty"`
	// To is where it's to be filed, of no kind: repo:folder/.
	To string `json:"to,omitempty"`
	// Unsaved: it's tied to a file, and has something to save there (as
	// it would be saved, it's not what it last agreed with the file).
	Unsaved bool `json:"unsaved,omitempty"`
	// Updated is when the page, or one of its cells (a turn, a run's output
	// as it goes), last changed.
	Updated time.Time `json:"updated"`
	Cells   int       `json:"cells,omitempty"`
	Filling int       `json:"filling,omitempty"` // cells an agent's filling now
	// Stale is how many of its cells were written from a file that's
	// changed since.
	Stale int `json:"stale,omitempty"`
	// Findings is how many of its passes' findings are open, and Passing
	// how many passes are running now.
	Findings int `json:"findings,omitempty"`
	Passing  int `json:"passing,omitempty"`
}

// PagePass is a pass over a page, as its tab shows it: what the pass is,
// and once it's been run, what it last said and found, and where each
// finding stands. Nothing of it is in the page's text.
type PagePass struct {
	pages.PassDef
	ID string `json:"id,omitempty"` // its conversation (w9), once run
	// Running: its agent's at it now; Run is what it's done so far.
	Running bool     `json:"running,omitempty"`
	Run     *TalkRun `json:"run,omitempty"`
	Runs    int      `json:"runs,omitempty"` // how many times it's said something
	// At is when it last said something; Summary, what it said of the page
	// as a whole; Findings, what it found, each open, done or dismissed.
	At       *time.Time    `json:"at,omitempty"`
	Summary  string        `json:"summary,omitempty"`
	Findings []PassFinding `json:"findings"`
	// Failed is why its latest run couldn't say anything, if it couldn't.
	Failed string `json:"failed,omitempty"`
	// Moved: the page has changed since it last ran.
	Moved bool `json:"moved,omitempty"`
}

// PassFinding is a finding and where it stands: Why, for one dismissed,
// is why (wrong, already, minor), where you said.
type PassFinding struct {
	conversation.Finding
	State FindingState `json:"state"`
	Why   DismissWhy   `json:"why,omitempty"`
	// Blocks: a review's finding of a kind that makes its pass no go
	// while it's open.
	Blocks bool `json:"blocks,omitempty"`
}

// pagePass is a pass as it stands over a page (text is the page's): c is
// its conversation, nil if it's never been run.
func pagePass(store conversation.Store, def pages.PassDef, c *conversation.Conversation, text []byte) PagePass {
	p := PagePass{PassDef: def, Findings: []PassFinding{}}
	if c == nil {
		return p
	}
	p.ID, p.Running = c.ID, c.Answering
	if c.Answering {
		p.Run = &TalkRun{Calls: []harness.ToolCall{}}
		if data, err := os.ReadFile(store.RunPath(c.ID)); err == nil {
			if rep := harness.ReadRun(string(data)); rep.Calls != nil {
				p.Run.Calls = rep.Calls
			}
		}
	}
	for _, t := range c.Turns {
		if t.By == conversation.Agent {
			p.Runs++
		}
	}
	if w := c.Written(); w != nil {
		at := w.At
		p.At, p.Summary = &at, w.Body
		p.Moved = w.Around != "" && doc.Version(text) != w.Around
		for _, f := range w.Findings {
			p.Findings = append(p.Findings, PassFinding{Finding: f, State: FindingState(c.FindingState(f.Key)), Why: DismissWhy(c.DismissedAs(f.Key))})
		}
	}
	if last := c.Last(); last.By == conversation.WS && !c.Answering {
		p.Failed = last.Body
	}
	return p
}

// pagePasses are the workspace's passes as they stand over a page.
func (s *Server) pagePasses(id string, text []byte) []PagePass {
	store := conversation.ForWorkspace(s.Root)
	out := []PagePass{}
	for _, def := range s.App.PassesOn(id) {
		out = append(out, pagePass(store, def, store.PassOn(id, def.Name), text))
	}
	return out
}

func (s *Server) getPagePasses(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	text, _, err := s.pageStore().Read(id)
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]PagePass{"passes": s.pagePasses(id, text)})
}

// runPass runs a pass over a page, the first time or again.
func (s *Server) runPass(w http.ResponseWriter, r *http.Request) {
	def, c, text, err := s.App.RunPass(r.PathValue("id"), r.PathValue("pass"))
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pagePass(conversation.ForWorkspace(s.Root), def, c, text))
}

// settleFinding is you marking a pass's finding done or dismissed, or
// opening it again.
func (s *Server) settleFinding(w http.ResponseWriter, r *http.Request) {
	var b struct {
		State string `json:"state"`
		Why   string `json:"why"`
	}
	if !readBody(w, r, &b) {
		return
	}
	def, c, text, err := s.App.SettleFinding(r.PathValue("id"), r.PathValue("pass"), r.PathValue("key"), b.State, b.Why)
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pagePass(conversation.ForWorkspace(s.Root), def, c, text))
}

// PageDetail is GET /api/pages/{id}: the page's text and its cells, and,
// for one tied to a file, where it stands with it.
type PageDetail struct {
	PageInfo
	Text string `json:"text"`
	// Version names Text: a save says which version it was edited from.
	Version string     `json:"version"`
	Cell    []PageCell `json:"cell"`
	Tied    *PageTied  `json:"tied,omitempty"`
}

// PageTied is where a page tied to a file stands with it: the file (its
// ref, and where it is), whether it's changed since the page last agreed
// with it (Changed: what changed is brought in before saving) or isn't
// there any more (Gone says why), and how saving writes it.
type PageTied struct {
	Ref     string    `json:"ref"`
	In      *DocIn    `json:"in,omitempty"`
	Changed bool      `json:"changed,omitempty"`
	Gone    string    `json:"gone,omitempty"`
	Saves   *DocSaves `json:"saves,omitempty"`
}

// PageBroughtIn is what bringing a file's changes into its page did:
// whether anything had changed, and how many places clash (marked on the
// page, to settle).
type PageBroughtIn struct {
	Changed bool `json:"changed"`
	Clashes int  `json:"clashes"`
}

// PageCell is a cell of a page: its conversation (what you asked for, each
// version the agent wrote, what you asked to change), and while the agent's
// filling it, what it's done so far.
type PageCell struct {
	*conversation.Conversation
	Run *TalkRun `json:"run,omitempty"`
	// Sources are the files it was written from, as it now reads, and
	// whether each has changed since.
	Sources []CellSource `json:"sources,omitempty"`
	// Moved says what it was written beside has changed since: "above", a
	// cell before it in the page; or "page", the page around it, for one
	// that's about the whole page (it has a goal).
	Moved string `json:"moved,omitempty"`
	// Blocks, on a cell whose agent planned an outline in place of writing
	// it, are the blocks it planned, in order: the page puts each in, empty
	// (once: see the conversation's placed), and each is a cell of its own
	// from then on (id w12-b1), with Of naming the cell it was planned by.
	// Queued is such a block waiting its turn to be written.
	Blocks []conversation.Block `json:"blocks,omitempty"`
	Of     string               `json:"of,omitempty"`
	Queued bool                 `json:"queued,omitempty"`
}

// CellSource is a file a cell was written from. In one of the workspace's
// repos, it's Repo, Worktree (".ground", or a capsule) and File there, for
// the Files page; State is "" (as it was), "changed" or "gone".
type CellSource struct {
	Path     string      `json:"path"`
	Repo     string      `json:"repo,omitempty"`
	Worktree string      `json:"worktree,omitempty"`
	File     string      `json:"file,omitempty"`
	State    SourceState `json:"state,omitempty"`
}

// cellSources are the files a cell was written from, with where each
// stands now.
func cellSources(root string, c *conversation.Conversation) []CellSource {
	w := c.Written()
	if w == nil {
		return nil
	}
	var out []CellSource
	for _, src := range w.Sources {
		cs := CellSource{Path: src.Path, State: SourceState(src.State(root))}
		if parts := strings.SplitN(src.Path, "/", 4); len(parts) == 4 && parts[0] == "repos" {
			cs.Repo, cs.Worktree, cs.File = parts[1], parts[2], parts[3]
		}
		out = append(out, cs)
	}
	return out
}

// PageSaved is what saving a page did: its version and title after.
type PageSaved struct {
	Version string `json:"version"`
	Title   string `json:"title"`
}

func (s *Server) pageStore() pages.Store { return pages.Store{Root: s.Root} }

// Pages is the workspace's pages, the latest changed first. A page's
// cells are those in its text: one asked for and since taken out is still
// a conversation, but isn't counted.
func Pages(root string) []PageInfo {
	ps := pages.Store{Root: root}
	list, _ := ps.List()
	store := conversation.ForWorkspace(root)
	all, _ := store.OnPages()
	out := []PageInfo{}
	for _, p := range list {
		info := PageInfo{ID: p.ID, Title: p.Title, Updated: p.Updated, Kind: p.Kind, Filed: p.Filed, Source: p.Source, To: p.To}
		text, _, _ := ps.Read(p.ID)
		if p.Source != nil {
			info.Unsaved = doc.Version(pages.AsText(text)) != p.Source.Version
		}
		for _, c := range all {
			if c.Anchor.Page != p.ID || (c.Kind != conversation.Cell && c.Kind != conversation.Pass) {
				continue
			}
			if at := store.Updated(c.ID); at.After(info.Updated) {
				info.Updated = at
			}
			if c.Kind == conversation.Pass {
				if c.Answering {
					info.Passing++
				}
				if w := c.Written(); w != nil {
					for _, f := range w.Findings {
						if c.FindingState(f.Key) == conversation.FindingOpen {
							info.Findings++
						}
					}
				}
				continue
			}
			if c.Answering {
				info.Filling++
			}
			// (An outline's blocks are each a cell of the page.)
			for _, v := range c.Views() {
				if !pages.HasCell(text, v.ID) {
					continue
				}
				info.Cells++
				if pages.StaleCell(root, text, v) {
					info.Stale++
				}
			}
		}
		out = append(out, info)
	}
	return out
}

// pageCells is a page's cells, oldest first.
func pageCells(root, id string) []PageCell {
	store := conversation.ForWorkspace(root)
	text, _, _ := pages.Store{Root: root}.Read(id)
	out := []PageCell{}
	for _, c := range store.OnPage(id) {
		cell := pageCell(store, c)
		cell.Moved = pages.CellMoved(text, c)
		out = append(out, cell)
		// An outline: the cell that planned it, then each of its blocks.
		for _, b := range c.Plan() {
			out = append(out, blockCell(store, c, b))
		}
	}
	return out
}

func pageCell(store conversation.Store, c *conversation.Conversation) PageCell {
	cell := PageCell{Conversation: c, Sources: cellSources(store.Missions.Root, c), Blocks: c.Plan()}
	// (What it's doing shows on the block it's on, once it has a plan.)
	if c.Answering && len(cell.Blocks) == 0 {
		cell.Run = runSoFar(store, c.ID)
	}
	return cell
}

// blockCell is a block of an outline as a cell of its own.
func blockCell(store conversation.Store, outline *conversation.Conversation, b conversation.Block) PageCell {
	v := outline.View(b)
	cell := PageCell{Conversation: v, Sources: cellSources(store.Missions.Root, v), Of: outline.ID}
	if v.Answering {
		cell.Run = runSoFar(store, outline.ID)
	}
	// Not written, and its outline's agent still has it to come to.
	next := outline.NextBlock()
	cell.Queued = v.Written() == nil && !v.Answering && len(v.Turns) == 1 && next != nil
	return cell
}

// runSoFar is what the run answering a conversation has done so far.
func runSoFar(store conversation.Store, id string) *TalkRun {
	run := &TalkRun{Calls: []harness.ToolCall{}}
	if data, err := os.ReadFile(store.RunPath(id)); err == nil {
		if rep := harness.ReadRun(string(data)); rep.Calls != nil {
			run.Calls = rep.Calls
		}
	}
	return run
}

func pageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, pages.ErrNotFound), errors.Is(err, conversation.ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, pages.ErrChanged):
		jsonError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, err, http.StatusUnprocessableEntity)
	}
}

func (s *Server) pageDetail(id string) (*PageDetail, error) {
	text, p, err := s.pageStore().Read(id)
	if err != nil {
		return nil, err
	}
	d := &PageDetail{PageInfo: PageInfo{ID: p.ID, Title: p.Title, Updated: p.Updated, Kind: p.Kind, Filed: p.Filed, Source: p.Source, To: p.To}, Text: string(text), Version: doc.Version(text), Cell: pageCells(s.Root, id)}
	d.Cells = len(d.Cell)
	if p.Source != nil {
		d.Tied = PageTiedOf(s.App.PageTied(id))
	}
	return d, nil
}

func (s *Server) getPage(w http.ResponseWriter, r *http.Request) {
	d, err := s.pageDetail(r.PathValue("id"))
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// newPage starts a page: named, it opens on its name as a heading. One
// started as a kind of document (Kind) is written as one, to be filed.
func (s *Server) newPage(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Title string `json:"title"`
		Kind  string `json:"kind"`
		To    string `json:"to"` // where it's to be filed, of no kind: repo:folder/
	}
	if !readBody(w, r, &b) {
		return
	}
	p, err := s.App.NewPage(b.Title, b.Kind, b.To)
	if err != nil {
		pageError(w, err)
		return
	}
	d, err := s.pageDetail(p.ID)
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// PageFile asks for a page to be filed as a document of one of the
// workspace's kinds: which kind, the title its name's made from (none: the
// page's), the commit's first line (none: the kind's), and Dry, to only
// say what it would do.
type PageFile struct {
	Kind    string `json:"kind"`
	Title   string `json:"title"`
	Message string `json:"message"`
	To      string `json:"to,omitempty"` // of no kind (kind ""): repo:folder/, or a file
	Dry     bool   `json:"dry"`
}

// PageFiling is what filing a page would do, or did: the kind and title,
// where it goes (Ref: repo:path; done and left in a capsule,
// repo/capsule:path), the commit's first line, and how it's saved (land:
// straight to Branch, once Verify passes; lift: in a capsule lifted for
// it, left to you). Taken says there's a file at that path already, as
// the ground has it. Done, Commit is the commit, and Message says what
// happened.
type PageFiling struct {
	Kind    string      `json:"kind"`
	Title   string      `json:"title"`
	Ref     string      `json:"ref"`
	Subject string      `json:"subject"`
	How     ops.SaveHow `json:"how"`
	Branch  string      `json:"branch"`
	Verify  string      `json:"verify,omitempty"`
	Taken   bool        `json:"taken,omitempty"`
	Commit  string      `json:"commit,omitempty"`
	Message string      `json:"message,omitempty"`
}

// openFile opens a file in a repo as a page: the one tied to it, or a new
// one. A file a mission records is that mission's to edit: the answer
// says which.
func (s *Server) openFile(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Ref string `json:"ref"`
	}
	if !readBody(w, r, &b) {
		return
	}
	id, om, err := s.App.OpenFile(b.Ref)
	var m *DocMission
	if om != nil {
		dm := DocMission(*om)
		m = &dm
	}
	switch {
	case errors.Is(err, ErrNotFound):
		jsonError(w, http.StatusNotFound, err.Error())
	case err != nil:
		pageError(w, err)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"id": id, "mission": m})
	}
}

// saveToFile writes a page tied to a file back to it. One whose file has
// changed since is refused (409): what changed is brought in first.
func (s *Server) saveToFile(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Message string `json:"message"`
	}
	if !readBody(w, r, &b) {
		return
	}
	saved, err := s.App.SavePageToFile(r.PathValue("id"), b.Message)
	if errors.Is(err, ErrChangedSince) {
		jsonError(w, http.StatusConflict, err.Error())
		return
	}
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, DocSavedOf(saved))
}

// bringIn brings what's changed in a page's file into the page.
func (s *Server) bringIn(w http.ResponseWriter, r *http.Request) {
	got, err := s.App.BringIn(r.PathValue("id"))
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, PageBroughtIn(*got))
}

// ErrChangedSince is a page's save refused because its file has changed
// since the page last took it in.
var ErrChangedSince = ops.ErrChangedSince

// filePage files a page as a document (or, Dry, says what that would do).
func (s *Server) filePage(w http.ResponseWriter, r *http.Request) {
	var b PageFile
	if !readBody(w, r, &b) {
		return
	}
	f, err := s.App.FilePage(r.PathValue("id"), ops.PageFile(b))
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, PageFiling(*f))
}

// pageRoom is how much a page's text may be.
const pageRoom = 4 << 20

// savePage saves a page's text, edited from the version given. One that's
// changed since (another tab's wrote it) is refused, with the version it's
// at.
func (s *Server) savePage(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Text string `json:"text"`
		Base string `json:"base"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, pageRoom))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&b); err != nil {
		jsonError(w, http.StatusBadRequest, "bad request body: "+err.Error())
		return
	}
	v, err := s.App.SavePage(r.PathValue("id"), b.Base, []byte(b.Text))
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, PageSaved{Version: v, Title: pages.Title([]byte(b.Text))})
}

func (s *Server) removePage(w http.ResponseWriter, r *http.Request) {
	if err := s.App.RemovePage(r.PathValue("id")); err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"removed": true})
}

func (s *Server) getPageCells(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, _, err := s.pageStore().Read(id); err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]PageCell{"cell": pageCells(s.Root, id)})
}

// pageCellOf is the cell a request names, if it's that page's: its
// conversation, and, for a block of an outline (w12-b1), the block's key.
func (s *Server) pageCellOf(r *http.Request) (*conversation.Conversation, string, error) {
	return s.App.Cell(r.PathValue("id"), r.PathValue("cell"))
}

// placedCell is the page saying it's put an outline's blocks in.
func (s *Server) placedCell(w http.ResponseWriter, r *http.Request) {
	c, _, err := s.pageCellOf(r)
	if err == nil {
		err = s.App.PlacedCell(c)
	}
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"placed": true})
}

// openCell is you asking for a cell: it's numbered, for the page to write
// into its text, and filled once the page says so (fillCell), when the
// text it's in is saved for its agent to read.
func (s *Server) openCell(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Prompt string `json:"prompt"`
		Goal   string `json:"goal"`
		// Holds is what the cell's made around: words of yours, to change.
		Holds string `json:"holds"`
		// Ask: it's a question (>?), answered beside the page.
		Ask bool `json:"ask"`
		// Finding: it's a question about a pass's finding (check:3f9a…),
		// asked from the pass's tab: it has no place in the page.
		Finding string `json:"finding"`
	}
	if !readBody(w, r, &b) {
		return
	}
	c, err := s.App.OpenCell(r.PathValue("id"), ops.CellAsk{Prompt: b.Prompt, Goal: b.Goal, Holds: b.Holds, Finding: b.Finding, Ask: b.Ask})
	if err != nil {
		pageError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pageCell(conversation.ForWorkspace(s.Root), c))
}

// fillCell has a cell's agent fill it, if what's last said in it is yours.
func (s *Server) fillCell(w http.ResponseWriter, r *http.Request) {
	c, key, err := s.pageCellOf(r)
	if err != nil {
		pageError(w, err)
		return
	}
	s.writeCell(w, s.App.FillCell(c), key)
}

// tweakCell is you saying something in a cell: asking for it to be changed
// (its agent writes it again, from the cell as you've edited it, if you
// have), or, asking, a question to be answered beside it.
func (s *Server) tweakCell(w http.ResponseWriter, r *http.Request) {
	var b struct {
		Body string `json:"body"`
		// Edited is the cell as it reads in the page, when that's not as
		// its agent last wrote it: you've edited it by hand.
		Edited string `json:"edited"`
		// Ask: it's a question about the cell, to be answered, not a change
		// to it.
		Ask bool `json:"ask"`
	}
	if !readBody(w, r, &b) {
		return
	}
	c, key, err := s.pageCellOf(r)
	if err != nil {
		pageError(w, err)
		return
	}
	if c, err = s.App.SayInCell(c, key, ops.CellSay{Body: b.Body, Edited: b.Edited, Ask: b.Ask}); err != nil {
		pageError(w, err)
		return
	}
	s.writeCell(w, c, key)
}

// writeCell replies with a cell: the block named, for one of an outline.
func (s *Server) writeCell(w http.ResponseWriter, c *conversation.Conversation, key string) {
	store := conversation.ForWorkspace(s.Root)
	if b, ok := c.BlockOf(key); ok {
		writeJSON(w, http.StatusOK, blockCell(store, c, b))
		return
	}
	writeJSON(w, http.StatusOK, pageCell(store, c))
}
