// Package store is what ws records: one SQLite file, .ws/ws.db, that only
// the workspace's daemon opens. The daemon takes it when it starts (Own)
// and serves it on its own socket, .ws/store.sock; a command borrows a
// connection there (Open), starting the daemon if it's down. The code that
// reads and writes records is the same either way: it's handed a DB and
// works in its transactions. Statements cross the socket, not operations:
// a command's operation still runs in the command.
//
// What's kept in the file is said by the packages that own its tables
// (Define), each with its own layout, brought forward when the daemon
// takes the file. A command and the daemon say their layouts when they
// meet, so two builds of ws that differ only in other ways get on.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/withworkspace/workspace/internal/cost"
	"github.com/withworkspace/workspace/internal/ref"
)

// DB is a workspace's records: the file, in the process that owns it (the
// daemon); anywhere else, connections to the daemon.
type DB struct {
	root string
	// write is what a write transaction begins on: the one writing
	// connection in the owner, the socket elsewhere. read is what a read
	// begins on: the owner's readers, or the same socket.
	write, read *sql.DB
	own         *owner // set in the process that owns the file
}

// Tx is one transaction: a Write's or a Read's.
type Tx struct {
	tx *sql.Tx
	// ctx is the one the transaction began with: each statement in it ends
	// with it too, not only the transaction.
	ctx    context.Context
	root   string
	write  bool
	topics []string
	after  []func()
	// known is what there is to name, once it's been asked for (names.go).
	known *ref.Known
	asked bool
}

// Root is the workspace whose records these are.
func (tx *Tx) Root() string { return tx.root }

// OnCommit has fn run once the transaction has committed, in the process
// that ran it: for what mustn't happen unless the records did (files moved
// once what they held is in the file). Nothing runs if it rolls back.
func (tx *Tx) OnCommit(fn func()) {
	if !tx.write {
		panic("store: OnCommit in a Read")
	}
	tx.after = append(tx.after, fn)
}

func (tx *Tx) committed() {
	for _, fn := range tx.after {
		fn()
	}
}

// Start, when set, starts the workspace's daemon and returns once it's
// listening: Open calls it when there's no daemon to borrow a connection
// from. The command line sets it; a test leaves it nil, and owns the file
// itself.
var Start func(root string) error

var (
	mu  sync.Mutex
	dbs = map[string]*DB{} // by root, made absolute
)

// Open is the workspace's records, started if they're not being served:
// the file in the process that owns it, else a connection to the daemon,
// which is started (Start) if it's down. A process has one for a root,
// however many times it's asked.
func Open(root string) (*DB, error) { return open(root, true) }

// Dial is Open without starting the daemon: for asking after the records
// only if they're there (ws daemon status).
func Dial(root string) (*DB, error) { return open(root, false) }

func open(root string, start bool) (*DB, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	if db := dbs[root]; db != nil {
		return db, nil
	}
	db, err := dial(root)
	if err != nil && start && NoServer(err) && Start != nil {
		if err = Start(root); err == nil {
			db, err = dial(root)
		}
	}
	if err != nil {
		return nil, err
	}
	dbs[root] = db
	return db, nil
}

// Borrow is a connection to the daemon's records over its socket, even in
// the process that owns them: for a test that stands in for a command.
func Borrow(root string) (*DB, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	return dial(root)
}

// NoServer reports whether an Open or Dial error means nothing is serving
// the records: no daemon, or one that went without removing its socket.
func NoServer(err error) bool {
	return errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED)
}

// Root is the workspace the records are of.
func (db *DB) Root() string { return db.root }

// Owned reports whether this process owns the file: it's the daemon (or a
// test that took it).
func (db *DB) Owned() bool { return db.own != nil }

// Close lets go of the records: in the owner, it stops serving them,
// rolling back what's under way, and closes the file; elsewhere, it closes
// the connections. Open after it starts again.
func (db *DB) Close() error {
	mu.Lock()
	if dbs[db.root] == db {
		delete(dbs, db.root)
	}
	mu.Unlock()
	if db.own != nil {
		return db.own.close()
	}
	return db.write.Close()
}

// Write runs fn in one transaction, and commits it: all of it happens, or
// none (an error from fn, or a panic, rolls it back). Writers take their
// turn in the daemon, in the order they asked. A Write doesn't nest: fn
// mustn't Write again, which would wait for itself.
func (db *DB) Write(ctx context.Context, fn func(*Tx) error) (err error) {
	if db.own != nil {
		waited := cost.Begin(cost.SQLTurn)
		release, err := db.own.turn(ctx)
		waited()
		if err != nil {
			return err
		}
		defer release()
	}
	defer cost.Begin(cost.SQLWrite)()
	stx, err := db.write.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	tx := &Tx{tx: stx, ctx: ctx, root: db.root, write: true}
	done := false
	defer func() {
		if !done {
			stx.Rollback()
		}
	}()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.noteChanges(); err != nil {
		return err
	}
	done = true
	if err := stx.Commit(); err != nil {
		return err
	}
	if db.own != nil {
		db.own.committed()
	}
	tx.committed()
	return nil
}

