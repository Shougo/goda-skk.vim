package engine

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"
)

// Dictionary gives the candidates of a reading. A dictionary that cannot
// answer (a server that is down) returns an error; the others are still
// used.
type Dictionary interface {
	// Lookup returns the candidates of word ("かんじ", or "おくr" for
	// OkuriAri), each maybe with an annotation after ";".
	Lookup(ctx context.Context, typ HenkanType, word string) ([]string, error)
	// Complete returns the okuri-nasi entries whose reading starts with
	// prefix, for completion.
	Complete(ctx context.Context, prefix string) ([]Entry, error)
}

// Entry is an entry of a dictionary: a reading and its candidates.
type Entry struct {
	Word       string
	Candidates []string
}

// ContextDictionary is a Dictionary that converts better knowing the text
// before the reading (dictionary/karukan): LookupContext is called instead of
// Lookup, with before, the text of the line before the reading (empty on the
// command line of a terminal job). Only a dictionary that keeps the text on
// the machine should be one: the text may be anything in the buffer.
type ContextDictionary interface {
	Dictionary
	LookupContext(ctx context.Context, typ HenkanType, word, before string) ([]string, error)
}

// Initializer is a Dictionary that prepares itself (reads its file) when
// the plugin starts, in the background.
type Initializer interface {
	Init(ctx context.Context) error
}

// NoLearn returns d whose candidates are not learned in the user
// dictionary when they are confirmed, unless a dictionary that is not
// NoLearn gives them too: the phrases converted by dictionary/karukan would
// fill it with entries used once.
func NoLearn(d Dictionary) Dictionary {
	o := wrap(d)
	o.noLearn = true
	return o
}

// Fallback returns d that is looked up only when the dictionaries before
// it, the user dictionary included, give no candidates: dictionary/karukan
// after the dictionary files, for the readings they do not have.
func Fallback(d Dictionary) Dictionary {
	o := wrap(d)
	o.fallback = true
	return o
}

// option is a dictionary with the options of NoLearn and Fallback.
type option struct {
	Dictionary
	noLearn, fallback bool
}

// wrap returns d with its options, NoLearn(Fallback(d)) being one option.
func wrap(d Dictionary) *option {
	if o, ok := d.(*option); ok {
		c := *o
		return &c
	}
	return &option{Dictionary: d}
}

// unwrap returns the dictionary of d and its options: the dictionary of an
// option is what is a ContextDictionary, an Initializer or an io.Closer.
func unwrap(d Dictionary) (Dictionary, option) {
	if o, ok := d.(*option); ok {
		return o.Dictionary, *o
	}
	return d, option{}
}

// library looks the dictionaries up in order, merging their candidates.
type library struct {
	user  *UserDictionary
	dicts []Dictionary
	logf  func(format string, args ...any)
	// maxLearn: a longer reading is not learned (0: no limit).
	maxLearn int

	mu sync.Mutex
	// unlearned are the candidates that only NoLearn dictionaries gave,
	// by reading.
	unlearned map[learnKey]map[string]bool
}

type learnKey struct {
	typ  HenkanType
	word string
}

// maxUnlearned bounds the readings kept in library.unlearned: a candidate
// is confirmed soon after it is looked up.
const maxUnlearned = 256

func (l *library) all() []Dictionary {
	var ds []Dictionary
	if l.user != nil {
		ds = append(ds, l.user)
	}
	return append(ds, l.dicts...)
}

// hasContext reports whether a dictionary is told the text before the
// reading.
func (l *library) hasContext() bool {
	for _, d := range l.all() {
		d, _ := unwrap(d)
		if _, ok := d.(ContextDictionary); ok {
			return true
		}
	}
	return false
}

// remember records the candidates of word that are not learned: in place
// of those recorded before (a lookup), or with them (a completion, which
// sees part of the dictionaries).
func (l *library) remember(typ HenkanType, word string, cands map[string]bool, replace bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	k := learnKey{typ, word}
	if len(cands) == 0 {
		if replace {
			delete(l.unlearned, k)
		}
		return
	}
	if l.unlearned == nil || len(l.unlearned) >= maxUnlearned {
		l.unlearned = map[learnKey]map[string]bool{}
	}
	if old := l.unlearned[k]; !replace && old != nil {
		for c := range cands {
			old[c] = true
		}
		return
	}
	l.unlearned[k] = cands
}

