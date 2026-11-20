package ops

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/artifact"
)

// A version's files, to work on: an agent revising an artifact that's
// more than its page is given the version's files in a folder
// (CheckoutArtifact), changes what it's changing there, and adds the
// folder as the next version (AddArtifact, or a conversation's proposal).
// The artifact's repo is ws's own: nothing's worked on in it.

// ArtifactCheckout is what checking a version out did: which version,
// the folder its files are in, and the files.
type ArtifactCheckout struct {
	Entry   artifact.Entry
	Version int
	Dir     string
	Files   []artifact.FileInfo
}

// CheckoutArtifact writes a version's files (a3.v2, or a3 for its latest)
// into dir, to be worked on. With no dir it's a new folder: in the
// system's temp folder, or, for a conversation's agent (conversation, in
// mission slug or the workspace's own), in the folder that's its own to
// write in. A conversation's agent names only a folder it may write in:
// ws runs outside its sandbox, and doesn't write for it where the
// sandbox wouldn't let it.
func (w *Workspace) CheckoutArtifact(ref, dir, slug, conversation string) (ArtifactCheckout, error) {
	id, v, err := artifactRef(ref)
	if err != nil {
		return ArtifactCheckout{}, err
	}
	var own []string
	if conversation != "" {
		own = w.ConversationFolders(slug, conversation)
		if len(own) == 0 {
			return ArtifactCheckout{}, invalid("%s has no folder of its own to check files out into", conversation)
		}
	}
	store, ctx := w.Artifacts(), context.Background()
	e, err := store.Get(ctx, id)
	if err != nil {
		return ArtifactCheckout{}, wrapArtifact(err)
	}
	if v == 0 {
		latest, _ := e.Latest()
		v = latest.N
	}
	switch {
	case dir != "":
		if dir, err = filepath.Abs(dir); err != nil {
			return ArtifactCheckout{}, err
		}
		if conversation != "" && !inAny(dir, append(own, resolved(os.TempDir()))) {
			return ArtifactCheckout{}, invalid("%s isn't somewhere you write: name a folder in your own (%s), or leave it off and ws makes one there", dir, own[0])
		}
	case conversation != "":
		dir = freeFolder(filepath.Join(own[0], id+"-v"+strconv.Itoa(v)))
	default:
		if dir, err = os.MkdirTemp("", "ws-"+id+"-v"+strconv.Itoa(v)+"-"); err != nil {
			return ArtifactCheckout{}, err
		}
	}
	e, n, files, err := store.Checkout(ctx, id, v, dir)
	if err != nil {
		return ArtifactCheckout{}, wrapArtifact(err)
	}
	return ArtifactCheckout{Entry: e, Version: n, Dir: dir, Files: files}, nil
}

// inAny reports whether path is one of dirs, or inside one.
func inAny(path string, dirs []string) bool {
	path = resolvedParent(path)
	for _, d := range dirs {
		if path == d || strings.HasPrefix(path, d+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

// resolvedParent is path as it resolves, where it, or the nearest folder
// above it that's there, is: a folder not yet made is where its parent
// says.
func resolvedParent(path string) string {
	rest := ""
	for p := path; ; {
		if r, err := filepath.EvalSymlinks(p); err == nil {
			return filepath.Join(r, rest)
		}
		parent := filepath.Dir(p)
		if parent == p {
			return path
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}

// freeFolder is dir, or dir-2, dir-3: one that isn't there, or is empty.
func freeFolder(dir string) string {
	for i := 1; i <= 100; i++ {
		d := dir
		if i > 1 {
			d = fmt.Sprintf("%s-%d", dir, i)
		}
		if entries, err := os.ReadDir(d); err != nil || len(entries) == 0 {
			return d
		}
	}
	return dir
}

// ReadArtifactFiles reads what ws artifact add is given as a version's
// files: a folder, whole (its page is its index.html), or one HTML file,
// which is the page alone.
func ReadArtifactFiles(path string) (artifact.Files, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("reading the artifact: %w", err)
	}
	if info.IsDir() {
		files, err := artifact.ReadFolder(path)
		if err != nil {
			return nil, fmt.Errorf("reading the artifact's folder: %w", err)
		}
		if _, ok := files.Index(); !ok {
			return nil, invalid("%s has no %s: a folder is added as a version's files, and that's its page, at the top", path, artifact.IndexFile)
		}
		return files, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading the artifact: %w", err)
	}
	return artifact.Page(data), nil
}
