package artifact

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/withworkspace/workspace/internal/store"
)

// An artifact that lives in a repo is a folder there (its Home): the
// version chosen, its record, and a thing.toml that says it's an artifact.
// ws finds such a folder in a space, whoever promoted it, on whichever
// machine. What's kept of one here is what the repo says, so it's found
// as an artifact on this machine is: by its title and its versions' notes.
// It's named by its folder (records/artifacts/2026-11-20-horizon-glow/),
// which is its name on every machine; a3 is only ever this machine's.

// MainFile is the file of an artifact's folder in a repo that its
// thing.toml names as the thing: its page.
const MainFile = HomeFile

// Found is an artifact as ws found it in a repo.
type Found struct {
	// Name is its folder, as ws names one: records/artifacts/2026-11-20-horizon-glow/.
	Name string `json:"name"`
	// Repo is the repo it's in, and Folder its folder there, with no / at
	// its end.
	Repo   string `json:"repo"`
	Folder string `json:"folder"`
	Title  string `json:"title"`
	// Version is what its folder holds, as git names it: another when any
	// of its files changes.
	Version string `json:"version"`
	// Changed is when it last changed there, and ChangedBy who by.
	Changed   time.Time `json:"changed"`
	ChangedBy string    `json:"changed_by,omitempty"`
	// Record is what its artifact.toml there says: which version it is,
	// who promoted it, and what each version up to it changed. Nil for a
	// folder with none ws could read.
	Record *Artifact `json:"record,omitempty"`
}

// Where is its folder as it's said to people, and as a document cites
// it: records:artifacts/2026-11-20-horizon-glow/.
func (f Found) Where() string { return f.Repo + ":" + f.Folder + "/" }

func foundOrigin(name string) store.Origin { return store.Origin("artifact-found:" + name) }

// Found are the artifacts ws has found in the workspace's repos, as they
// were last kept: the latest changed first.
func (s Store) Found(ctx context.Context) ([]Found, error) {
	db, err := s.records()
	if err != nil {
		return nil, err
	}
	var out []Found
	err = db.Read(ctx, func(tx *store.Tx) error {
		rows, err := tx.Query(`SELECT name, repo, folder, title, version, changed, changed_by, record FROM artifacts_found ORDER BY changed DESC, name`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var f Found
			var changed, record string
			if err := rows.Scan(&f.Name, &f.Repo, &f.Folder, &f.Title, &f.Version, &changed, &f.ChangedBy, &record); err != nil {
				return err
			}
			f.Changed, _ = time.Parse(store.Stamp, changed)
			if record != "" {
				var a Artifact
				if json.Unmarshal([]byte(record), &a) == nil {
					f.Record = &a
				}
			}
			out = append(out, f)
		}
		return rows.Err()
	})
	return out, err
}

// KeepFound keeps the artifacts ws found in the repos: each of keep as
// it is now, in place of how it was kept, and nothing of those named in
// gone. One that's kept is in the graph, by its folder's name.
func (s Store) KeepFound(ctx context.Context, keep []Found, gone []string) error {
	if len(keep) == 0 && len(gone) == 0 {
		return nil
	}
	db, err := s.records()
	if err != nil {
		return err
	}
	return db.Write(ctx, func(tx *store.Tx) error {
		for _, name := range gone {
			if _, err := tx.Exec(`DELETE FROM artifacts_found WHERE name = ?`, name); err != nil {
				return err
			}
			if err := tx.Forget(foundOrigin(name)); err != nil {
				return err
			}
		}
		for _, f := range keep {
			record := ""
			if f.Record != nil {
				data, err := json.Marshal(f.Record)
				if err != nil {
					return err
				}
				record = string(data)
			}
			if _, err := tx.Exec(`INSERT INTO artifacts_found (name, repo, folder, title, version, changed, changed_by, record) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
				ON CONFLICT (name) DO UPDATE SET repo = excluded.repo, folder = excluded.folder, title = excluded.title, version = excluded.version,
					changed = excluded.changed, changed_by = excluded.changed_by, record = excluded.record`,
				f.Name, f.Repo, f.Folder, f.Title, f.Version, stampOf(f.Changed), f.ChangedBy, record); err != nil {
				return err
			}
			if err := graphFound(tx, f); err != nil {
				return err
			}
		}
		tx.Changed("workspace", "artifacts-found")
		return nil
	})
}

// graphFound puts an artifact found in a repo in the graph: a thing, by
// its folder's name, whose words are its title and what its record says
// each version changed.
func graphFound(tx *store.Tx, f Found) error {
	o := foundOrigin(f.Name)
	if err := tx.Forget(o); err != nil {
		return err
	}
	me := store.Key{Name: f.Name}
	if err := tx.Thing(o, store.Thing{Key: me, Kind: ThingKind, Title: f.Title, State: string(Promoted), Changed: f.Changed}); err != nil {
		return err
	}
	if f.Record == nil {
		return nil
	}
	for _, v := range f.Record.Versions {
		heading := "v" + strconv.Itoa(v.N)
		if err := tx.Words(o, store.Place{Thing: me, At: heading, Kind: placeVersion, Title: f.Title, Heading: heading, Text: v.Note, Said: v.At}); err != nil {
			return err
		}
	}
	return nil
}
