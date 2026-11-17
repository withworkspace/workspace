// Package space is what a workspace's spaces of documents hold (a repo of
// documents, or a folder of one: workspace.Space), read from each one's
// default branch into the workspace's records: each document's title, its
// sections to search, and what it cites. It's a cache, not a record: what
// it says of a document is of that document's content (git's id for its
// blob), so it can't be stale, and it can be thrown away. When a ground's
// head moves, only the documents a commit changed are read again.
package space

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/git"
	"github.com/withworkspace/workspace/internal/ref"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/workspace"
)

func init() {
	store.Define("space", store.SQL(`
		-- Where each space's ground was when it was last read.
		CREATE TABLE space_heads (
			repo TEXT PRIMARY KEY, folder TEXT NOT NULL, head TEXT NOT NULL, read TEXT NOT NULL
		) WITHOUT ROWID;
		-- A document: its content (blob), and what it's called.
		CREATE TABLE docs (
			repo TEXT NOT NULL, path TEXT NOT NULL, blob TEXT NOT NULL,
			title TEXT NOT NULL, size INTEGER NOT NULL,
			PRIMARY KEY (repo, path)
		) WITHOUT ROWID;
		-- Its sections: where a search leads. Each one's words are the row
		-- of doc_words with its id.
		CREATE TABLE doc_sections (
			id INTEGER PRIMARY KEY, repo TEXT NOT NULL, path TEXT NOT NULL,
			n INTEGER NOT NULL, anchor TEXT NOT NULL, heading TEXT NOT NULL, level INTEGER NOT NULL
		);
		CREATE INDEX doc_sections_doc ON doc_sections (repo, path);
		CREATE VIRTUAL TABLE doc_words USING fts5 (
			title, heading, body, tokenize = 'porter unicode61', prefix = '2 3'
		);
		-- What it cites: a document, by a link or a ref.
		CREATE TABLE doc_links (
			repo TEXT NOT NULL, path TEXT NOT NULL,
			to_repo TEXT NOT NULL, to_path TEXT NOT NULL, line INTEGER NOT NULL, how TEXT NOT NULL
		);
		CREATE INDEX doc_links_to ON doc_links (to_repo, to_path);
		CREATE INDEX doc_links_from ON doc_links (repo, path);
	`), store.SQL(`
		-- Who changed each document last, and when (the commit that made
		-- it what it is). It's a cache: what's in it is let go, and read
		-- again with this.
		ALTER TABLE docs ADD COLUMN changed TEXT NOT NULL DEFAULT '';
		ALTER TABLE docs ADD COLUMN changed_by TEXT NOT NULL DEFAULT '';
		DELETE FROM doc_words; DELETE FROM doc_sections; DELETE FROM doc_links; DELETE FROM docs; DELETE FROM space_heads;
	`), store.SQL(`
		-- A document with a title and nothing else has a section, so it's
		-- found by its title: read again.
		DELETE FROM doc_words; DELETE FROM doc_sections; DELETE FROM doc_links; DELETE FROM docs; DELETE FROM space_heads;
	`))
}

// Stats is what a Refresh read.
type Stats struct {
	Spaces  int // spaces looked at
	Moved   int // of them, those whose ground had moved since they were read
	Read    int // documents read, new or changed
	Removed int // documents gone from a space
	Took    time.Duration
}

