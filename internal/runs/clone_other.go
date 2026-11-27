//go:build !darwin

package runs

import "errors"

// cloneFile can't clone here: the caller copies.
func cloneFile(from, to string) error { return errors.ErrUnsupported }
