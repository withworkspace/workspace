// Package releasetest is a place ws's releases are fetched from, for
// tests: a server that answers as a repo's releases on GitHub do.
package releasetest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Platforms are those a release is built for.
var Platforms = []string{"darwin_arm64", "linux_amd64", "linux_arm64"}

// Releases is a repo's releases: Latest is the tag its "latest" page
// sends you to ("" for a repo with none), and Files what each release's
// downloads are, by tag and then name.
type Releases struct {
	*httptest.Server
	Latest string
	Files  map[string]map[string][]byte
}

// New serves a repo with no release yet. It's closed when the test ends.
func New(t *testing.T) *Releases {
	t.Helper()
	r := &Releases{Files: map[string]map[string][]byte{}}
	r.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/releases/latest" {
			to := "/releases"
			if r.Latest != "" {
				to = "/releases/tag/" + r.Latest
			}
			http.Redirect(w, req, to, http.StatusFound)
			return
		}
		rest, ok := strings.CutPrefix(req.URL.Path, "/releases/download/")
		tag, name, _ := strings.Cut(rest, "/")
		body, found := r.Files[tag][name]
		if !ok || !found {
			http.NotFound(w, req)
			return
		}
		w.Write(body)
	}))
	t.Cleanup(r.Close)
	return r
}

// Release publishes a release whose ws says it's version, for every
// platform, with its checksums, and makes it the latest unless it's a
// prerelease.
func (r *Releases) Release(version string) {
	r.Publish(version, Script(version))
	if !strings.Contains(version, "-") {
		r.Latest = version
	}
}

// Script is a ws that says its version and does nothing else.
func Script(version string) []byte {
	return fmt.Appendf(nil, "#!/bin/sh\necho workspace version %s\n", version)
}

// Publish publishes a release with the given file as its ws, for every
// platform, with its checksums.
func (r *Releases) Publish(tag string, ws []byte) {
	files := map[string][]byte{}
	var sums strings.Builder
	for _, p := range Platforms {
		name := fmt.Sprintf("workspace_%s_%s.tar.gz", tag, p)
		files[name] = Archive(ws)
		sum := sha256.Sum256(files[name])
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	files["checksums.txt"] = []byte(sums.String())
	r.Files[tag] = files
}

// Archive is a release's download: a gzipped tar with ws in it, as
// workspace.
func Archive(ws []byte) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	// To memory: nothing to fail.
	_ = tw.WriteHeader(&tar.Header{Name: "workspace", Mode: 0o755, Size: int64(len(ws)), Typeflag: tar.TypeReg})
	_, _ = tw.Write(ws)
	tw.Close()
	gz.Close()
	return buf.Bytes()
}
