// Package plugin is ws's side of plugins: programs that add a view to the
// web UI, commands to ws, and a guide for agents, without being part of
// ws. A plugin is one process the daemon holds (Host), spoken to over one
// JSON-RPC connection on its stdin and stdout. A command reaches it through
// the daemon's socket for them (Client).
package plugin

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// ProtocolVersion is the version of the protocol ws speaks to a plugin.
const ProtocolVersion = 1

// ManifestName is the file that says how to start a plugin.
const ManifestName = "plugin.toml"

// Manifest is a plugin's plugin.toml: what ws needs before the plugin can
// speak for itself.
type Manifest struct {
	Name    string   `toml:"name"`
	Version string   `toml:"version"`
	Command []string `toml:"command"`
	// Build is the command that builds the plugin from its source, run in
	// its folder: for a plugin being written (ws plugin dev), and one added
	// from a folder that hasn't been built. None: it's installed built.
	Build []string `toml:"build"`
}

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9-]{0,39}$`)

// ValidName says whether name can be a plugin's: it's a folder, a word
// after ws and a part of an address.
func ValidName(name string) bool { return nameRE.MatchString(name) }

// ReadManifest reads the plugin.toml in dir.
func ReadManifest(dir string) (Manifest, error) {
	var m Manifest
	path := filepath.Join(dir, ManifestName)
	if _, err := toml.DecodeFile(path, &m); err != nil {
		if os.IsNotExist(err) {
			return m, fmt.Errorf("%s has no %s: it isn't a plugin", dir, ManifestName)
		}
		return m, fmt.Errorf("reading %s: %w", path, err)
	}
	switch {
	case !ValidName(m.Name):
		return m, fmt.Errorf("%s: name %q must be lower-case letters, digits and hyphens, starting with a letter", path, m.Name)
	case len(m.Command) == 0 || m.Command[0] == "":
		return m, fmt.Errorf("%s: command is missing: it's how the plugin is started", path)
	}
	return m, nil
}

// Dir is where the workspace at root keeps its plugins.
func Dir(root string) string { return filepath.Join(root, ".ws", "plugins") }

// Where a plugin's files are. ws's own (the registration it last gave,
// its data) are kept apart from the plugin's (app), so a plugin linked
// from a folder someone's working in isn't written into.
func home(root, name string) string    { return filepath.Join(Dir(root), name) }
func appDir(root, name string) string  { return filepath.Join(home(root, name), "app") }
func dataDir(root, name string) string { return filepath.Join(home(root, name), "data") }
func registrationPath(root, name string) string {
	return filepath.Join(home(root, name), "registration.json")
}

// LogPath is where a plugin's stderr is kept.
func LogPath(root, name string) string { return filepath.Join(dataDir(root, name), "plugin.log") }

// Installed are the names of the plugins the workspace at root has, in
// order.
func Installed(root string) []string {
	entries, err := os.ReadDir(Dir(root))
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !ValidName(e.Name()) {
			continue
		}
		if _, err := os.Stat(filepath.Join(appDir(root, e.Name()), ManifestName)); err == nil {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names
}

// IsInstalled says whether the workspace at root has a plugin called name.
func IsInstalled(root, name string) bool {
	if !ValidName(name) {
		return false
	}
	_, err := os.Stat(filepath.Join(appDir(root, name), ManifestName))
	return err == nil
}

// Linked says whether a plugin is a link to a folder elsewhere (ws plugin
// add --link), and where to.
func Linked(root, name string) (string, bool) {
	target, err := os.Readlink(appDir(root, name))
	return target, err == nil
}

// Registration is what a plugin adds to ws, as it said when it last
// started. Every part is optional.
type Registration struct {
	Description   string      `json:"description,omitempty"`
	Views         []View      `json:"views,omitempty"`
	UI            *UI         `json:"ui,omitempty"`
	Commands      *Commands   `json:"commands,omitempty"`
	Agent         *Agent      `json:"agent,omitempty"`
	Syncs         []Sync      `json:"syncs,omitempty"`
	ConfigVersion int         `json:"config_version,omitempty"`
	Config        []ConfigKey `json:"config,omitempty"`
}

// View is an entry in the web UI's sidebar, and where in the plugin's
// app it opens.
type View struct {
	Title string `json:"title"`
	Icon  string `json:"icon,omitempty"`
	Under string `json:"under,omitempty"`
	Path  string `json:"path,omitempty"`
}

// UI is a plugin's app: what ws's page for it (the shell, at
// /plugin-frame/<name>/…) loads. A plugin has one, whatever views it has,
// and serves no HTML of its own. Each is a path in the plugin's own HTTP,
// with whatever query it has (/ui/app.js?v=3f9c2a), loaded from under
// ViewBase.
type UI struct {
	// Script is the app's entry, loaded as a module.
	Script string `json:"script,omitempty"`
	// Style is its stylesheet, if it has one, in the page's head.
	Style string `json:"style,omitempty"`
}

// ValidUIPath says whether p can be one of a UI's: a path from the root of
// the plugin's HTTP, which stays inside it. A query is the plugin's own.
func ValidUIPath(p string) bool {
	if !strings.HasPrefix(p, "/") || strings.HasPrefix(p, "//") || strings.ContainsAny(p, "\\#") {
		return false
	}
	u, err := url.Parse(p)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return false
	}
	// A browser reads %2e%2e as .. too: it's the path as it's read that
	// mustn't climb out.
	for _, seg := range strings.Split(u.Path, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// Commands are what a plugin adds after ws: one word, and what follows it.
type Commands struct {
	Name        string       `json:"name,omitempty"`
	Short       string       `json:"short,omitempty"`
	Subcommands []Subcommand `json:"subcommands,omitempty"`
}

// Subcommand is one of a plugin's commands, for help.
type Subcommand struct {
	Name  string `json:"name"`
	Short string `json:"short,omitempty"`
	Args  string `json:"args,omitempty"`
}

// Agent is what a plugin tells agents: a guide, and the roles it's for
// (none: every role).
type Agent struct {
	Roles []string `json:"roles,omitempty"`
	Guide string   `json:"guide,omitempty"`
}

// Sync is something a plugin fetches in the background, for show.
type Sync struct {
	Name  string `json:"name"`
	Every int    `json:"every,omitempty"` // seconds
}

// ConfigKey is one thing a plugin can be told in config.
type ConfigKey struct {
	Key      string   `json:"key"`
	Title    string   `json:"title,omitempty"`
	Type     string   `json:"type,omitempty"`
	Required bool     `json:"required,omitempty"`
	Default  any      `json:"default,omitempty"`
	Help     string   `json:"help,omitempty"`
	Example  string   `json:"example,omitempty"`
	Options  []string `json:"options,omitempty"`
	Scope    string   `json:"scope,omitempty"`
}

// Secret says whether the key's value is kept apart from config's files.
func (k ConfigKey) Secret() bool { return k.Type == "secret" }

// CommandName is the word after ws that runs the plugin's commands: its
// own, or the plugin's name. "" if it adds none.
func (r Registration) CommandName(plugin string) string {
	if r.Commands == nil {
		return ""
	}
	if r.Commands.Name != "" {
		return r.Commands.Name
	}
	return plugin
}

// ReadRegistration is what the plugin said it adds when it last started;
// false if it never has.
func ReadRegistration(root, name string) (Registration, bool) {
	var r Registration
	data, err := os.ReadFile(registrationPath(root, name))
	if err != nil || json.Unmarshal(data, &r) != nil {
		return Registration{}, false
	}
	return r, true
}

func writeRegistration(root, name string, r Registration) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(registrationPath(root, name), append(data, '\n'), 0o644)
}

// Add installs the plugin in the folder src into the workspace at root: a
// copy of it, or with link a link to it (for a plugin being written). It
// isn't started.
func Add(root, src string, link bool) (Manifest, error) {
	src, err := filepath.Abs(src)
	if err != nil {
		return Manifest{}, err
	}
	m, err := ReadManifest(src)
	if err != nil {
		return m, err
	}
	if err := os.MkdirAll(home(root, m.Name), 0o755); err != nil {
		return m, err
	}
	app := appDir(root, m.Name)
	if _, err := os.Lstat(app); err == nil {
		// Its data and config stay: adding again is how one's replaced.
		if err := os.RemoveAll(app); err != nil {
			return m, err
		}
	}
	if link {
		return m, os.Symlink(src, app)
	}
	return m, os.CopyFS(app, os.DirFS(src))
}

// Remove takes the plugin out of the workspace at root, with what ws kept
// for it and, with data, what it kept for itself.
func Remove(root, name string, data bool) error {
	if !ValidName(name) {
		return fmt.Errorf("%q isn't a plugin's name", name)
	}
	if _, err := os.Lstat(appDir(root, name)); err != nil {
		return fmt.Errorf("there's no plugin called %s", name)
	}
	if data {
		return os.RemoveAll(home(root, name))
	}
	if err := os.RemoveAll(appDir(root, name)); err != nil {
		return err
	}
	if err := os.Remove(registrationPath(root, name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return WriteSource(root, name, Source{}) // it's no longer from anywhere
}
