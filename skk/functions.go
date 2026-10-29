package skk

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// Func is a function a key or a kana rule runs, with the key that runs it.
type Func func(c *Context, key string)

// functions are the functions by name, for Config.Keymaps and the Func of
// kana rules.
var functions map[string]Func

func init() {
	functions = map[string]Func{
		// Confirming.
		"kakutei":        kakuteiKey,
		"newline":        newline,
		"cancel":         cancel,
		"purgeCandidate": purgeCandidate,
		// Leaving.
		"disable": disableFunc,
		"escape":  escape,
		// Converting.
		"henkanFirst":    henkanFirst,
		"henkanForward":  henkanForward,
		"kakuteiUndo":    kakuteiUndo,
		"henkanBackward": henkanBackward,
		"henkanInput":    henkanInput,
		"suffix":         suffix,
		// Typing.
		"kanaInput":   kanaInput,
		"kakuteiFeed": func(c *Context, _ string) { kakuteiFeed(c) },
		"henkanPoint": func(c *Context, _ string) { henkanPoint(c) },
		"deleteChar":  deleteChar,
		"prefix":      prefix,
		// Modes.
		"abbrev":   abbrev,
		"hirakana": hirakana,
		"katakana": katakana,
		"hankata":  hankatakana,
		"zenkaku":  zenkaku,
	}
}

// ---------------------------------------------------------------------------
// Typing

// kakuteiFeed confirms the feed when it is kana by itself ("n" is ん).
func kakuteiFeed(c *Context) {
	s := &c.state
	if s.kind != stateInput {
		return
	}
	if r, ok := s.table.find(s.feed); ok && r.Func == "" {
		kakuteiKana(c, r.Kana, "")
	}
}

// kakuteiKana puts kana where the input goes: the buffer, the reading or
// the okurigana. feed is what is left to type.
func kakuteiKana(c *Context, kana, feed string) {
	s := &c.state
	switch s.mode {
	case modeDirect:
		c.kakutei(s.converter.apply(kana))
	case modeOkuriNasi:
		s.henkanFeed += kana
	case modeOkuriAri:
		if feed != "" && s.previousFeed {
			s.henkanFeed += kana
		} else {
			s.okuriFeed += kana
		}
		s.previousFeed = false
	}
	s.feed = feed
}

// doKakutei is kakuteiKana, converting once the okurigana is complete.
func doKakutei(c *Context, kana, feed string) {
	s := &c.state
	if s.kind != stateInput {
		return
	}
	kakuteiKana(c, kana, feed)
	if s.mode == modeOkuriAri && feed == "" {
		// "つかっ": waiting for the rest of the okurigana, unless asked to
		// convert at once.
		if !strings.HasSuffix(s.okuriFeed, "っ") || *c.cfg.ImmediatelyOkuriConvert {
			henkanFirst(c, "")
		}
	}
}

// acceptResult applies a rule: its kana, or its function (with the keys
// typed for it).
func acceptResult(c *Context, r Rule, feed string) {
	if r.Func == "" {
		doKakutei(c, r.Kana, r.Feed)
		return
	}
	c.state.feed = ""
	functions[r.Func](c, feed)
}

// kanaInput types the key through the kana table.
func kanaInput(c *Context, key string) {
	char := keyChar(key)
	c.state.kind = stateInput
	s := &c.state

	lower, ok := c.cfg.LowercaseMap[char]
	if !ok {
		lower = strings.ToLower(char)
	}
	if !s.directInput && char != lower {
		withShift := "<s-" + lower + ">"
		if s.table.hasPrefix(s.feed + withShift) {
			char = withShift
		} else {
			// An upper-case letter starts a reading (or the okurigana).
			henkanPoint(c)
			kanaInput(c, lower)
			return
		}
	}

	next := s.feed + char
	found := s.table.withPrefix(next)
	switch {
	case len(found) == 1 && found[0].From == next:
		acceptResult(c, found[0], next)
	case len(found) > 0:
		s.feed = next
	case s.feed != "":
		// The feed cannot go on: it is confirmed as it is, and the key
		// typed again.
		if r, ok := s.table.find(s.feed); ok {
			acceptResult(c, r, next)
		} else if c.cfg.AcceptIllegalResult {
			kakuteiKana(c, s.feed, "")
		} else {
			s.feed = ""
		}
		kanaInput(c, keyNotation(char))
	default:
		// Not in the table at all: as it is.
		kakuteiKana(c, char, "")
	}
}

