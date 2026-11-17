package ops

import (
	"errors"
	"fmt"

	"github.com/withworkspace/workspace/internal/conversation"
	"github.com/withworkspace/workspace/internal/mission"
	"github.com/withworkspace/workspace/internal/review"
)

// ErrNotFound is no such mission, capsule, crew or thread.
var ErrNotFound = errors.New("not found")

// Invalid is a request that's wrong whatever the state: an empty answer, an
// unknown kind.
type Invalid struct{ Reason string }

func (e *Invalid) Error() string { return e.Reason }

// Refused is a request the state doesn't allow now: the capsule's busy,
// crew are still on the stage, the file's changed since. Code is a short
// word a page can act on; Reason says it.
type Refused struct {
	Code   string
	Reason string
	Err    error // the store's own error, when there's one
}

func (e *Refused) Error() string { return e.Reason }
func (e *Refused) Unwrap() error { return e.Err }

// Upstream is GitHub, or a remote, that couldn't be reached.
type Upstream struct{ Err error }

func (e *Upstream) Error() string { return e.Err.Error() }
func (e *Upstream) Unwrap() error { return e.Err }

func invalid(format string, a ...any) error { return &Invalid{Reason: fmt.Sprintf(format, a...)} }

func refused(code, format string, a ...any) error {
	return &Refused{Code: code, Reason: fmt.Sprintf(format, a...)}
}

// wrap is a store's error as one of ours: the one place domain errors are
// told apart.
func wrap(err error) error {
	var r *Refused
	var inv *Invalid
	switch {
	case err == nil:
		return nil
	case errors.As(err, &r), errors.As(err, &inv), errors.Is(err, ErrNotFound):
		return err
	case errors.Is(err, mission.ErrNotFound), errors.Is(err, conversation.ErrNotFound), errors.Is(err, review.ErrNotFound):
		return fmt.Errorf("%w: %v", ErrNotFound, err)
	case errors.Is(err, mission.ErrStageBusy):
		return &Refused{Code: "stage-busy", Reason: err.Error(), Err: err}
	case errors.Is(err, mission.ErrAllAccepted):
		return &Refused{Code: "all-accepted", Reason: err.Error(), Err: err}
	case errors.Is(err, mission.ErrNoTask):
		return &Refused{Code: "nothing-asked", Reason: err.Error(), Err: err}
	case errors.Is(err, mission.ErrDispatchQueued):
		return &Refused{Code: "dispatch-queued", Reason: err.Error(), Err: err}
	case errors.Is(err, mission.ErrDispatchLimit):
		return &Refused{Code: "dispatch-limit", Reason: err.Error(), Err: err}
	}
	return err
}
