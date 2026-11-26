// Package web is the workspace's web UI: an HTTP server on 127.0.0.1, run
// inside the workspace's daemon, serving a single-page app (web/, built into dist/) and
// the JSON API it reads. It's for what the terminal is poor at, starting
// with review.
//
// Every request needs the workspace's token (a cookie set by /auth, or a
// bearer header; an artifact's files, the key in their address, which is
// made from it: artifacts.go), a Host naming this server (against DNS
// rebinding), no other site's page asking but by a link to the app (the
// cookie is Lax, and same-site from other ports: server.go's
// fromAnotherSite), and,
// for anything but a read, a matching Origin and a JSON body (against
// cross-site requests, including from other ports on 127.0.0.1).
package web

import (
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/withworkspace/workspace/internal/cost"
	"github.com/withworkspace/workspace/internal/find"
	"github.com/withworkspace/workspace/internal/nav"
	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/pulls"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/team"
)

//go:embed all:dist
var embedded embed.FS

// Built is the single-page app, as built into ws.
func Built() fs.FS {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic(err) // dist is embedded at build time
	}
	return sub
}

// Server serves one workspace's web UI.
type Server struct {
	Root string
	// Version is the ws this is, as ws --version says it: the web UI says
	// which it's talking to.
	Version string
	// Port is [web] port; 0 picks the workspace's own (DefaultPort), or one
	// of the next few if that's taken.
	Port int
	Log  *log.Logger
	// App is what the web UI asks of the workspace: every write a page
	// makes goes through it.
	App App
	// Find is what the find window asks of the server (GET /api/find):
	// every repo's files, what capsules have changed, and the pull
	// requests Reviews keeps. nil for nothing to look through.
	Find *find.Index
	// Reviews is the Reviews page's: the open pull requests across the
	// workspace's repos, their diffs, and your reviews of them (posted to
	// GitHub when submitted). nil where there's no GitHub to ask.
	Reviews *pulls.Service
	// ReviewsPoll is how often the list is checked while a Reviews page is
	// open (default 15s; GitHub is asked only when it hasn't been lately).
	ReviewsPoll time.Duration
	// Team is the Team page's: what the people working in the workspace's
	// repos have been doing. nil for none. TeamPoll is how often a new
	// look, or a newly written gist, is checked for while a Team page is
	// open (default 5s; git and GitHub are looked at only when they
	// haven't been lately).
	Team     *team.Service
	TeamPoll time.Duration
	// Nav is code navigation in a review: names, where they're defined
	// and used. nil for none.
	Nav *nav.Service
	// Plugins are the workspace's plugins: what each adds to the sidebar,
	// its app, which ws serves a page for, and its own HTTP, which is
	// passed to it. nil for none.
	Plugins *plugin.Host
	// Labels are what ws's own commands are doing, as a conversation
	// shows an agent's step that ran one (GET /api/commands, with the
	// plugins' after them). nil for none: steps show their commands.
	Labels []CommandLabel
	// Hold keeps the daemon up while a page is open (each holds a
	// socket); nil for nothing to hold.
	Hold func() (release func())
	// Shutdown closes the daemon, as ws daemon stop does: its crew are hung
	// up and the web UI goes with it (POST /api/shutdown, from the
	// workspace's menu). nil for none to close.
	Shutdown func(reason string)
	// Names are the names it's also reached at through ws's router
	// (<workspace>.localhost, with its port where it has one): nil for
	// none.
	Names func() []string
	// Silos is where each silo the daemon keeps stands, for the Runs
	// page: nil for none kept.
	Silos func() []ops.SiloState
	// ResyncSilo gives a repo's silo its target's files in full (POST
	// /api/silos/{repo}/resync): nil when nothing keeps them.
	ResyncSilo func(repo string) error
	// Poll is how often open pages are checked for changes to tell them
	// about (default 2s).
	Poll time.Duration
	// Assets is the app to serve (default Built()).
	Assets fs.FS

	listener net.Listener     // tests set it
	clock    func() time.Time // tests set it
	info     Info
	srv      *http.Server
	// topics are what pages follow (workspace, reviews, team): each a hub
	// that tells its open pages when what they show has changed.
	topics map[string]*hub
	// missions tells a mission's open page when it changes (mission:<slug>).
	missions *keyedHub
	// views is what's been built of the workspace's view, for whoever asks
	// next.
	views viewCache
	talks *keyedHub // conversations'
	// transcripts is crew's sessions', each read as it grows by whoever
	// follows it (followed).
	transcripts *keyedHub
	// filesFollowed is the Files page's grounds and capsules, each told when
	// the runs move; a page that follows one has it open, for the runs
	// started from it. runLogs is runs' logs, each told what's written.
	filesFollowed *keyedHub
	runLogs       *keyedHub
	// apps is apps' pages, each told when its run moves (and following one
	// keeps it); docs is documents being edited, each told when its file's
	// written.
	apps *keyedHub
	docs *keyedHub
	// files are the grounds and capsules open on the Files page, for the
	// runs started there.
	files filesOpen
	// prs tells a pull request's open page when it changes
	// (pr:<repo>#<number>): nil without Reviews.
	prs    *prHub
	findMu sync.Mutex // guards giving Find its pull requests
	// reads are a capsule's files as pages have read them (kept.go).
	reads  keptReads
	ctx    context.Context
	cancel context.CancelFunc
}