// Refresh reads into the records each space whose ground has moved since
// it was last read, and lets go of what's no longer a space's. known is
// what finds what documents cite by name, asked for only when there's
// something to read (nil, or giving nil: only their links are read).
func Refresh(ctx context.Context, db *store.DB, ws *workspace.Workspace, known func() *ref.Known) (Stats, error) {
	start := time.Now()
	var st Stats
	spaces := ws.Spaces()
	heads := map[string][2]string{} // repo: folder, head
	if err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT repo, folder, head FROM space_heads`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var repo, folder, head string
			if err := rows.Scan(&repo, &folder, &head); err != nil {
				return err
			}
			heads[repo] = [2]string{folder, head}
		}
		return rows.Err()
	}); err != nil {
		return st, err
	}
	isSpace := map[string]bool{}
	for _, s := range spaces {
		isSpace[s.Repo] = true
	}
	for repo := range heads {
		if !isSpace[repo] {
			if err := db.Write(ctx, func(tx *store.Tx) error { return forgetRepo(tx, repo) }); err != nil {
				return st, err
			}
		}
	}
	var errs []error
	for _, s := range spaces {
		st.Spaces++
		ground := ws.MainWorktree(s.Repo)
		if _, err := os.Stat(ground); err != nil {
			continue // not cloned here
		}
		head, err := git.ReadLine(ctx, ground, "rev-parse", "HEAD")
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Repo, err))
			continue
		}
		if was := heads[s.Repo]; was == [2]string{s.Folder, head} {
			continue
		}
		st.Moved++
		var k *ref.Known
		if known != nil {
			k = known()
		}
		read, removed, err := refreshSpace(ctx, db, ws, s, head, k)
		st.Read += read
		st.Removed += removed
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", s.Repo, err))
		}
	}
	st.Took = time.Since(start)
	return st, errors.Join(errs...)
}

// entry is a document on a ground: its path, and its content's id.
type entry struct{ path, blob string }

func refreshSpace(ctx context.Context, db *store.DB, ws *workspace.Workspace, s workspace.Space, head string, known *ref.Known) (nread, nremoved int, err error) {
	ground := ws.MainWorktree(s.Repo)
	args := []string{"ls-tree", "-r", "-z", "--full-tree", head}
	if s.Folder != "" {
		args = append(args, "--", s.Folder)
	}
	out, err := git.Cmd{Ctx: ctx, Dir: ground, Args: args, ReadOnly: true}.Output()
	if err != nil {
		return 0, 0, err
	}
	var now []entry
	for _, rec := range strings.Split(string(out), "\x00") {
		meta, p, ok := strings.Cut(rec, "\t")
		f := strings.Fields(meta)
		if !ok || len(f) != 3 || f[1] != "blob" || !s.Has(p) {
			continue
		}
		now = append(now, entry{path: p, blob: f[2]})
	}
	had := map[string]string{}
	if err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT path, blob FROM docs WHERE repo = ?`, s.Repo)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var p, b string
			if err := rows.Scan(&p, &b); err != nil {
				return err
			}
			had[p] = b
		}
		return rows.Err()
	}); err != nil {
		return 0, 0, err
	}

	var changed []entry
	keep := map[string]bool{}
	for _, e := range now {
		keep[e.path] = true
		if had[e.path] != e.blob {
			changed = append(changed, e)
		}
	}
	var gone []string
	for p := range had {
		if !keep[p] {
			gone = append(gone, p)
		}
	}
	// What's read is read before the records are written: nothing slow
	// happens in a transaction.
	texts, err := blobs(ctx, ground, changed)
	if err != nil {
		return 0, 0, err
	}
	reads := make([]read, len(changed))
	for i, e := range changed {
		reads[i] = parse(s.Repo, e.path, texts[i], known)
	}
	last, err := lastChanged(ctx, ground, head, s.Folder, changed)
	if err != nil {
		return 0, 0, err
	}
	err = db.Write(ctx, func(tx *store.Tx) error {
		for _, p := range gone {
			if err := forget(tx, s.Repo, p); err != nil {
				return err
			}
		}
		for i, e := range changed {
			if err := forget(tx, s.Repo, e.path); err != nil {
				return err
			}
			if err := keepDoc(tx, s.Repo, e, len(texts[i]), reads[i], last[e.path]); err != nil {
				return err
			}
		}
		_, err := tx.Exec(`INSERT INTO space_heads (repo, folder, head, read) VALUES (?, ?, ?, ?)
			ON CONFLICT (repo) DO UPDATE SET folder = excluded.folder, head = excluded.head, read = excluded.read`,
			s.Repo, s.Folder, head, time.Now().UTC().Format(time.RFC3339))
		if err == nil && (len(gone) > 0 || len(changed) > 0) {
			tx.Changed("space:" + s.Repo)
		}
		return err
	})
	if err != nil {
		return 0, 0, err
	}
	return len(changed), len(gone), nil
}

