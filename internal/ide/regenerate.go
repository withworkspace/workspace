package ide

import (
	"errors"

	"github.com/withworkspace/workspace/internal/ref"
)

// Regenerate updates all detected IDE workspace files from board state.
// Only mutates files that already exist.
func Regenerate(root string, boarded map[string][]string, displayNames map[string]string, remote func(repo string) ref.Remote) error {
	var errs []error
	if err := GenerateVSCode(root, boarded, displayNames); err != nil {
		errs = append(errs, err)
	}
	if err := GenerateIDEA(root, boarded, remote); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
