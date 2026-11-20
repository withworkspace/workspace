package web

import (
	"net/http"

	"github.com/withworkspace/workspace/internal/plugin"
)

// CommandLabel is what one of ws's commands is doing, as a conversation
// shows an agent's step that ran it: "Looking up sketch-5" for ws show
// sketch-5 -f llm. Words are the command's after ws (none for ws on its
// own); in Label, {1} is its first argument, {2} its second. Options are
// the flags it takes that have a value, as they're typed (--format, -f):
// how the web UI tells an option's value from an argument.
type CommandLabel struct {
	Words   []string `json:"words"`
	Label   string   `json:"label"`
	Options []string `json:"options,omitempty"`
}

// commandLabels is GET /api/commands: ws's own commands' labels, then
// each plugin's, as it last registered them, running or not (as ws's
// commands are added for them).
func (s *Server) commandLabels(w http.ResponseWriter, r *http.Request) {
	out := append([]CommandLabel{}, s.Labels...)
	if s.Plugins != nil {
		for _, name := range plugin.Installed(s.Plugins.Root) {
			reg, ok := plugin.ReadRegistration(s.Plugins.Root, name)
			cmds, word := reg.Commands, reg.CommandName(name)
			if !ok || cmds == nil || word == "" {
				continue
			}
			for _, sub := range cmds.Subcommands {
				if sub.Label == "" {
					continue
				}
				options := []string{"--format", "-f"}
				for _, o := range sub.Options {
					options = append(options, "--"+o)
				}
				out = append(out, CommandLabel{Words: []string{word, sub.Name}, Label: sub.Label, Options: options})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}
