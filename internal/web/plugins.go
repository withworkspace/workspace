package web

import (
	"context"
	"errors"
	"html"
	"io/fs"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"

	"github.com/withworkspace/workspace/internal/plugin"
)

// Plugins in the web UI: each one's view is an app of the plugin's, which
// runs in a page ws serves (the shell), shown in a frame.
// /plugin-frame/<name>/… is the shell; /plugins/<name>/… is passed to the
// plugin, for its app's files and what its app asks it; /api/plugins says
// what each adds to the sidebar, and where it stands. How the app and ws
// talk is the frame protocol: PROTOCOL.md in github.com/withworkspace/ui-lib.

const topicPlugins = "plugins"

// PluginPolicy is the Content-Security-Policy of what a plugin's own HTTP
// answers: its own scripts and styles, inline too, and images from
// anywhere. It's a program the workspace's owner installed.
const PluginPolicy = "default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; frame-ancestors 'self'"

// ShellPolicy is the Content-Security-Policy of the shell: scripts, styles,
// fonts and connections from this origin alone, and no inline script (the
// shell has none, and the app is a file). Inline style is allowed, for ws's
// tokens and for an app that styles from its script.
const ShellPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https:; font-src 'self' data:; connect-src 'self'; frame-ancestors 'self'"

// shellStyles is the file in the built web UI that the shell's <style>
// holds: ws's tokens, light and dark, under the names a plugin's app knows
// them by (web/vite.config.ts writes it).
const shellStyles = "plugin-shell.css"

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
	// Changed counts the times it's said what its view shows is stale: an
	// app showing it is told when it moves.
	Changed uint64 `json:"changed"`
	// Starts counts the times it's been started: a page showing its view
	// loads its app again when it moves.
	Starts uint64 `json:"starts"`
}

