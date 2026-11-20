package artifact

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
)

// An artifact's files are kept in a git repo of its own, in its folder
// (files.git, bare): each version a commit, tagged v1, v2, …, whose tree
// is the version's files and whose parent is the version it's based on.
// So a file that doesn't change between versions is kept once, two
// versions are diffed file by file, and one's checked out to be worked
// on, as any repo's is.
//
// The repo is ws's: nothing's worked on in it. An agent revising an
// artifact is given a version's files in a folder of its own (Checkout),
// and what it made of them is added as the next version, whole. ws makes
// the commits, unsigned and by no one's configuration but its own: what
// your git signs with, ignores or hooks into has no say here.

// RepoDir is the name of an artifact's repo, in its folder.
const RepoDir = "files.git"

// ErrNoFile is the answer for a file a version hasn't got.
var ErrNoFile = errors.New("no such file")

// Repo is an artifact's repo: where it is.
type Repo string

// tag is version n's tag.
func tag(n int) string { return "refs/tags/v" + strconv.Itoa(n) }

// cmd is git run on the repo, with none of the user's configuration.
func (r Repo) cmd(ctx context.Context, stdin io.Reader, args ...string) git.Cmd {
	return git.Cmd{Ctx: ctx, Args: args, Stdin: stdin, Env: []string{
		"GIT_DIR=" + string(r),
		"GIT_CONFIG_GLOBAL=" + os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_TERMINAL_PROMPT=0",
	}}
}

func (r Repo) out(ctx context.Context, stdin io.Reader, args ...string) ([]byte, error) {
	return r.cmd(ctx, stdin, args...).Output()
}

// Exists reports whether the repo's there.
func (r Repo) Exists() bool {
	info, err := os.Stat(filepath.Join(string(r), "HEAD"))
	return err == nil && !info.IsDir()
}

// Init makes the repo, where there's none.
func (r Repo) Init(ctx context.Context) error {
	if r.Exists() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(string(r)), 0o755); err != nil {
		return err
	}
	// Nothing packs or prunes it behind ws's back.
	_, err := git.Cmd{Ctx: ctx, Args: []string{"-c", "init.defaultBranch=main", "-c", "gc.auto=0", "init", "--bare", "--quiet", string(r)}, Env: []string{
		"GIT_CONFIG_GLOBAL=" + os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
	}}.Output()
	if err != nil {
		return fmt.Errorf("making %s: %w", string(r), err)
	}
	_, err = r.out(ctx, nil, "config", "gc.auto", "0")
	return err
}

// Has reports whether the repo has version n.
func (r Repo) Has(ctx context.Context, n int) bool {
	_, err := r.out(ctx, nil, "rev-parse", "--quiet", "--verify", tag(n)+"^{commit}")
	return err == nil
}

// Versions is the versions the repo has, by number.
func (r Repo) Versions(ctx context.Context) []int {
	out, err := r.out(ctx, nil, "for-each-ref", "--format=%(refname:strip=2)", "refs/tags/v*")
	if err != nil {
		return nil
	}
	var ns []int
	for _, l := range strings.Fields(string(out)) {
		if n, err := strconv.Atoi(strings.TrimPrefix(l, "v")); err == nil && n > 0 && l == "v"+strconv.Itoa(n) {
			ns = append(ns, n)
		}
	}
	sort.Ints(ns)
	return ns
}

// quotePath is a path as git fast-import reads one, quoted.
func quotePath(p string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`).Replace(p) + `"`
}

