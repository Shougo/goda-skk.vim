package engine

import (
	"cmp"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"
)

// UserDictionary is the dictionary the confirmed conversions are learned in:
// a file in the SKK format, in UTF-8. It is read again when the file
// changed (another Vim learned something), and written after every change.
//
// As in other SKK implementations, the lines are in the order the readings
// were learned, the latest first: a reading learned goes up to the top of
// its section. The completion is in this order.
type UserDictionary struct {
	path string

	mu      sync.Mutex
	j       *jisyo
	modTime time.Time
	logf    func(format string, args ...any)
	// ariAt and nasiAt order the readings, the latest learned the largest:
	// those of the file from its lines (the top is 0, the next -1, ...),
	// those learned after from 1.
	ariAt, nasiAt map[string]int64
	next          int64
}

// NewUserDictionary returns the user dictionary at path ("~/" is the home
// directory; "" keeps it in memory only).
func NewUserDictionary(path string) *UserDictionary {
	u := &UserDictionary{path: expandHome(path)}
	u.setJisyo(newJisyo())
	u.reload()
	return u
}

// setJisyo takes the entries of j, ordered by its lines.
func (u *UserDictionary) setJisyo(j *jisyo) {
	at := func(lines []string) map[string]int64 {
		m := make(map[string]int64, len(lines))
		for i, w := range lines {
			m[w] = -int64(i)
		}
		return m
	}
	u.j, u.ariAt, u.nasiAt, u.next = j, at(j.ariLines), at(j.nasiLines), 1
	j.ariLines, j.nasiLines = nil, nil
}

func (u *UserDictionary) at(typ HenkanType) map[string]int64 {
	if typ == OkuriAri {
		return u.ariAt
	}
	return u.nasiAt
}

// reload reads the file again when it changed.
func (u *UserDictionary) reload() {
	if u.path == "" {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	st, err := os.Stat(u.path)
	if err != nil || st.ModTime().Equal(u.modTime) {
		return
	}
	b, err := os.ReadFile(u.path)
	if err != nil {
		return
	}
	u.setJisyo(parseJisyo(string(b)))
	u.modTime = st.ModTime()
}

func (u *UserDictionary) Lookup(_ context.Context, typ HenkanType, word string) ([]string, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.j.lookup(typ, word), nil
}

func (u *UserDictionary) Complete(_ context.Context, prefix string) ([]Entry, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.j.complete(prefix), nil
}

// Register puts candidate first among the candidates of word, in place of
// those of the same word whatever their annotations (see candidateWord),
// moves word to the top, and writes the file.
func (u *UserDictionary) Register(typ HenkanType, word, candidate string) {
	u.reload()
	u.mu.Lock()
	defer u.mu.Unlock()
	u.at(typ)[word] = u.next
	u.next++
	t := u.j.table(typ)
	old := t[word]
	w := candidateWord(normalizeCandidate(candidate))
	cs := append([]string{candidate}, slices.DeleteFunc(slices.Clone(old), func(c string) bool { return candidateWord(c) == w })...)
	t[word] = cs
	if typ == OkuriNasi && old == nil {
		u.j.index()
	}
	u.save()
}

// recency orders the okuri-nasi readings, the latest learned the largest
// (false: not in the dictionary).
func (u *UserDictionary) recency(word string) (int64, bool) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if _, ok := u.j.okuriNasi[word]; !ok {
		return 0, false
	}
	at, ok := u.nasiAt[word]
	return at, ok
}

// Purge removes candidate from the candidates of word, with those of the
// same word whatever their annotations (see candidateWord), and writes the
// file.
func (u *UserDictionary) Purge(typ HenkanType, word, candidate string) {
	u.reload()
	u.mu.Lock()
	defer u.mu.Unlock()
	t := u.j.table(typ)
	w := candidateWord(normalizeCandidate(candidate))
	cs := slices.DeleteFunc(slices.Clone(t[word]), func(c string) bool { return candidateWord(c) == w })
	if len(cs) == 0 {
		delete(t, word)
		delete(u.at(typ), word)
		u.j.index()
	} else {
		t[word] = cs
	}
	u.save()
}

// save writes the file (through a temporary file, so that a crash leaves
// the old one), the latest learned first in each section.
func (u *UserDictionary) save() {
	if u.path == "" {
		return
	}
	entries := func(m map[string][]string, at map[string]int64) []string {
		words := make([]string, 0, len(m))
		for w := range m {
			words = append(words, w)
		}
		// Those not ordered (none but in a test) last, by reading.
		slices.SortFunc(words, func(a, b string) int {
			aa, aok := at[a]
			ba, bok := at[b]
			switch {
			case aok != bok:
				if aok {
					return -1
				}
				return 1
			case aa != ba:
				return cmp.Compare(ba, aa)
			}
			return strings.Compare(a, b)
		})
		lines := make([]string, len(words))
		for i, w := range words {
			lines[i] = w + " /" + strings.Join(m[w], "/") + "/"
		}
		return lines
	}
	var b strings.Builder
	b.WriteString(okuriAriMarker + "\n")
	for _, l := range entries(u.j.okuriAri, u.ariAt) {
		b.WriteString(l + "\n")
	}
	b.WriteString(okuriNasiMarker + "\n")
	for _, l := range entries(u.j.okuriNasi, u.nasiAt) {
		b.WriteString(l + "\n")
	}
	if err := os.MkdirAll(filepath.Dir(u.path), 0o755); err != nil {
		u.log(err)
		return
	}
	tmp := u.path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o600); err != nil {
		u.log(err)
		return
	}
	if err := os.Rename(tmp, u.path); err != nil {
		u.log(err)
		return
	}
	if st, err := os.Stat(u.path); err == nil {
		u.modTime = st.ModTime()
	}
}

func (u *UserDictionary) log(err error) {
	if u.logf != nil {
		u.logf("goda-skk: user dictionary: %v", err)
	}
}

var _ Dictionary = (*UserDictionary)(nil)