// Start binds, writes .ws/web.json and serves in the background.
func (s *Server) Start() error {
	if s.Log == nil {
		s.Log = log.New(io.Discard, "", 0)
	}
	if s.Poll == 0 {
		s.Poll = 2 * time.Second
	}
	if s.Assets == nil {
		s.Assets = Built()
	}
	ln := s.listener
	if ln == nil {
		var err error
		if ln, err = s.listen(); err != nil {
			return err
		}
	}
	prev, err := ReadInfo(s.Root)
	if err != nil {
		s.Log.Printf("web: %v; making a new token", err)
	}
	token := prev.Token
	if token == "" {
		if token, err = newToken(); err != nil {
			ln.Close()
			return err
		}
	}
	s.info = Info{Port: ln.Addr().(*net.TCPAddr).Port, Pid: os.Getpid(), Token: token}
	if err := writeInfo(s.Root, s.info); err != nil {
		ln.Close()
		return fmt.Errorf("writing %s: %w", InfoPath(s.Root), err)
	}
	s.ctx, s.cancel = context.WithCancel(context.Background())
	s.topics = map[string]*hub{}
	ws := newHub(topicWorkspace, s.Poll, s.fingerprint)
	// The view's built only when what it's built from has moved, and
	// every half a minute regardless.
	// What's kept for pages behind their reads (a capsule's tree and
	// summary, the repos' files refs are checked against) moves the view
	// too, when a read behind finds it changed: looked at as soon as it has.
	ws.stamp, ws.floor = func() string { return s.App.Stamp() + " " + s.keptStamp() }, 30*time.Second
	s.reads.told = ws.nudge
	s.topics[topicWorkspace] = ws
	if s.Reviews != nil {
		if s.ReviewsPoll == 0 {
			s.ReviewsPoll = 15 * time.Second
		}
		h := newHub(topicReviews, s.ReviewsPoll, s.reviewsFingerprint)
		// A page that opens starts from the list as it's kept, and GitHub
		// is asked straight after: opening mustn't wait on it.
		h.baseline = func() string {
			l := s.Reviews.Peek()
			return listFingerprint(l, s.App.PassMarks(listHeads(l)))
		}
		s.topics[topicReviews] = h
		// A pull request's page: its page as kept is looked at as often as
		// any page's, and read again behind when it's stale.
		s.prs = newPRHub(s.Poll, func(r prRef) string {
			s.Reviews.Freshen(r.repo, r.number)
			return strconv.FormatInt(s.Reviews.PageSeen(r.repo, r.number), 10)
		})
		go s.prs.run(s.ctx)
	}
	if s.Team != nil {
		if s.TeamPoll == 0 {
			s.TeamPoll = 5 * time.Second
		}
		h := newHub(topicTeam, s.TeamPoll, s.teamFingerprint)
		// As with Reviews: a page that opens starts from what's kept, and
		// the repos are looked at straight after.
		h.baseline = func() string { return s.snapFingerprint(s.Team.Peek()) }
		s.topics[topicTeam] = h
		// What's near you is read behind the page's read: it's told when
		// that finds something.
		s.Team.Moved = h.nudge
	}
	if s.Plugins != nil {
		// What plugins add, and where each stands: the host counts its
		// changes, so there's nothing to work out but which are installed.
		// One removed is stopped before its files go, and one added from a
		// folder may never start: neither is a change the host counts.
		s.topics[topicPlugins] = newHub(topicPlugins, s.Poll, func() string {
			return s.Plugins.Stamp() + " " + strings.Join(plugin.Installed(s.Plugins.Root), ",")
		})
	}
	// Adding a repo from GitHub: what GitHub said of owners and their
	// repos, and the repos added being cloned, moved when a look behind
	// a read finishes.
	s.topics[topicGitHub] = newHub(topicGitHub, s.Poll, s.App.GitHubSeen)
	// The Runs page: built when a run starts, comes up, ends or is cleared
	// away (the runs' stamp, which reads no process table), or a silo
	// moves; and every half a minute, for when a run ends (its owner's
	// gone, its time's up) as it's swept.
	runsHub := newHub(topicRuns, s.Poll, s.runsFingerprint)
	runsHub.stamp, runsHub.floor = s.runsStamp, 30*time.Second
	s.topics[topicRuns] = runsHub
	for _, h := range s.topics {
		go h.run(s.ctx)
	}
	s.missions = newKeyedHub(missionTopic, s.Poll, func(slug string) string { return s.App.MissionStamp(slug) })
	s.missions.keep = func(slug string) { s.App.KeepMission(slug) }
	go s.missions.run(s.ctx)
	// A conversation's agent writes its steps as it answers, which the
	// records don't see: looked at every second while it's followed, as
	// often as a page read one again before.
	s.talks = newKeyedHub(conversationTopic, min(s.Poll, time.Second), func(key string) string {
		scope, id, _ := strings.Cut(key, "/")
		return s.App.ConversationStamp(scope, id)
	})
	// A conversation's followers hold it, and are told how it's changed.
	s.talks.change = func(key string, was any) (any, any) { return conversationChange(s.Root, key, was) }
	go s.talks.run(s.ctx)
	s.transcripts = s.transcriptHub()
	go s.transcripts.run(s.ctx)
	s.filesFollowed = newKeyedHub(worktreeTopic, s.Poll, func(string) string { return s.App.RunsStamp() })
	go s.filesFollowed.run(s.ctx)
	s.runLogs = s.runLogHub()
	go s.runLogs.run(s.ctx)
	s.apps = newKeyedHub(appTopic, s.Poll, func(key string) string {
		name, p, _ := appOfKey(key)
		return s.App.AppStamp(p, name)
	})
	go s.apps.run(s.ctx)
	s.docs = s.docHub()
	go s.docs.run(s.ctx)
	go s.markLooked() // runs started from a mission's page, or Files, last while it's open
	// A write to the records tells the followers of what it names at once:
	// a mission's page, a conversation's; and one that names the
	// workspace (a conversation's turn, a page, a mission's record) has its
	// hub look now, not at its next poll, which is for what only files and
	// capsules show.
	var records *store.DB
	if db, err := store.Dial(s.Root); err == nil {
		records = db
	}
	go watchRecords(s.ctx, records, func(topic string) {
		if key, ok := topicKey(topic, missionTopic); ok {
			s.missions.tell(key)
		} else if key, ok := topicKey(topic, conversationTopic); ok {
			s.talks.tell(key)
		} else if topic == topicWorkspace {
			ws.nudge()
		}
	})
	// What's read behind a read that didn't wait (a space of documents,
	// where a capsule stands) and found something new is looked at now.
	seen := s.App.Seen() // the App the server starts with: read here, not in the goroutine
	// And what git writes in a capsule or a repo is looked at as it's
	// written: the hubs' polls are for what that can't see.
	go s.App.WatchGit(s.ctx.Done(), s.Log.Printf)
	go func() {
		for {
			select {
			case <-s.ctx.Done():
				return
			case <-seen:
				ws.nudge()
				s.missions.nudge()
			}
		}
	}()
	s.srv = &http.Server{
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return s.ctx },
	}
	go func() {
		if err := s.srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.Log.Printf("web: %v", err)
		}
	}()
	s.Log.Printf("web UI on %s", s.info.Origin())
	return nil
}