// henkanPoint starts a reading (▽), or in a reading, the okurigana.
func henkanPoint(c *Context) {
	s := &c.state
	if s.kind != stateInput {
		return
	}
	// No okurigana for an empty reading.
	if s.mode == modeOkuriNasi && s.henkanFeed == "" {
		return
	}
	found := s.table.withPrefix(s.feed)
	switch s.mode {
	case modeDirect:
		if len(found) == 0 {
			c.kakutei(s.feed)
			s.feed = ""
		}
		c.kakuteiWithUndoPoint("")
		s.mode = modeOkuriNasi
	case modeOkuriNasi:
		switch {
		case s.feed == "" || len(found) == 0:
			s.feed = ""
		case found[0].From == s.feed:
			if found[0].Func == "" {
				s.henkanFeed += found[0].Kana
			}
			s.feed = ""
		default:
			s.previousFeed = true
		}
		s.mode = modeOkuriAri
	}
}

// deleteChar deletes the last character typed: of the feed, the
// okurigana or the reading, or in the buffer.
func deleteChar(c *Context, _ string) {
	s := &c.state
	if s.kind != stateInput {
		return
	}
	switch {
	case s.feed != "":
		s.feed = s.feed[:len(s.feed)-1]
	case s.mode == modeOkuriAri:
		if s.okuriFeed != "" {
			s.okuriFeed = dropLastRune(s.okuriFeed)
		} else {
			s.mode = modeOkuriNasi
		}
	case s.mode == modeOkuriNasi:
		if s.henkanFeed != "" {
			s.henkanFeed = dropLastRune(s.henkanFeed)
		} else {
			c.resetState(true, false)
		}
	default:
		c.kakutei("\b")
	}
}

func dropLastRune(s string) string {
	_, size := utf8.DecodeLastRuneInString(s)
	return s[:len(s)-size]
}

// prefix: ">" after a reading converts it as a prefix ("ちょう>" for 超).
func prefix(c *Context, key string) {
	s := &c.state
	if s.kind != stateInput {
		return
	}
	if !s.directInput && s.henkanFeed != "" && s.mode != modeDirect {
		acceptResult(c, Rule{Kana: ">"}, "")
		c.state.affix = affixPrefix
		henkanFirst(c, key)
		return
	}
	kanaInput(c, key)
}

// ---------------------------------------------------------------------------
// Converting

// henkanFirst converts the reading; in direct input, it types the key.
func henkanFirst(c *Context, key string) {
	if c.state.kind != stateInput {
		return
	}
	kakuteiFeed(c)
	s := &c.state
	if s.mode == modeDirect {
		// A key mapped to henkanFirst alone types itself.
		if c.inHenkanFirst {
			c.kakutei(keyChar(key))
		} else {
			c.inHenkanFirst = true
			kanaInput(c, key)
			c.inHenkanFirst = false
		}
		return
	}
	if s.henkanFeed == "" {
		return
	}
	s.kind = stateHenkan
	s.candidates = nil
	s.index = -1
	if s.mode == modeOkuriNasi {
		s.word = s.henkanFeed
	} else {
		s.word = okuriWord(s.henkanFeed, s.okuriFeed)
	}
	if s.affix == affixNone && !s.directInput {
		// A reading typed as "ちょう>" or ">てき" without prefix().
		switch {
		case strings.HasSuffix(s.henkanFeed, ">"):
			s.affix = affixPrefix
		case strings.HasPrefix(s.henkanFeed, ">"):
			s.affix = affixSuffix
		}
	}
	s.candidates = c.lib.lookup(c.ctx, s.mode.henkanType(), s.word)
	henkanForward(c, "")
}

// henkanForward shows the next candidate, or the next page of the list.
func henkanForward(c *Context, _ string) {
	s := &c.state
	if s.kind != stateHenkan {
		return
	}
	old := s.index
	if s.index >= c.cfg.ShowCandidatesCount {
		s.index += len(c.cfg.SelectCandidateKeys)
	} else {
		s.index++
	}
	if s.index >= len(s.candidates) {
		if registerWord(c) {
			return
		}
		s.index = old
		if s.index == -1 {
			s.kind = stateInput
		}
	}
	if s.index >= c.cfg.ShowCandidatesCount {
		showCandidates(c)
	}
}

