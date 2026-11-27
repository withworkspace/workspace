package mission

import (
	"fmt"

	"github.com/withworkspace/workspace/internal/store"
)

// LandingAttempt is the candidate written before publishing a direct landing.
type LandingAttempt struct {
	Landing
	Branch  string   `toml:"branch" json:"branch"`
	Commits []string `toml:"commits" json:"commits"`
	Settled []string `toml:"settled,omitempty" json:"settled,omitempty"`
}

func capsuleForLanding(m *Mission, repo, name string) (*Capsule, error) {
	for i := range m.Capsules {
		if c := &m.Capsules[i]; c.Repo == repo && c.Name == name {
			return c, nil
		}
	}
	return nil, fmt.Errorf("mission %s no longer has capsule %s/%s", m.Slug, repo, name)
}

// PrepareLanding records what ws is about to push.
func (s Store) PrepareLanding(slug, repo, name string, attempt LandingAttempt) error {
	_, err := s.Update(slug, func(m *Mission) error {
		c, err := capsuleForLanding(m, repo, name)
		if err != nil {
			return err
		}
		c.PendingLanding = &attempt
		return nil
	})
	return err
}

// RecordLanding finishes a pending landing and logs it in the same transaction.
func (s Store) RecordLanding(slug, repo, name, head, text string) error {
	return s.write(func(tx *store.Tx) error {
		m, err := s.loadIn(tx, slug)
		if err != nil {
			return err
		}
		c, err := capsuleForLanding(m, repo, name)
		if err != nil {
			return err
		}
		if c.PendingLanding == nil && c.Landed != nil && c.Landed.Head == head {
			return nil
		}
		if c.PendingLanding == nil || c.PendingLanding.Head != head {
			return fmt.Errorf("mission %s has no pending landing of %s/%s at %s", slug, repo, name, head)
		}
		landed := c.PendingLanding.Landing
		c.Landed = &landed
		c.PendingLanding = nil
		if err := s.keep(tx, m); err != nil {
			return err
		}
		return s.appendLog(tx, slug, event(Event{Kind: EventLanded, Repo: repo, Capsule: name, Crew: "you", Text: text}))
	})
}