// PluginView is an entry a plugin adds to the sidebar.
type PluginView struct {
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
	Under string `json:"under,omitempty"`
	// Path is where in the plugin's app it opens.
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

// pluginHeaders are what every answer under /plugins has, in place of the
// guard's, which would refuse to be framed and run no inline script.
func pluginHeaders(h http.Header) {
	h.Set("Content-Security-Policy", PluginPolicy)
	h.Set("X-Frame-Options", "SAMEORIGIN")
	// What a plugin serves says where it is to its own HTTP, and to nowhere
	// else. The app's pages say it to nobody: the sign-in link's address
	// carries the token.
	h.Set("Referrer-Policy", "same-origin")
}

// shellHeaders are what the shell has: as pluginHeaders, under the shell's
// own policy. It carries the accent and what the plugin registered, so
// it's never kept.
func shellHeaders(h http.Header) {
	h.Set("Content-Security-Policy", ShellPolicy)
	h.Set("X-Frame-Options", "SAMEORIGIN")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Cache-Control", "no-store")
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

// FrameBase is the path a plugin's shell is served under: the address of
// its view in ws, /x/<name>/…, under a prefix of the frame's own.
func FrameBase(name string) string { return "/plugin-frame/" + name }

// shell is the page ws serves for a plugin's view: ws's tokens, an empty
// #root, and the plugin's app. Where the app can't be loaded, it has a
// message of ws's in #root and no app.
type shell struct {
	Title string
	// Script and Style are the app's entry and its stylesheet, as addresses
	// in ws; "" for none.
	Script, Style string
	// Message is HTML for #root, in place of the app.
	Message string
}

// pluginSocket is the socket of a plugin's own HTTP, the plugin started if
// it isn't running. Where it can't be asked, the answer's written, as the
// shell with a message saying why, and ok is false.
func (s *Server) pluginSocket(w http.ResponseWriter, name string) (sock string, ok bool) {
	if s.Plugins == nil || !plugin.IsInstalled(s.Plugins.Root, name) {
		s.shellMessage(w, http.StatusNotFound, "There's no such plugin", "<p><code>ws plugin list</code> says which this workspace has.</p>")
		return "", false
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
			keys[i] = "<li>" + html.EscapeString(title) + " (<code>" + html.EscapeString(k.Key) + "</code>)</li>"
		}
		s.shellMessage(w, http.StatusServiceUnavailable, name+" needs setting up",
			"<p>It can't be used until these are set:</p><ul>"+strings.Join(keys, "")+"</ul><p>In a terminal: <code>ws plugin config "+html.EscapeString(name)+" --ask</code></p>")
		return "", false
	case err != nil:
		s.shellMessage(w, http.StatusBadGateway, name+" didn't start",
			"<p>"+html.EscapeString(err.Error())+"</p><p>Its log: <code>ws plugin logs "+html.EscapeString(name)+"</code></p>")
		return "", false
	}
	return sock, true
}

// pluginShell is GET /plugin-frame/{name}/{rest...}: the shell, for every
// path under the plugin's name. Which paths the app has is the app's to
// say, once it's loaded: so a link to somewhere deep in it, and a reload,
// load it there.
func (s *Server) pluginShell(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if outsideFrame(w, r, name) {
		return
	}
	if _, ok := s.pluginSocket(w, name); !ok {
		return
	}
	reg := s.Plugins.Status(name).Registration
	switch {
	case reg.UI == nil || reg.UI.Script == "":
		s.shellMessage(w, http.StatusBadGateway, name+" has no app to show",
			"<p>It was built on a framework from before plugin pages were apps: update it.</p>")
		return
	case !plugin.ValidUIPath(reg.UI.Script) || reg.UI.Style != "" && !plugin.ValidUIPath(reg.UI.Style):
		s.shellMessage(w, http.StatusBadGateway, name+"'s app can't be loaded",
			"<p>What it registered as its <code>ui</code> isn't a path in its own HTTP. <code>ws plugin check</code> says what's wrong with it.</p>")
		return
	}
	page := shell{Title: name, Script: plugin.ViewBase(name) + reg.UI.Script}
	if len(reg.Views) > 0 && reg.Views[0].Title != "" {
		page.Title = reg.Views[0].Title
	}
	if reg.UI.Style != "" {
		page.Style = plugin.ViewBase(name) + reg.UI.Style
	}
	s.writeShell(w, http.StatusOK, page)
}

// pluginShellRoot is GET /plugin-frame/{name}: the shell at the app's
// first path.
func (s *Server) pluginShellRoot(w http.ResponseWriter, r *http.Request) {
	if outsideFrame(w, r, r.PathValue("name")) {
		return
	}
	http.Redirect(w, r, FrameBase(r.PathValue("name"))+"/", http.StatusSeeOther)
}

// outsideFrame sends a shell asked for as a page of its own, not in ws's
// frame (a relative link in the app opened in a tab, say), to the same
// path under /x/<name>/, where ws shows it: the bare shell is no use to
// anyone. The browser says which (Sec-Fetch-Dest); one that doesn't say is
// given the shell.
func outsideFrame(w http.ResponseWriter, r *http.Request, name string) bool {
	if r.Header.Get("Sec-Fetch-Dest") != "document" {
		return false
	}
	to := "/x/" + name + "/" + r.PathValue("rest")
	if r.URL.RawQuery != "" {
		to += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
	return true
}

// pluginView is /plugins/{name}/{rest...}: the plugin's own HTTP, asked of
// the plugin, which is started if it isn't running. It's where its app's
// files are, and what its app asks it.
func (s *Server) pluginView(w http.ResponseWriter, r *http.Request) {
	pluginHeaders(w.Header())
	name := r.PathValue("name")
	sock, ok := s.pluginSocket(w, name)
	if !ok {
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
			// ws's policy for what a plugin serves is the one that holds.
			res.Header.Del("Content-Security-Policy")
			res.Header.Del("X-Frame-Options")
			// A redirect to a path of the plugin's own is to it under its base.
			if loc := res.Header.Get("Location"); strings.HasPrefix(loc, "/") && !strings.HasPrefix(loc, "//") && loc != base && !strings.HasPrefix(loc, base+"/") {
				res.Header.Set("Location", base+loc)
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			s.shellMessage(w, http.StatusBadGateway, name+" isn't answering",
				"<p>"+html.EscapeString(err.Error())+"</p><p>Its log: <code>ws plugin logs "+html.EscapeString(name)+"</code></p>")
		},
	}
	proxy.ServeHTTP(w, r)
}

// pluginRoot is /plugins/{name}: the root of the plugin's own HTTP.
func (s *Server) pluginRoot(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, plugin.ViewBase(r.PathValue("name"))+"/", http.StatusSeeOther)
}

// shellMessage is the shell with a message of ws's in place of an app: why
// the plugin's can't be shown. title is text, and body is HTML.
func (s *Server) shellMessage(w http.ResponseWriter, status int, title, body string) {
	s.writeShell(w, status, shell{Title: title, Message: "<h2>" + html.EscapeString(title) + "</h2>" + body})
}

// writeShell writes the shell. The theme is there before anything of the
// plugin's has loaded: /theme.js (the web UI's own, a file, as the policy
// runs no inline script) sets light or dark on <html> as it does for the
// web UI, the accent is on <html> as it is there, and the <style> has every
// token for both, worked out from those two by the browser. So the frame's
// first paint is in ws's colours.
func (s *Server) writeShell(w http.ResponseWriter, status int, p shell) {
	styles, err := fs.ReadFile(s.Assets, shellStyles)
	if err != nil {
		page(w, http.StatusInternalServerError, "No web UI built", "This ws was built without its web UI: run <code>just build</code>.")
		return
	}
	var b strings.Builder
	b.WriteString("<!doctype html>\n<html lang=\"en\"")
	if accent := s.App.Accent(); hexColor(accent) {
		b.WriteString(` style="--accent-pick: ` + accent + `"`)
	}
	b.WriteString(">\n<head>\n<meta charset=\"utf-8\">\n<meta name=\"color-scheme\" content=\"light dark\">\n<script src=\"/theme.js\"></script>\n")
	b.WriteString("<title>" + html.EscapeString(p.Title) + "</title>\n<style>")
	b.Write(styles)
	b.WriteString("</style>\n")
	if p.Message == "" {
		if p.Style != "" {
			b.WriteString(`<link rel="stylesheet" href="` + html.EscapeString(p.Style) + "\">\n")
		}
		b.WriteString(`<script type="module" src="` + html.EscapeString(p.Script) + "\"></script>\n")
	}
	b.WriteString("</head>\n<body><div id=\"root\">")
	if p.Message != "" {
		b.WriteString(`<div class="ws-shell-message">` + p.Message + "</div>")
	}
	b.WriteString("</div></body>\n</html>\n")
	shellHeaders(w.Header())
	w.WriteHeader(status)
	w.Write([]byte(b.String()))
}
