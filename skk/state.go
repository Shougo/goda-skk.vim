package skk

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// HenkanType is the kind of a reading in a dictionary.
type HenkanType string

// The kinds of readings: without okurigana ("かんじ") and with ("おくr").
const (
	OkuriNasi HenkanType = "okurinasi"
	OkuriAri  HenkanType = "okuriari"
)

type stateKind int

const (
	stateInput  stateKind = iota
	stateHenkan           // choosing a candidate (▼)
	stateEscape           // left by <Esc>
)

// inputMode is where the input goes: to the buffer, or to the reading of a
// conversion (▽), without or with okurigana.
type inputMode int

const (
	modeDirect inputMode = iota
	modeOkuriNasi
	modeOkuriAri
)

func (m inputMode) henkanType() HenkanType {
	if m == modeOkuriAri {
		return OkuriAri
	}
	return OkuriNasi
}

type affix int

const (
	affixNone affix = iota
	affixPrefix
	affixSuffix
)

// converter converts the kana typed (the katakana modes).
type converter int

const (
	convNone converter = iota
	convKatakana
	convHankata
)

func (c converter) apply(s string) string {
	switch c {
	case convKatakana:
		return toKatakana(s)
	case convHankata:
		return toHankata(s)
	}
	return s
}

// state is where the input is.
type state struct {
	kind  stateKind
	mode  inputMode
	affix affix
	// directInput: an upper-case letter does not start a conversion (the
	// abbrev mode).
	directInput bool
	table       *KanaTable
	converter   converter
	// feed is what is typed and not yet kana; henkanFeed the reading of
	// the conversion, okuriFeed its okurigana.
	feed       string
	henkanFeed string
	okuriFeed  string
	// previousFeed: the feed is before the okurigana point (typing "SaS"
	// for 察し).
	previousFeed bool

	// The conversion (stateHenkan).
	word       string
	candidates []string
	index      int
}

// reset puts the state back to direct input, keeping the converter and the
// table when keep says so.
func (s *state) reset(table *KanaTable, keepConverter, keepTable bool) {
	conv, tbl := s.converter, s.table
	*s = state{kind: stateInput, table: table}
	if keepConverter {
		s.converter = conv
	}
	if keepTable && tbl != nil {
		s.table = tbl
	}
}

// candidate is the candidate shown: without its annotation, and without
// the ">" of an affix.
func candidateText(c string, a affix) string {
	if i := strings.IndexByte(c, ';'); i >= 0 {
		c = c[:i]
	}
	switch a {
	case affixPrefix:
		c = strings.TrimSuffix(c, ">")
	case affixSuffix:
		c = strings.TrimPrefix(c, ">")
	}
	return c
}

// text is the pre-edit of the state: what is shown in the buffer.
func (s *state) text(cfg *Config) string {
	switch s.kind {
	case stateHenkan:
		c := ""
		if s.index >= 0 && s.index < len(s.candidates) {
			c = candidateText(s.candidates[s.index], s.affix)
		}
		return *cfg.MarkerHenkanSelect + c + s.converter.apply(s.okuriFeed)
	case stateEscape:
		return "\x1b"
	}
	ret := ""
	if s.mode != modeDirect {
		ret = *cfg.MarkerHenkan + s.henkanFeed
	}
	if s.mode == modeOkuriAri {
		if s.previousFeed {
			return ret + s.feed + "*"
		}
		ret += "*" + s.okuriFeed
	}
	return s.converter.apply(ret) + s.feed
}

// phase is the state for Vim script: "input", "input:okurinasi",
// "input:okuriari", "henkan" or "escape".
func (s *state) phase() string {
	switch s.kind {
	case stateHenkan:
		return "henkan"
	case stateEscape:
		return "escape"
	}
	switch s.mode {
	case modeOkuriNasi:
		return "input:okurinasi"
	case modeOkuriAri:
		return "input:okuriari"
	}
	return "input"
}

// preEdit makes the keys that turn the pre-edit in the buffer into the next
// one: backspaces over what is there (unless the next one only adds to it),
// what was confirmed, and the new pre-edit.
type preEdit struct {
	current string
	kakutei strings.Builder
}

func (p *preEdit) doKakutei(s string) { p.kakutei.WriteString(s) }

func (p *preEdit) output(next string) string {
	var out string
	if p.kakutei.Len() == 0 && strings.HasPrefix(next, p.current) {
		// No backspaces when nothing is taken back: the popup of a
		// completion would flicker.
		out = next[len(p.current):]
	} else {
		out = strings.Repeat("\b", graphemes(p.current)) + p.kakutei.String() + next
	}
	p.current = next
	p.kakutei.Reset()
	return out
}

// graphemes is how many backspaces delete s: its characters, a combining
// mark or a variation selector going with the character before it.
func graphemes(s string) int {
	n := 0
	prevZWJ := false
	for len(s) > 0 {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		joins := unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) ||
			unicode.Is(unicode.Variation_Selector, r) || r == '‍' || prevZWJ ||
			r == '゙' || r == '゚' || ('\U0001f3fb' <= r && r <= '\U0001f3ff')
		if n == 0 || !joins {
			n++
		}
		prevZWJ = r == '‍'
	}
	return n
}
