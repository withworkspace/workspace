package store

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	sqlite "modernc.org/sqlite"
)

// ErrOwned is the answer for owning records this process owns already.
var ErrOwned = errors.New("this process owns the records already")

// owner is the process that holds the file: the daemon. It writes on one
// connection, a transaction at a time in the order they asked, reads on
// as many as are reading, and serves both to commands on store.sock.
type owner struct {
	root           string
	lock           *os.File
	writer, reader *sql.DB
	ln             net.Listener
	sem            chan struct{} // the turn to write: one holder, the rest queued in order

	copyErr error // why the day's copy couldn't be taken, if it couldn't

	mu       sync.Mutex
	hold     func() (release func())
	conns    map[net.Conn]bool
	watchers map[chan struct{}]bool
	closed   bool
	wg       sync.WaitGroup
}

// Own takes the workspace's records for this process (the daemon, when it
// starts, or a test): it locks them so no other process can, opens the
// file, brings each part's layout forward, and serves it on store.sock
// until Close. Open in this process then returns it.
func Own(root string) (*DB, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	if db := dbs[root]; db != nil {
		if db.own != nil {
			return nil, ErrOwned
		}
		db.write.Close() // a connection to a daemon that's no longer wanted
		delete(dbs, root)
	}
	if err := os.MkdirAll(filepath.Join(root, ".ws"), 0o755); err != nil {
		return nil, err
	}
	lock, err := takeLock(root)
	if err != nil {
		return nil, err
	}
	o := &owner{root: root, lock: lock, sem: make(chan struct{}, 1), conns: map[net.Conn]bool{}, watchers: map[chan struct{}]bool{}}
	fail := func(err error) (*DB, error) {
		o.shut()
		return nil, err
	}
	// One connection writes, taking SQLite's write lock as it begins, so
	// what a transaction read can't change under it.
	if o.writer, err = sql.Open("sqlite", Path(root)+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_txlock=immediate"); err != nil {
		return fail(err)
	}
	o.writer.SetMaxOpenConns(1)
	o.writer.SetMaxIdleConns(1)
	if o.reader, err = sql.Open("sqlite", Path(root)+"?_pragma=busy_timeout(5000)&_query_only=1"); err != nil {
		return fail(err)
	}
	o.reader.SetMaxOpenConns(8)
	ctx := context.Background()
	// The day's copy, before anything brings the layout forward.
	o.copyErr = backup(ctx, root, o.writer, time.Now())
	if err := bringForward(ctx, root, o.writer); err != nil {
		return fail(err)
	}
	if err := o.listen(); err != nil {
		return fail(err)
	}
	db := &DB{root: root, write: o.writer, read: o.reader, own: o}
	dbs[root] = db
	o.wg.Add(1)
	go o.serve()
	return db, nil
}

// takeLock locks store.lock for as long as this process runs, and writes
// its id in it, so a second daemon (or a test) can say who has them.
func takeLock(root string) (*os.File, error) {
	f, err := os.OpenFile(lockPath(root), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		data, _ := os.ReadFile(lockPath(root))
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			who := "another process"
			if pid, _ := strconv.Atoi(strings.TrimSpace(string(data))); pid > 0 {
				who = fmt.Sprintf("another process (pid %d)", pid)
			}
			return nil, fmt.Errorf("%s has this workspace's records: its daemon", who)
		}
		return nil, err
	}
	f.Truncate(0)
	f.WriteAt([]byte(strconv.Itoa(os.Getpid())+"\n"), 0)
	return f, nil
}

func (o *owner) listen() error {
	path := SocketPath(o.root)
	os.Remove(path) // left by a daemon that died: the lock says none is here
	ln, err := net.Listen("unix", path)
	if err != nil {
		return err
	}
	os.Chmod(path, 0o600)
	o.ln = ln
	return nil
}

// CopyError is why the day's copy of the records couldn't be taken, when
// this process took them, if it couldn't: for the daemon's log.
func (db *DB) CopyError() error {
	if db.own == nil {
		return nil
	}
	return db.own.copyErr
}