// listen binds [web] port, or the workspace's own port or one of the next
// few.
func (s *Server) listen() (net.Listener, error) {
	if s.Port != 0 {
		ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(s.Port)))
		if err != nil {
			return nil, fmt.Errorf("[web] port %d: %w", s.Port, err)
		}
		return ln, nil
	}
	first := DefaultPort(s.Root)
	var lastErr error
	for i := range portTries {
		port := portBase + (first-portBase+i)%portRange
		ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
		if err == nil {
			return ln, nil
		}
		lastErr = err
		if !errors.Is(err, syscall.EADDRINUSE) {
			break
		}
	}
	return nil, fmt.Errorf("no free port from %d (set [web] port): %w", first, lastErr)
}

func (s *Server) now() time.Time {
	if s.clock != nil {
		return s.clock()
	}
	return time.Now()
}

// Info is where the server listens, and its token.
func (s *Server) Info() Info { return s.info }

// Close stops serving, closes open sockets, and marks .ws/web.json as
// not running (keeping the token).
func (s *Server) Close() {
	if s.srv == nil {
		return
	}
	s.cancel()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.srv.Shutdown(ctx); err != nil {
		s.srv.Close()
	}
	if cur, err := ReadInfo(s.Root); err == nil && cur.Pid == s.info.Pid {
		writeInfo(s.Root, Info{Token: s.info.Token})
	}
}

// cookieName is per workspace: cookies are kept per host, not per port, so
// two workspaces' UIs on 127.0.0.1 would otherwise share one.
func (s *Server) cookieName() string {
	h := fnv.New32a()
	h.Write([]byte(s.Root))
	return fmt.Sprintf("ws_%08x", h.Sum32())
}

