package engine

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
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
		cands := splitCandidates(rest, ari)
		if prev, ok := cur[word]; ok {
			// A reading on two lines (がいひ of SKK-JISYO.L): the
			// candidates of both, the first line's first (#48 of
			// skkeleton.nvim).
			for _, c := range cands {
				if !slices.Contains(prev, c) {
					prev = append(prev, c)
				}
			}
			cands = prev
		}
		cur[word] = cands
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

// A candidate with "/" (which separates the candidates) is written
// (concat "a\057b") in a dictionary: Emacs Lisp, of which SKK knows only
// this (#37 of skkeleton.nvim).
const concatPrefix = `(concat "`

// CandidateWord is the word of a candidate of a dictionary and its
// annotation (after ";"), each without (concat "...").
func CandidateWord(c string) (word, note string) {
	word, note, _ = strings.Cut(c, ";")
	return decodeConcat(word), decodeConcat(note)
}

// decodeConcat is s, or the string of (concat "..." ...) with its escapes
// ("\057", "\"", "\\").
func decodeConcat(s string) string {
	if !strings.HasPrefix(s, concatPrefix) || !strings.HasSuffix(s, ")") {
		return s
	}
	rest := strings.TrimSpace(s[len("(concat") : len(s)-1])
	var b strings.Builder
	for rest != "" {
		if rest[0] != '"' {
			return s
		}
		i := 1
		for ; i < len(rest) && rest[i] != '"'; i++ {
			if rest[i] != '\\' || i+1 >= len(rest) {
				b.WriteByte(rest[i])
				continue
			}
			i++
			if n := octal(rest[i:]); n > 0 {
				v, _ := strconv.ParseUint(rest[i:i+n], 8, 8)
				b.WriteByte(byte(v))
				i += n - 1
				continue
			}
			switch rest[i] {
			case 'n':
				b.WriteByte('\n')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteByte(rest[i])
			}
		}
		if i >= len(rest) {
			return s
		}
		rest = strings.TrimSpace(rest[i+1:])
	}
	return b.String()
}

// octal is the number of octal digits (3 at most) s starts with.
func octal(s string) int {
	n := 0
	for n < len(s) && n < 3 && s[n] >= '0' && s[n] <= '7' {
		n++
	}
	return n
}

// encodeConcat is s written as (concat "...") when it has to be: with "/",
// or looking like one.
func encodeConcat(s string) string {
	if !strings.Contains(s, "/") && !strings.HasPrefix(s, concatPrefix) {
		return s
	}
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "/", `\057`, ";", `\073`)
	return concatPrefix + r.Replace(s) + `")`
}

// normalizeCandidate is a candidate as a dictionary writes it: its word and
// annotation with "/" in (concat "..."). (An okuri block of a user
// dictionary, "[る/走/]", is as it is.)
func normalizeCandidate(c string) string {
	if strings.HasPrefix(c, "[") && strings.HasSuffix(c, "]") {
		return c
	}
	word, note, ok := strings.Cut(c, ";")
	if !ok {
		return encodeField(word)
	}
	return encodeField(word) + ";" + encodeField(note)
}

// encodeField is a word or an annotation as a dictionary writes it.
func encodeField(s string) string {
	if strings.HasPrefix(s, concatPrefix) && decodeConcat(s) != s {
		// Written so already.
		return s
	}
	return encodeConcat(s)
}
