package store

import (
	"bufio"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"syscall"
	"time"
)

// A command's records are a database/sql pool whose connections are to
// the daemon: each its own connection to store.sock, holding at most one
// transaction there. database/sql does the rest (scanning, pooling), so
// what reads and writes records is the same code in a command as in the
// daemon.

const remoteDriver = "ws-store"

func init() { sql.Register(remoteDriver, remote{}) }

// dial is the records of a workspace whose daemon is serving them: a
// pool of connections to it, one tried now so that a daemon that isn't
// there, or doesn't speak this ws's layouts, is said at once.
func dial(root string) (*DB, error) {
	pool, err := sql.Open(remoteDriver, SocketPath(root))
	if err != nil {
		return nil, err
	}
	pool.SetMaxOpenConns(4)
	pool.SetMaxIdleConns(2)
	// An idle connection holds the daemon up: let it go before long.
	pool.SetConnMaxIdleTime(10 * time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := pool.PingContext(ctx); err != nil {
		pool.Close()
		if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) {
			return nil, fmt.Errorf("can't reach the workspace's daemon on %s (%w): a command in a sandbox can't, so run ws outside it", SocketPath(root), err)
		}
		return nil, err
	}
	return &DB{root: root, write: pool, read: pool}, nil
}

type remote struct{}

func (remote) Open(sock string) (driver.Conn, error) {
	nc, err := net.DialTimeout("unix", sock, 2*time.Second)
	if err != nil {
		return nil, err
	}
	c := &rconn{nc: nc, enc: json.NewEncoder(nc), dec: json.NewDecoder(bufio.NewReader(nc))}
	if _, err := c.call(request{Op: "hello", Layouts: layouts()}); err != nil {
		nc.Close()
		return nil, err
	}
	return c, nil
}

// rconn is one connection to the daemon's records. database/sql uses a
// connection from one goroutine at a time, so a call is a request and
// then its reply.
type rconn struct {
	nc   net.Conn
	enc  *json.Encoder
	dec  *json.Decoder
	dead bool // the connection failed: database/sql is to drop it
}

// Error is an error the daemon gave for a statement: SQLite's code for it,
// and what it said.
type Error struct {
	Code int
	Msg  string
}

func (e *Error) Error() string { return e.Msg }

func (c *rconn) call(req request) (reply, error) {
	if c.dead {
		return reply{}, driver.ErrBadConn
	}
	// A write's begin waits for the turn, which a stuck command can hold
	// for txLimit; past twice that, the daemon's gone quiet.
	c.nc.SetDeadline(time.Now().Add(2*txLimit + 10*time.Second))
	if err := c.enc.Encode(req); err != nil {
		c.dead = true
		return reply{}, driver.ErrBadConn // not sent: nothing was done
	}
	var rep reply
	if err := c.dec.Decode(&rep); err != nil {
		c.dead = true
		if errors.Is(err, io.EOF) {
			return reply{}, fmt.Errorf("the workspace's daemon closed the connection (a transaction open past %s is rolled back): %w", txLimit, err)
		}
		return reply{}, err
	}
	if rep.Error != "" {
		return rep, &Error{Code: rep.Code, Msg: rep.Error}
	}
	return rep, nil
}

func (c *rconn) Prepare(query string) (driver.Stmt, error) { return &rstmt{c: c, query: query}, nil }

func (c *rconn) Close() error { return c.nc.Close() }

func (c *rconn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}

func (c *rconn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if _, err := c.call(request{Op: "begin", Write: !opts.ReadOnly}); err != nil {
		return nil, err
	}
	return rtx{c}, nil
}

func (c *rconn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	rep, err := c.call(request{Op: "exec", SQL: query, Args: values(args)})
	if err != nil {
		return nil, err
	}
	return result{rep.Last, rep.Affected}, nil
}

func (c *rconn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	rep, err := c.call(request{Op: "query", SQL: query, Args: values(args)})
	if err != nil {
		return nil, err
	}
	return &rrows{cols: rep.Cols, rows: rep.Rows}, nil
}

// IsValid and ResetSession tell database/sql to drop a connection that's
// failed, rather than hand it out again.
func (c *rconn) IsValid() bool { return !c.dead }

func (c *rconn) ResetSession(ctx context.Context) error {
	if c.dead {
		return driver.ErrBadConn
	}
	return nil
}

func values(args []driver.NamedValue) []value {
	out := make([]value, len(args))
	for i, a := range args {
		out[i] = value{a.Value}
	}
	return out
}

type rtx struct{ c *rconn }

func (t rtx) Commit() error {
	_, err := t.c.call(request{Op: "commit"})
	return err
}

func (t rtx) Rollback() error {
	_, err := t.c.call(request{Op: "rollback"})
	return err
}

// rstmt is a statement that's sent whole each time it's run: the daemon
// prepares it there.
type rstmt struct {
	c     *rconn
	query string
}

func (s *rstmt) Close() error  { return nil }
func (s *rstmt) NumInput() int { return -1 }

func (s *rstmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.c.ExecContext(context.Background(), s.query, named(args))
}

func (s *rstmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.c.QueryContext(context.Background(), s.query, named(args))
}

func named(args []driver.Value) []driver.NamedValue {
	out := make([]driver.NamedValue, len(args))
	for i, a := range args {
		out[i] = driver.NamedValue{Ordinal: i + 1, Value: a}
	}
	return out
}

type result struct{ last, affected int64 }

func (r result) LastInsertId() (int64, error) { return r.last, nil }
func (r result) RowsAffected() (int64, error) { return r.affected, nil }

// rrows is a statement's rows, all of them, as the daemon sent them.
type rrows struct {
	cols []string
	rows [][]value
	i    int
}

func (r *rrows) Columns() []string { return r.cols }
func (r *rrows) Close() error      { return nil }

func (r *rrows) Next(dest []driver.Value) error {
	if r.i >= len(r.rows) {
		return io.EOF
	}
	for j, v := range r.rows[r.i] {
		dest[j] = v.v
	}
	r.i++
	return nil
}