// Handler is the whole web UI behind its guard.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /auth", s.auth)
	mux.HandleFunc("GET /api/workspace", s.workspace)
	mux.HandleFunc("POST /api/shutdown", s.shutdown)
	mux.HandleFunc("GET /api/updates", s.updates)
	mux.HandleFunc("POST /api/updates/check", s.checkUpdates)
	mux.HandleFunc("POST /api/upgrade", s.upgrade)
	mux.HandleFunc("GET /api/whats-new", s.whatsNew)
	mux.HandleFunc("POST /api/whats-new/seen", s.seenWhatsNew)
	mux.HandleFunc("POST /api/needs/announce", s.announceNeeds)
	mux.HandleFunc("GET /api/notifications", s.notifications)
	mux.HandleFunc("POST /api/notifications/read", s.markNotifications(false))
	mux.HandleFunc("POST /api/notifications/dismiss", s.markNotifications(true))
	mux.HandleFunc("POST /api/notifications/announce", s.announceNotifications)
	mux.HandleFunc("GET /api/whats-new/pictures/{name}", s.whatsNewPicture)
	mux.HandleFunc("GET /api/socket", s.socket)
	mux.HandleFunc("GET /api/missions/{slug}", s.mission)
	mux.HandleFunc("GET /api/missions/{slug}/artifact", s.artifact)
	mux.HandleFunc("GET /api/missions/{slug}/artifact/raw", s.artifactRaw)
	mux.HandleFunc("GET /api/missions/{slug}/bundles/{id}/file/{name}", s.bundleFile)
	mux.HandleFunc("GET /api/find", s.find)
	mux.HandleFunc("GET /api/around", s.around)
	mux.HandleFunc("GET /api/search", s.codeSearch)
	mux.HandleFunc("GET /api/repos", s.repos)
	mux.HandleFunc("POST /api/repos/{repo}/share", s.shareRepo)
	mux.HandleFunc("POST /api/repos", s.addRepo)
	mux.HandleFunc("POST /api/repos/{repo}/group", s.groupRepo)
	mux.HandleFunc("GET /api/groups", s.groups)
	mux.HandleFunc("POST /api/groups/{group}/shown", s.showGroup)
	mux.HandleFunc("GET /api/github/owners", s.githubOwners)
	mux.HandleFunc("GET /api/github/repos", s.githubRepos)
	mux.HandleFunc("POST /api/refs/paths", s.refPaths)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees", s.worktrees)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{name}/tree", s.tree)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{name}/file", s.file)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{name}/raw", s.raw)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{name}/nav/now", s.navNow)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{name}/nav/names", s.navNames)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{name}/nav/definition", s.navDefinition)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{name}/nav/references", s.navReferences)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{name}/landing", s.landing)
	mux.HandleFunc("POST /api/repos/{repo}/worktrees/{name}/land", s.land)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{name}/runs", s.worktreeRuns)
	mux.HandleFunc("POST /api/repos/{repo}/worktrees/{name}/runs", s.runInWorktree)
	mux.HandleFunc("POST /api/missions/{slug}/artifact/save", s.saveDoc)
	mux.HandleFunc("GET /api/missions/{slug}/draft", s.getDraft)
	mux.HandleFunc("PUT /api/missions/{slug}/draft", s.putDraft)
	mux.HandleFunc("DELETE /api/missions/{slug}/draft", s.dropDraft)
	mux.HandleFunc("POST /api/missions", s.startMission)
	mux.HandleFunc("POST /api/missions/{slug}/act", s.act)
	mux.HandleFunc("POST /api/missions/{slug}/decisions", s.decide)
	mux.HandleFunc("POST /api/missions/{slug}/decisions/{id}/withdraw", s.withdrawDecision)
	mux.HandleFunc("POST /api/missions/{slug}/pr-write", s.writePR)
	mux.HandleFunc("POST /api/missions/{slug}/runs", s.startRun)
	mux.HandleFunc("POST /api/missions/{slug}/runs/stop", s.stopRun)
	mux.HandleFunc("GET /api/runs", s.allRuns)
	mux.HandleFunc("POST /api/runs/stop", s.endRun)
	mux.HandleFunc("GET /api/runs/log", s.runLog)
	mux.HandleFunc("GET /api/runs/past", s.runPast)
	mux.HandleFunc("GET /api/apps/{name}", s.getApp)
	mux.HandleFunc("POST /api/apps/{name}/open", s.openApp)
	mux.HandleFunc("POST /api/apps/{name}/restart", s.restartApp)
	mux.HandleFunc("POST /api/apps/{name}/stop", s.stopApp)
	mux.HandleFunc("POST /api/apps/{name}/point", s.pointApp)
	mux.HandleFunc("GET /api/repos/{repo}/worktrees/{worktree}/apps/{name}", s.getApp)
	mux.HandleFunc("POST /api/repos/{repo}/worktrees/{worktree}/apps/{name}/open", s.openApp)
	mux.HandleFunc("POST /api/repos/{repo}/worktrees/{worktree}/apps/{name}/restart", s.restartApp)
	mux.HandleFunc("POST /api/repos/{repo}/worktrees/{worktree}/apps/{name}/stop", s.stopApp)
	mux.HandleFunc("POST /api/silos/{repo}/point", s.pointSilo)
	mux.HandleFunc("POST /api/silos/{repo}/resync", s.resyncSilo)
	mux.HandleFunc("POST /api/missions/{slug}/crew/{id}/input", s.crewInput)
	mux.HandleFunc("GET /api/missions/{slug}/crew/{id}/transcript", s.crewTranscript)
	mux.HandleFunc("POST /api/missions/{slug}/crew/{id}/say", s.crewSay)
	mux.HandleFunc("POST /api/missions/{slug}/crew/{id}/answer", s.crewAnswer)
	mux.HandleFunc("POST /api/missions/{slug}/crew/{id}/model", s.crewModel)
	mux.HandleFunc("GET /api/missions/{slug}/crew/{id}/prompt", s.crewPrompt)
	mux.HandleFunc("GET /api/orchestrator", s.orchestrator)
	mux.HandleFunc("GET /api/settings", s.settings)
	mux.HandleFunc("POST /api/settings", s.setSettings)
	mux.HandleFunc("POST /api/focus", s.focus)
	mux.HandleFunc("GET /api/settings/models", s.models)
	mux.HandleFunc("GET /api/process", s.process)
	mux.HandleFunc("GET /api/missions/{slug}/orchestrator/{id}", s.orchestratorRun)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/diff", s.diff)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/file-diff", s.fileDiff)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/blob", s.blob)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/nav/now", s.navNow)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/nav/names", s.navNames)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/nav/definition", s.navDefinition)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/nav/references", s.navReferences)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/tree", s.tree)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/file", s.file)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/raw", s.raw)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/summary", s.summary)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/review", s.getReview)
	mux.HandleFunc("DELETE /api/missions/{slug}/capsules/{repo}/{name}/review", s.discardReview)
	mux.HandleFunc("POST /api/missions/{slug}/capsules/{repo}/{name}/review/comments", s.addNote)
	mux.HandleFunc("PATCH /api/missions/{slug}/capsules/{repo}/{name}/review/comments/{id}", s.editNote)
	mux.HandleFunc("DELETE /api/missions/{slug}/capsules/{repo}/{name}/review/comments/{id}", s.deleteNote)
	mux.HandleFunc("PUT /api/missions/{slug}/capsules/{repo}/{name}/review/viewed", s.setViewed)
	mux.HandleFunc("POST /api/missions/{slug}/capsules/{repo}/{name}/review/submit", s.submitReview)
	mux.HandleFunc("POST /api/missions/{slug}/capsules/{repo}/{name}/review/comments/{id}/respond", s.respond)
	// A capsule's review has passes and a guide, as a pull request's does.
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/passes", s.reviewPasses)
	mux.HandleFunc("POST /api/missions/{slug}/capsules/{repo}/{name}/passes/{pass}/run", s.runReviewPass)
	mux.HandleFunc("POST /api/missions/{slug}/capsules/{repo}/{name}/passes/{pass}/findings/{key}", s.settleReviewFinding)
	mux.HandleFunc("GET /api/missions/{slug}/capsules/{repo}/{name}/guide", s.reviewGuideGet)
	mux.HandleFunc("POST /api/missions/{slug}/capsules/{repo}/{name}/guide/run", s.runReviewGuide)
	mux.HandleFunc("GET /api/reviews", s.reviewsList)
	mux.HandleFunc("GET /api/prs/{repo}/{number}", s.prDetail)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/diff", s.prDiff)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/file-diff", s.fileDiff)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/blob", s.blob)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/nav/names", s.navNames)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/nav/definition", s.navDefinition)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/nav/references", s.navReferences)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/review", s.getReview)
	mux.HandleFunc("DELETE /api/prs/{repo}/{number}/review", s.discardReview)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/review/comments", s.addNote)
	mux.HandleFunc("PATCH /api/prs/{repo}/{number}/review/comments/{id}", s.editNote)
	mux.HandleFunc("DELETE /api/prs/{repo}/{number}/review/comments/{id}", s.deleteNote)
	mux.HandleFunc("PUT /api/prs/{repo}/{number}/review/viewed", s.setViewed)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/review/preview", s.prPreview)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/review/submit", s.prSubmit)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/review/comments/{id}/respond", s.prNoRespond)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/threads", s.prThreads)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/threads/{id}/respond", s.prRespond)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/passes", s.reviewPasses)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/passes/{pass}/run", s.runReviewPass)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/passes/{pass}/findings/{key}", s.settleReviewFinding)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/guide", s.reviewGuideGet)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/guide/run", s.runReviewGuide)
	mux.HandleFunc("GET /api/prs/{repo}/{number}/discussion", s.prDiscussion)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/discussion", s.prComment)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/decline", s.prDecline)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/unsent/{id}/resend", s.prUnsent)
	mux.HandleFunc("DELETE /api/prs/{repo}/{number}/unsent/{id}", s.prUnsent)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/ask", s.prAsk)
	mux.HandleFunc("POST /api/prs/{repo}/{number}/dock", s.prDock)
	mux.HandleFunc("GET /api/team", s.teamView)
	mux.HandleFunc("POST /api/team/narrate", s.teamNarrate)
	mux.HandleFunc("POST /api/team/seen", s.teamSeen)
	mux.HandleFunc("POST /api/team/summarise", s.teamSummarise)
	// A document outside a mission, and its threads: the workspace's own.
	mux.HandleFunc("GET /api/docs", s.doc)
	mux.HandleFunc("GET /api/documents", s.documents)
	mux.HandleFunc("GET /api/documents/search", s.searchDocuments)
	mux.HandleFunc("GET /api/documents/{repo}/cites", s.docCites)
	mux.HandleFunc("GET /api/documents/{repo}", s.spaceDocuments)
	mux.HandleFunc("GET /api/documents/{repo}/activity", s.spaceActivity)
	mux.HandleFunc("POST /api/documents/{repo}/fetch", s.fetchSpace)
	mux.HandleFunc("POST /api/documents/{repo}/pull", s.pullSpace)
	mux.HandleFunc("GET /api/docs/draft", s.getDraft)
	mux.HandleFunc("DELETE /api/docs/draft", s.dropDraft)
	mux.HandleFunc("GET /api/docs/threads", s.getThreads)
	mux.HandleFunc("GET /api/docs/threads/version", s.threadVersion)
	mux.HandleFunc("GET /api/missions/{slug}/threads", s.getThreads)
	mux.HandleFunc("POST /api/missions/{slug}/threads", s.openThread)
	mux.HandleFunc("GET /api/missions/{slug}/threads/version", s.threadVersion)
	mux.HandleFunc("POST /api/missions/{slug}/threads/approve", s.approveDoc)
	mux.HandleFunc("POST /api/missions/{slug}/threads/send", s.sendThreads)
	mux.HandleFunc("POST /api/missions/{slug}/threads/{id}/say", s.sayOnThread)
	mux.HandleFunc("POST /api/missions/{slug}/threads/{id}/status", s.setThreadStatus)
	mux.HandleFunc("POST /api/missions/{slug}/threads/{id}/suggestion", s.settleSuggestion)
	mux.HandleFunc("POST /api/missions/{slug}/threads/{id}/decision", s.recordDecision)
	mux.HandleFunc("POST /api/asks", s.ask)
	mux.HandleFunc("POST /api/conversations", s.startTalk)
	mux.HandleFunc("GET /api/conversations/{id}", s.getTalk)
	mux.HandleFunc("POST /api/conversations/{id}/say", s.sayInTalk)
	mux.HandleFunc("POST /api/conversations/{id}/status", s.setTalkStatus)
	mux.HandleFunc("GET /api/conversations/{id}/model", s.talkModel)
	mux.HandleFunc("POST /api/conversations/{id}/model", s.setTalkModel)
	mux.HandleFunc("POST /api/conversations/{id}/keep", s.keepAsk)
	mux.HandleFunc("POST /api/conversations/{id}/proposals/{n}", s.settleProposal)
	mux.HandleFunc("POST /api/pages", s.newPage)
	mux.HandleFunc("GET /api/pages/{id}", s.getPage)
	mux.HandleFunc("PUT /api/pages/{id}", s.savePage)
	mux.HandleFunc("DELETE /api/pages/{id}", s.removePage)
	mux.HandleFunc("GET /api/pages/{id}/cells", s.getPageCells)
	mux.HandleFunc("POST /api/pages/{id}/cells", s.openCell)
	mux.HandleFunc("POST /api/pages/{id}/cells/{cell}/fill", s.fillCell)
	mux.HandleFunc("POST /api/pages/{id}/cells/{cell}/say", s.tweakCell)
	mux.HandleFunc("POST /api/pages/{id}/cells/{cell}/placed", s.placedCell)
	mux.HandleFunc("GET /api/pages/{id}/passes", s.getPagePasses)
	mux.HandleFunc("POST /api/pages/{id}/file", s.filePage)
	mux.HandleFunc("POST /api/pages/open", s.openFile)
	mux.HandleFunc("POST /api/pages/{id}/save", s.saveToFile)
	mux.HandleFunc("POST /api/pages/{id}/bring-in", s.bringIn)
	mux.HandleFunc("POST /api/pages/{id}/passes/{pass}/run", s.runPass)
	mux.HandleFunc("POST /api/pages/{id}/passes/{pass}/findings/{key}", s.settleFinding)
	mux.HandleFunc("POST /api/missions/{slug}/conversations", s.startTalk)
	mux.HandleFunc("GET /api/missions/{slug}/conversations/{id}", s.getTalk)
	mux.HandleFunc("POST /api/missions/{slug}/conversations/{id}/say", s.sayInTalk)
	mux.HandleFunc("POST /api/missions/{slug}/conversations/{id}/status", s.setTalkStatus)
	mux.HandleFunc("GET /api/missions/{slug}/conversations/{id}/model", s.talkModel)
	mux.HandleFunc("POST /api/missions/{slug}/conversations/{id}/model", s.setTalkModel)
	mux.HandleFunc("POST /api/missions/{slug}/conversations/{id}/keep", s.keepAsk)
	mux.HandleFunc("POST /api/missions/{slug}/conversations/{id}/proposals/{n}", s.settleProposal)
	// Artifacts: the list is the workspace's; a version runs from /frame.
	mux.HandleFunc("GET /api/artifacts/{id}", s.artifactDetail)
	mux.HandleFunc("GET /api/artifacts/compare", s.artifactCompare)
	mux.HandleFunc("GET /api/artifacts/found", s.artifactsFound)
	mux.HandleFunc("GET /api/artifacts/found/detail", s.artifactFoundDetail)
	mux.HandleFunc("GET /api/artifacts/{id}/source", s.artifactSource)
	mux.HandleFunc("POST /api/artifacts/{id}/promote", s.promoteArtifact)
	mux.HandleFunc("POST /api/artifacts/{id}/discard", s.discardArtifact)
	mux.HandleFunc("POST /api/artifacts/{id}/restore", s.restoreArtifact)
	mux.HandleFunc("POST /api/artifacts/clear", s.clearArtifacts)
	mux.HandleFunc("GET /frame/{id}/{version}", s.frame)
	mux.HandleFunc("GET /frame/in/{repo}/tree/{tree}", s.foundFrame)
	mux.HandleFunc("GET /frame/{id}/proposals/{n}", s.proposalFrame)
	mux.HandleFunc("GET /frame/missions/{slug}/{id}/proposals/{n}", s.proposalFrame)
	mux.HandleFunc("GET "+framedRoot+"{rest...}", s.framedFile)
	// Plugins: what they add, the page ws serves for one's app (the
	// shell), and each one's own HTTP (asked of the plugin).
	mux.HandleFunc("GET /api/plugins", s.pluginsList)
	mux.HandleFunc("GET /api/commands", s.commandLabels)
	mux.HandleFunc("GET /api/plugins/installed", s.pluginsInstalled)
	mux.HandleFunc("POST /api/plugins", s.addPlugin)
	mux.HandleFunc("POST /api/plugins/{name}/update", s.updatePlugin)
	mux.HandleFunc("POST /api/plugins/{name}/remove", s.removePlugin)
	mux.HandleFunc("GET /api/plugins/{name}/refs/{kind}/{ref}", s.pluginThing)
	mux.HandleFunc("POST /api/plugins/{name}/actions/{id}", s.pluginAct)
	mux.HandleFunc("POST /api/plugins/{name}/things/promote", s.promoteThing)
	mux.HandleFunc("POST /api/plugins/{name}/things/save", s.saveThing)
	mux.HandleFunc("GET /api/plugins/{name}/things/offer", s.thingOffered)
	mux.HandleFunc("GET /plugin-frame/{name}", s.pluginShellRoot)
	mux.HandleFunc("GET /plugin-frame/{name}/{rest...}", s.pluginShell)
	mux.HandleFunc("/plugins/{name}", s.pluginRoot)
	mux.HandleFunc("/plugins/{name}/{rest...}", s.pluginView)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		jsonError(w, http.StatusNotFound, "no such API: "+r.Method+" "+r.URL.Path)
	})
	mux.HandleFunc("/", s.app)
	cost.Start(s.Root)
	return s.guard(s.forgetsOnWrite(cost.Wrap(mux)))
}

