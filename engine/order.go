package engine

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// candidateOrder keeps the candidates pinned (first whatever is learned)
// and demoted (last), by reading: a file of JSON Lines, apart from the user
// dictionary, which other SKK implementations read too.
type candidateOrder struct {
	path string
	logf func(string, ...any)

	mu      sync.Mutex
	entries map[orderKey][]orderEntry
	modTime time.Time
	loaded  bool
}

type orderKey struct {
	typ     HenkanType
	reading string
}

// orderEntry is a line of the file: a word of a reading, pinned or
// demoted. The lines of a reading are in the order they were made, the
// latest first.
type orderEntry struct {
	Type    HenkanType `json:"type"`
	Reading string     `json:"reading"`
	Word    string     `json:"word"`
	Order   string     `json:"order"`
}

const (
	orderPin    = "pin"
	orderDemote = "demote"
)

func newCandidateOrder(path string) *candidateOrder {
	return &candidateOrder{path: path}
}

// load reads the file again when it changed (another Vim).
func (o *candidateOrder) load() {
	path := expandHome(o.path)
	fi, err := os.Stat(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			o.report(err)
		}
		if !o.loaded {
			o.entries, o.loaded = map[orderKey][]orderEntry{}, true
		}
		return
	}
	if o.loaded && fi.ModTime().Equal(o.modTime) {
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		o.report(err)
		return
	}
	entries := map[orderKey][]orderEntry{}
	sc := bufio.NewScanner(bytes.NewReader(b))
	for sc.Scan() {
		var e orderEntry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil || e.Word == "" ||
			(e.Order != orderPin && e.Order != orderDemote) {
			continue
		}
		k := orderKey{e.Type, e.Reading}
		entries[k] = append(entries[k], e)
	}
	o.entries, o.modTime, o.loaded = entries, fi.ModTime(), true
}

func (o *candidateOrder) save() {
	path := expandHome(o.path)
	var buf bytes.Buffer
	keys := make([]orderKey, 0, len(o.entries))
	for k := range o.entries {
		keys = append(keys, k)
	}
	slices.SortFunc(keys, func(a, b orderKey) int {
		if a.typ != b.typ {
			return strings.Compare(string(a.typ), string(b.typ))
		}
		return strings.Compare(a.reading, b.reading)
	})
	for _, k := range keys {
		for _, e := range o.entries[k] {
			b, err := json.Marshal(e)
			if err != nil {
				o.report(err)
				return
			}
			buf.Write(b)
			buf.WriteByte('\n')
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		o.report(err)
		return
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o600); err != nil {
		o.report(err)
		return
	}
	if err := os.Rename(tmp, path); err != nil {
		o.report(err)
		return
	}
	if fi, err := os.Stat(path); err == nil {
		o.modTime = fi.ModTime()
	}
}

func (o *candidateOrder) report(err error) {
	if o.logf != nil {
		o.logf("goda-skk: %v", err)
	}
}

// toggle pins or demotes (order) the word of a reading, the latest first,
// or takes it back when it is already: it returns whether it is now.
func (o *candidateOrder) toggle(typ HenkanType, reading, word, order string) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.load()
	k := orderKey{typ, reading}
	es := o.entries[k]
	i := slices.IndexFunc(es, func(e orderEntry) bool { return e.Word == word })
	was := i >= 0 && es[i].Order == order
	if i >= 0 {
		es = slices.Delete(es, i, i+1)
	}
	if !was {
		es = slices.Insert(es, 0, orderEntry{Type: typ, Reading: reading, Word: word, Order: order})
	}
	if len(es) == 0 {
		delete(o.entries, k)
	} else {
		o.entries[k] = es
	}
	o.save()
	return !was
}

// apply puts the words pinned for the reading first (the latest first),
// and those demoted last (the latest last), the others as they are.
func (o *candidateOrder) apply(typ HenkanType, reading string, cands []string) []string {
	if o == nil || len(cands) < 2 {
		return cands
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.load()
	es := o.entries[orderKey{typ, reading}]
	if len(es) == 0 {
		return cands
	}
	rank := map[string]int{}
	for i, e := range es {
		if e.Order == orderPin {
			rank[e.Word] = -len(es) + i
		} else {
			rank[e.Word] = len(es) - i
		}
	}
	out := slices.Clone(cands)
	slices.SortStableFunc(out, func(a, b string) int {
		return rank[candidateWord(a)] - rank[candidateWord(b)]
	})
	return out
}
