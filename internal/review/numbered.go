package review

import (
	"fmt"
	"strings"
)

// A diff as an agent with nothing else reads it: each file's changes with
// its lines numbered, so what it finds can say which lines it's on.

// NumberedHow says how Numbered's lines read, for whoever's given them.
const NumberedHow = "Each file starts with a line of its path. Each line under it starts with its number in the file as the change leaves it, then + if the change adds it, or a space if it's unchanged; a line the change removes starts with o and its number as it was before, then -. `...` is lines left out between."

// Left is a file Numbered left out, and why: what it's to skim as (a
// lockfile, generated), binary, too large, or no room.
type Left struct {
	Path string `json:"path"`
	Why  string `json:"why"`
}

// What a file's left out for, besides being one to skim (Skim*).
const (
	LeftBinary = "binary"
	LeftLarge  = "too large"
	LeftNoRoom = "no room"
)

// String is how an agent's told of it: go.sum (lockfile).
func (l Left) String() string { return fmt.Sprintf("%s (%s)", l.Path, l.Why) }

// Numbered is a diff's files as text, their lines numbered, to room bytes
// in all: a file to skim (a lockfile, generated code), a binary one and
// one that would go over is left out, and named in left.
func Numbered(files []File, room int) (text string, left []Left) {
	var b strings.Builder
	for _, f := range files {
		if f.Skim != "" || f.Binary || f.Large {
			why := f.Skim
			if why == "" {
				why = LeftBinary
				if f.Large {
					why = LeftLarge
				}
			}
			left = append(left, Left{Path: f.Path, Why: why})
			continue
		}
		var fb strings.Builder
		head := f.Path + " (" + f.Status
		if f.OldPath != "" {
			head += " from " + f.OldPath
		}
		fmt.Fprintf(&fb, "=== %s)\n", head)
		for i, h := range f.Hunks {
			if i > 0 {
				fb.WriteString("...\n")
			}
			for _, l := range h.Lines {
				switch l.Kind {
				case "d":
					fmt.Fprintf(&fb, "o%-5d -%s\n", l.Old, l.Text)
				case "a":
					fmt.Fprintf(&fb, "%-6d +%s\n", l.New, l.Text)
				default:
					fmt.Fprintf(&fb, "%-6d  %s\n", l.New, l.Text)
				}
			}
		}
		fb.WriteString("\n")
		if b.Len()+fb.Len() > room {
			left = append(left, Left{Path: f.Path, Why: LeftNoRoom})
			continue
		}
		b.WriteString(fb.String())
	}
	return strings.TrimRight(b.String(), "\n"), left
}
