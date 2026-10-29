package engine

import (
	"slices"
	"strings"
)

// Rule is a rule of a kana table: typing From gives Kana, with Feed left to
// be typed on ("kk" gives "っ" with "k" left), or runs the function Func
// (" " starts a conversion). An upper-case letter of From is written
// "<s-x>" (the letter with Shift).
type Rule struct {
	From string
	Kana string
	Feed string
	Func string
}

// KanaTable is a kana table: its rules, sorted by From.
type KanaTable struct {
	rules []Rule
}

// NewKanaTable returns a table of rules. An upper-case letter in From is
// turned into its "<s-x>" form, and a later rule for the same From replaces
// an earlier one.
func NewKanaTable(rules ...Rule) *KanaTable {
	t := &KanaTable{}
	return t.With(rules...)
}

// With returns the table with rules added (replacing the rules for the same
// From). A Rule with neither Kana nor Func removes the rule for its From.
func (t *KanaTable) With(rules ...Rule) *KanaTable {
	byFrom := map[string]Rule{}
	for _, r := range t.rules {
		byFrom[r.From] = r
	}
	for _, r := range rules {
		r.From = shiftNotation(r.From)
		if r.Kana == "" && r.Func == "" {
			delete(byFrom, r.From)
			continue
		}
		byFrom[r.From] = r
	}
	out := &KanaTable{rules: make([]Rule, 0, len(byFrom))}
	for _, r := range byFrom {
		out.rules = append(out.rules, r)
	}
	slices.SortFunc(out.rules, func(a, b Rule) int { return strings.Compare(a.From, b.From) })
	return out
}

// Rules returns the rules of the table.
func (t *KanaTable) Rules() []Rule { return slices.Clone(t.rules) }

// shiftNotation writes the upper-case letters of s as "<s-x>".
func shiftNotation(s string) string {
	if strings.HasPrefix(s, "<") || strings.ToLower(s) == s {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if 'A' <= r && r <= 'Z' {
			b.WriteString("<s-" + string(r-'A'+'a') + ">")
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// find is the rule for exactly from.
func (t *KanaTable) find(from string) (Rule, bool) {
	i, ok := slices.BinarySearchFunc(t.rules, from, func(r Rule, s string) int { return strings.Compare(r.From, s) })
	if !ok {
		return Rule{}, false
	}
	return t.rules[i], true
}

// withPrefix returns the rules whose From starts with prefix.
func (t *KanaTable) withPrefix(prefix string) []Rule {
	i, _ := slices.BinarySearchFunc(t.rules, prefix, func(r Rule, s string) int { return strings.Compare(r.From, s) })
	j := i
	for j < len(t.rules) && strings.HasPrefix(t.rules[j].From, prefix) {
		j++
	}
	return t.rules[i:j]
}

// hasPrefix reports whether a rule starts with prefix.
func (t *KanaTable) hasPrefix(prefix string) bool {
	i, _ := slices.BinarySearchFunc(t.rules, prefix, func(r Rule, s string) int { return strings.Compare(r.From, s) })
	return i < len(t.rules) && strings.HasPrefix(t.rules[i].From, prefix)
}

// onlyFunc returns the table of the rules that run fn (the abbrev mode keeps
// only the one that starts the conversion).
func (t *KanaTable) onlyFunc(fn string) *KanaTable {
	out := &KanaTable{}
	for _, r := range t.rules {
		if r.Func == fn {
			out.rules = append(out.rules, r)
		}
	}
	return out
}

// The tables of this package: "rom" (romaji to hiragana) and "zen" (ASCII
// to full-width, the zenkaku mode).
var (
	RomTable = NewKanaTable(romHira...)
	ZenTable = NewKanaTable(romZen...)
)

// toKatakana converts hiragana to katakana.
func toKatakana(s string) string {
	var b strings.Builder
	for _, r := range s {
		if k, ok := hiraKata[r]; ok {
			b.WriteString(k)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// toHankata converts hiragana (and katakana) to half-width katakana.
func toHankata(s string) string {
	var b strings.Builder
	for _, r := range toKatakana(s) {
		if k, ok := kataHankata[r]; ok {
			b.WriteString(k)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// okuriWord is the reading of an okuri-ari entry: the stem and the
// consonant of the okurigana ("おく" and "っ" give "おくt": a small tsu
// alone is taken for the t row, as libskk does).
func okuriWord(stem, okuri string) string {
	if okuri == "っ" {
		return stem + "t"
	}
	for _, r := range okuri {
		if r != 'っ' {
			return stem + okuriConsonant[r]
		}
	}
	return stem
}
