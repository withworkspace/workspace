//go:build !cgo

package nav

// Builtin is the runtime this build has: none, without cgo.
func Builtin() Runtime { return noRuntime{} }

type noRuntime struct{}

func (noRuntime) Has(string) bool { return false }

func (noRuntime) Captures(string, string, []byte) ([]Match, bool, error) {
	return nil, false, ErrNoParser
}
