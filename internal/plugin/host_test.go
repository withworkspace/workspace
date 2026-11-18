package plugin_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/withworkspace/workspace/internal/plugin"
	"github.com/withworkspace/workspace/internal/plugin/fakeplugin"
)

func TestMain(m *testing.M) {
	fakeplugin.RunIfAsked()
	os.Exit(m.Run())
}

// workspace is a folder with the least a workspace needs, and a host for
// its plugins that's shut down with the test.
func workspace(t *testing.T) (string, *plugin.Host) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "ws.toml"), []byte("[workspace]\norg = \"acme\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := &plugin.Host{Root: root, Version: "test", Reserved: func(w string) bool { return w == "status" }}
	t.Cleanup(h.Shutdown)
	return root, h
}

func add(t *testing.T, root, name string, o fakeplugin.Options) {
	t.Helper()
	if _, err := plugin.Add(root, fakeplugin.Install(t, name, o), false); err != nil {
		t.Fatal(err)
	}
}

// output collects a command's output, by stream.
type output struct {
	mu          sync.Mutex
	out, errOut strings.Builder
}

func (o *output) write(stream, data string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if stream == "stderr" {
		o.errOut.WriteString(data)
		return
	}
	o.out.WriteString(data)
}

func (o *output) stdout() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.out.String()
}

func TestAddStartsAndRegisters(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "fake", fakeplugin.Options{})
	if got := plugin.Installed(root); len(got) != 1 || got[0] != "fake" {
		t.Fatalf("installed = %v", got)
	}
	if st := h.Status("fake"); st.State != plugin.Stopped || len(st.Registration.Views) != 0 {
		t.Errorf("before it's started: %+v", st)
	}
	st, err := h.Refresh("fake")
	if err != nil {
		t.Fatal(err)
	}
	if st.State != plugin.Ready || st.Version != "1.2.3" {
		t.Errorf("started: %+v", st)
	}
	if len(st.Registration.Views) != 1 || st.Registration.Views[0].Title != "Fake" {
		t.Errorf("views = %+v", st.Registration.Views)
	}
	if got := st.Registration.CommandName("fake"); got != "fake" {
		t.Errorf("command = %q", got)
	}
	// What it registered is kept, for when it isn't running.
	h.Stop("fake")
	if reg, ok := plugin.ReadRegistration(root, "fake"); !ok || len(reg.Views) != 1 {
		t.Errorf("kept registration = %+v, %v", reg, ok)
	}
	if st := h.Status("fake"); st.State != plugin.Stopped || len(st.Registration.Views) != 1 {
		t.Errorf("stopped: %+v", st)
	}
}

func TestRunCommand(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "fake", fakeplugin.Options{})
	ctx := context.Background()

	var o output
	exit, err := h.Run(ctx, "fake", []string{"echo", "one", "two"}, "/somewhere", plugin.Terminal{}, o.write)
	if err != nil || exit != 0 || o.stdout() != "one two\n" {
		t.Errorf("echo: exit %d, err %v, out %q", exit, err, o.stdout())
	}

	o = output{}
	exit, err = h.Run(ctx, "fake", []string{"dir"}, "/somewhere", plugin.Terminal{}, o.write)
	if err != nil || o.stdout() != "/somewhere\n" {
		t.Errorf("dir: exit %d, err %v, out %q", exit, err, o.stdout())
	}

	o = output{}
	exit, err = h.Run(ctx, "fake", []string{"fail"}, "", plugin.Terminal{}, o.write)
	if err != nil || exit != 3 || o.errOut.String() != "it failed\n" {
		t.Errorf("fail: exit %d, err %v, stderr %q", exit, err, o.errOut.String())
	}

	// Two at once each get their own output.
	var wg sync.WaitGroup
	for _, word := range []string{"alpha", "beta"} {
		wg.Go(func() {
			var o output
			if _, err := h.Run(ctx, "fake", []string{"echo", word}, "", plugin.Terminal{}, o.write); err != nil || o.stdout() != word+"\n" {
				t.Errorf("%s: err %v, out %q", word, err, o.stdout())
			}
		})
	}
	wg.Wait()
}

