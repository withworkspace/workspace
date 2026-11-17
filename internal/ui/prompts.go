package ui

import (
	"fmt"
	"os"

	"charm.land/huh/v2"
	"golang.org/x/term"
)

func IsInteractive() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

func PickRepo(repos []string, displayNames map[string]string) (string, error) {
	if !IsInteractive() {
		return "", fmt.Errorf("no repo specified and not in an interactive terminal\n\nUsage: pass repo as argument or run from inside a repo directory")
	}

	var selected string
	opts := make([]huh.Option[string], len(repos))
	for i, r := range repos {
		label := r
		if dn, ok := displayNames[r]; ok {
			label = fmt.Sprintf("%s (%s)", dn, r)
		}
		opts[i] = huh.NewOption(label, r)
	}

	err := huh.NewSelect[string]().
		Title("Select repo").
		Options(opts...).
		Value(&selected).
		Run()

	return selected, err
}

func PickWorktree(worktrees []string) (string, error) {
	if !IsInteractive() {
		return "", fmt.Errorf("no branch specified and not in an interactive terminal\n\nUsage: pass branch as argument or run from inside a worktree")
	}

	var selected string
	opts := make([]huh.Option[string], len(worktrees))
	for i, w := range worktrees {
		opts[i] = huh.NewOption(w, w)
	}

	err := huh.NewSelect[string]().
		Title("Select worktree").
		Options(opts...).
		Value(&selected).
		Run()

	return selected, err
}

// ConfirmFunc is the implementation behind Confirm. Tests can replace it to
// avoid interactive prompts (same pattern as cli.SetOverride).
var ConfirmFunc = confirmImpl

func confirmImpl(message string) (bool, error) {
	if !IsInteractive() {
		return false, fmt.Errorf("confirmation required but not in an interactive terminal")
	}

	var confirmed bool
	err := huh.NewConfirm().
		Title(message).
		Value(&confirmed).
		Run()

	return confirmed, err
}

func Confirm(message string) (bool, error) {
	return ConfirmFunc(message)
}