// forgetsOnWrite has the workspace's view built afresh after a write a
// page makes (anything but a read): what it wrote may be what the view's
// built from in a way its stamp doesn't say (an update looked for), and
// the page reads the view next.
func (s *Server) forgetsOnWrite(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			s.views.forget()
		}
	})
}

func (s *Server) origins() []string {
	p := strconv.Itoa(s.info.Port)
	out := []string{"http://127.0.0.1:" + p, "http://localhost:" + p}
	for _, n := range s.names() {
		out = append(out, "http://"+n)
	}
	return out
}

func (s *Server) hostAllowed(host string) bool {
	p := strconv.Itoa(s.info.Port)
	if host == "127.0.0.1:"+p || host == "localhost:"+p {
		return true
	}
	for _, n := range s.names() {
		if host == n {
			return true
		}
	}
	return false
}

func (s *Server) names() []string {
	if s.Names == nil {
		return nil
	}
	return s.Names()
}

func (s *Server) signedIn(r *http.Request) bool {
	given := ""
	if c, err := r.Cookie(s.cookieName()); err == nil {
		given = c.Value
	} else if b, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); ok {
		given = b
	}
	return given != "" && subtle.ConstantTimeCompare([]byte(given), []byte(s.info.Token)) == 1
}

// fromAnotherSite is whether a request another site's page made is to be
// refused, by what the browser says of it (Sec-Fetch-Site). Our cookie is
// Lax, so it goes with a link followed from anywhere, and with anything
// at all from a page on another port of this machine or a run under the
// workspace's name, which are the same site. Such a page can't read what
// it's answered, and a write needs our Origin, but a read that does
// something (a plugin's, which may answer a GET as it likes) would be
// done. So another site gets the app's pages, followed as a link, and a
// frame's files by their key (the frame is sandboxed: to the browser it's
// no one's); nothing else. A request that says nothing is a client that
// isn't a browser, or one too old to say.
func (s *Server) fromAnotherSite(r *http.Request) bool {
	switch r.Header.Get("Sec-Fetch-Site") {
	case "cross-site", "same-site":
	default:
		return false
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return true
	}
	if _, _, keyed := s.framed(r.URL.Path); keyed {
		return false
	}
	link := r.Header.Get("Sec-Fetch-Mode") == "navigate"
	return !link || strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/plugins/")
}

