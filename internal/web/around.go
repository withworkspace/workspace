package web

import (
	"net/http"
	"strconv"

	"github.com/withworkspace/workspace/internal/ops"
)

// GET /api/around?ref=…&scope=…&limit=…&fresh=1 is what's one link from a
// thing in the graph, either way (ops.Around): what the find window shows
// beside where you are before anything's typed. ref is the thing as ws
// show names it (a mission's slug, w4, p2, demo#12, RAM-123,
// demo/main.go), scope the mission it's read in, if it's one's own (t3,
// d2). Without fresh, as the records stand, at once; with it, after
// what's edited outside ws is read again.
//
//	{"results": [{…SaidFound, "how": "from", "out": true}]}

// NearFound is a thing around another, as the find window draws one: how
// it's linked, and which way (out: the thing asked about points at it).
type NearFound struct {
	SaidFound
	How string `json:"how"`
	Out bool   `json:"out,omitempty"`
}

func (s *Server) around(w http.ResponseWriter, r *http.Request) {
	if s.App == nil {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	name := q.Get("ref")
	if name == "" {
		jsonError(w, http.StatusBadRequest, "ref is needed: the thing to say what's around")
		return
	}
	keys, _, _ := ops.KeysOf(q.Get("scope"), name)
	limit, _ := strconv.Atoi(q.Get("limit"))
	near, err := s.App.Around(r.Context(), keys, limit, q.Get("fresh") == "1")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := struct {
		Results []NearFound `json:"results"`
	}{Results: []NearFound{}}
	for _, n := range near {
		out.Results = append(out.Results, NearFound{SaidFound: saidFound(n.Found), How: n.How, Out: n.Out})
	}
	writeJSON(w, http.StatusOK, out)
}
