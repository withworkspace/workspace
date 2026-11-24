package web

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// A page's socket: one per tab, over which it follows every topic it
// shows (follow) and is told when one has changed. A browser allows a
// few connections to a server, shared by every tab, and a stream held
// for each topic used them up; a socket is one, whatever a page follows,
// and doesn't count against them.

// SocketOp is what a page asks of its socket.
type SocketOp string

const (
	// SocketFollow follows a topic: the page's told it's following
	// (SocketFollowing), and fetches what it shows then, so nothing that
	// changes after is missed.
	SocketFollow SocketOp = "follow"
	// SocketLeave stops following a topic.
	SocketLeave SocketOp = "leave"
)

// SocketAsk is what a page sends its socket: to follow a topic, or leave
// it. App is the plugin whose app the page shows, for the plugins topic.
type SocketAsk struct {
	Op    SocketOp `json:"op"`
	Topic string   `json:"topic"`
	App   string   `json:"app,omitempty"`
}

// SocketKind is what a socket tells a page.
type SocketKind string

const (
	// SocketFollowing: the topic's followed from now (a page fetches what
	// it shows).
	SocketFollowing SocketKind = "following"
	// SocketChanged: what the topic shows has changed.
	SocketChanged SocketKind = "changed"
	// SocketRefused: the topic can't be followed here, and why (Error).
	SocketRefused SocketKind = "refused"
)

// SocketSaid is what a socket sends a page, of one of its topics.
type SocketSaid struct {
	Kind  SocketKind `json:"kind"`
	Topic string     `json:"topic"`
	Error string     `json:"error,omitempty"`
	// Conversation is how a conversation changed (its topic's, changed),
	// when the page can be told: with none, it reads it again.
	Conversation *ConversationOp `json:"conversation,omitempty"`
	// Transcript is how a crew's session changed (its transcript topic's).
	Transcript *TranscriptOp `json:"transcript,omitempty"`
	// RunLog is what's been written to a run's log (its runlog topic's).
	RunLog *RunLogOp `json:"runLog,omitempty"`
	// Screen is a running crew's screen, or its end (its screen topic's).
	Screen *ScreenOp `json:"screen,omitempty"`
}

// socketPing is how often an idle socket is pinged, so one whose page has
// gone without closing it is found and let go.
const socketPing = 30 * time.Second

