package testutil

import "github.com/withworkspace/workspace/internal/github"

// StubClient is a test double for github.Client (github.Stub, which
// packages that testutil itself imports can use too).
type StubClient = github.Stub