func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer") // /auth's URL carries the token
		// An app's page frames its run, which is on this machine: at its
		// name under .localhost, or at 127.0.0.1 and its port.
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-src 'self' http://*.localhost:* http://127.0.0.1:*; frame-ancestors 'none'")
		if !s.hostAllowed(r.Host) {
			http.Error(w, "this server answers only to 127.0.0.1 and localhost", http.StatusMisdirectedRequest)
			return
		}
		if r.URL.Path == "/auth" {
			next.ServeHTTP(w, r)
			return
		}
		if s.fromAnotherSite(r) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				jsonError(w, http.StatusForbidden, "refused: another site's page asked for this")
				return
			}
			page(w, http.StatusForbidden, "Refused", "Another site's page asked for this. Open the web UI with <code>ws web</code>.")
			return
		}
		if !s.signedIn(r) {
			// A frame's files are read by the key in their address: the
			// frame has an origin of its own, and sends no cookie.
			if _, _, keyed := s.framed(r.URL.Path); keyed && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
				next.ServeHTTP(w, r)
				return
			}
			if strings.HasPrefix(r.URL.Path, "/api/") {
				jsonError(w, http.StatusUnauthorized, "not signed in: open the web UI with `ws web`")
				return
			}
			page(w, http.StatusUnauthorized, "Not signed in", "This page signs in through a link from your terminal. Run <code>ws web</code> in the workspace.")
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			origin := r.Header.Get("Origin")
			ok := false
			for _, o := range s.origins() {
				ok = ok || origin == o
			}
			if !ok {
				jsonError(w, http.StatusForbidden, "cross-origin request refused")
				return
			}
			// A plugin's app posts to the plugin as the plugin takes it, a
			// form too: what's refused there is another site's request, above.
			plugins := strings.HasPrefix(r.URL.Path, "/plugins/")
			if mt, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type")); mt != "application/json" && !plugins {
				jsonError(w, http.StatusUnsupportedMediaType, "send JSON")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// auth signs a browser in with the token from ws web's link, then sends it
