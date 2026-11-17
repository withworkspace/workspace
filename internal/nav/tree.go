package nav

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// FileMax is the largest file navigation reads.
const FileMax = 1 << 20

// Tree is a repo's files at one commit.
type Tree interface {
	Rev() string
	// Blob is a file's git object, and whether there's such a file.
	Blob(path string) (string, bool)
	// Read is a file's bytes: no more than FileMax.
	Read(path string) ([]byte, error)
	// Dir is the files directly in a folder ("" for the top).
	Dir(dir string) []string
	// Under is the files anywhere under a folder that end in one of the
	// suffixes (any, with none).
	Under(dir string, suffixes ...string) []string
	// Grep is where a word is written, as a whole word, in files that end
	// in one of the suffixes.
	Grep(word string, suffixes ...string) ([]Hit, error)
}

// Hit is a line a word is on.
type Hit struct {
	Path string
	Line int
	Text string
}

// ErrTooLarge is Read's answer for a file larger than FileMax.
var ErrTooLarge = errors.New("too large to read")

// ErrBinary is Read's answer for a file that isn't text.
var ErrBinary = errors.New("not text")

// gitTree is a commit in a repo's git directory: a capsule's worktree or
// a bare clone, alike, since a review is of commits.
type gitTree struct {
	repo  *repo
	rev   string
	blobs map[string]string // path: blob
	sizes map[string]int64
	paths []string // sorted

	// What's worked out about the commit as a whole (its go.mod, its
	// types of a name, where a word is written): a commit doesn't change,
	// so it's kept as long as the tree is.
	mu    sync.Mutex
	memo  map[string]any
	greps map[string][]Hit
}

// treeMemoMax is how many things a tree keeps worked out.
const treeMemoMax = 5000

// remember is something worked out about the commit, worked out once.
func (t *gitTree) remember(key string, fn func() any) any {
	t.mu.Lock()
	if v, ok := t.memo[key]; ok {
		t.mu.Unlock()
		return v
	}
	t.mu.Unlock()
	v := fn()
	t.mu.Lock()
	if t.memo == nil || len(t.memo) > treeMemoMax {
		t.memo = map[string]any{}
	}
	t.memo[key] = v
	t.mu.Unlock()
	return v
}

func (t *gitTree) Rev() string { return t.rev }

func (t *gitTree) Blob(p string) (string, bool) {
	b, ok := t.blobs[p]
	return b, ok
}

func (t *gitTree) Read(p string) ([]byte, error) {
	b, ok := t.blobs[p]
	if !ok {
		return nil, fmt.Errorf("no %s at %s", p, short(t.rev))
	}
	if t.sizes[p] > FileMax {
		return nil, ErrTooLarge
	}
	data, err := t.repo.blob(b)
	if err != nil {
		return nil, err
	}
	if bytes.IndexByte(data, 0) >= 0 {
		return nil, ErrBinary
	}
	return data, nil
}

func (t *gitTree) Dir(dir string) []string {
	var out []string
	for _, p := range t.Under(dir) {
		if path.Dir(p) == dirOrDot(dir) {
			out = append(out, p)
		}
	}
	return out
}

func (t *gitTree) Under(dir string, suffixes ...string) []string {
	prefix := ""
	if dir != "" && dir != "." {
		prefix = strings.TrimSuffix(dir, "/") + "/"
	}
	i := sort.SearchStrings(t.paths, prefix)
	var out []string
	for ; i < len(t.paths) && strings.HasPrefix(t.paths[i], prefix); i++ {
		if hasSuffix(t.paths[i], suffixes) {
			out = append(out, t.paths[i])
		}
	}
	return out
}

func (t *gitTree) Grep(word string, suffixes ...string) ([]Hit, error) {
	key := word + "\x00" + strings.Join(suffixes, ",")
	t.mu.Lock()
	if hits, ok := t.greps[key]; ok {
		t.mu.Unlock()
		return hits, nil
	}
	t.mu.Unlock()
	hits, err := t.grep(word, suffixes)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	if t.greps == nil || len(t.greps) > treeMemoMax {
		t.greps = map[string][]Hit{}
	}
	t.greps[key] = hits
	t.mu.Unlock()
	return hits, nil
}

