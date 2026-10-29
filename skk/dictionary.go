package skk

import (
	"context"
	"regexp"
	"slices"
	"strconv"
	"strings"
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

// Initializer is a Dictionary that prepares itself (reads its file) when
// the plugin starts, in the background.
type Initializer interface {
	Init(ctx context.Context) error
}

// library looks the dictionaries up in order, merging their candidates.
type library struct {
	user  *UserDictionary
	dicts []Dictionary
	logf  func(format string, args ...any)
}

func (l *library) all() []Dictionary {
	var ds []Dictionary
	if l.user != nil {
		ds = append(ds, l.user)
	}
	return append(ds, l.dicts...)
}

// lookup returns the candidates of word from every dictionary, each once,
// with readings of numbers converted (see numberWord).
func (l *library) lookup(ctx context.Context, typ HenkanType, word string) []string {
	if l.user != nil {
		l.user.reload()
	}
	var out []string
	seen := map[string]bool{}
	add := func(cs []string) {
		for _, c := range cs {
			if !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
	}
	pattern := numberWord(word)
	for _, d := range l.all() {
		if pattern != word {
			// The entries of the digits as typed come first, then those of
			// "#", converted.
			cs, err := d.Lookup(ctx, typ, word)
			l.report(err)
			ps, err := d.Lookup(ctx, typ, pattern)
			l.report(err)
			for _, p := range ps {
				cs = append(cs, convertNumber(p, word))
			}
			add(cs)
			continue
		}
		cs, err := d.Lookup(ctx, typ, word)
		l.report(err)
		add(cs)
	}
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
	for _, d := range l.all() {
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
					if !slices.Contains(out[i].Candidates, c) {
						out[i].Candidates = append(out[i].Candidates, c)
					}
				}
			}
		}
	}
	slices.SortStableFunc(out, func(a, b Entry) int { return strings.Compare(a.Word, b.Word) })
	return out
}

func (l *library) report(err error) {
	if err != nil && l.logf != nil {
		l.logf("goda-skk: %v", err)
	}
}

// learn puts candidate first for word in the user dictionary.
func (l *library) learn(typ HenkanType, word, candidate string) {
	if l.user != nil && candidate != "" {
		l.user.Register(typ, word, candidate)
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