// SetHold is what keeps the daemon up while a command has a connection
// open, as an open page does: called for each, and released when it
// closes.
func (db *DB) SetHold(hold func() (release func())) {
	if db.own == nil {
		return
	}
	db.own.mu.Lock()
	db.own.hold = hold
	db.own.mu.Unlock()
}

// turn waits for the turn to write, in the order asked.
func (o *owner) turn(ctx context.Context) (release func(), err error) {
	select {
	case o.sem <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	var once sync.Once
	return func() { once.Do(func() { <-o.sem }) }, nil
}

func (o *owner) committed() {
	o.mu.Lock()
	defer o.mu.Unlock()
	for ch := range o.watchers {
		select {
		case ch <- struct{}{}:
		default: // told already, and not yet heard
		}
	}
}

func (o *owner) watch() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	o.mu.Lock()
	o.watchers[ch] = true
	o.mu.Unlock()
	return ch, func() {
		o.mu.Lock()
		delete(o.watchers, ch)
		o.mu.Unlock()
	}
}

func (o *owner) serve() {
	defer o.wg.Done()
	for {
		nc, err := o.ln.Accept()
		if err != nil {
			return // closed
		}
		o.mu.Lock()
		if o.closed {
			o.mu.Unlock()
			nc.Close()
			return
		}
		o.conns[nc] = true
		hold := o.hold
		o.mu.Unlock()
		release := func() {}
		if hold != nil {
			release = hold()
		}
		o.wg.Go(func() {
			defer release()
			o.handle(nc)
			o.mu.Lock()
			delete(o.conns, nc)
			o.mu.Unlock()
		})
	}
}

// afterCommit is told as a command's commit is made, before it's answered:
// a test holds it there.
var afterCommit = func() {}

// handle is one command's connection: its hello, then its statements, a
// transaction at a time. Whatever it leaves open when it goes (or when
// its transaction outlives txLimit) is rolled back.
func (o *owner) handle(nc net.Conn) {
	defer nc.Close()
	dec := json.NewDecoder(bufio.NewReader(nc))
	enc := json.NewEncoder(nc)
	var (
		tx      *sql.Tx
		release func()
		limit   *time.Timer
	)
	end := func() {
		if limit != nil {
			limit.Stop()
			limit = nil
		}
		if tx != nil {
			tx.Rollback()
			tx = nil
		}
		if release != nil {
			release()
			release = nil
		}
	}
	defer end()
	ctx := context.Background()

	var hello request
	if err := dec.Decode(&hello); err != nil || hello.Op != "hello" {
		return
	}
	mine := layouts()
	if err := differ(mine, hello.Layouts); err != nil {
		enc.Encode(reply{Error: err.Error(), Layouts: mine, Pid: os.Getpid()})
		return
	}
	if enc.Encode(reply{Layouts: mine, Pid: os.Getpid()}) != nil {
		return
	}

	for {
		var req request
		if err := dec.Decode(&req); err != nil {
			return
		}
		var rep reply
		var err error
		switch req.Op {
		case "begin":
			if tx != nil {
				err = errors.New("store: a transaction is open on this connection already")
				break
			}
			if req.Write {
				if release, err = o.turn(ctx); err != nil {
					break
				}
				tx, err = o.writer.BeginTx(ctx, nil)
			} else {
				tx, err = o.reader.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			}
			if err != nil {
				end()
				break
			}
			limit = time.AfterFunc(txLimit, func() { nc.Close() })
		case "exec":
			if tx != nil {
				rep, err = execOn(tx, req)
				break
			}
			// One statement, a transaction of its own.
			var r func()
			if r, err = o.turn(ctx); err != nil {
				break
			}
			rep, err = execOn(o.writer, req)
			r()
			if err == nil {
				o.committed()
			}
		case "query":
			if tx != nil {
				rep, err = queryOn(tx, req)
			} else {
				rep, err = queryOn(o.reader, req)
			}
		case "check":
			rep.Found, err = o.check(ctx)
		case "commit", "rollback":
			if tx == nil {
				err = errors.New("store: no transaction is open on this connection")
				break
			}
			wrote := release != nil
			// The limit's stopped before it's committed: once the commit's
			// made, its answer must reach the command, which a limit
			// closing the connection after it would lose (the command would
			// say it failed, and might do it again). Past the limit
			// already, the connection's being closed: it's rolled back, as
			// it would have been, and the command's told no more.
			if limit != nil && !limit.Stop() {
				return
			}
			limit = nil
			if req.Op == "commit" {
				err = tx.Commit()
				afterCommit()
			} else {
				err = tx.Rollback()
			}
			tx = nil
			end()
			if wrote && req.Op == "commit" && err == nil {
				o.committed()
			}
		default:
			err = fmt.Errorf("store: no such request: %q", req.Op)
		}
		if err != nil {
			rep = reply{Error: err.Error()}
			if se, ok := errors.AsType[*sqlite.Error](err); ok {
				rep.Code = se.Code()
			}
		}
		if enc.Encode(rep) != nil {
			return
		}
	}
}