// henkanBackward shows the previous candidate, or goes back to the reading.
func henkanBackward(c *Context, _ string) {
	s := &c.state
	if s.kind != stateHenkan {
		return
	}
	if s.index >= c.cfg.ShowCandidatesCount {
		s.index = max(s.index-len(c.cfg.SelectCandidateKeys), c.cfg.ShowCandidatesCount-1)
	} else {
		s.index--
	}
	if s.index < 0 {
		s.kind = stateInput
		return
	}
	if s.index >= c.cfg.ShowCandidatesCount {
		showCandidates(c)
	}
}

// showCandidates lists the page of candidates from the current one, with
// the keys that choose them.
func showCandidates(c *Context) {
	s := &c.state
	keys := []rune(c.cfg.SelectCandidateKeys)
	c.popup = c.popup[:0]
	for i := 0; i < len(keys) && s.index+i < len(s.candidates); i++ {
		c.popup = append(c.popup, fmt.Sprintf("%c: %s", keys[i], candidateText(s.candidates[s.index+i], s.affix)))
	}
}

// henkanInput is a key typed while choosing: a key of the list chooses,
// another confirms the candidate and is typed.
func henkanInput(c *Context, key string) {
	s := &c.state
	if s.index >= c.cfg.ShowCandidatesCount {
		if i := strings.Index(c.cfg.SelectCandidateKeys, key); i >= 0 && utf8.RuneCountInString(key) == 1 {
			i = utf8.RuneCountInString(c.cfg.SelectCandidateKeys[:i])
			if s.index+i < len(s.candidates) {
				s.index += i
				kakutei(c)
			}
			return
		}
	}
	kakutei(c)
	handleKey(c, key)
}

// suffix: ">" while choosing confirms, and starts a reading of a suffix
// (">てき" for 的).
func suffix(c *Context, _ string) {
	if c.state.kind != stateHenkan {
		return
	}
	kakutei(c)
	henkanPoint(c)
	acceptResult(c, Rule{Kana: ">"}, "")
	c.state.affix = affixSuffix
}

// registerWord asks for a word to register when there are no more
// candidates. (Not yet: false keeps the last candidate.)
func registerWord(*Context) bool { return false }

// ---------------------------------------------------------------------------
// Confirming

// kakutei confirms the candidate, or the reading as it is, and learns it.
func kakutei(c *Context) {
	s := &c.state
	switch s.kind {
	case stateHenkan:
		cand := ""
		if s.index >= 0 && s.index < len(s.candidates) {
			cand = s.candidates[s.index]
		}
		if cand != "" {
			c.lib.learn(s.mode.henkanType(), s.word, cand)
			c.lastCandidate = candidateRef{s.mode.henkanType(), s.word, cand}
		}
		ret := candidateText(cand, s.affix) + s.converter.apply(s.okuriFeed)
		snapshot := s.clone()
		c.kakuteiWithUndoPoint(ret)
		// What kakuteiUndo needs to take it back; and a conversion that
		// kakuteiUndo took back puts the cursor back where it was.
		c.recordKakutei(ret, snapshot)
		c.undo.restore = c.usePointRestore(ret)
	case stateInput:
		kakuteiFeed(c)
		c.kakutei(s.converter.apply(s.henkanFeed + s.okuriFeed + s.feed))
	}
	c.resetState(true, true)
}

// kakuteiKey is the key that confirms (<C-j>); with nothing to confirm, it
// goes back to hiragana (as ddskk does).
func kakuteiKey(c *Context, key string) {
	s := &c.state
	if s.kind == stateInput && s.mode == modeDirect && s.feed == "" {
		hirakana(c, key)
		return
	}
	kakutei(c)
}

// newline confirms and starts a new line (only confirms with
// EggLikeNewline while converting).
func newline(c *Context, _ string) {
	s := &c.state
	converting := s.kind == stateHenkan || (s.kind == stateInput && s.mode != modeDirect)
	kakutei(c)
	if !(c.cfg.EggLikeNewline && converting) {
		c.kakutei("\n")
	}
}

