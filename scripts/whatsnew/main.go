// Command whatsnew works out which release each of ws's what's-new
// entries is in (internal/whatsnew says how), and writes what the binary
// holds: internal/whatsnew/gen/whatsnew.json. just build runs it, and so
// does a release's build (.github/workflows/release.yml), which also asks
// it for the release's notes and pictures.
//
//	go run ./scripts/whatsnew                  # write gen/whatsnew.json
//	go run ./scripts/whatsnew -check           # only read the entries: fail on one that doesn't
//	go run ./scripts/whatsnew -notes v1.4.0 -base https://github.com/withworkspace/workspace [-pictures dir]
//	                                           # print v1.4.0's notes, and copy its pictures into dir
//
// It's run from the repo's root.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/withworkspace/workspace/internal/whatsnew"
)

const out = "internal/whatsnew/gen/whatsnew.json"

func main() {
	check := flag.Bool("check", false, "only read the entries, and fail on one that doesn't")
	notes := flag.String("notes", "", "print this release's notes (its entries) rather than writing "+out)
	base := flag.String("base", "", "with -notes: the repo's page, where the release's pictures are")
	pictures := flag.String("pictures", "", "with -notes: copy the release's pictures into this folder, named as they're published")
	flag.Parse()
	if err := run(*check, *notes, *base, *pictures); err != nil {
		fmt.Fprintln(os.Stderr, "whatsnew:", err)
		os.Exit(1)
	}
}

func run(check bool, notes, base, pictures string) error {
	entries, err := whatsnew.Find(".")
	if err != nil {
		return err
	}
	switch {
	case check:
		return nil
	case notes != "":
		fmt.Print(whatsnew.Notes(entries, notes, base))
		if pictures == "" {
			return nil
		}
		for name, from := range whatsnew.Pictures(".", entries, notes) {
			if err := copyFile(from, filepath.Join(pictures, name)); err != nil {
				return err
			}
		}
		return nil
	}
	data, err := json.MarshalIndent(whatsnew.Arrange(entries, whatsnew.Keep), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(out, append(data, '\n'), 0o644)
}

func copyFile(from, to string) error {
	src, err := os.Open(from)
	if err != nil {
		return err
	}
	defer src.Close()
	dst, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		return err
	}
	return dst.Close()
}