// Read runs fn on the records as they stood when it began. Reads don't
// wait for writes, or for each other.
func (db *DB) Read(ctx context.Context, fn func(*Tx) error) error {
	defer cost.Begin(cost.SQLRead)()
	stx, err := db.read.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer stx.Rollback() // a read has nothing to commit
	return fn(&Tx{tx: stx, ctx: ctx, root: db.root})
}

// Seq is the latest change's number: it moves whenever a write that named
// a topic commits, so it names the records as they are for a change check.
func (db *DB) Seq(ctx context.Context) (seq int64, err error) {
	err = db.Read(ctx, func(tx *Tx) error {
		return tx.QueryRow(`SELECT value FROM meta WHERE key = 'seq'`).Scan(&seq)
	})
	return seq, err
}

// Exec runs a statement that returns no rows.
func (tx *Tx) Exec(query string, args ...any) (sql.Result, error) {
	cost.Stmt()
	return tx.tx.ExecContext(tx.ctx, query, args...)
}

// Query runs a statement that returns rows.
func (tx *Tx) Query(query string, args ...any) (*sql.Rows, error) {
	cost.Stmt()
	return tx.tx.QueryContext(tx.ctx, query, args...)
}

// QueryRow runs a statement that returns at most one row.
func (tx *Tx) QueryRow(query string, args ...any) *sql.Row {
	cost.Stmt()
	return tx.tx.QueryRowContext(tx.ctx, query, args...)
}

// Changed says what this transaction changed: topics a page follows
// (workspace, mission:<slug>, page:<id>). Whoever's watching hears of
// them once it commits. Only a Write changes anything.
func (tx *Tx) Changed(topics ...string) {
	if !tx.write {
		panic("store: Changed in a Read")
	}
	for _, t := range topics {
		if !slices.Contains(tx.topics, t) {
			tx.topics = append(tx.topics, t)
		}
	}
}

// noteChanges records the transaction's topics at the next number in
// changes, in the transaction itself: so it's the same whether it's
// committed in the daemon or from a command.
func (tx *Tx) noteChanges() error {
	if len(tx.topics) == 0 {
		return nil
	}
	var seq int64
	if err := tx.tx.QueryRowContext(tx.ctx, `UPDATE meta SET value = value + 1 WHERE key = 'seq' RETURNING value`).Scan(&seq); err != nil {
		return fmt.Errorf("store: numbering a change: %w", err)
	}
	for _, t := range tx.topics {
		if _, err := tx.tx.ExecContext(tx.ctx, `INSERT INTO changes (topic, seq) VALUES (?, ?) ON CONFLICT (topic) DO UPDATE SET seq = excluded.seq`, t, seq); err != nil {
			return fmt.Errorf("store: noting a change: %w", err)
		}
	}
	return nil
}

// Since is the topics changed after seq, and the latest number: what a
// page that's been told something changed asks.
func (db *DB) Since(ctx context.Context, seq int64) (topics []string, latest int64, err error) {
	err = db.Read(ctx, func(tx *Tx) error {
		if err := tx.QueryRow(`SELECT value FROM meta WHERE key = 'seq'`).Scan(&latest); err != nil {
			return err
		}
		rows, err := tx.Query(`SELECT topic FROM changes WHERE seq > ? ORDER BY topic`, seq)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var t string
			if err := rows.Scan(&t); err != nil {
				return err
			}
			topics = append(topics, t)
		}
		return rows.Err()
	})
	return topics, latest, err
}

// Watch is told after each write commits, a command's included: a
// receive that may stand for several. Only the process that owns the
// file hears anything; elsewhere it's never told. Stop when done.
func (db *DB) Watch() (told <-chan struct{}, stop func()) {
	if db.own == nil {
		return make(chan struct{}), func() {}
	}
	return db.own.watch()
}

// Layouts is the layout of each part of the file, as it is: what the
// daemon brought it forward to.
func (db *DB) Layouts(ctx context.Context) (map[string]int, error) {
	out := map[string]int{}
	err := db.Read(ctx, func(tx *Tx) error {
		rows, err := tx.Query(`SELECT substr(key, 8), value FROM meta WHERE key LIKE 'layout/%'`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var name string
			var v int
			if err := rows.Scan(&name, &v); err != nil {
				return err
			}
			out[name] = v
		}
		return rows.Err()
	})
	return out, err
}

// txLimit is how long a command's transaction may stay open in the
// daemon before it's rolled back and its connection closed, so a command
// that's stuck can't hold up every writer. Tests shorten it.
var txLimit = 10 * time.Second
