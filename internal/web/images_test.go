package web

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
)

// A picture pasted is kept in the workspace's .ws/images, named by its
// path (for the agent) and drawn from its URL (for the web UI).
func TestPastedImages(t *testing.T) {
	root := t.TempDir()
	s := startTest(t, root, &fakeWorkspace{}, nil)
	var pic bytes.Buffer
	if err := png.Encode(&pic, image.NewRGBA(image.Rect(0, 0, 3, 3))); err != nil {
		t.Fatal(err)
	}
	paste := func(data []byte) *http.Response {
		return do(t, s, "POST", "/api/images", true, func(r *http.Request) {
			r.Header.Set("Origin", s.Info().Origin())
			r.Header.Set("Content-Type", "application/json")
			sent, _ := json.Marshal(Paste{Data: data})
			r.Body = io.NopCloser(bytes.NewReader(sent))
			r.ContentLength = int64(len(sent))
		})
	}

	res := paste(pic.Bytes())
	if res.StatusCode != http.StatusOK {
		t.Fatalf("pasting: %d %s", res.StatusCode, body(res))
	}
	var got Pasted
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Path != filepath.Join(root, ".ws", "images", got.Name) || got.URL != "/api/images/"+got.Name {
		t.Fatalf("pasted as %+v", got)
	}

	res = do(t, s, "GET", got.URL, true, nil)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "image/png" || body(res) != pic.String() {
		t.Fatalf("drawing it: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}
	if res := do(t, s, "GET", got.URL, false, nil); res.StatusCode != http.StatusUnauthorized {
		t.Errorf("signed out: %d", res.StatusCode)
	}
	for _, name := range []string{"..%2Fws.db", strings.Repeat("0", 32) + ".png"} {
		if res := do(t, s, "GET", "/api/images/"+name, true, nil); res.StatusCode != http.StatusNotFound {
			t.Errorf("GET %s: %d", name, res.StatusCode)
		}
	}
	if res := paste([]byte("not a picture")); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("words pasted as a picture: %d %s", res.StatusCode, body(res))
	}
}
