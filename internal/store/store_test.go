package store

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var bg = context.Background()

// own takes a fresh workspace's records for the test, and lets go of them
// when it ends.
func own(t testing.TB) *DB {
	t.Helper()
	root := t.TempDir()
	db, err := Own(root)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// borrow is a command's connection to records the test owns: what Open
// gives a process that isn't the daemon.
func borrow(t testing.TB, db *DB) *DB {
	t.Helper()
	c, err := dial(db.Root())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.write.Close() })
	return c
}

func table(t testing.TB, db *DB) {
	t.Helper()
	if err := db.Write(bg, func(tx *Tx) error {
		_, err := tx.Exec(`CREATE TABLE t (id INTEGER PRIMARY KEY, n INTEGER NOT NULL DEFAULT 0, s TEXT)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
}

func count(t testing.TB, db *DB) int {
	t.Helper()
	var n int
	if err := db.Read(bg, func(tx *Tx) error { return tx.QueryRow(`SELECT count(*) FROM t`).Scan(&n) }); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestOwn_WritesAndReads(t *testing.T) {
	db := own(t)
	table(t, db)
	if err := db.Write(bg, func(tx *Tx) error {
		_, err := tx.Exec(`INSERT INTO t (s) VALUES ('a'), ('b')`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if n := count(t, db); n != 2 {
		t.Fatalf("rows = %d, want 2", n)
	}
	if _, err := os.Stat(Path(db.Root())); err != nil {
		t.Fatalf("the file: %v", err)
	}
}

func TestWrite_AnErrorOrAPanicRollsBack(t *testing.T) {
	db := own(t)
	table(t, db)
	boom := errors.New("boom")
	err := db.Write(bg, func(tx *Tx) error {
		tx.Exec(`INSERT INTO t (s) VALUES ('a')`)
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v", err)
	}
	func() {
		defer func() { recover() }()
		db.Write(bg, func(tx *Tx) error {
			tx.Exec(`INSERT INTO t (s) VALUES ('b')`)
			panic("in the middle")
		})
	}()
	if n := count(t, db); n != 0 {
		t.Fatalf("rows = %d, want none", n)
	}
	// And the turn to write was given back each time.
	ctx, cancel := context.WithTimeout(bg, time.Second)
	defer cancel()
	if err := db.Write(ctx, func(tx *Tx) error { return nil }); err != nil {
		t.Fatalf("a write after: %v", err)
	}
}

func TestOwn_OneProcessAtATime(t *testing.T) {
	db := own(t)
	if _, err := Own(db.Root()); err == nil {
		t.Fatal("owned twice")
	}
	// What a second daemon would hear: who has them.
	if _, err := takeLock(db.Root()); err == nil || !strings.Contains(err.Error(), fmt.Sprintf("pid %d", os.Getpid())) {
		t.Fatalf("err = %v, want one naming this process", err)
	}
	db.Close()
	again, err := Own(db.Root())
	if err != nil {
		t.Fatalf("owned after it was let go: %v", err)
	}
	again.Close()
}

func TestOpen_IsTheOwnersInItsProcess(t *testing.T) {
	db := own(t)
	got, err := Open(db.Root())
	if err != nil {
		t.Fatal(err)
	}
	if got != db || !got.Owned() {
		t.Fatal("Open in the owning process gave something else")
	}
}

func TestOpen_NoDaemonNoStart(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, ".ws"), 0o755)
	_, err := Open(root)
	if !NoServer(err) {
		t.Fatalf("err = %v, want no server", err)
	}
	started := ""
	Start = func(r string) error { started = r; return errors.New("not here") }
	defer func() { Start = nil }()
	if _, err := Open(root); err == nil || started == "" {
		t.Fatalf("Start wasn't asked (err %v)", err)
	}
	if _, err := Dial(root); !NoServer(err) {
		t.Fatalf("Dial: err = %v, want no server without starting", err)
	}
}

func TestChanged_Since(t *testing.T) {
	db := own(t)
	topics, latest, err := db.Since(bg, 0)
	if err != nil || len(topics) != 0 || latest != 0 {
		t.Fatalf("fresh: %v %d %v", topics, latest, err)
	}
	db.Write(bg, func(tx *Tx) error { tx.Changed("workspace", "mission:a", "workspace"); return nil })
	db.Write(bg, func(tx *Tx) error { return nil }) // names nothing: no number
	db.Write(bg, func(tx *Tx) error { tx.Changed("mission:b"); return nil })
	topics, latest, _ = db.Since(bg, 0)
	if !slices.Equal(topics, []string{"mission:a", "mission:b", "workspace"}) || latest != 2 {
		t.Fatalf("since 0: %v, latest %d", topics, latest)
	}
	topics, _, _ = db.Since(bg, 1)
	if !slices.Equal(topics, []string{"mission:b"}) {
		t.Fatalf("since 1: %v", topics)
	}
}

func TestRead_CantNameChanges(t *testing.T) {
	db := own(t)
	defer func() {
		if recover() == nil {
			t.Fatal("Changed in a Read didn't panic")
		}
	}()
	db.Read(bg, func(tx *Tx) error { tx.Changed("x"); return nil })
}

func TestWatch_HearsEveryWrite_ACommandsToo(t *testing.T) {
	db := own(t)
	table(t, db)
	told, stop := db.Watch()
	defer stop()

	c := borrow(t, db)
	if err := c.Write(bg, func(tx *Tx) error {
		_, err := tx.Exec(`INSERT INTO t (s) VALUES ('from a command')`)
		tx.Changed("workspace")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-told:
	case <-time.After(2 * time.Second):
		t.Fatal("not told of a command's write")
	}
	if topics, _, _ := db.Since(bg, 0); !slices.Equal(topics, []string{"workspace"}) {
		t.Fatalf("topics = %v", topics)
	}
}

func TestRemote_WritesAndReadsThroughTheDaemon(t *testing.T) {
	db := own(t)
	table(t, db)
	c := borrow(t, db)
	if c.Owned() {
		t.Fatal("a borrowed connection says it owns the file")
	}
	var id int64
	if err := c.Write(bg, func(tx *Tx) error {
		res, err := tx.Exec(`INSERT INTO t (n, s) VALUES (?, ?)`, 41, "x")
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		_, err = tx.Exec(`UPDATE t SET n = n + 1 WHERE id = ?`, id)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var n int
	var s string
	if err := c.Read(bg, func(tx *Tx) error { return tx.QueryRow(`SELECT n, s FROM t WHERE id = ?`, id).Scan(&n, &s) }); err != nil {
		t.Fatal(err)
	}
	if n != 42 || s != "x" {
		t.Fatalf("read back %d %q", n, s)
	}
	// An error rolls back there as here.
	c.Write(bg, func(tx *Tx) error {
		tx.Exec(`INSERT INTO t (s) VALUES ('gone')`)
		return errors.New("no")
	})
	if got := count(t, db); got != 1 {
		t.Fatalf("rows = %d, want 1", got)
	}
}

func TestRemote_Values(t *testing.T) {
	db := own(t)
	if err := db.Write(bg, func(tx *Tx) error {
		_, err := tx.Exec(`CREATE TABLE v (i INTEGER, f REAL, s TEXT, b BLOB, e BLOB, z TEXT)`)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	c := borrow(t, db)
	big := int64(1<<62 + 3)
	if err := c.Write(bg, func(tx *Tx) error {
		_, err := tx.Exec(`INSERT INTO v VALUES (?, ?, ?, ?, ?, ?)`, big, 1.25, "naïve \"text\"\n", []byte{0, 1, 255}, []byte{}, nil)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	var (
		i     int64
		f     float64
		s     string
		b, e  []byte
		z     sql.NullString
		count int
	)
	if err := c.Read(bg, func(tx *Tx) error {
		if err := tx.QueryRow(`SELECT i, f, s, b, e, z FROM v`).Scan(&i, &f, &s, &b, &e, &z); err != nil {
			return err
		}
		return tx.QueryRow(`SELECT count(*) FROM v WHERE i = ?`, big).Scan(&count)
	}); err != nil {
		t.Fatal(err)
	}
	if i != big || f != 1.25 || s != "naïve \"text\"\n" || !bytes.Equal(b, []byte{0, 1, 255}) || len(e) != 0 || z.Valid || count != 1 {
		t.Fatalf("got %d %v %q %v %v %v %d", i, f, s, b, e, z, count)
	}
}

func TestRemote_ASQLiteErrorKeepsItsCode(t *testing.T) {
	db := own(t)
	table(t, db)
	c := borrow(t, db)
	err := c.Write(bg, func(tx *Tx) error {
		_, err := tx.Exec(`INSERT INTO t (id, s) VALUES (1, 'a'), (1, 'b')`)
		return err
	})
	var se *Error
	if !errors.As(err, &se) || se.Code&0xff != 19 { // SQLITE_CONSTRAINT
		t.Fatalf("err = %#v", err)
	}
}

func TestRemote_ACommandThatGoesMidTransactionIsRolledBack(t *testing.T) {
	db := own(t)
	table(t, db)
	c := borrow(t, db)
	conn, err := c.write.Conn(bg)
	if err != nil {
		t.Fatal(err)
	}
	stx, err := conn.BeginTx(bg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stx.Exec(`INSERT INTO t (s) VALUES ('never')`); err != nil {
		t.Fatal(err)
	}
	// The command goes: its connection closes with the transaction open.
	conn.Raw(func(dc any) error { return dc.(*rconn).nc.Close() })
	ctx, cancel := context.WithTimeout(bg, 3*time.Second)
	defer cancel()
	if err := db.Write(ctx, func(tx *Tx) error {
		_, err := tx.Exec(`INSERT INTO t (s) VALUES ('after')`)
		return err
	}); err != nil {
		t.Fatalf("the next writer: %v", err)
	}
	if n := count(t, db); n != 1 {
		t.Fatalf("rows = %d, want only the next writer's", n)
	}
}

func TestRemote_ATransactionLeftOpenIsRolledBackAfterTheLimit(t *testing.T) {
	old := txLimit
	txLimit = 300 * time.Millisecond
	defer func() { txLimit = old }()
	db := own(t)
	table(t, db)
	c := borrow(t, db)
	err := c.Write(bg, func(tx *Tx) error {
		tx.Exec(`INSERT INTO t (s) VALUES ('slow')`)
		time.Sleep(600 * time.Millisecond) // against the rules: something slow
		return nil
	})
	if err == nil {
		t.Fatal("committed past the limit")
	}
	if n := count(t, db); n != 0 {
		t.Fatalf("rows = %d", n)
	}
	// The pool's let go of the dead connection, and goes on.
	if err := c.Write(bg, func(tx *Tx) error { _, err := tx.Exec(`INSERT INTO t (s) VALUES ('ok')`); return err }); err != nil {
		t.Fatalf("after: %v", err)
	}
}

// A command's commit that's made is answered as made, even when the limit
// comes as it's being answered: the limit can't close the connection on
// an answer, which the command would take for a failure and do again.
func TestRemote_ACommitMadeIsAnsweredAsMade(t *testing.T) {
	old, oldAfter := txLimit, afterCommit
	txLimit = 200 * time.Millisecond
	afterCommit = func() { time.Sleep(400 * time.Millisecond) } // past the limit, answering
	defer func() { txLimit, afterCommit = old, oldAfter }()
	db := own(t)
	table(t, db)
	c := borrow(t, db)
	err := c.Write(bg, func(tx *Tx) error {
		_, err := tx.Exec(`INSERT INTO t (s) VALUES ('made')`)
		return err
	})
	if n := count(t, db); err != nil || n != 1 {
		t.Fatalf("told %v, with %d rows: a commit made is answered as made", err, n)
	}
}

func TestRemote_LayoutsMustAgree(t *testing.T) {
	db := own(t)
	// A command from a ws whose store is a layout ahead of the daemon's.
	nc, err := net.Dial("unix", SocketPath(db.Root()))
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	ahead := len(parts["store"]) + 1
	json.NewEncoder(nc).Encode(request{Op: "hello", Layouts: map[string]int{"store": ahead}})
	var rep reply
	json.NewDecoder(nc).Decode(&rep)
	if !strings.Contains(rep.Error, "ws daemon restart") || rep.Layouts["store"] != ahead-1 {
		t.Fatalf("a newer command against an older daemon: %+v", rep)
	}
	if err := differ(map[string]int{"store": 2}, map[string]int{"store": 1}); err == nil || !strings.Contains(err.Error(), "older than the workspace's daemon") {
		t.Fatalf("an older command: %v", err)
	}
	if err := differ(map[string]int{"store": 1, "space": 1}, map[string]int{"store": 1}); err == nil || !strings.Contains(err.Error(), "doesn't know") {
		t.Fatalf("a part the command doesn't know: %v", err)
	}
	if err := differ(layouts(), layouts()); err != nil {
		t.Fatal(err)
	}
}

func TestOwn_BringsALayoutForward_AndRefusesANewerOne(t *testing.T) {
	db := own(t)
	root := db.Root()
	db.Close()
	parts["extra"] = []Step{SQL(`CREATE TABLE extra (x)`), SQL(`INSERT INTO extra VALUES (1)`)}
	defer delete(parts, "extra")
	db, err := Own(root)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := db.Layouts(bg)
	if got["store"] != len(parts["store"]) || got["extra"] != 2 {
		t.Fatalf("layouts = %v", got)
	}
	db.Close()
	parts["extra"] = parts["extra"][:1]
	if _, err := Own(root); err == nil || !strings.Contains(err.Error(), "newer ws") {
		t.Fatalf("err = %v", err)
	}
}

func TestWrite_TakesTurnsInOrder(t *testing.T) {
	db := own(t)
	table(t, db)
	var mu sync.Mutex
	var order []int
	release, _ := db.own.turn(bg)
	var wg sync.WaitGroup
	for i := range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			db.Write(bg, func(tx *Tx) error {
				mu.Lock()
				order = append(order, i)
				mu.Unlock()
				return nil
			})
		}()
		time.Sleep(20 * time.Millisecond) // so each is queued before the next
	}
	release()
	wg.Wait()
	if !slices.Equal(order, []int{0, 1, 2, 3, 4}) {
		t.Fatalf("order = %v", order)
	}
}

// What the spike measured, through the daemon: commands in processes of
// their own each doing read-then-write transactions on records the test
// owns, a reader looking all the while. None may be lost to another.
func TestProcesses_WriteAtOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("starts processes")
	}
	db := own(t)
	table(t, db)
	db.Write(bg, func(tx *Tx) error { _, err := tx.Exec(`INSERT INTO t (id, n) VALUES (1, 0)`); return err })

	const procs, each = 12, 100
	stop, looked := make(chan struct{}), make(chan struct{})
	looks := 0
	go func() {
		defer close(looked)
		for {
			select {
			case <-stop:
				return
			default:
			}
			count(t, db)
			looks++
		}
	}()
	start := time.Now()
	var wg sync.WaitGroup
	errs := make(chan error, procs)
	for range procs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.Command(os.Args[0], "-test.run=^TestHelperWriter$")
			cmd.Env = append(os.Environ(), "WS_STORE_HELPER_ROOT="+db.Root(), "WS_STORE_HELPER_WRITES="+strconv.Itoa(each))
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("%v: %s", err, out)
			}
		}()
	}
	wg.Wait()
	close(stop)
	<-looked // done with looks
	took := time.Since(start)
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	var n int
	db.Read(bg, func(tx *Tx) error { return tx.QueryRow(`SELECT n FROM t WHERE id = 1`).Scan(&n) })
	if n != procs*each {
		t.Fatalf("count = %d, want %d: writes were lost", n, procs*each)
	}
	t.Logf("%d processes × %d writes in %v (%.0f a second), with %d reads beside them", procs, each, took.Round(time.Millisecond), float64(procs*each)/took.Seconds(), looks)
}

// TestHelperWriter is a command, for TestProcesses_WriteAtOnce: it does
// nothing unless that started it.
func TestHelperWriter(t *testing.T) {
	root := os.Getenv("WS_STORE_HELPER_ROOT")
	if root == "" {
		t.Skip("run by TestProcesses_WriteAtOnce")
	}
	writes, _ := strconv.Atoi(os.Getenv("WS_STORE_HELPER_WRITES"))
	db, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for range writes {
		if err := db.Write(bg, func(tx *Tx) error {
			var n int
			if err := tx.QueryRow(`SELECT n FROM t WHERE id = 1`).Scan(&n); err != nil {
				return err
			}
			_, err := tx.Exec(`UPDATE t SET n = ? WHERE id = 1`, n+1)
			tx.Changed("workspace")
			return err
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func BenchmarkWrite_InTheDaemon(b *testing.B) {
	db := own(b)
	table(b, db)
	b.ResetTimer()
	for range b.N {
		db.Write(bg, func(tx *Tx) error {
			_, err := tx.Exec(`INSERT INTO t (s) VALUES ('x')`)
			tx.Changed("workspace")
			return err
		})
	}
}

func BenchmarkWrite_FromACommand(b *testing.B) {
	db := own(b)
	table(b, db)
	c := borrow(b, db)
	b.ResetTimer()
	for range b.N {
		c.Write(bg, func(tx *Tx) error {
			_, err := tx.Exec(`INSERT INTO t (s) VALUES ('x')`)
			tx.Changed("workspace")
			return err
		})
	}
}

func BenchmarkRead_FromACommand(b *testing.B) {
	db := own(b)
	table(b, db)
	c := borrow(b, db)
	b.ResetTimer()
	for range b.N {
		c.Read(bg, func(tx *Tx) error {
			var n int
			return tx.QueryRow(`SELECT count(*) FROM t`).Scan(&n)
		})
	}
}