// keepDoc writes what a document is: its row, its sections and their
// words, and what it cites.
func keepDoc(tx *store.Tx, repo string, e entry, size int, r read, c change) error {
	title := r.title
	if title == "" {
		title = e.path
	}
	if _, err := tx.Exec(`INSERT INTO docs (repo, path, blob, title, size, changed, changed_by) VALUES (?, ?, ?, ?, ?, ?, ?)`, repo, e.path, e.blob, title, size, c.at, c.by); err != nil {
		return err
	}
	for n, sec := range r.sections {
		res, err := tx.Exec(`INSERT INTO doc_sections (repo, path, n, anchor, heading, level) VALUES (?, ?, ?, ?, ?, ?)`, repo, e.path, n, sec.Anchor, sec.Heading, sec.Level)
		if err != nil {
			return err
		}
		id, err := res.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(`INSERT INTO doc_words (rowid, title, heading, body) VALUES (?, ?, ?, ?)`, id, title, sec.Heading, sec.Body); err != nil {
			return err
		}
	}
	for _, l := range r.links {
		if _, err := tx.Exec(`INSERT INTO doc_links (repo, path, to_repo, to_path, line, how) VALUES (?, ?, ?, ?, ?, ?)`, repo, e.path, l.repo, l.path, l.line, l.how); err != nil {
			return err
		}
	}
	return nil
}

// forget removes what's kept of a document: its words by their sections'
// ids (a full-text table is found by its rowid, not by what's in it).
func forget(tx *store.Tx, repo, p string) error {
	for _, q := range []string{
		`DELETE FROM doc_words WHERE rowid IN (SELECT id FROM doc_sections WHERE repo = ? AND path = ?)`,
		`DELETE FROM doc_sections WHERE repo = ? AND path = ?`,
		`DELETE FROM doc_links WHERE repo = ? AND path = ?`,
		`DELETE FROM docs WHERE repo = ? AND path = ?`,
	} {
		if _, err := tx.Exec(q, repo, p); err != nil {
			return err
		}
	}
	return nil
}

// forgetRepo removes what's kept of a repo that's no longer a space.
func forgetRepo(tx *store.Tx, repo string) error {
	for _, q := range []string{
		`DELETE FROM doc_words WHERE rowid IN (SELECT id FROM doc_sections WHERE repo = ?)`,
		`DELETE FROM doc_sections WHERE repo = ?`,
		`DELETE FROM doc_links WHERE repo = ?`,
		`DELETE FROM docs WHERE repo = ?`,
		`DELETE FROM space_heads WHERE repo = ?`,
	} {
		if _, err := tx.Exec(q, repo); err != nil {
			return err
		}
	}
	tx.Changed("space:" + repo)
	return nil
}

// blobs reads documents' contents from a repo through one cat-file
// --batch, in the order asked.
func blobs(ctx context.Context, dir string, es []entry) ([][]byte, error) {
	if len(es) == 0 {
		return nil, nil
	}
	var in strings.Builder
	for _, e := range es {
		in.WriteString(e.blob + "\n")
	}
	cmd := git.Cmd{Ctx: ctx, Dir: dir, Args: []string{"cat-file", "--batch"}, ReadOnly: true, Stdin: strings.NewReader(in.String())}.Command()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer cmd.Wait()
	r := bufio.NewReaderSize(stdout, 64<<10)
	out := make([][]byte, len(es))
	for i := range es {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, err
		}
		f := strings.Fields(header)
		if len(f) != 3 {
			return nil, fmt.Errorf("cat-file said %q of %s", strings.TrimSpace(header), es[i].path)
		}
		size, err := strconv.Atoi(f[2])
		if err != nil {
			return nil, err
		}
		data := make([]byte, size+1) // and its newline
		if _, err := io.ReadFull(r, data); err != nil {
			return nil, err
		}
		out[i] = data[:size]
	}
	return out, nil
}

