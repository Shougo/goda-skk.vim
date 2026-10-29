package skk

import (
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
type UserDictionary struct {
	path string

	mu      sync.Mutex
	j       *jisyo
	modTime time.Time
	logf    func(format string, args ...any)
}

// NewUserDictionary returns the user dictionary at path ("~/" is the home
// directory; "" keeps it in memory only).
func NewUserDictionary(path string) *UserDictionary {
	u := &UserDictionary{path: expandHome(path), j: newJisyo()}
	u.reload()
	return u
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
	u.j = parseJisyo(string(b))
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

// Register puts candidate first among the candidates of word, and writes
// the file.
func (u *UserDictionary) Register(typ HenkanType, word, candidate string) {
	u.reload()
	u.mu.Lock()
	defer u.mu.Unlock()
	t := u.j.table(typ)
	old := t[word]
	cs := append([]string{candidate}, slices.DeleteFunc(slices.Clone(old), func(c string) bool { return c == candidate })...)
	t[word] = cs
	if typ == OkuriNasi && old == nil {
		u.j.index()
	}
	u.save()
}

// Purge removes candidate from the candidates of word, and writes the file.
func (u *UserDictionary) Purge(typ HenkanType, word, candidate string) {
	u.reload()
	u.mu.Lock()
	defer u.mu.Unlock()
	t := u.j.table(typ)
	cs := slices.DeleteFunc(slices.Clone(t[word]), func(c string) bool { return c == candidate })
	if len(cs) == 0 {
		delete(t, word)
		u.j.index()
	} else {
		t[word] = cs
	}
	u.save()
}

// save writes the file (through a temporary file, so that a crash leaves
// the old one), the okuri-ari entries in reverse order, as SKK does.
func (u *UserDictionary) save() {
	if u.path == "" {
		return
	}
	entries := func(m map[string][]string, reverse bool) []string {
		words := make([]string, 0, len(m))
		for w := range m {
			words = append(words, w)
		}
		slices.Sort(words)
		if reverse {
			slices.Reverse(words)
		}
		lines := make([]string, len(words))
		for i, w := range words {
			lines[i] = w + " /" + strings.Join(m[w], "/") + "/"
		}
		return lines
	}
	var b strings.Builder
	b.WriteString(okuriAriMarker + "\n")
	for _, l := range entries(u.j.okuriAri, true) {
		b.WriteString(l + "\n")
	}
	b.WriteString(okuriNasiMarker + "\n")
	for _, l := range entries(u.j.okuriNasi, false) {
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