// cancel drops the reading or the conversion (<C-g>); on the command line
// with nothing to drop, it leaves the command line.
func cancel(c *Context, _ string) {
	s := &c.state
	if s.kind == stateInput && s.mode == modeDirect && c.vimMode == "c" {
		c.kakutei("\x03")
	}
	if *c.cfg.ImmediatelyCancel {
		c.resetState(false, false)
		return
	}
	switch s.kind {
	case stateInput:
		c.resetState(false, false)
	case stateHenkan:
		s.kind = stateInput
	}
}

// purgeCandidate removes the candidate (the one chosen, or the last one
// confirmed) from the user dictionary, after asking.
func purgeCandidate(c *Context, _ string) {
	s := &c.state
	var ref candidateRef
	switch s.kind {
	case stateInput:
		ref = c.lastCandidate
	case stateHenkan:
		if s.index < 0 || s.index >= len(s.candidates) {
			return
		}
		ref = candidateRef{s.mode.henkanType(), s.word, s.candidates[s.index]}
	default:
		return
	}
	if ref.word == "" || c.host == nil {
		return
	}
	v, err := c.host.Call(c.ctx, "confirm", fmt.Sprintf("Really purge? %s /%s/", ref.word, ref.candidate), "&Yes\n&No\n", 2)
	if err != nil || toInt(v) != 1 {
		return
	}
	c.lib.purge(ref.typ, ref.word, ref.candidate)
	// Taking the kakutei back would choose the purged candidate again.
	c.invalidateKakutei()
	c.state.reset(c.table, false, false)
	c.lastCandidate.word = ""
}

// ---------------------------------------------------------------------------
// Modes

// abbrev starts a reading of ASCII ("/" then "file" for ファイル).
func abbrev(c *Context, _ string) {
	s := &c.state
	if s.kind != stateInput || s.mode != modeDirect {
		return
	}
	henkanPoint(c)
	s.table = s.table.onlyFunc("henkanFirst")
	s.directInput = true
	c.modeChange(ModeAbbrev)
}

// hirakana goes back to hiragana.
func hirakana(c *Context, _ string) {
	s := &c.state
	if s.kind != stateInput {
		return
	}
	if s.mode != modeDirect {
		kakutei(c)
	}
	c.table = c.cfg.KanaTable
	s.converter = convNone
	s.reset(c.table, true, false)
	c.modeChange(ModeHira)
}

// katakana switches between hiragana and katakana; in a reading, it
// confirms the reading in katakana.
func katakana(c *Context, _ string) { toKana(c, convKatakana, ModeKata) }

// hankatakana is katakana for half-width katakana.
func hankatakana(c *Context, _ string) { toKana(c, convHankata, ModeHankata) }

func toKana(c *Context, conv converter, mode string) {
	s := &c.state
	if s.kind != stateInput {
		return
	}
	if s.mode == modeDirect {
		if s.converter == conv {
			s.converter = convNone
			c.modeChange(ModeHira)
			return
		}
		if conv == convHankata && c.table == c.cfg.ZenkakuTable {
			c.table = c.cfg.KanaTable
			s.table = c.table
		}
		s.converter = conv
		c.modeChange(mode)
		return
	}
	kakuteiFeed(c)
	kana := s.henkanFeed + s.okuriFeed
	result := kana
	if s.converter != conv {
		result = conv.apply(kana)
		if c.cfg.RegisterConvertResult {
			c.lib.learn(OkuriNasi, kana, result)
			c.lastCandidate = candidateRef{OkuriNasi, kana, result}
		}
	}
	c.kakuteiWithUndoPoint(result)
	s.reset(c.table, true, false)
}

// zenkaku types full-width ASCII.
func zenkaku(c *Context, _ string) {
	s := &c.state
	if s.kind != stateInput {
		return
	}
	if s.mode != modeDirect {
		kakutei(c)
	}
	c.table = c.cfg.ZenkakuTable
	s.table = c.table
	c.modeChange(ModeZenkaku)
}

// ---------------------------------------------------------------------------
// Leaving

// disableFunc confirms and turns the input method off.
func disableFunc(c *Context, _ string) {
	kakutei(c)
	c.state.reset(c.table, false, false)
	c.disable = true
}

// escape confirms, turns the input method off, and leaves insert mode.
func escape(c *Context, key string) {
	disableFunc(c, key)
	c.state.kind = stateEscape
	c.reenable = c.cfg.KeepState
}

func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case uint64:
		return int(n)
	case int8:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}
