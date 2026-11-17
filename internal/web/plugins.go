package web

import (
	"context"
	"embed"
	"errors"
	"html"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"

	"github.com/withworkspace/workspace/internal/plugin"
)

// Plugins in the web UI: each one's view is its own pages, served by the
// plugin and shown in a frame. /plugins/<name>/… is passed to the plugin;
// /api/plugins says what each adds to the sidebar, and where it stands.

//go:embed pluginkit/plugin-kit.js pluginkit/plugin-kit.css
var pluginKit embed.FS

const topicPlugins = "plugins"

// PluginPolicy is the Content-Security-Policy of a plugin's page: its own
// scripts and styles, inline too, and images from anywhere. It's a program
// the workspace's owner installed, in a frame of the web UI's own.
const PluginPolicy = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; frame-ancestors 'self'"

// Plugin is one of the workspace's plugins, as GET /api/plugins serves it.
type Plugin struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// State is ready, setup (something it requires isn't set), failed, or
	// stopped (it's started when its view is opened).
	State   string       `json:"state"`
	Error   string       `json:"error,omitempty"`
	Missing []string     `json:"missing,omitempty"`
	Views   []PluginView `json:"views"`
	// Changed counts the times it's said what its view shows is stale: a
	// page showing it reloads when it moves.
	Changed uint64 `json:"changed"`
	// Starts counts the times it's been started: a page showing its view
	// loads it again when it moves.
	Starts uint64 `json:"starts"`
}

// PluginView is an entry a plugin adds to the sidebar.
type PluginView struct {
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
	Under string `json:"under,omitempty"`
	// Path is where in the plugin's pages it opens.
	Path  string `json:"path"`
	Badge int    `json:"badge,omitempty"`
}

func (s *Server) pluginsList(w http.ResponseWriter, r *http.Request) {
	out := []Plugin{}
	if s.Plugins != nil {
		for _, st := range s.Plugins.Statuses() {
			p := Plugin{Name: st.Name, Description: st.Registration.Description, State: string(st.State), Error: st.Error, Missing: st.Missing, Views: []PluginView{}, Changed: st.Changed, Starts: st.Starts}
			for _, v := range st.Registration.Views {
				path := v.Path
				if path == "" {
					path = "/"
				}
				p.Views = append(p.Views, PluginView{Title: v.Title, Icon: v.Icon, Under: v.Under, Path: path, Badge: st.Badges[path]})
			}
			out = append(out, p)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

// pluginKitFile is GET /plugin-kit.js and /plugin-kit.css: a plugin's
// page's side of the frame, and ws's look for it.
func (s *Server) pluginKitFile(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFileFS(w, r, pluginKit, "pluginkit"+r.URL.Path)
}

// pluginHeaders are what every answer under /plugins has, in place of the
// guard's, which would refuse to be framed and run no inline script.
func pluginHeaders(h http.Header) {
	h.Set("Content-Security-Policy", PluginPolicy)
	h.Set("X-Frame-Options", "SAMEORIGIN")
	// A plugin's page says where it is to its own pages (a form's handler
	// sends the person back there), and to nowhere else. The app's pages
	// say it to nobody: the sign-in link's address carries the token.
	h.Set("Referrer-Policy", "same-origin")
}

// pluginTransports are the connections to plugins' views, by socket.
var pluginTransports sync.Map // socket path → *http.Transport

func pluginTransport(sock string) *http.Transport {
	if t, ok := pluginTransports.Load(sock); ok {
		return t.(*http.Transport)
	}
	t := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", sock)
		},
	}
	actual, _ := pluginTransports.LoadOrStore(sock, t)
	return actual.(*http.Transport)
}

// pluginView is /plugins/{name}/{rest...}: a plugin's own page, asked of
// the plugin, which is started if it isn't running.
func (s *Server) pluginView(w http.ResponseWriter, r *http.Request) {
	pluginHeaders(w.Header())
	name := r.PathValue("name")
	if s.Plugins == nil || !plugin.ValidName(name) {
		pluginPage(w, http.StatusNotFound, "There's no such plugin", "")
		return
	}
	sock, err := s.Plugins.ViewSocket(name)
	var setup *plugin.SetupError
	switch {
	case errors.As(err, &setup):
		keys := make([]string, len(setup.Missing))
		for i, k := range setup.Missing {
			title := k.Title
			if title == "" {
				title = k.Key
			}
			keys[i] = "<li>" + html.EscapeString(title) + " <span class=\"dim\">(<code>" + html.EscapeString(k.Key) + "</code>)</span></li>"
		}
		pluginPage(w, http.StatusServiceUnavailable, html.EscapeString(name)+" needs setting up",
			"<p>It can't be used until these are set:</p><ul>"+strings.Join(keys, "")+"</ul><p>In a terminal: <code>ws plugin config "+html.EscapeString(name)+" --ask</code></p>")
		return
	case err != nil:
		pluginPage(w, http.StatusBadGateway, html.EscapeString(name)+" didn't start",
			"<p>"+html.EscapeString(err.Error())+"</p><p>Its log: <code>ws plugin logs "+html.EscapeString(name)+"</code></p>")
		return
	}
	base := plugin.ViewBase(name)
	proxy := &httputil.ReverseProxy{
		Transport: pluginTransport(sock),
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.Out.URL.Scheme, pr.Out.URL.Host = "http", "plugin"
			pr.Out.URL.Path, pr.Out.URL.RawPath = "/"+r.PathValue("rest"), ""
			pr.Out.Host = r.Host
			// The plugin is given the request, not who's signed in to ws.
			pr.Out.Header.Del("Cookie")
			pr.Out.Header.Del("Authorization")
			pr.Out.Header.Set("X-Forwarded-Prefix", base)
		},
		ModifyResponse: func(res *http.Response) error {
			// ws's policy for a plugin's page is the one that holds.
			res.Header.Del("Content-Security-Policy")
			res.Header.Del("X-Frame-Options")
			// A redirect to a path of the plugin's own is to it under its base.
			if loc := res.Header.Get("Location"); strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//") && loc != base && !strings.HasPrefix(loc, base+"/") {
				res.Header.Set("Location", base+loc)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			pluginPage(w, http.StatusBadGateway, html.EscapeString(name)+" isn't answering",
				"<p>"+html.EscapeString(err.Error())+"</p><p>Its log: <code>ws plugin logs "+html.EscapeString(name)+"</code></p>")
		},
	}
	proxy.ServeHTTP(w, r)
}

// pluginRoot is /plugins/{name}: the plugin's first page.
func (s *Server) pluginRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, plugin.ViewBase(r.PathValue("name"))+"/", http.StatusSeeOther)
}

// pluginPage is a page of ws's own in a plugin's frame: why the plugin's
// can't be shown. body is HTML.
func pluginPage(w http.ResponseWriter, status int, title, body string) {
	pluginHeaders(w.Header())
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	w.Write([]byte(`<!doctype html><html><head><meta charset="utf-8"><title>` + title + `</title>` +
		`<link rel="stylesheet" href="/plugin-kit.css"><script src="/plugin-kit.js"></script></head>` +
		`<body><main><div class="empty"><h2>` + title + `</h2>` + body + `</div></main></body></html>`))
}