// Commit writes files as a commit named by ref (a version's tag, or a
// proposal's), whose parent is from ("" for none): what it says, and
// when. It's the commit. A ref that's there already is left as it is,
// and the answer is os.ErrExist.
func (r Repo) Commit(ctx context.Context, ref, from string, files Files, message string, at time.Time) (string, error) {
	if err := r.Init(ctx); err != nil {
		return "", err
	}
	if _, err := r.out(ctx, nil, "rev-parse", "--quiet", "--verify", ref); err == nil {
		return "", fmt.Errorf("%s: %w", ref, os.ErrExist)
	}
	var in bytes.Buffer
	fmt.Fprintf(&in, "commit %s\n", ref)
	fmt.Fprintf(&in, "committer ws <ws@localhost> %d +0000\n", at.Unix())
	fmt.Fprintf(&in, "data %d\n%s\n", len(message), message)
	if from != "" {
		fmt.Fprintf(&in, "from %s\n", from)
	}
	in.WriteString("deleteall\n")
	for _, f := range files {
		fmt.Fprintf(&in, "M 100644 inline %s\ndata %d\n", quotePath(f.Path), len(f.Data))
		in.Write(f.Data)
		in.WriteByte('\n')
	}
	in.WriteString("done\n")
	if _, err := r.out(ctx, &in, "fast-import", "--quiet", "--done", "--date-format=raw"); err != nil {
		return "", fmt.Errorf("committing %s: %w", ref, err)
	}
	out, err := r.out(ctx, nil, "rev-parse", "--verify", ref+"^{commit}")
	return strings.TrimSpace(string(out)), err
}

// CommitVersion writes files as version n, based on version on (0 for
// none, or one the repo hasn't): its commit.
func (r Repo) CommitVersion(ctx context.Context, n, on int, files Files, message string, at time.Time) (string, error) {
	from := ""
	if on > 0 {
		if out, err := r.out(ctx, nil, "rev-parse", "--quiet", "--verify", tag(on)+"^{commit}"); err == nil {
			from = strings.TrimSpace(string(out))
		}
	}
	return r.Commit(ctx, tag(n), from, files, message, at)
}

// Drop takes version n's tag away: a version whose record was never
// written.
func (r Repo) Drop(ctx context.Context, n int) {
	r.out(ctx, nil, "update-ref", "-d", tag(n))
}

// entry is one file of a tree as git lists it.
type entry struct {
	path, blob string
	size       int
}

// tree lists the files of what rev names.
func (r Repo) tree(ctx context.Context, rev string) ([]entry, error) {
	out, err := r.out(ctx, nil, "ls-tree", "-r", "-l", "-z", rev)
	if err != nil {
		return nil, err
	}
	var es []entry
	for _, rec := range strings.Split(string(out), "\x00") {
		if rec == "" {
			continue
		}
		// <mode> SP <type> SP <object> SP+ <size> TAB <path>
		meta, p, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 4 || f[1] != "blob" {
			continue
		}
		size, _ := strconv.Atoi(f[3])
		es = append(es, entry{path: p, blob: f[2], size: size})
	}
	return es, nil
}

// List is version n's files, by path.
func (r Repo) List(ctx context.Context, n int) ([]FileInfo, error) { return r.ListAt(ctx, tag(n)) }

// ListAt is the files of what rev names, by path.
func (r Repo) ListAt(ctx context.Context, rev string) ([]FileInfo, error) {
	es, err := r.tree(ctx, rev)
	if err != nil {
		return nil, err
	}
	out := make([]FileInfo, 0, len(es))
	for _, e := range es {
		out = append(out, FileInfo{Path: e.path, Size: e.size})
	}
	return out, nil
}

// Read is version n's file at path, and what git names it (its blob),
// which is the same for the same bytes.
func (r Repo) Read(ctx context.Context, n int, p string) ([]byte, string, error) {
	return r.ReadAt(ctx, tag(n), p)
}

// ReadAt is the file at path in what rev names, and its blob.
func (r Repo) ReadAt(ctx context.Context, rev, p string) ([]byte, string, error) {
	if _, err := CleanPath(p); err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrNoFile, err)
	}
	blobs, err := r.blobs(ctx, []string{rev + ":" + p})
	if err != nil {
		return nil, "", err
	}
	if blobs[0].missing {
		return nil, "", fmt.Errorf("%w: %s", ErrNoFile, p)
	}
	return blobs[0].data, blobs[0].name, nil
}

// All is version n's files, whole.
func (r Repo) All(ctx context.Context, n int) (Files, error) { return r.AllAt(ctx, tag(n)) }

