package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/ops"
)

// A mission's image or video is shown from its raw route, as it is: only
// the mission's own artifacts, only images and videos, a video in ranges
// to seek in, and an SVG with nothing it can run.
func TestArtifactRaw(t *testing.T) {
	s := startTest(t, t.TempDir(), &fakeWorkspace{}, nil)
	if _, err := (mission.Store{Root: s.Root}).Create("M"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	png := filepath.Join(dir, "proof.png")
	webm := filepath.Join(dir, "run.webm")
	os.WriteFile(png, []byte("\x89PNG\r\n\x1a\nimage bytes"), 0o644)
	os.WriteFile(webm, []byte("0123456789video"), 0o644)
	fake(s).artifact = func(slug, ref string) (*ops.DocText, error) {
		if slug != "m" {
			return nil, ErrNotFound
		}
		switch ref {
		case "mission:proof.png":
			return &ops.DocText{Ref: ref, Kind: "proof", Path: png, Media: "image/png"}, nil
		case "mission:run.webm":
			return &ops.DocText{Ref: ref, Kind: "proof", Path: webm, Media: "video/webm"}, nil
		case "mission:notes.md":
			return &ops.DocText{Ref: ref, Kind: "note", Text: "# Notes", Markdown: true}, nil
		}
		return nil, ErrNotFound
	}
	raw := func(ref string, set func(*http.Request)) *http.Response {
		return do(t, s, "GET", "/api/missions/m/artifact/raw?ref="+url.QueryEscape(ref), true, set)
	}

	// The artifact says it's an image, and where to show it from.
	var a ArtifactContent
	json.NewDecoder(do(t, s, "GET", "/api/missions/m/artifact?ref="+url.QueryEscape("mission:proof.png"), true, nil).Body).Decode(&a)
	if a.Media != "image/png" || a.Raw != "/api/missions/m/artifact/raw?ref=mission%3Aproof.png" || a.Annotatable {
		t.Errorf("artifact = %+v", a)
	}

	res := raw("mission:proof.png", nil)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/png" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "sandbox") || !strings.HasPrefix(string(body), "\x89PNG") {
		t.Errorf("image: %d %v %q", res.StatusCode, res.Header, body)
	}
	res = raw("mission:run.webm", func(r *http.Request) { r.Header.Set("Range", "bytes=10-") })
	body, _ = io.ReadAll(res.Body)
	if res.StatusCode != http.StatusPartialContent || res.Header.Get("Content-Type") != "video/webm" || string(body) != "video" {
		t.Errorf("a video's range: %d %q", res.StatusCode, body)
	}
	if res := raw("mission:notes.md", nil); res.StatusCode != http.StatusBadRequest {
		t.Errorf("text isn't served raw: %d", res.StatusCode)
	}
	if res := raw("mission:../../ws.toml", nil); res.StatusCode != http.StatusNotFound {
		t.Errorf("not the mission's: %d", res.StatusCode)
	}
	if res := do(t, s, "GET", "/api/missions/m/artifact/raw?ref="+url.QueryEscape("mission:proof.png"), false, nil); res.StatusCode == 200 {
		t.Error("served without signing in")
	}
}
