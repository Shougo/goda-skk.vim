package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"golang.org/x/text/encoding/japanese"
)

// The markers of the two parts of an SKK dictionary.
const (
	okuriAriMarker  = ";; okuri-ari entries."
	okuriNasiMarker = ";; okuri-nasi entries."
)

// jisyo is the contents of a dictionary in the SKK format.
type jisyo struct {
	okuriAri  map[string][]string
	okuriNasi map[string][]string
	// words are the okuri-nasi readings, sorted, for completion.
	words []string
}

func newJisyo() *jisyo {
	return &jisyo{okuriAri: map[string][]string{}, okuriNasi: map[string][]string{}}
}

func (j *jisyo) table(typ HenkanType) map[string][]string {
	if typ == OkuriAri {
		return j.okuriAri
	}
	return j.okuriNasi
}

// parseJisyo reads a dictionary in the SKK format: "reading /c1/c2;note/",
// the okuri-ari entries after their marker, then the okuri-nasi ones.
func parseJisyo(text string) *jisyo {
	j := newJisyo()
	var cur map[string][]string
	ari := false
	for line := range strings.SplitSeq(text, "\n") {
		line = strings.TrimSuffix(line, "\r")
		switch line {
		case okuriAriMarker:
			cur, ari = j.okuriAri, true
			continue
		case okuriNasiMarker:
			cur, ari = j.okuriNasi, false
			continue
		}
		if cur == nil || strings.HasPrefix(line, ";") {
			continue
		}
		word, rest, ok := strings.Cut(line, " ")
		if !ok || !strings.HasPrefix(rest, "/") {
			continue
		}
		cur[word] = splitCandidates(rest, ari)
	}
	j.index()
	return j
}

// splitCandidates splits "/c1/c2/" into its candidates. In an okuri-ari
// entry, a block of candidates for an okurigana ("[る/走/]", written by
// other SKK implementations) is one item.
func splitCandidates(s string, okuriAri bool) []string {
	s = strings.TrimPrefix(s, "/")
	var out []string
	for s != "" {
		if okuriAri && s[0] == '[' && okuriBlock.MatchString(s) {
			if i := strings.Index(s, "]"); i >= 0 {
				out = append(out, s[:i+1])
				s = strings.TrimPrefix(s[i+1:], "/")
				continue
			}
		}
		c, rest, _ := strings.Cut(s, "/")
		if c != "" {
			out = append(out, c)
		}
		s = rest
	}
	return out
}

var okuriBlock = regexp.MustCompile(`^\[[^/\]]+/`)

func (j *jisyo) index() {
	j.words = j.words[:0]
	for w := range j.okuriNasi {
		j.words = append(j.words, w)
	}
	slices.Sort(j.words)
}

func (j *jisyo) lookup(typ HenkanType, word string) []string {
	return candidates(j.table(typ)[word])
}

// candidates are the items of an entry without its okuri blocks (those of
// a user dictionary), which are not candidates.
func candidates(items []string) []string {
	var out []string
	for _, c := range items {
		if !(strings.HasPrefix(c, "[") && strings.Contains(c, "/")) {
			out = append(out, c)
		}
	}
	return out
}

// Candidates are the candidates of an entry of a dictionary in the SKK
// format: "/c1/c2/" (the line without its reading), of a reading of typ. For
// dictionaries that get entries elsewhere (an SKK server).
func Candidates(entry string, typ HenkanType) []string {
	return candidates(splitCandidates(entry, typ == OkuriAri))
}

func (j *jisyo) complete(prefix string) []Entry {
	if prefix == "" {
		return nil
	}
	i, _ := slices.BinarySearch(j.words, prefix)
	var out []Entry
	for ; i < len(j.words) && strings.HasPrefix(j.words[i], prefix); i++ {
		w := j.words[i]
		out = append(out, Entry{Word: w, Candidates: j.lookup(OkuriNasi, w)})
	}
	return out
}

// decode converts the bytes of a dictionary to text: from EUC-JP or UTF-8
// as encoding says, or, without one, UTF-8 when they are valid UTF-8 and
// EUC-JP otherwise.
func decode(b []byte, encoding string) (string, error) {
	switch strings.ToLower(strings.ReplaceAll(encoding, "_", "-")) {
	case "utf-8", "utf8":
		return string(b), nil
	case "euc-jp", "eucjp":
	case "":
		if utf8.Valid(b) {
			return string(b), nil
		}
	default:
		return "", fmt.Errorf("unsupported encoding %q (euc-jp or utf-8)", encoding)
	}
	out, err := japanese.EUCJP.NewDecoder().Bytes(b)
	if err != nil {
		return "", err
	}
	// The wave dash of JIS X 0208 is decoded as the full-width tilde
	// (U+FF5E); SKK means the wave dash (U+301C), which is what is typed
	// ("z-").
	return string(bytes.ReplaceAll(out, []byte("～"), []byte("〜"))), nil
}

// Jisyo is a dictionary file in the SKK format (SKK-JISYO.L, say), read
// once, in the background when the plugin starts.
type Jisyo struct {
	path     string
	encoding string

	once sync.Once
	j    *jisyo
	err  error
}

// NewJisyo returns the dictionary file at path ("~/" is the home directory).
// encoding is "euc-jp", "utf-8" or "" (detected).
func NewJisyo(path, encoding string) *Jisyo {
	return &Jisyo{path: path, encoding: encoding}
}

// Init reads the file.
func (d *Jisyo) Init(context.Context) error {
	d.once.Do(func() {
		b, err := os.ReadFile(expandHome(d.path))
		if err != nil {
			d.err = err
			return
		}
		text, err := decode(b, d.encoding)
		if err != nil {
			d.err = fmt.Errorf("%s: %w", d.path, err)
			return
		}
		d.j = parseJisyo(text)
	})
	return d.err
}

func (d *Jisyo) Lookup(ctx context.Context, typ HenkanType, word string) ([]string, error) {
	if err := d.Init(ctx); err != nil {
		return nil, err
	}
	return d.j.lookup(typ, word), nil
}

func (d *Jisyo) Complete(ctx context.Context, prefix string) ([]Entry, error) {
	if err := d.Init(ctx); err != nil {
		return nil, err
	}
	return d.j.complete(prefix), nil
}

// expandHome expands "~" at the head of path.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			return home + path[1:]
		}
	}
	return path
}

var (
	_ Dictionary  = (*Jisyo)(nil)
	_ Initializer = (*Jisyo)(nil)
)
