package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/withworkspace/workspace/internal/ops"
)

// Code search: what the files say, as ws rg finds it (ops.CodeSearch).
//
//	GET /api/search?p=…[&p=…][&repo=…[&worktree=…]][&regex=1][&case=smart|sensitive|ignore][&max=…][&bytes=…]
//
// Each p is a pattern (any matches): literal text unless regex is set.
// With no repo it searches the workspace: every .ground, and of capsules
// only what they've changed, a capsule's copy in place of .ground's; repo
// narrows that to one repo (Find's tag); repo and worktree search all of
// that worktree (the explorer's). max and bytes lower the caps.
//
// It answers a line of JSON for each hit as ripgrep finds it (an
// ops.CodeHit: repo, worktree, path, line, column, text, ranges), then
// {"done":true,"matches":…,"bytes":…} with "capped":"matches"|"bytes" when
// a cap stopped it, and "problem" when ripgrep failed after it began. A
// request the page drops (the query changed) stops ripgrep. One that's
// wrong before anything's found is an error, as JSON, with its status.

// CodeSearching is code search, for the Find window and the explorer.
type CodeSearching interface {
	CodeSearch(ctx context.Context, req ops.CodeSearchRequest, emit func(ops.CodeHit) error) (ops.CodeSearchResult, error)
}

var _ CodeSearching = (*ops.Workspace)(nil)

func (s *Server) codeSearch(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	req := ops.CodeSearchRequest{
		Patterns: q["p"],
		Repo:     q.Get("repo"),
		Worktree: q.Get("worktree"),
		Regex:    q.Get("regex") == "1" || q.Get("regex") == "true",
		Case:     q.Get("case"),
	}
	for key, to := range map[string]*int{"max": &req.MaxMatches, "bytes": &req.MaxBytes} {
		if v := q.Get(key); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n <= 0 {
				jsonError(w, http.StatusBadRequest, key+" is a number above 0")
				return
			}
			*to = n
		}
	}

	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	started := false
	line := func(v any) error {
		if !started {
			started = true
			w.Header().Set("Content-Type", "application/x-ndjson")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusOK)
		}
		if err := enc.Encode(v); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}

	res, err := s.App.CodeSearch(r.Context(), req, func(h ops.CodeHit) error { return line(h) })
	switch {
	case r.Context().Err() != nil:
		return // dropped: nobody's reading
	case err != nil && !started:
		if exit, ok := errors.AsType[*ops.RgExit](err); ok {
			jsonError(w, http.StatusUnprocessableEntity, rgProblem(exit))
			return
		}
		writeError(w, err, http.StatusInternalServerError)
		return
	}
	end := map[string]any{"done": true, "matches": res.Matches, "bytes": res.Bytes}
	if res.Capped != "" {
		end["capped"] = res.Capped
	}
	if err != nil {
		if exit, ok := errors.AsType[*ops.RgExit](err); ok {
			end["problem"] = rgProblem(exit)
		} else {
			end["problem"] = err.Error()
		}
	}
	_ = line(end) // a client that's gone isn't told
}

// rgProblem is what ripgrep said of its error, or that it failed.
func rgProblem(e *ops.RgExit) string {
	if msg := strings.TrimSpace(e.Stderr); msg != "" {
		return msg
	}
	return e.Error()
}