// isUnlearned reports whether candidate of word is not learned.
func (l *library) isUnlearned(typ HenkanType, word, candidate string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.unlearned[learnKey{typ, word}][candidate]
}

// lookup returns the candidates of word from every dictionary, each once,
// with readings of numbers converted (see numberWord). before is the text
// before the reading, for a ContextDictionary ("" when unknown).
func (l *library) lookup(ctx context.Context, typ HenkanType, word, before string) []string {
	if l.user != nil {
		l.user.reload()
	}
	var out []string
	seen := map[string]bool{}
	// learned: the candidates a dictionary that is not NoLearn gave.
	learned := map[string]bool{}
	add := func(cs []string, noLearn bool) {
		for _, c := range cs {
			// As the user dictionary writes it: the same candidate of
			// another dictionary is the same.
			c = normalizeCandidate(c)
			if !noLearn {
				learned[c] = true
			}
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	pattern := numberWord(word)
	for _, d := range l.all() {
		d, opt := unwrap(d)
		if opt.fallback && len(out) > 0 {
			continue
		}
		find := d.Lookup
		if cd, ok := d.(ContextDictionary); ok {
			find = func(ctx context.Context, typ HenkanType, word string) ([]string, error) {
				return cd.LookupContext(ctx, typ, word, before)
			}
		}
		if pattern != word {
			// The entries of the digits as typed come first, then those of
			// "#", converted.
			cs, err := find(ctx, typ, word)
			l.report(err)
			ps, err := find(ctx, typ, pattern)
			l.report(err)
			for _, p := range ps {
				cs = append(cs, convertNumber(p, word))
			}
			add(cs, opt.noLearn)
			continue
		}
		cs, err := find(ctx, typ, word)
		l.report(err)
		add(cs, opt.noLearn)
	}
	var unlearned map[string]bool
	for _, c := range out {
		if !learned[c] {
			if unlearned == nil {
				unlearned = map[string]bool{}
			}
			unlearned[c] = true
		}
	}
	l.remember(typ, word, unlearned, true)
	return out
}

// complete returns the entries whose reading starts with one of prefixes,
// merged by reading.
func (l *library) complete(ctx context.Context, prefixes []string) []Entry {
	if l.user != nil {
		l.user.reload()
	}
	byWord := map[string]int{}
	var out []Entry
	// learned: the candidates, by reading, a dictionary that is not
	// NoLearn gave.
	learned := map[[2]string]bool{}
	for _, d := range l.all() {
		d, opt := unwrap(d)
		if opt.fallback && len(out) > 0 {
			continue
		}
		for _, p := range prefixes {
			es, err := d.Complete(ctx, p)
			l.report(err)
			for _, e := range es {
				i, ok := byWord[e.Word]
				if !ok {
					byWord[e.Word] = len(out)
					out = append(out, Entry{Word: e.Word})
					i = len(out) - 1
				}
				for _, c := range e.Candidates {
					c = normalizeCandidate(c)
					if !opt.noLearn {
						learned[[2]string{e.Word, c}] = true
					}
					if !slices.Contains(out[i].Candidates, c) {
						out[i].Candidates = append(out[i].Candidates, c)
					}
				}
			}
		}
	}
	for _, e := range out {
		var unlearned map[string]bool
		for _, c := range e.Candidates {
			if !learned[[2]string{e.Word, c}] {
				if unlearned == nil {
					unlearned = map[string]bool{}
				}
				unlearned[c] = true
			}
		}
		l.remember(OkuriNasi, e.Word, unlearned, false)
	}
	slices.SortStableFunc(out, func(a, b Entry) int { return strings.Compare(a.Word, b.Word) })
	return out
}

func (l *library) report(err error) {
	if err != nil && l.logf != nil {
		l.logf("goda-skk: %v", err)
	}
}

// learn puts candidate first for word in the user dictionary, unless word
// is longer than maxLearn or only NoLearn dictionaries gave candidate.
func (l *library) learn(typ HenkanType, word, candidate string) {
	if l.maxLearn > 0 && utf8.RuneCountInString(word) > l.maxLearn {
		return
	}
	if l.isUnlearned(typ, word, normalizeCandidate(candidate)) {
		return
	}
	l.register(typ, word, candidate)
}

// register puts candidate first for word in the user dictionary, whatever
// the length of word: a word registered is asked for.
func (l *library) register(typ HenkanType, word, candidate string) {
	if l.user != nil && candidate != "" {
		l.user.Register(typ, word, normalizeCandidate(candidate))
	}
}

func (l *library) purge(typ HenkanType, word, candidate string) {
	if l.user != nil {
		l.user.Purge(typ, word, candidate)
	}
}

var digits = regexp.MustCompile(`[0-9]+`)

// numberWord is word with its numbers replaced by "#", the reading of the
// numeric entries of SKK dictionaries ("#ねん /#1年/#3年/").
func numberWord(word string) string { return digits.ReplaceAllString(word, "#") }

var numberField = regexp.MustCompile(`#[0-9]?`)

// convertNumber fills the numbers of word into the candidate pattern of a
// numeric entry: "#0" (and "#") as typed, "#1" full-width, "#2" kanji digit
// by digit, "#3" kanji numerals, "#5" daiji, "#8" Roman, "#9" shogi.
func convertNumber(pattern, word string) string {
	nums := digits.FindAllString(word, -1)
	i := 0
	return numberField.ReplaceAllStringFunc(pattern, func(f string) string {
		if i >= len(nums) {
			return f
		}
		n := nums[i]
		i++
		v, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			return n
		}
		switch f {
		case "#1":
			return zenkakuDigits(n)
		case "#2":
			return kanjiDigits(n)
		case "#3":
			return kanjiNumber(v)
		case "#5":
			return daiji(v)
		case "#8":
			return roman(v)
		case "#9":
			return kifu(v, n)
		}
		return n
	})
}

func zenkakuDigits(n string) string {
	var b strings.Builder
	for _, r := range n {
		b.WriteRune('０' + (r - '0'))
	}
	return b.String()
}

var kanjiDigit = []string{"〇", "一", "二", "三", "四", "五", "六", "七", "八", "九"}

func kanjiDigits(n string) string {
	var b strings.Builder
	for _, r := range n {
		b.WriteString(kanjiDigit[r-'0'])
	}
	return b.String()
}

// kanjiNumber writes v in kanji numerals: 1234 is 千二百三十四, 10000 is
// 一万.
func kanjiNumber(v uint64) string {
	if v == 0 {
		return "〇"
	}
	units := []string{"", "万", "億", "兆", "京"}
	var parts []string
	for u := 0; v > 0 && u < len(units); u++ {
		g := v % 10000
		v /= 10000
		if g == 0 {
			continue
		}
		parts = append([]string{under10000(g) + units[u]}, parts...)
	}
	return strings.Join(parts, "")
}

func under10000(g uint64) string {
	var b strings.Builder
	for _, p := range []struct {
		div  uint64
		name string
	}{{1000, "千"}, {100, "百"}, {10, "十"}} {
		d := g / p.div
		g %= p.div
		if d == 0 {
			continue
		}
		if d > 1 {
			b.WriteString(kanjiDigit[d])
		}
		b.WriteString(p.name)
	}
	if g > 0 {
		b.WriteString(kanjiDigit[g])
	}
	return b.String()
}

// daiji writes v in the numerals of legal documents (壱, 弐, 拾...).
func daiji(v uint64) string {
	r := strings.NewReplacer("一", "壱", "二", "弐", "三", "参", "四", "肆", "五", "伍",
		"六", "陸", "七", "漆", "八", "捌", "九", "玖", "〇", "零", "十", "拾", "百", "佰",
		"千", "阡", "万", "萬")
	return r.Replace(kanjiNumber(v))
}

func roman(v uint64) string {
	if v == 0 || v >= 4000 {
		return strconv.FormatUint(v, 10)
	}
	vals := []uint64{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}
	var b strings.Builder
	for i, n := range vals {
		for v >= n {
			b.WriteString(syms[i])
			v -= n
		}
	}
	return b.String()
}

// kifu writes a square of shogi: 34 is ３四.
func kifu(v uint64, n string) string {
	x, y := v/10, v%10
	if 0 < x && x < 10 && 0 < y && y < 10 {
		return zenkakuDigits(strconv.FormatUint(x, 10)) + kanjiDigit[y]
	}
	return n
}
