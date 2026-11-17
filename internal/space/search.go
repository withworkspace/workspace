package space

import (
	"context"
	"strings"

	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/words"
)

// Search: the documents of the workspace's spaces, ranked. Full-text
// search ranks by BM25, so the best come first, and the few asked for are
// the best few. What makes the ranking good: a row for each section, not
// each file; the document's title and the section's heading weighted
// above its words; words stemmed.

// How a document is ranked is words.Rank's: a document is a thing, and
// its sections its places. testdata/ranking.json holds it.

// Query is a search: whole words, as typed. (Not as they're typed: a word
// half typed past its stem, "decisio", finds nothing in a stemmed index,
// whose word is "decis". Searching as you type wants titles matched
// another way, as find's matcher does.)
type Query struct {
	Words string   // as typed
	Repos []string // the spaces to look in, by repo; none for every one
	Limit int      // documents at most (20 if none)
}

// Hit is a document found: the section of it that's the best match.
type Hit struct {
	Repo, Path, Title string
	Anchor, Heading   string // the section ("" for the top)
	Snippet           []Part // a few of its words, those found marked
	Score             float64
	// Every word was found in it. Where too few have every word, those
	// with some of them fill the page.
	Every bool
}

// Part is a run of a snippet: marked when it's a word that was searched for.
type Part = words.Part

// Search is the documents that best match q, best first, and how many more
// matched than are given. Every document with any of the words is ranked
// together: BM25 adds up each word's worth (a rare word counts for more,
// a word in every document, "the", for nothing), so one with more of what
// was asked rises without a rule that every word must be there, which a
// question asked in a sentence never meets.
func Search(ctx context.Context, db *store.DB, q Query) (hits []Hit, more int, err error) {
	if q.Limit <= 0 {
		q.Limit = 20
	}
	m := words.Match(q.Words)
	if m.All == "" {
		return nil, 0, nil
	}
	err = db.Read(ctx, func(tx *store.Tx) error {
		every, err := matching(tx, m.Every(), q.Repos)
		if err != nil {
			return err
		}
		found, err := search(tx, m, q.Repos, every)
		if err != nil {
			return err
		}
		for _, h := range found {
			h.Every = every[h.Repo+":"+h.Path]
			if len(hits) < q.Limit {
				hits = append(hits, h)
			} else {
				more++
			}
		}
		return nil
	})
	return hits, more, err
}

// matching is the documents a full-text query matches, by repo:path.
func matching(tx *store.Tx, match string, repos []string) (map[string]bool, error) {
	q := `SELECT DISTINCT s.repo, s.path FROM doc_words JOIN doc_sections s ON s.id = doc_words.rowid WHERE doc_words MATCH ?`
	args := []any{match}
	q, args = inRepos(q, args, repos)
	rows, err := tx.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var repo, p string
		if err := rows.Scan(&repo, &p); err != nil {
			return nil, err
		}
		out[repo+":"+p] = true
	}
	return out, rows.Err()
}

func inRepos(q string, args []any, repos []string) (string, []any) {
	if len(repos) == 0 {
		return q, args
	}
	q += ` AND s.repo IN (` + strings.TrimSuffix(strings.Repeat("?, ", len(repos)), ", ") + `)`
	for _, r := range repos {
		args = append(args, r)
	}
	return q, args
}

// search is each document with any of the words, led to its best
// section, best first.
func search(tx *store.Tx, m words.Query, repos []string, every map[string]bool) ([]Hit, error) {
	q := `SELECT s.repo, s.path, s.n, d.title, s.anchor, s.heading,
			highlight(doc_words, 0, char(2), char(3)), highlight(doc_words, 1, char(2), char(3)),
			snippet(doc_words, 2, char(2), char(3), '…', 16), substr(doc_words.body, 1, 160),
			bm25(doc_words, 0.0, 1.0, 1.0)
		FROM doc_words JOIN doc_sections s ON s.id = doc_words.rowid JOIN docs d ON d.repo = s.repo AND d.path = s.path
		WHERE doc_words MATCH ?`
	args := []any{m.Looked()}
	q, args = inRepos(q, args, repos)
	q += ` LIMIT 5000`
	rows, err := tx.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var found []words.Row
	for rows.Next() {
		var (
			h Hit
			r words.Row
		)
		if err := rows.Scan(&h.Repo, &h.Path, &r.N, &h.Title, &h.Anchor, &h.Heading, &r.TitleMarked, &r.HeadingMarked, &r.Snippet, &r.Lead, &r.BM25); err != nil {
			return nil, err
		}
		r.Key, r.Title, r.Heading, r.Of = h.Repo+":"+h.Path, h.Title, h.Heading, h
		r.Every = every[r.Key]
		found = append(found, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	ranked := words.Rank(m, found)
	out := make([]Hit, 0, len(ranked))
	for _, r := range ranked {
		h := r.Of.(Hit)
		h.Score, h.Snippet = r.Score, r.Snippet
		if r.Top {
			h.Anchor, h.Heading = "", "" // it's the document that's asked for
		}
		out = append(out, h)
	}
	return out, nil
}

// Text is a snippet as plain text, its marks as the given strings either
// side.
func Text(ps []Part, open, close string) string { return words.Text(ps, open, close) }
