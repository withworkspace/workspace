package daemon

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/withworkspace/plugin/protocol"
	"github.com/withworkspace/workspace/internal/plugin"
)

// answerPlugin answers what a plugin asks ws (plugin.Host's Answer): ws
// show's view, as the commands make it (Config.Show), and everything else
// from the workspace's operations.
func (d *daemon) answerPlugin(c context.Context, who, method string, params json.RawMessage) (any, error) {
	if method != protocol.MethodShow {
		return d.w.AnswerPlugin(c, who, method, params)
	}
	if d.cfg.Show == nil {
		return nil, plugin.InvalidParams(errors.New("this daemon can't show what ws show does"))
	}
	return d.cfg.Show(d.w, c, params)
}

// hostRunner is the daemon's own plugins, asked directly
// (ops.Options.Plugins).
type hostRunner struct{ h *plugin.Host }

func (r hostRunner) Statuses(context.Context) ([]plugin.Status, error) { return r.h.Statuses(), nil }
func (r hostRunner) Refresh(_ context.Context, name string) (plugin.Status, error) {
	return r.h.Refresh(name)
}
func (r hostRunner) Stop(_ context.Context, name string) error { r.h.Stop(name); return nil }
func (hostRunner) Close()                                      {}
