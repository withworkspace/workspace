// Package store keeps things.
package store

// Store keeps things by key.
type Store struct{ items map[string]string }

// New is an empty store.
func New() *Store { return &Store{items: map[string]string{}} }

// Get is a thing by its key.
func (s *Store) Get(key string) string { return s.items[key] }

func hidden() {}
