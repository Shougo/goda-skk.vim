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

// SureDictionary is a dictionary that says, with its candidates, whether it
// is sure of the first: wrapped with Prefer, LookupSure is called instead of
// the other lookups (okuri is the okurigana typed of a reading with
// okurigana, "" for one without), and its first candidate comes first only
// when it is sure (dictionary/karukan: by the scores of the model).
type SureDictionary interface {
	Dictionary
	LookupSure(ctx context.Context, typ HenkanType, word, okuri, before string) (candidates []string, sure bool, err error)
}

// OkuriDictionary is a ContextDictionary that also converts a reading with
// okurigana knowing the okurigana typed ("い" of "おおi"): LookupOkuri is
// called instead of LookupContext for one, with okuri.
type OkuriDictionary interface {
	ContextDictionary
	LookupOkuri(ctx context.Context, word, okuri, before string) ([]string, error)
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

// Prefer returns d whose first candidate comes first, before those of the
// dictionaries before it and the order they learned, when the text before
// the reading is known: dictionary/karukan tells the words of a reading by
// it ("汽車" after "駅のホームに", "掻" of "かk" after "恥を"), where the user
// dictionary puts the one confirmed last. Without the text, the order is
// theirs, and so it is when d, a SureDictionary, is not sure of its first.
// Fallback(d) is looked up only when they give no candidates, so it never
// comes first.
func Prefer(d Dictionary) Dictionary {
	o := wrap(d)
	o.prefer = true
	return o
}

// option is a dictionary with the options of NoLearn, Fallback and Prefer.
type option struct {
	Dictionary
	noLearn, fallback, prefer bool
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

// blockDictionary is a dictionary in the SKK format, whose entries with
// okurigana may have okuri blocks: lookupOkuri is called for a reading with
// okurigana, with the okurigana typed.
type blockDictionary interface {
	lookupOkuri(ctx context.Context, word, okuri string) ([]string, error)
}

// library looks the dictionaries up in order, merging their candidates.
type library struct {
	user  *UserDictionary
	dicts []Dictionary
	logf  func(format string, args ...any)
	// maxLearn: a longer reading is not learned (0: no limit).
	maxLearn int

	mu sync.Mutex
	// unlearned are the words of the candidates (candidateWord) that only
	// NoLearn dictionaries gave, by reading.
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

// isUnlearned reports whether the candidate of word whose word (see
// candidateWord) is candidate is not learned.
func (l *library) isUnlearned(typ HenkanType, word, candidate string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.unlearned[learnKey{typ, word}][candidate]
}

// lookup returns the candidates of word from every dictionary, each word
// once (the first, whatever the annotations of the others), with readings
// of numbers converted (see numberWord). before gives the
// text before the reading, for a ContextDictionary (nil: unknown); it is
// called only when one is looked up, once.
func (l *library) lookup(ctx context.Context, typ HenkanType, word string, before func() string) []string {
	return l.lookupOkuri(ctx, typ, word, "", before)
}

// lookupOkuri is lookup with the okurigana typed of a reading with
// okurigana, for an OkuriDictionary ("": unknown).
func (l *library) lookupOkuri(ctx context.Context, typ HenkanType, word, okuri string, before func() string) []string {
	if l.user != nil {
		l.user.reload()
	}
	var out []string
	// seen: the index in out of each word.
	seen := map[string]int{}
	// learned: the words a dictionary that is not NoLearn gave.
	learned := map[string]bool{}
	add := func(cs []string, noLearn bool) {
		for _, c := range cs {
			// As the user dictionary writes it: the same candidate of
			// another dictionary is the same.
			c = normalizeCandidate(c)
			w := candidateWord(c)
			if !noLearn {
				learned[w] = true
			}
			if i, ok := seen[w]; !ok {
				seen[w] = len(out)
				out = append(out, c)
			} else {
				out[i] = annotated(out[i], c)
			}
		}
	}
	// The text before the reading may ask Vim (getbufline()): not for a
	// conversion that looks up no ContextDictionary (engine.Fallback).
	var text *string
	beforeText := func() string {
		if text == nil {
			s := ""
			if before != nil {
				s = before()
			}
			text = &s
		}
		return *text
	}
	// preferred: the first word of a Prefer dictionary, with the text.
	preferred := ""
	prefer := func(cs []string, opt option, sure bool) {
		if opt.prefer && sure && preferred == "" && len(cs) > 0 && beforeText() != "" {
			preferred = candidateWord(normalizeCandidate(cs[0]))
		}
	}
	pattern := numberWord(word)
	for _, d := range l.all() {
		d, opt := unwrap(d)
		if opt.fallback && len(out) > 0 {
			continue
		}
		find := d.Lookup
		// sure: the dictionary is sure of its first candidate for word.
		sure := true
		if sd, ok := d.(SureDictionary); ok && opt.prefer {
			find = func(ctx context.Context, typ HenkanType, w string) ([]string, error) {
				o := ""
				if typ == OkuriAri {
					o = okuri
				}
				cs, s, err := sd.LookupSure(ctx, typ, w, o, beforeText())
				if w == word {
					sure = s
				}
				return cs, err
			}
		} else if od, ok := d.(OkuriDictionary); ok && typ == OkuriAri && okuri != "" {
			find = func(ctx context.Context, _ HenkanType, word string) ([]string, error) {
				return od.LookupOkuri(ctx, word, okuri, beforeText())
			}
		} else if bd, ok := d.(blockDictionary); ok && typ == OkuriAri && okuri != "" {
			find = func(ctx context.Context, _ HenkanType, word string) ([]string, error) {
				return bd.lookupOkuri(ctx, word, okuri)
			}
		} else if cd, ok := d.(ContextDictionary); ok {
			find = func(ctx context.Context, typ HenkanType, word string) ([]string, error) {
				return cd.LookupContext(ctx, typ, word, beforeText())
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
			prefer(cs, opt, sure)
			continue
		}
		cs, err := find(ctx, typ, word)
		l.report(err)
		add(cs, opt.noLearn)
		prefer(cs, opt, sure)
	}
	if i, ok := seen[preferred]; ok && i > 0 {
		c := out[i]
		copy(out[1:i+1], out[:i])
		out[0] = c
	}
	var unlearned map[string]bool
	for _, c := range out {
		if w := candidateWord(c); !learned[w] {
			if unlearned == nil {
				unlearned = map[string]bool{}
			}
			unlearned[w] = true
		}
	}
	l.remember(typ, word, unlearned, true)
	return out
}

// complete returns the entries whose reading starts with one of prefixes,
// merged by reading, each word of a reading once as in lookup.
func (l *library) complete(ctx context.Context, prefixes []string) []Entry {
	if l.user != nil {
		l.user.reload()
	}
	byWord := map[string]int{}
	var out []Entry
	// seen and learned: the words, by reading, a dictionary gave (their
	// index in the candidates), and one that is not NoLearn gave.
	seen := map[[2]string]int{}
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
					k := [2]string{e.Word, candidateWord(c)}
					if !opt.noLearn {
						learned[k] = true
					}
					if j, ok := seen[k]; !ok {
						seen[k] = len(out[i].Candidates)
						out[i].Candidates = append(out[i].Candidates, c)
					} else {
						out[i].Candidates[j] = annotated(out[i].Candidates[j], c)
					}
				}
			}
		}
	}
	for _, e := range out {
		var unlearned map[string]bool
		for _, c := range e.Candidates {
			if w := candidateWord(c); !learned[[2]string{e.Word, w}] {
				if unlearned == nil {
					unlearned = map[string]bool{}
				}
				unlearned[w] = true
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
// is longer than maxLearn or only NoLearn dictionaries gave candidate. okuri
// is the okurigana typed of a reading with okurigana, for its okuri block
// ("": none).
func (l *library) learn(typ HenkanType, word, okuri, candidate string) {
	if l.maxLearn > 0 && utf8.RuneCountInString(word) > l.maxLearn {
		return
	}
	if l.isUnlearned(typ, word, candidateWord(normalizeCandidate(candidate))) {
		return
	}
	l.register(typ, word, okuri, candidate)
}

// register puts candidate first for word in the user dictionary, whatever
// the length of word: a word registered is asked for.
func (l *library) register(typ HenkanType, word, okuri, candidate string) {
	if l.user != nil && candidate != "" {
		l.user.register(typ, word, okuri, normalizeCandidate(candidate))
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
