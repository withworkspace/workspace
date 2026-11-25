package acp

// Usage is what a turn spent, as the agent says in its prompt's response
// (ACP's PromptResponse.usage): tokens sent that weren't cached, written
// (thinking among them), and read from and written to the cache. Model is
// the model it was spent on, where the agent says (codex-acp's
// _meta.quota.model_usage): "" where it doesn't.
//
// Not every agent says: codex-acp and opencode do; cursor-agent's ACP
// doesn't.
type Usage struct {
	Model      string `json:"model,omitempty"`
	Input      int64  `json:"input,omitempty"`
	Output     int64  `json:"output,omitempty"`
	CacheRead  int64  `json:"cacheRead,omitempty"`
	CacheWrite int64  `json:"cacheWrite,omitempty"`
}

// promptResponse is session/prompt's answer.
type promptResponse struct {
	StopReason string `json:"stopReason"`
	Usage      *struct {
		Input      int64 `json:"inputTokens"`
		Output     int64 `json:"outputTokens"`
		CacheRead  int64 `json:"cachedReadTokens"`
		CacheWrite int64 `json:"cachedWriteTokens"`
	} `json:"usage"`
	Meta struct {
		Quota struct {
			Models []struct {
				Model string `json:"model"`
			} `json:"model_usage"`
		} `json:"quota"`
	} `json:"_meta"`
}

// usage is what the response says the turn spent: nil for nothing.
func (r promptResponse) usage() *Usage {
	if r.Usage == nil {
		return nil
	}
	u := Usage{Input: r.Usage.Input, Output: r.Usage.Output, CacheRead: r.Usage.CacheRead, CacheWrite: r.Usage.CacheWrite}
	if u == (Usage{}) {
		return nil
	}
	// A turn on one model says which; one across several doesn't split
	// the tokens between them, so it's kept on none.
	if ms := r.Meta.Quota.Models; len(ms) == 1 {
		u.Model = ms[0].Model
	}
	return &u
}