// socket serves a page's socket until it closes: what it follows is
// followed while it's open, and an open one holds the daemon up, as a
// stream does.
func (s *Server) socket(w http.ResponseWriter, r *http.Request) {
	// A socket's opened with a GET, which the guard lets through from any
	// page of ours' origins; another site's page could open one with our
	// cookie, so its Origin must be ours.
	if !slices.Contains(s.origins(), r.Header.Get("Origin")) {
		jsonError(w, http.StatusForbidden, "cross-origin socket refused")
		return
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{InsecureSkipVerify: true}) // the Origin is checked above
	if err != nil {
		return // Accept has answered
	}
	defer c.CloseNow()
	if s.Hold != nil {
		defer s.Hold()()
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	ss := &socketSession{s: s, c: c, ctx: ctx, follows: map[string]func(){}, out: make(chan SocketSaid, 64)}
	defer ss.leaveAll()
	go ss.write(cancel)
	for {
		var ask SocketAsk
		if err := readJSON(ctx, c, &ask); err != nil {
			return
		}
		switch ask.Op {
		case SocketFollow:
			ss.follow(ask.Topic, ask.App)
		case SocketLeave:
			ss.leave(ask.Topic)
		}
	}
}

// socketSession is one page's socket: the topics it follows, each with
// its way to stop, and what's to be sent it.
type socketSession struct {
	s   *Server
	c   *websocket.Conn
	ctx context.Context
	out chan SocketSaid

	mu      sync.Mutex
	follows map[string]func() // by topic
}

// follow follows topic for the page, if it isn't already: following it
// again is said again (the page fetches again), and nothing more.
func (ss *socketSession) follow(topic, app string) {
	ss.mu.Lock()
	_, already := ss.follows[topic]
	ss.mu.Unlock()
	if already {
		ss.say(SocketSaid{Kind: SocketFollowing, Topic: topic})
		return
	}
	ch, done, _, err := ss.s.follow(topic, app)
	if err != nil {
		ss.say(SocketSaid{Kind: SocketRefused, Topic: topic, Error: err.Error()})
		return
	}
	stop := make(chan struct{})
	var once sync.Once
	ss.mu.Lock()
	ss.follows[topic] = func() { once.Do(func() { close(stop); done() }) }
	ss.mu.Unlock()
	ss.say(SocketSaid{Kind: SocketFollowing, Topic: topic})
	go func() {
		// A screen that can't be sent now (the page is behind) waits, and
		// is sent once it can, unless a newer comes first: it's the latest
		// a page wants, not every one.
		var waiting *SocketSaid
		retry := time.NewTicker(screenEvery)
		defer retry.Stop()
		for {
			select {
			case <-ss.ctx.Done():
				return
			case <-stop:
				return
			case <-retry.C:
				if waiting != nil && ss.offer(*waiting) {
					waiting = nil
				}
			case t := <-ch:
				said := SocketSaid{Kind: SocketChanged, Topic: topic}
				switch op := t.Op.(type) {
				case *ConversationOp:
					said.Conversation = op
				case *TranscriptOp:
					said.Transcript = op
				case *RunLogOp:
					said.RunLog = op
				case *ScreenOp:
					said.Screen = op
					if !op.Exit {
						waiting = nil
						if !ss.offer(said) {
							waiting = &said
						}
						continue
					}
				}
				ss.say(said)
			}
		}
	}()
}

// leave stops following topic.
func (ss *socketSession) leave(topic string) {
	ss.mu.Lock()
	stop := ss.follows[topic]
	delete(ss.follows, topic)
	ss.mu.Unlock()
	if stop != nil {
		stop()
	}
}

func (ss *socketSession) leaveAll() {
	ss.mu.Lock()
	stops := ss.follows
	ss.follows = map[string]func(){}
	ss.mu.Unlock()
	for _, stop := range stops {
		stop()
	}
}

// say queues what's to be sent. A page so far behind that its queue is
// full has its socket closed: it opens another, follows again, and
// fetches what it shows, which is what it would have been told to do.
func (ss *socketSession) say(m SocketSaid) {
	select {
	case ss.out <- m:
	case <-ss.ctx.Done():
	default:
		ss.c.Close(websocket.StatusPolicyViolation, "too far behind")
	}
}

// offer queues what's said if there's room, and says whether there was: for
// what a newer will take the place of (a screen), which a page that's
// behind needn't have its socket closed for.
func (ss *socketSession) offer(m SocketSaid) bool {
	select {
	case ss.out <- m:
		return true
	default:
		return false
	}
}

// write sends what's said, and pings when there's nothing to, until the
// socket's done; it ends the session (cancel) when a write fails.
func (ss *socketSession) write(cancel func()) {
	defer cancel()
	ping := time.NewTicker(socketPing)
	defer ping.Stop()
	for {
		select {
		case <-ss.ctx.Done():
			return
		case m := <-ss.out:
			data, _ := json.Marshal(m)
			wctx, done := context.WithTimeout(ss.ctx, 10*time.Second)
			err := ss.c.Write(wctx, websocket.MessageText, data)
			done()
			if err != nil {
				return
			}
		case <-ping.C:
			pctx, done := context.WithTimeout(ss.ctx, 10*time.Second)
			err := ss.c.Ping(pctx)
			done()
			if err != nil {
				return
			}
		}
	}
}

// readJSON reads one message of JSON from c into v.
func readJSON(ctx context.Context, c *websocket.Conn, v any) error {
	_, data, err := c.Read(ctx)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}