// Counts is how many documents each space has, by repo.
func Counts(ctx context.Context, db *store.DB) (map[string]int, error) {
	out := map[string]int{}
	err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT repo, count(*) FROM docs GROUP BY repo`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var repo string
			var n int
			if err := rows.Scan(&repo, &n); err != nil {
				return err
			}
			out[repo] = n
		}
		return rows.Err()
	})
	return out, err
}

// change is who last changed a document, and when (RFC 3339).
type change struct{ at, by string }

// lastChanged is who changed each of es last, and when, as head's history
// says: one git log over the space, read until every one's found.
func lastChanged(ctx context.Context, dir, head, folder string, es []entry) (map[string]change, error) {
	out := map[string]change{}
	if len(es) == 0 {
		return out, nil
	}
	want := map[string]bool{}
	for _, e := range es {
		want[e.path] = true
	}
	args := []string{"log", "--format=%x01%aI%x09%an", "--name-only", "--no-renames", head}
	if folder != "" {
		args = append(args, "--", folder)
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel() // what's left of it, once every one's found
	cmd := git.Cmd{Ctx: ctx, Dir: dir, Args: args, ReadOnly: true}.Command()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer cmd.Wait()
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	var c change
	for sc.Scan() && len(want) > 0 {
		line := sc.Text()
		if rest, ok := strings.CutPrefix(line, "\x01"); ok {
			c.at, c.by, _ = strings.Cut(rest, "\t")
			continue
		}
		if want[line] {
			out[line] = c
			delete(want, line)
		}
	}
	return out, nil
}

// Doc is a document a space has: its path, its title, and who changed it
// last and when (RFC 3339).
type Doc struct {
	Repo, Path, Title  string
	Size               int
	Changed, ChangedBy string
}

// Docs is a space's documents, by their paths.
func Docs(ctx context.Context, db *store.DB, repo string) ([]Doc, error) {
	var out []Doc
	err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT path, title, size, changed, changed_by FROM docs WHERE repo = ? ORDER BY path`, repo)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			d := Doc{Repo: repo}
			if err := rows.Scan(&d.Path, &d.Title, &d.Size, &d.Changed, &d.ChangedBy); err != nil {
				return err
			}
			out = append(out, d)
		}
		return rows.Err()
	})
	return out, err
}

// Citation is a document citing another: where, and how.
type Citation struct {
	Repo, Path, Title string
	Line              int
	How               string // link, or ref
}

// CitedBy is the documents in any space that cite repo:path.
func CitedBy(ctx context.Context, db *store.DB, repo, p string) ([]Citation, error) {
	return citations(ctx, db, `SELECT l.repo, l.path, d.title, l.line, l.how FROM doc_links l JOIN docs d ON d.repo = l.repo AND d.path = l.path
		WHERE l.to_repo = ? AND l.to_path = ? ORDER BY d.title, l.path`, repo, p)
}

// Cites is the documents repo:path cites, where they're in a space (a
// title for each; a path for one that isn't read).
func Cites(ctx context.Context, db *store.DB, repo, p string) ([]Citation, error) {
	return citations(ctx, db, `SELECT l.to_repo, l.to_path, coalesce(d.title, ''), l.line, l.how FROM doc_links l LEFT JOIN docs d ON d.repo = l.to_repo AND d.path = l.to_path
		WHERE l.repo = ? AND l.path = ? ORDER BY l.line`, repo, p)
}

func citations(ctx context.Context, db *store.DB, q string, args ...any) ([]Citation, error) {
	var out []Citation
	err := db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(q, args...)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var c Citation
			if err := rows.Scan(&c.Repo, &c.Path, &c.Title, &c.Line, &c.How); err != nil {
				return err
			}
			out = append(out, c)
		}
		return rows.Err()
	})
	return out, err
}