// check is SQLite's check of the file (integrity_check), on a connection
// opened for it: one of the readers can hold what full-text search knew of
// its index before the writer last changed it, and finds a sound file
// damaged. Outside a transaction, so a large file isn't held to a
// command's limit on one.
func (o *owner) check(ctx context.Context) ([]string, error) {
	c, err := sql.Open("sqlite", Path(o.root)+"?_pragma=busy_timeout(5000)&_query_only=1")
	if err != nil {
		return nil, err
	}
	defer c.Close()
	c.SetMaxOpenConns(1)
	rows, err := c.QueryContext(ctx, `PRAGMA integrity_check`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		if s != "ok" {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

// on is what a statement runs on: a transaction, or a pool.
type on interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func argsOf(req request) []any {
	args := make([]any, len(req.Args))
	for i, a := range req.Args {
		args[i] = a.v
	}
	return args
}

func execOn(on on, req request) (reply, error) {
	res, err := on.ExecContext(context.Background(), req.SQL, argsOf(req)...)
	if err != nil {
		return reply{}, err
	}
	var rep reply
	rep.Affected, _ = res.RowsAffected()
	rep.Last, _ = res.LastInsertId()
	return rep, nil
}

func queryOn(on on, req request) (reply, error) {
	rows, err := on.QueryContext(context.Background(), req.SQL, argsOf(req)...)
	if err != nil {
		return reply{}, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return reply{}, err
	}
	rep := reply{Cols: cols, Rows: [][]value{}}
	dest := make([]any, len(cols))
	for i := range dest {
		dest[i] = new(any)
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			return reply{}, err
		}
		row := make([]value, len(cols))
		for i, d := range dest {
			row[i] = value{*(d.(*any))}
		}
		rep.Rows = append(rep.Rows, row)
	}
	return rep, rows.Err()
}

// close stops serving: connections are closed (what's open on them is
// rolled back), then the file, the socket and the lock.
func (o *owner) close() error {
	o.mu.Lock()
	if o.closed {
		o.mu.Unlock()
		return nil
	}
	o.closed = true
	for nc := range o.conns {
		nc.Close()
	}
	o.mu.Unlock()
	if o.ln != nil {
		o.ln.Close()
	}
	o.wg.Wait()
	return o.shut()
}

// shut closes what Own opened.
func (o *owner) shut() error {
	var err error
	if o.reader != nil {
		o.reader.Close()
	}
	if o.writer != nil {
		err = o.writer.Close()
	}
	if o.ln != nil {
		os.Remove(SocketPath(o.root))
	}
	if o.lock != nil {
		syscall.Flock(int(o.lock.Fd()), syscall.LOCK_UN)
		o.lock.Close()
	}
	return err
}
