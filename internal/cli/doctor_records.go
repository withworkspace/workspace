package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/withworkspace/workspace/internal/ops"
	"github.com/withworkspace/workspace/internal/store"
	"github.com/withworkspace/workspace/internal/workspace"
)

// checkRecords is what's said of the workspace's records (.ws/ws.db): that
// SQLite finds the file sound, how old the latest copy of it is, whether
// it's somewhere it isn't safe, and what's beside it that no record has. The
// daemon has the file, so asking starts it if it's down.
func checkRecords(ctx *ops.Workspace) workspace.CheckCategory {
	cat := workspace.CheckCategory{Name: "Records"}
	root := ctx.WS().Root
	if _, err := os.Stat(store.Path(root)); err != nil {
		return cat // none yet: nothing's been recorded
	}
	file := workspace.CheckResult{Name: ".ws/ws.db", Status: workspace.CheckOK}
	if db, err := ctx.Records(); err != nil {
		file.Status, file.Detail = workspace.CheckFail, "can't be reached: "+err.Error()
	} else if bad, err := db.Check(context.Background()); err != nil {
		file.Status, file.Detail = workspace.CheckFail, "couldn't be checked: "+err.Error()
	} else if len(bad) > 0 {
		file.Status = workspace.CheckFail
		file.Detail = fmt.Sprintf("SQLite finds it damaged: %s", strings.Join(bad[:min(len(bad), 3)], "; "))
		file.FixHint = "stop the daemon (ws daemon stop) and put the latest copy in .ws/backup/ in its place"
	} else {
		file.Detail = "sound, as SQLite checks it"
	}
	cat.Checks = append(cat.Checks, file)

	copies := store.Copies(root)
	kept := workspace.CheckResult{Name: "a copy of it", Status: workspace.CheckOK}
	if len(copies) == 0 {
		kept.Status, kept.Detail = workspace.CheckWarn, "none: the daemon takes one in .ws/backup/ the first time it starts each day"
	} else {
		latest := copies[len(copies)-1]
		kept.Detail = latest + ", in .ws/backup/"
		if day, err := time.ParseInLocation("ws-2006-01-02.db", latest, time.Local); err == nil {
			// Asking started the daemon if it was down, which takes the
			// day's copy: an old one is copies failing.
			if days := int(time.Since(day).Hours() / 24); days > 1 {
				kept.Status, kept.Detail = workspace.CheckWarn, fmt.Sprintf("the latest, %s, is %d days old: the daemon couldn't take today's (.ws/daemon.log says why)", latest, days)
			}
		}
	}
	cat.Checks = append(cat.Checks, kept)

	if why := store.Disk(root); why != "" {
		cat.Checks = append(cat.Checks, workspace.CheckResult{
			Name: "where it is", Status: workspace.CheckWarn, Detail: why,
			FixHint: "keep the workspace on a local disk, outside any folder a sync service copies",
		})
	}

	// Files beside the records that no record has: what a write that
	// didn't finish left, or what was kept before the records.
	beside := workspace.CheckResult{Name: "files beside it", Status: workspace.CheckOK, Detail: "each has its record"}
	if litter, err := ctx.Litter(context.Background()); err != nil {
		beside.Status, beside.Detail = workspace.CheckWarn, "couldn't be checked: "+err.Error()
	} else if len(litter) > 0 {
		beside.Status = workspace.CheckWarn
		beside.Detail = litterSaid(litter) + " (ws doctor --fix moves them aside, to .ws/swept/)"
		beside.Fix = func() error {
			_, err := ctx.Sweep(litter)
			return err
		}
	}
	cat.Checks = append(cat.Checks, beside)
	return cat
}

// litterSaid says what's under .ws with no record: how many, whose, and
// the first few.
func litterSaid(litter []ops.Litter) string {
	var whose []string
	count := map[string]int{}
	for _, l := range litter {
		if count[l.What] == 0 {
			whose = append(whose, l.What)
		}
		count[l.What]++
	}
	var parts []string
	for _, w := range whose {
		parts = append(parts, fmt.Sprintf("%d %s", count[w], w))
	}
	var first []string
	for _, l := range litter[:min(len(litter), 3)] {
		first = append(first, ".ws/"+l.Path)
	}
	if len(litter) > 3 {
		first = append(first, "…")
	}
	return fmt.Sprintf("%d %s no record has (%s): %s", len(litter), pluralize(len(litter), "file or folder", "files or folders"), strings.Join(parts, ", "), strings.Join(first, ", "))
}