func TestRunCancelled(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "fake", fakeplugin.Options{})
	ctx, cancel := context.WithCancel(context.Background())
	var o output
	done := make(chan int, 1)
	go func() {
		exit, _ := h.Run(ctx, "fake", []string{"sleep"}, "", plugin.Terminal{}, o.write)
		done <- exit
	}()
	for deadline := time.Now().Add(5 * time.Second); o.stdout() == "" && time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
	}
	cancel()
	select {
	case exit := <-done:
		if exit != 130 {
			t.Errorf("exit = %d, want 130", exit)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the command didn't end when it was cancelled")
	}
	// The plugin's still there for the next one.
	o = output{}
	if _, err := h.Run(context.Background(), "fake", []string{"echo", "after"}, "", plugin.Terminal{}, o.write); err != nil || o.stdout() != "after\n" {
		t.Errorf("after a cancel: err %v, out %q", err, o.stdout())
	}
}

func TestSetupThenConfig(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "fake", fakeplugin.Options{Require: "token"})
	st, err := h.Refresh("fake")
	if err != nil {
		t.Fatal(err)
	}
	if st.State != plugin.Setup || len(st.Missing) != 1 || st.Missing[0] != "token" {
		t.Fatalf("with a required key unset: %+v", st)
	}
	var setup *plugin.SetupError
	if _, err := h.Run(context.Background(), "fake", []string{"echo"}, "", plugin.Terminal{}, func(string, string) {}); !errors.As(err, &setup) {
		t.Fatalf("a command before it's set up: %v", err)
	}
	if _, err := h.ViewSocket("fake"); !errors.As(err, &setup) {
		t.Errorf("its view before it's set up: %v", err)
	}

	keys := map[string]plugin.ConfigKey{}
	for _, k := range st.Registration.Config {
		keys[k.Key] = k
	}
	if err := plugin.SetValue(root, "fake", keys["token"], "s3cret"); err != nil {
		t.Fatal(err)
	}
	if err := plugin.SetValue(root, "fake", keys["greeting"], "Howdy"); err != nil {
		t.Fatal(err)
	}
	// A secret is in neither config file.
	local, _ := os.ReadFile(filepath.Join(root, "ws.local.toml"))
	if strings.Contains(string(local), "s3cret") || !strings.Contains(string(local), "Howdy") {
		t.Errorf("ws.local.toml = %q", local)
	}
	if err := h.ConfigChanged("fake"); err != nil {
		t.Fatal(err)
	}
	if st := h.Status("fake"); st.State != plugin.Ready {
		t.Errorf("once it's set up: %+v", st)
	}
	var o output
	if _, err := h.Run(context.Background(), "fake", []string{"config"}, "", plugin.Terminal{}, o.write); err != nil {
		t.Fatal(err)
	}
	if got := o.stdout(); !strings.Contains(got, `"token":"s3cret"`) || !strings.Contains(got, `"greeting":"Howdy"`) {
		t.Errorf("the plugin's config after config/changed = %s", got)
	}
}

func TestViewSocket(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "fake", fakeplugin.Options{})
	sock, err := h.ViewSocket("fake")
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return net.Dial("unix", sock)
	}}}
	res, err := client.Get("http://plugin/api/greeting")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "from the fake") {
		t.Errorf("what it answered = %q", body)
	}
}

func TestBadgeAndChanged(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "fake", fakeplugin.Options{})
	run := func(word string) {
		t.Helper()
		if _, err := h.Run(context.Background(), "fake", []string{word}, "", plugin.Terminal{}, func(string, string) {}); err != nil {
			t.Fatal(err)
		}
	}
	run("echo")
	before := h.Stamp()
	run("badge")
	run("changed")
	st := h.Status("fake")
	if st.Badges["/"] != 3 || st.Changed != 1 {
		t.Errorf("badges %v, changed %d", st.Badges, st.Changed)
	}
	if h.Stamp() == before {
		t.Error("the stamp didn't move")
	}
}

