package daemon

import (
	"io"
	"log"
	"reflect"
	"testing"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/store/storetest"
)

// mayBeUnset are web.Server's fields that are nil by design here: the
// app it serves is the one built in (Assets), and with no GitHub there are
// no pull requests to list (Reviews).
var mayBeUnset = map[string]bool{"Assets": true, "Reviews": true}

// The web UI the daemon serves is given everything it asks of ws: a hook
// left unset is an endpoint that says "not found" for good.
func TestNewWeb_LeavesNoHookUnset(t *testing.T) {
	root := t.TempDir()
	storetest.Own(t, root)
	s := &Server{Root: root, Log: log.New(io.Discard, "", 0)}
	d := &daemon{cfg: Config{Root: root, Build: "test"}, s: s, prs: ops.NewPRCache(), plugins: &plugin.Host{Root: root}}
	d.w = ops.Open(root, ops.Options{Host: s, Plugins: hostRunner{d.plugins}, PRs: d.prs})
	d.w.KeepTeam()
	w := reflect.ValueOf(*d.newWeb())
	for i := range w.NumField() {
		f, field := w.Field(i), w.Type().Field(i)
		if !field.IsExported() || mayBeUnset[field.Name] {
			continue
		}
		switch f.Kind() {
		case reflect.Func, reflect.Pointer, reflect.Interface:
			if f.IsNil() {
				t.Errorf("web.Server.%s is unset", field.Name)
			}
		}
	}
}
