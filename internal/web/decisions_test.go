package web

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/withworkspace/workspace/internal/mission"
)

// decisionWrite posts v to path as you, signed in.
func decisionWrite(t *testing.T, s *Server, path string, v any) *http.Response {
	t.Helper()
	data, _ := json.Marshal(v)
	return do(t, s, "POST", path, true, func(r *http.Request) {
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", s.Info().Origin())
	})
}

// A mission's page records a decision of yours, one in place of another
// that stands, and withdraws one, saying why: each is refused when it
// says nothing, or names a decision that doesn't stand.
func TestDecisionsFromThePage(t *testing.T) {
	s, _ := docServer(t)
	api := "/api/missions/m/decisions"
	store := mission.Store{Root: s.Root}

	res := decisionWrite(t, s, api, DecideRequest{What: "Key the cache per tenant.", Why: "Tokens are per tenant."})
	var got DecideResult
	json.NewDecoder(res.Body).Decode(&got)
	if res.StatusCode != 200 || got.ID != "d1" || got.Message != "Recorded d1." {
		t.Fatalf("record: %d %+v", res.StatusCode, got)
	}
	if res := decisionWrite(t, s, api, DecideRequest{What: "  "}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("nothing decided: %d %s", res.StatusCode, body(res))
	}

	res = decisionWrite(t, s, api, DecideRequest{What: "Key it per user.", Supersedes: "d1"})
	got = DecideResult{}
	json.NewDecoder(res.Body).Decode(&got)
	if res.StatusCode != 200 || got.ID != "d2" || got.Message != "Recorded d2, replacing d1." {
		t.Fatalf("replace: %d %+v", res.StatusCode, got)
	}
	if res := decisionWrite(t, s, api, DecideRequest{What: "Again.", Supersedes: "d1"}); res.StatusCode != http.StatusConflict {
		t.Errorf("replacing one that's replaced: %d %s", res.StatusCode, body(res))
	}
	if res := decisionWrite(t, s, api, DecideRequest{What: "Again.", Supersedes: "d9"}); res.StatusCode != http.StatusNotFound {
		t.Errorf("replacing one there isn't: %d %s", res.StatusCode, body(res))
	}

	if res := decisionWrite(t, s, api+"/d2/withdraw", WithdrawRequest{}); res.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("withdrawn without a why: %d %s", res.StatusCode, body(res))
	}
	if res := decisionWrite(t, s, api+"/d2/withdraw", WithdrawRequest{Why: "Tokens moved to the gateway."}); res.StatusCode != 200 {
		t.Fatalf("withdraw: %d %s", res.StatusCode, body(res))
	}
	if res := decisionWrite(t, s, api+"/d2/withdraw", WithdrawRequest{Why: "Again."}); res.StatusCode != http.StatusConflict {
		t.Errorf("withdrawing one that's withdrawn: %d %s", res.StatusCode, body(res))
	}

	all, err := store.Decisions("m")
	if err != nil || len(all) != 2 {
		t.Fatalf("decisions = %+v, %v", all, err)
	}
	if d := all[0]; d.By != "you" || d.State != mission.DecisionSuperseded || d.SupersededBy != "d2" {
		t.Errorf("d1 = %+v", d)
	}
	if d := all[1]; d.By != "you" || d.State != mission.DecisionWithdrawn || d.WithdrawnWhy != "Tokens moved to the gateway." {
		t.Errorf("d2 = %+v", d)
	}
}