func (t *gitTree) grep(word string, suffixes []string) ([]Hit, error) {
	args := []string{"grep", "-n", "-I", "-w", "-F", "--no-color", "-e", word, t.rev, "--"}
	for _, s := range suffixes {
		args = append(args, "*"+s)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := git.Read(ctx, t.repo.dir, args...)
	if err != nil && git.ExitCode(err) != 1 { // 1: nothing found
		return nil, err
	}
	var hits []Hit
	prefix := t.rev + ":"
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimPrefix(line, prefix)
		p, rest, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		n, text, ok := strings.Cut(rest, ":")
		if !ok {
			continue
		}
		num, err := strconv.Atoi(n)
		if err != nil {
			continue
		}
		hits = append(hits, Hit{Path: p, Line: num, Text: text})
	}
	return hits, nil
}

func hasSuffix(p string, suffixes []string) bool {
	if len(suffixes) == 0 {
		return true
	}
	for _, s := range suffixes {
		if strings.HasSuffix(p, s) {
			return true
		}
	}
	return false
}

func dirOrDot(dir string) string {
	if dir == "" {
		return "."
	}
	return strings.TrimSuffix(dir, "/")
}

func short(rev string) string {
	if len(rev) > 7 {
		return rev[:7]
	}
	return rev
}

// loadTree lists a commit's files.
func loadTree(r *repo, rev string) (*gitTree, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := git.Read(ctx, r.dir, "ls-tree", "-r", "-z", "-l", "--full-tree", rev)
	if err != nil {
		return nil, fmt.Errorf("no commit %s here", short(rev))
	}
	t := &gitTree{repo: r, rev: rev, blobs: map[string]string{}, sizes: map[string]int64{}}
	for _, rec := range strings.Split(string(out), "\x00") {
		// <mode> SP <type> SP <object> SP+ <size> TAB <path>
		meta, p, ok := strings.Cut(rec, "\t")
		if !ok {
			continue
		}
		f := strings.Fields(meta)
		if len(f) < 4 || f[1] != "blob" {
			continue
		}
		size, _ := strconv.ParseInt(f[3], 10, 64)
		t.blobs[p], t.sizes[p] = f[2], size
		t.paths = append(t.paths, p)
	}
	sort.Strings(t.paths)
	return t, nil
}

// repo reads a git directory's objects through one cat-file --batch,
// started when first asked and stopped when it's been idle a while.
type repo struct {
	dir  string
	mu   sync.Mutex
	cmd  *exec.Cmd
	in   io.WriteCloser
	out  *bufio.Reader
	idle *time.Timer
}

const batchIdle = 2 * time.Minute

func (r *repo) blob(id string) ([]byte, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for attempt := 0; ; attempt++ {
		data, err := r.read(id)
		if err == nil || attempt > 0 || errors.Is(err, errMissing) {
			return data, err
		}
		r.stop() // a batch that's died: once more, with a new one
	}
}

var errMissing = errors.New("no such object")

func (r *repo) read(id string) ([]byte, error) {
	if r.cmd == nil {
		if err := r.start(); err != nil {
			return nil, err
		}
	}
	r.idle.Reset(batchIdle)
	if _, err := fmt.Fprintln(r.in, id); err != nil {
		return nil, err
	}
	header, err := r.out.ReadString('\n')
	if err != nil {
		return nil, err
	}
	f := strings.Fields(header)
	if len(f) == 2 && f[1] == "missing" {
		return nil, errMissing
	}
	if len(f) != 3 {
		return nil, fmt.Errorf("cat-file said %q", strings.TrimSpace(header))
	}
	size, err := strconv.Atoi(f[2])
	if err != nil {
		return nil, err
	}
	data := make([]byte, size+1) // and its newline
	if _, err := io.ReadFull(r.out, data); err != nil {
		return nil, err
	}
	return data[:size], nil
}

func (r *repo) start() error {
	cmd := git.Cmd{Dir: r.dir, Args: []string{"cat-file", "--batch"}, ReadOnly: true}.Command()
	in, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	out, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	r.cmd, r.in, r.out = cmd, in, bufio.NewReaderSize(out, 64<<10)
	if r.idle == nil {
		r.idle = time.AfterFunc(batchIdle, func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			r.stop()
		})
	}
	return nil
}

func (r *repo) stop() {
	if r.cmd == nil {
		return
	}
	r.in.Close()
	_ = r.cmd.Wait()
	r.cmd, r.in, r.out = nil, nil, nil
}
