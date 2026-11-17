package acp

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func permission(kinds ...string) *PermissionRequest {
	r := &PermissionRequest{SessionID: "s1"}
	for _, k := range kinds {
		r.Options = append(r.Options, PermissionOption{OptionID: k + "-id", Name: k, Kind: k})
	}
	return r
}

// waitOpen waits until n asks are open.
func waitOpen(t *testing.T, a *Asks, n int) []Ask {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if open := a.Open(); len(open) == n {
			return open
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("%d asks open, want %d", len(a.Open()), n)
	return nil
}

// An ask waits for someone; a wrong answer is refused, a right one ends
// the wait, and what's asked and answered is recorded.
func TestAsksWaitForAnAnswer(t *testing.T) {
	var mu sync.Mutex
	var events []Event
	changes := 0
	asks := NewAsks(nil, func() { mu.Lock(); changes++; mu.Unlock() }, func(e Event) { mu.Lock(); events = append(events, e); mu.Unlock() })

	got := make(chan Answer, 1)
	go func() { got <- asks.Wait(context.Background(), Ask{Permission: permission(AllowOnce, RejectOnce)}) }()
	open := waitOpen(t, asks, 1)
	if open[0].ID != "a1" || open[0].Permission == nil || open[0].At.IsZero() {
		t.Fatalf("open = %+v", open)
	}

	if err := asks.Answer("a1", Answer{Option: "allow_always-id"}); err == nil {
		t.Error("an option the ask didn't offer was taken")
	}
	if err := asks.Answer("a9", Answer{Option: "allow_once-id"}); !errors.Is(err, ErrNoAsk) {
		t.Errorf("answering an ask that isn't open = %v", err)
	}
	if err := asks.Answer("a1", Answer{Option: "allow_once-id", By: "web"}); err != nil {
		t.Fatal(err)
	}
	ans := <-got
	if ans.Option != "allow_once-id" || ans.Ask != "a1" || ans.By != "web" || ans.Cancelled {
		t.Errorf("answer = %+v", ans)
	}
	if len(asks.Open()) != 0 {
		t.Error("still open once answered")
	}
	if err := asks.Answer("a1", Answer{Option: "allow_once-id"}); !errors.Is(err, ErrNoAsk) {
		t.Errorf("answering twice = %v", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 2 || events[0].Ask == nil || events[0].Ask.ID != "a1" || events[1].Answer == nil || events[1].Answer.Option != "allow_once-id" {
		t.Errorf("recorded %+v", events)
	}
	if changes != 2 {
		t.Errorf("%d changes, want an open and a close", changes)
	}
}

// A policy answers first, and what it leaves goes to a person.
func TestAsksPolicy(t *testing.T) {
	policy := func(a Ask) (Answer, bool) {
		if a.Permission == nil {
			return Answer{}, false
		}
		if a.Permission.ToolCall.Kind == "read" {
			return Allow(*a.Permission), true
		}
		return Reject(*a.Permission), true
	}
	asks := NewAsks(policy, nil, nil)
	read := permission(AllowOnce, AllowAlways, RejectOnce)
	read.ToolCall.Kind = "read"
	if ans := asks.Wait(context.Background(), Ask{Permission: read}); ans.Option != "allow_once-id" || ans.By != "policy" {
		t.Errorf("a read = %+v", ans)
	}
	exec := permission(AllowOnce, RejectOnce)
	exec.ToolCall.Kind = "execute"
	if ans := asks.Wait(context.Background(), Ask{Permission: exec}); ans.Option != "reject_once-id" {
		t.Errorf("a command = %+v", ans)
	}
	// Neither offered once: cancelled, never allowed or rejected for good.
	always := permission(AllowAlways, RejectAlways)
	if a, r := Allow(*always), Reject(*always); !a.Cancelled || !r.Cancelled {
		t.Errorf("allow %+v, reject %+v, want both cancelled", a, r)
	}

	got := make(chan Answer, 1)
	go func() { got <- asks.Wait(context.Background(), Ask{Method: "cursor/ask_question"}) }()
	open := waitOpen(t, asks, 1)
	if open[0].ID != "a3" || open[0].Method != "cursor/ask_question" {
		t.Errorf("left to a person: %+v", open)
	}
	asks.Answer("a3", Answer{Result: []byte(`{"outcome":{"outcome":"answered"}}`)})
	if ans := <-got; string(ans.Result) != `{"outcome":{"outcome":"answered"}}` {
		t.Errorf("answer = %+v", ans)
	}
}

// Interrupting cancels every ask; the agent giving up cancels its own.
func TestAsksCancelled(t *testing.T) {
	asks := NewAsks(nil, nil, nil)
	got := make(chan Answer, 2)
	ctx, giveUp := context.WithCancel(context.Background())
	go func() { got <- asks.Wait(ctx, Ask{Permission: permission(AllowOnce)}) }()
	waitOpen(t, asks, 1)
	go func() { got <- asks.Wait(context.Background(), Ask{Permission: permission(AllowOnce)}) }()
	waitOpen(t, asks, 2)

	giveUp()
	if ans := <-got; !ans.Cancelled || ans.Ask != "a1" || ans.By != "agent" {
		t.Errorf("given up = %+v", ans)
	}
	waitOpen(t, asks, 1)
	asks.CancelAll()
	if ans := <-got; !ans.Cancelled || ans.Ask != "a2" || ans.By != "interrupted" {
		t.Errorf("interrupted = %+v", ans)
	}
	if len(asks.Open()) != 0 {
		t.Error("asks left open")
	}
}