// AllAt is the files of what rev names, whole.
func (r Repo) AllAt(ctx context.Context, rev string) (Files, error) {
	es, err := r.tree(ctx, rev)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(es))
	for i, e := range es {
		names[i] = e.blob
	}
	blobs, err := r.blobs(ctx, names)
	if err != nil {
		return nil, err
	}
	out := make(Files, 0, len(es))
	for i, e := range es {
		if blobs[i].missing {
			return nil, fmt.Errorf("%s's file %s is missing from %s", rev, e.path, string(r))
		}
		out = append(out, File{Path: e.path, Data: blobs[i].data})
	}
	return out, nil
}

type blob struct {
	name    string
	data    []byte
	missing bool
}

// blobs reads objects by what names them (a blob, or rev:path), in one
// run of git.
func (r Repo) blobs(ctx context.Context, names []string) ([]blob, error) {
	if len(names) == 0 {
		return nil, nil
	}
	for _, n := range names {
		if strings.ContainsAny(n, "\n\x00") {
			return nil, fmt.Errorf("%w: %q", ErrNoFile, n)
		}
	}
	out, err := r.out(ctx, strings.NewReader(strings.Join(names, "\n")+"\n"), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	rd := bufio.NewReader(bytes.NewReader(out))
	blobs := make([]blob, 0, len(names))
	for range names {
		head, err := rd.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("reading %s: git said less than was asked", string(r))
		}
		head = strings.TrimSuffix(head, "\n")
		if strings.HasSuffix(head, " missing") || strings.HasSuffix(head, " ambiguous") {
			blobs = append(blobs, blob{missing: true})
			continue
		}
		f := strings.Fields(head)
		if len(f) != 3 {
			return nil, fmt.Errorf("reading %s: git said %q", string(r), head)
		}
		size, err := strconv.Atoi(f[2])
		if err != nil {
			return nil, fmt.Errorf("reading %s: git said %q", string(r), head)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(rd, data); err != nil {
			return nil, err
		}
		rd.ReadByte() // the newline after it
		if f[1] != "blob" {
			blobs = append(blobs, blob{missing: true}) // a folder, not a file
			continue
		}
		blobs = append(blobs, blob{name: f[0], data: data})
	}
	return blobs, nil
}

// versionMessage is what a version's commit says: which it is, and its
// note.
func versionMessage(title string, v Version) string {
	s := fmt.Sprintf("v%d: %s", v.N, title)
	if body := Body(v); body != "" {
		s += "\n\n" + body
	}
	return s + "\n"
}

// convert puts an artifact's versions in a repo, for one from when each
// was a file in its folder (v1.html, v2.html, …): each becomes a commit,
// its file the page, and the files go. Its record's versions say what
// each now is (one file, and its size); the record's the caller's to
// write. It's made beside, and moved into place whole, so two doing it
// at once make one. It's whether there was anything to do.
func convert(ctx context.Context, dir string, a *Artifact) (bool, error) {
	repo := Repo(filepath.Join(dir, RepoDir))
	if repo.Exists() {
		return false, nil
	}
	versions := append([]Version(nil), a.Versions...)
	sort.Slice(versions, func(i, j int) bool { return versions[i].N < versions[j].N })
	tmp, err := os.MkdirTemp(dir, RepoDir+".*.tmp")
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(tmp)
	making := Repo(filepath.Join(tmp, RepoDir))
	if err := making.Init(ctx); err != nil {
		return false, err
	}
	var done []string
	sizes := map[int]int{}
	for _, v := range versions {
		p := filepath.Join(dir, VersionFile(v.N))
		data, err := os.ReadFile(p)
		if errors.Is(err, os.ErrNotExist) {
			continue // a version whose file had gone: it has none still
		}
		if err != nil {
			return false, err
		}
		if _, err := making.CommitVersion(ctx, v.N, v.BasedOn, Page(data), versionMessage(a.Title, v), v.At); err != nil {
			return false, err
		}
		done = append(done, p)
		sizes[v.N] = len(data)
	}
	if err := os.Rename(string(making), string(repo)); err != nil {
		if repo.Exists() {
			return false, nil // made meanwhile
		}
		return false, err
	}
	for _, p := range done {
		os.Remove(p)
	}
	for i := range a.Versions {
		if size, ok := sizes[a.Versions[i].N]; ok {
			a.Versions[i].Files, a.Versions[i].Size = 1, size
		}
	}
	return true, nil
}