func TestRefusals(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "taker", fakeplugin.Options{Command: "status"})
	if _, err := h.Refresh("taker"); err == nil || !strings.Contains(err.Error(), "ws's own command") {
		t.Errorf("a plugin taking ws's command: %v", err)
	}
	if st := h.Status("taker"); st.State != plugin.Failed {
		t.Errorf("status = %+v", st)
	}

	add(t, root, "old", fakeplugin.Options{Protocol: 9})
	if _, err := h.Refresh("old"); err == nil || !strings.Contains(err.Error(), "version 9") {
		t.Errorf("a plugin speaking another protocol: %v", err)
	}

	if _, err := h.Refresh("nobody"); err == nil {
		t.Error("a plugin that isn't installed started")
	}
}

func TestCrashIsStartedAgain(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "fake", fakeplugin.Options{})
	if _, err := h.Run(context.Background(), "fake", []string{"crash"}, "", plugin.Terminal{}, func(string, string) {}); err == nil || !strings.Contains(err.Error(), "stopped while") {
		t.Errorf("a command the plugin died in: %v", err)
	}
	var o output
	if _, err := h.Run(context.Background(), "fake", []string{"echo", "back"}, "", plugin.Terminal{}, o.write); err != nil || o.stdout() != "back\n" {
		t.Errorf("after a crash: err %v, out %q", err, o.stdout())
	}
}

func TestClient(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "fake", fakeplugin.Options{})
	ln, err := h.Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go h.Serve(ln)

	c, err := plugin.Dial(root)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	ctx := context.Background()
	st, err := c.Refresh(ctx, "fake")
	if err != nil || st.State != plugin.Ready {
		t.Fatalf("refresh: %+v, %v", st, err)
	}
	var o output
	exit, err := c.Run(ctx, "fake", []string{"echo", "over", "the", "socket"}, "", plugin.Terminal{}, o.write)
	if err != nil || exit != 0 || o.stdout() != "over the socket\n" {
		t.Errorf("run: exit %d, err %v, out %q", exit, err, o.stdout())
	}
	exit, err = c.Run(ctx, "fake", []string{"fail"}, "", plugin.Terminal{}, o.write)
	if err != nil || exit != 3 {
		t.Errorf("fail: exit %d, err %v", exit, err)
	}
	if _, err := c.Run(ctx, "nobody", nil, "", plugin.Terminal{}, o.write); err == nil {
		t.Error("a command of a plugin that isn't installed ran")
	}
	all, err := c.Statuses(ctx)
	if err != nil || len(all) != 1 || all[0].Name != "fake" {
		t.Errorf("statuses: %+v, %v", all, err)
	}

	// Ctrl-C in the command reaches the plugin.
	cctx, cancel := context.WithCancel(ctx)
	o = output{}
	done := make(chan int, 1)
	go func() {
		exit, _ := c.Run(cctx, "fake", []string{"sleep"}, "", plugin.Terminal{}, o.write)
		done <- exit
	}()
	for deadline := time.Now().Add(5 * time.Second); o.stdout() == "" && time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
	}
	cancel()
	select {
	case exit := <-done:
		if exit != 130 {
			t.Errorf("cancelled: exit %d", exit)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the command didn't end when it was cancelled")
	}
}

func TestRemove(t *testing.T) {
	root, h := workspace(t)
	add(t, root, "fake", fakeplugin.Options{})
	if _, err := h.Refresh("fake"); err != nil {
		t.Fatal(err)
	}
	h.Stop("fake")
	if err := plugin.Remove(root, "fake", false); err != nil {
		t.Fatal(err)
	}
	if got := plugin.Installed(root); len(got) != 0 {
		t.Errorf("installed after removing = %v", got)
	}
	if _, err := os.Stat(filepath.Join(plugin.Dir(root), "fake", "data")); err != nil {
		t.Errorf("its data went with it: %v", err)
	}
	if err := plugin.Remove(root, "fake", false); err == nil {
		t.Error("removing one that isn't there succeeded")
	}
}