// on to where the link was for.
func (s *Server) auth(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if subtle.ConstantTimeCompare([]byte(q.Get("t")), []byte(s.info.Token)) != 1 {
		page(w, http.StatusForbidden, "That link has expired", "Sign-in links work once. Run <code>ws web</code> again for a fresh one.")
		return
	}
	// Lax, not Strict: a link to the UI from elsewhere (a notification, a
	// chat) still opens signed in. Reads are all a cross-site request can
	// make, and it can't see their answers; writes need our Origin.
	http.SetCookie(w, &http.Cookie{
		Name: s.cookieName(), Value: s.info.Token, Path: "/",
		HttpOnly: true, SameSite: http.SameSiteLaxMode, MaxAge: 400 * 24 * 60 * 60,
	})
	http.Redirect(w, r, safeNext(q.Get("next")), http.StatusSeeOther)
}

// shutdown closes the daemon. It answers before the daemon goes, so the
// page hears it's been closed rather than only losing it.
func (s *Server) shutdown(w http.ResponseWriter, r *http.Request) {
	if s.Shutdown == nil {
		jsonError(w, http.StatusNotFound, "this web UI has no daemon to close")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
	go s.Shutdown("closed from the web UI")
}

func (s *Server) workspace(w http.ResponseWriter, r *http.Request) {
	// While a page follows the workspace, its hub keeps the view current
	// (built again when what it's built from moves, and every half a
	// minute): the view's served as it's kept, built by nobody's read. A
	// write from a page forgets it, so that page reads what it wrote.
	if h := s.topics[topicWorkspace]; h != nil && h.watching() {
		if data, ok := s.views.kept("workspace"); ok {
			w.Header().Set("Content-Type", "application/json")
			w.Write(data)
			return
		}
	}
	data, err := s.views.workspace(s)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

// fingerprint says whether what the API serves has changed.
func (s *Server) fingerprint() string {
	data, err := s.views.workspace(s)
	if err != nil {
		return "error: " + err.Error()
	}
	h := fnv.New64a()
	h.Write(data)
	return strconv.FormatUint(h.Sum64(), 16)
}

// follow is a page's following of a topic (?topic=workspace, the
// default; reviews; team; plugins; mission:<slug>; pr:<repo>#<number>): a
// channel of event names, said when something the page shows has changed,
// and a function to stop. The plugins topic takes app, the plugin whose
// app the page shows, if it shows one: plugins are told who's looking, to
// fetch to suit. A topic that can't be followed here is refused with its
// status and why.
func (s *Server) follow(topic, app string) (ch <-chan told, done func(), status int, err error) {
	var subscribe func() (<-chan told, func())
	if h := s.topics[topic]; h != nil {
		subscribe = h.subscribe
	}
	if slug, ok := topicKey(topic, missionTopic); ok {
		subscribe = func() (<-chan told, func()) { return s.missions.subscribe(slug) }
	}
	if key, ok := topicKey(topic, conversationTopic); ok {
		subscribe = func() (<-chan told, func()) { return s.talks.subscribe(key) }
	}
	if key, ok := topicKey(topic, transcriptTopic); ok {
		subscribe = func() (<-chan told, func()) { return s.transcripts.subscribe(key) }
	}
	if key, ok := topicKey(topic, worktreeTopic); ok {
		subscribe = func() (<-chan told, func()) { return s.filesFollowed.subscribe(key) }
	}
	if ref, ok := topicKey(topic, runLogTopic); ok {
		subscribe = func() (<-chan told, func()) { return s.runLogs.subscribe(ref) }
	}
	if key, ok := topicKey(topic, appTopic); ok {
		subscribe = func() (<-chan told, func()) { return s.apps.subscribe(key) }
	}
	if key, ok := topicKey(topic, docTopic); ok {
		subscribe = func() (<-chan told, func()) { return s.docs.subscribe(key) }
	}
	if key, ok := topicKey(topic, screenTopic); ok {
		ch, done, running := s.followScreen(key)
		if !running {
			_, id, _ := strings.Cut(key, "/")
			return nil, nil, http.StatusNotFound, errors.New(id + " isn't running")
		}
		return ch, done, 0, nil
	}
	if ref, ok := prTopicRef(topic); ok && s.prs != nil {
		subscribe = func() (<-chan told, func()) { return s.followPR(ref) }
	}
	switch {
	case subscribe != nil:
	case topic == topicReviews, strings.HasPrefix(topic, prTopic):
		return nil, nil, http.StatusNotFound, errors.New("there's no GitHub to ask here")
	case topic == topicTeam:
		return nil, nil, http.StatusNotFound, errors.New("the team's events aren't available here")
	default:
		return nil, nil, http.StatusNotFound, errors.New("no such topic: " + topic)
	}
	look := func() {}
	if topic == topicPlugins && s.Plugins != nil {
		if !plugin.IsInstalled(s.Plugins.Root, app) {
			app = ""
		}
		look = s.Plugins.Look(app)
	}
	ch, stop := subscribe()
	return ch, func() { stop(); look() }, 0, nil
}

// app serves the single-page app: its files, and index.html for any other
// path (the app routes).
func (s *Server) app(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name != "" && name != "index.html" {
		if info, err := fs.Stat(s.Assets, name); err == nil && !info.IsDir() {
			if strings.HasPrefix(name, "assets/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable") // hashed names
			}
			http.ServeFileFS(w, r, s.Assets, name)
			return
		}
	}
	index, err := fs.ReadFile(s.Assets, "index.html")
	if err != nil {
		page(w, http.StatusInternalServerError, "No web UI built", "This ws was built without its web UI: run <code>just build</code>.")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Write(withAccent(index, s.App.Accent()))
}

// withAccent is the app's index.html with the accent set on its <html>
// (:root) as --accent-pick, so the page has it before it first paints and
// never flashes the default first. It's a style attribute, not a script,
// as the page's Content-Security-Policy runs no inline script, and allows
// inline style. With no accent, or one that isn't #rgb or #rrggbb, the
// page is as built, and foundations.css's default applies: nothing but a hex
// colour reaches the attribute.
func withAccent(index []byte, accent string) []byte {
	tag := htmlTag.FindIndex(index)
	if tag == nil || !hexColor(accent) {
		return index
	}
	at := tag[1] - 1 // the tag's closing >
	out := make([]byte, 0, len(index)+40)
	out = append(out, index[:at]...)
	out = append(out, ` style="--accent-pick: `+accent+`"`...)
	return append(out, index[at:]...)
}

// htmlTag is a page's <html> tag, with whatever attributes it has.
var htmlTag = regexp.MustCompile(`(?i)<html(?:\s[^>]*)?>`)

// hexColor reports whether s is a hex colour, #rgb or #rrggbb.
func hexColor(s string) bool {
	if len(s) != 4 && len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !('0' <= c && c <= '9' || 'a' <= c && c <= 'f' || 'A' <= c && c <= 'F') {
			return false
		}
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeError answers an operation that failed: ops's errors each with
// their own status (not found 404, invalid 422, refused 409 with its code,
// GitHub unreachable 502), and anything else as fallback, which handlers
// not yet on ops still choose.
func writeError(w http.ResponseWriter, err error, fallback int) {
	body := map[string]string{"error": err.Error()}
	status := fallback
	var refused *ops.Refused
	var inv *ops.Invalid
	var up *ops.Upstream
	switch {
	case errors.Is(err, ops.ErrNotFound), errors.Is(err, ErrNotFound):
		status = http.StatusNotFound
	case errors.As(err, &inv):
		status = http.StatusUnprocessableEntity
	case errors.As(err, &refused):
		status, body["code"] = http.StatusConflict, refused.Code
	case errors.As(err, &up):
		status = http.StatusBadGateway
	}
	writeJSON(w, status, body)
}

// page is a minimal HTML page for when the app can't be served.
func page(w http.ResponseWriter, status int, title, body string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	// Drawn as the web UI draws a notice, with its styles inline: a page
	// that isn't signed in gets no scripts or stylesheets. Without the web
	// UI's styles there's no accent to draw the mark in (it's worked out
	// there, and its default written there only), so it's in the page's
	// own colours.
	fmt.Fprintf(w, `<!doctype html><html lang="en"><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><meta name="color-scheme" content="light dark"><title>%s · ws</title>
<style>
:root{--bg:#fbfbfc;--fg:#1d1d21;--dim:#66666f;--panel:#f4f4f6;--line:#e6e6ea}
@media (prefers-color-scheme:dark){:root{--bg:#0f0f11;--fg:#e8e8ec;--dim:#8f8f99;--panel:#151518;--line:#24242a}}
body{margin:0;background:var(--bg);color:var(--fg);font:13px/1.5 -apple-system,BlinkMacSystemFont,"Segoe UI",system-ui,sans-serif;-webkit-font-smoothing:antialiased}
main{max-width:30rem;margin:18vh auto;padding:0 20px}
.mark{display:grid;place-items:center;width:32px;height:32px;border-radius:9px;font:700 16px/1 inherit;color:var(--bg);background:var(--fg);margin-bottom:16px}
h1{font-size:20px;font-weight:600;letter-spacing:-.01em;margin:0 0 8px}
p{color:var(--dim);margin:0}
code{font:12px ui-monospace,"SF Mono",Menlo,monospace;color:var(--fg);background:var(--panel);border:1px solid var(--line);border-radius:4px;padding:1px 5px;user-select:all}
</style>
<main><div class="mark">w</div><h1>%s</h1><p>%s</p></main>`, title, title, body)
}
