package nav

import (
	"bytes"
	"compress/gzip"
	"container/list"
	"encoding/gob"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// lru is a map that forgets what was least lately used past its size.
type lru[V any] struct {
	mu    sync.Mutex
	max   int
	order *list.List // of *entry[V], latest first
	at    map[string]*list.Element
}

type entry[V any] struct {
	key string
	val V
}

func newLRU[V any](max int) *lru[V] {
	return &lru[V]{max: max, order: list.New(), at: map[string]*list.Element{}}
}

func (c *lru[V]) get(key string) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.at[key]; ok {
		c.order.MoveToFront(e)
		return e.Value.(*entry[V]).val, true
	}
	var zero V
	return zero, false
}

func (c *lru[V]) put(key string, val V) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.at[key]; ok {
		e.Value.(*entry[V]).val = val
		c.order.MoveToFront(e)
		return
	}
	c.at[key] = c.order.PushFront(&entry[V]{key, val})
	for c.order.Len() > c.max {
		last := c.order.Back()
		c.order.Remove(last)
		delete(c.at, last.Value.(*entry[V]).key)
	}
}

// group makes one call of a thing at a time: those who ask while it's
// running wait for its answer.
type group[V any] struct {
	mu      sync.Mutex
	running map[string]*call[V]
}

type call[V any] struct {
	done chan struct{}
	val  V
	err  error
}

func (g *group[V]) do(key string, fn func() (V, error)) (V, error) {
	g.mu.Lock()
	if g.running == nil {
		g.running = map[string]*call[V]{}
	}
	if c, ok := g.running[key]; ok {
		g.mu.Unlock()
		<-c.done
		return c.val, c.err
	}
	c := &call[V]{done: make(chan struct{})}
	g.running[key] = c
	g.mu.Unlock()
	c.val, c.err = fn()
	close(c.done)
	g.mu.Lock()
	delete(g.running, key)
	g.mu.Unlock()
	return c.val, c.err
}

// disk keeps facts on disk, by blob: a blob is the same file in any repo
// and at any commit, so it's parsed once.
type disk struct{ dir string }

func (d disk) path(blob, grammar string) string {
	return filepath.Join(d.dir, version, blob[:2], blob+"-"+grammar)
}

func (d disk) get(blob, grammar string) (*Facts, bool) {
	if d.dir == "" || len(blob) < 3 {
		return nil, false
	}
	p := d.path(blob, grammar)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, false
	}
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, false
	}
	var f Facts
	if err := gob.NewDecoder(zr).Decode(&f); err != nil {
		return nil, false
	}
	f.index()
	now := time.Now()
	_ = os.Chtimes(p, now, now) // lately read, so it's kept
	return &f, true
}

func (d disk) put(blob, grammar string, f *Facts) {
	if d.dir == "" || len(blob) < 3 {
		return
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := gob.NewEncoder(zw).Encode(f); err != nil {
		return
	}
	if zw.Close() != nil {
		return
	}
	p := d.path(blob, grammar)
	if os.MkdirAll(filepath.Dir(p), 0o755) != nil {
		return
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), ".facts-*")
	if err != nil {
		return
	}
	_, werr := tmp.Write(buf.Bytes())
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		os.Remove(tmp.Name())
		return
	}
	if os.Rename(tmp.Name(), p) != nil {
		os.Remove(tmp.Name())
	}
}

// prune keeps the cache under max bytes: facts of older versions go, then
// the least lately read.
func (d disk) prune(max int64) {
	if d.dir == "" {
		return
	}
	entries, _ := os.ReadDir(d.dir)
	for _, e := range entries {
		if e.IsDir() && e.Name() != version {
			os.RemoveAll(filepath.Join(d.dir, e.Name()))
		}
	}
	type file struct {
		path string
		size int64
		at   time.Time
	}
	var files []file
	var total int64
	_ = filepath.WalkDir(filepath.Join(d.dir, version), func(p string, e fs.DirEntry, err error) error { // its walk passes over what it can't read
		if err != nil || e.IsDir() {
			return nil
		}
		info, err := e.Info()
		if err != nil {
			return nil
		}
		files = append(files, file{p, info.Size(), info.ModTime()})
		total += info.Size()
		return nil
	})
	if total <= max {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].at.Before(files[j].at) })
	for _, f := range files {
		if total <= max {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}
