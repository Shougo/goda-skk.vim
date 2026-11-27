package engine

import (
	"context"
	"regexp"
	"strings"
	"unicode/utf8"
)

// A conjugation dictionary (skk-verb-adjective-dictionary, of vim-caskk)
// has the stems of verbs and adjectives without okurigana, with their
// conjugation as the annotation: "か /書;@verb-5-k/買;@verb-5-w/". They are
// no words of their own: a reading with okurigana converts with the stem
// whose conjugation takes the okurigana typed (かk + く: 書, not 買).

// conjugationTag matches the annotation of a stem: its conjugation.
var conjugationTag = regexp.MustCompile(`^@(verb|adj)(-[a-z0-9]+)*$`)

// ichidan: what an ichidan stem takes (食べ: る, れ, ろ, よ, ない, ます, た,
// て, ず, ぬ, ん, られる, させる).
const ichidan = "るれろよなまたてずぬんらさ"

// conjugations: what each conjugation takes after its stem, the first kana
// of the okurigana. The classical ones (@verb-bungo-*) are not
// here: their stems are not converted.
var conjugations = map[string]string{
	"@verb-5-k":       "かきくけこい",
	"@verb-5-k-iku":   "かきくけこっ",
	"@verb-5-k-yuku":  "かきくけこっい",
	"@verb-5-g":       "がぎぐげごい",
	"@verb-5-s":       "さしすせそ",
	"@verb-5-t":       "たちつてとっ",
	"@verb-5-n":       "なにぬねのん",
	"@verb-5-b":       "ばびぶべぼん",
	"@verb-5-m":       "まみむめもん",
	"@verb-5-r":       "らりるれろっ",
	"@verb-5-r-aru":   "らりるれろっい",
	"@verb-5-w":       "わいうえおっ",
	"@verb-5-w-tou":   "わいうえおう",
	"@verb-1":         ichidan,
	"@verb-1-kureru":  ichidan,
	"@verb-1-uru":     ichidan,
	"@verb-henkaku-s": "るれろよ",
	"@verb-henkaku-z": "るれろよ",
	"@verb-henkaku-k": "るれ",
	"@adj":            "いかくけさそみき",
}

// conjugationOrder is the order of the conjugations to choose from when a
// word is registered: the common ones of today first.
var conjugationOrder = []string{
	"@verb-5-w", "@verb-5-k", "@verb-5-r", "@verb-1", "@verb-5-s",
	"@verb-5-t", "@verb-5-m", "@verb-5-g", "@verb-5-b", "@verb-5-n",
	"@adj", "@verb-henkaku-s", "@verb-henkaku-z", "@verb-henkaku-k",
	"@verb-5-k-iku", "@verb-5-r-aru", "@verb-5-w-tou", "@verb-5-k-yuku",
	"@verb-1-kureru", "@verb-1-uru",
}

// stemTag returns the conjugation of cand, a stem of a conjugation
// dictionary ("書;@verb-5-k").
func stemTag(cand string) (string, bool) {
	_, note, ok := strings.Cut(cand, ";")
	if !ok || !conjugationTag.MatchString(note) {
		return "", false
	}
	return note, true
}

// takes reports whether the conjugation tag takes ending (the okurigana
// after the stem; "" for none typed yet).
func takes(tag, ending string) bool {
	kana, ok := conjugations[tag]
	if !ok {
		return false
	}
	r, _ := utf8.DecodeRuneInString(ending)
	return ending == "" || strings.ContainsRune(kana, r)
}

// conjugationSplits returns how a reading with okurigana (word, its
// okurigana typed) splits into a stem and what follows it: after the
// consonant ("かk" + "く": か, く), or after the first kana of the
// okurigana, which the stem then ends with ("たb" + "べ": たべ, "", its
// candidate 食べ written 食 before the okurigana).
func conjugationSplits(word, okuri string) (base string, first string, rest string, ok bool) {
	last, size := utf8.DecodeLastRuneInString(word)
	if size != 1 || last < 'a' || last > 'z' || okuri == "" {
		return "", "", "", false
	}
	_, n := utf8.DecodeRuneInString(okuri)
	return word[:len(word)-size], okuri[:n], okuri[n:], true
}

// lookupConjugated returns the candidates of a reading with okurigana from
// the stems of the conjugation dictionaries (the user dictionary and the
// dictionary files, see blockDictionary) that take the okurigana.
func (l *library) lookupConjugated(ctx context.Context, word, okuri string) []string {
	base, first, rest, ok := conjugationSplits(word, okuri)
	if !ok {
		return nil
	}
	var out []string
	for _, d := range l.all() {
		d, opt := unwrap(d)
		if _, ok := d.(blockDictionary); !ok || skipped(d, opt) {
			continue
		}
		// After the consonant: the stem written as it is.
		if base != "" {
			cs, err := d.Lookup(ctx, OkuriNasi, base)
			l.report(err)
			for _, c := range cs {
				if tag, ok := stemTag(c); ok && takes(tag, okuri) {
					out = append(out, candidateWord(c))
				}
			}
		}
		// After the first kana: the stem without it.
		cs, err := d.Lookup(ctx, OkuriNasi, base+first)
		l.report(err)
		for _, c := range cs {
			w := candidateWord(c)
			if tag, ok := stemTag(c); ok && takes(tag, rest) && strings.HasSuffix(w, first) && w != first {
				out = append(out, strings.TrimSuffix(w, first))
			}
		}
	}
	return out
}

// ieRow: the kana an ichidan stem ends with (食べ, 見).
const ieRow = "いきぎしじちぢにひびぴみりえけげせぜてでねへべぺめれ"

// conjugationStem returns where a word with its conjugation (candidate,
// "思;@verb-5-w"), registered for a reading with okurigana, is written:
// the reading of its stem, and the stem with its conjugation. A stem ends
// at the consonant (おも + 思 of おもw + わ), or with the first kana of the
// okurigana when the word does (たべ + 食べ), or when the conjugation keeps
// it in the stem (一段: 食 of たb + べ is たべ + 食べ, 見 of みr + る is み +
// 見).
func conjugationStem(word, okuri, candidate string) (string, string, bool) {
	base, first, _, ok := conjugationSplits(word, okuri)
	tag, tagged := stemTag(candidate)
	if !ok || !tagged {
		return "", "", false
	}
	w := candidateWord(candidate)
	switch {
	case strings.HasSuffix(w, first):
		return base + first, candidate, true
	case strings.HasPrefix(tag, "@verb-1") && strings.Contains(ieRow, first):
		return base + first, w + first + ";" + tag, true
	}
	return base, candidate, base != ""
}

// conjugationCompletions returns the words with a conjugation to register
// for a reading with okurigana: text is the word typed, and maybe part of
// the tag after ";" ("思", "思;@verb-5"). Those that take the okurigana
// typed come first.
func conjugationCompletions(text, okuri string) []string {
	w, typed, _ := strings.Cut(text, ";")
	if w == "" {
		return nil
	}
	first, n := "", 0
	if okuri != "" {
		_, n = utf8.DecodeRuneInString(okuri)
		first = okuri[:n]
	}
	var fit, other []string
	for _, tag := range conjugationOrder {
		if !strings.HasPrefix(tag, typed) && !strings.HasPrefix(tag[1:], typed) {
			continue
		}
		c := w + ";" + tag
		if takes(tag, okuri) || (first != "" && strings.HasSuffix(w, first) && takes(tag, okuri[n:])) {
			fit = append(fit, c)
		} else {
			other = append(other, c)
		}
	}
	return append(fit, other...)
}
