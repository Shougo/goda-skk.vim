package skk

import (
	"fmt"
	"maps"
	"unicode/utf8"
)

// Config is the configuration of the input method. Its zero values are the
// defaults (see Defaults); New fills them in.
type Config struct {
	// KanaTable is the table of the kana input (default RomTable). Add to
	// it with RomTable.With(...).
	KanaTable *KanaTable
	// ZenkakuTable is the table of the zenkaku mode (default ZenTable).
	ZenkakuTable *KanaTable

	// Dictionaries are looked up after the user dictionary, in order. See
	// NewJisyo for a dictionary file.
	Dictionaries []Dictionary
	// UserDictionary is the file the confirmed conversions are learned in
	// (default "~/.goda-skk-jisyo"; "-" for none). It is read again when it
	// changed, and written after every change.
	UserDictionary string

	// Keymaps change what keys do: by state ("input" while typing,
	// "henkan" while choosing a candidate), the key in lower-case
	// notation ("<c-j>", "<space>", "x") and the name of a function
	// ("" removes the mapping). See the documentation for the functions.
	Keymaps map[string]map[string]string

	// MarkerHenkan and MarkerHenkanSelect mark a reading (default "▽") and
	// the candidate chosen (default "▼"); new("") for no marker (the
	// highlight GodaSkkHenkan shows the pre-edit then).
	MarkerHenkan       *string
	MarkerHenkanSelect *string
	// ShowCandidatesCount is how many candidates are shown one by one
	// before the rest is listed in a popup (default 4).
	ShowCandidatesCount int
	// SelectCandidateKeys are the 7 keys that choose from the popup
	// (default "asdfjkl").
	SelectCandidateKeys string
	// LowercaseMap maps a key typed with Shift to the key it starts a
	// conversion with (for keyboards where Shift gives another character).
	LowercaseMap map[string]string

	// AcceptIllegalResult keeps a feed that is no kana when the next key
	// does not continue it ("q" alone), instead of dropping it.
	AcceptIllegalResult bool
	// EggLikeNewline: <CR> while converting confirms without a new line.
	EggLikeNewline bool
	// ImmediatelyCancel: <C-g> while choosing a candidate cancels the
	// conversion at once (default true; false goes back to the reading).
	ImmediatelyCancel *bool
	// ImmediatelyOkuriConvert converts as soon as the okurigana is "っ"
	// (default true).
	ImmediatelyOkuriConvert *bool
	// KeepMode: enabling starts in the mode it was left in.
	KeepMode bool
	// KeepState: leaving insert mode with <Esc> enables again at the next
	// insert mode.
	KeepState bool
	// RegisterConvertResult learns a conversion to katakana in the user
	// dictionary.
	RegisterConvertResult bool
	// SetUndoPoint makes a confirmed conversion a step of undo (default
	// true).
	SetUndoPoint *bool
}

// Defaults is the default configuration.
var Defaults = Config{
	UserDictionary:      "~/.goda-skk-jisyo",
	MarkerHenkan:        new("▽"),
	MarkerHenkanSelect:  new("▼"),
	ShowCandidatesCount: 4,
	SelectCandidateKeys: "asdfjkl",
}

// fill returns c with the defaults filled in, or an error for an invalid
// setting.
func (c Config) fill() (Config, error) {
	if c.KanaTable == nil {
		c.KanaTable = RomTable
	}
	if c.ZenkakuTable == nil {
		c.ZenkakuTable = ZenTable
	}
	if c.UserDictionary == "" {
		c.UserDictionary = Defaults.UserDictionary
	}
	if c.MarkerHenkan == nil {
		c.MarkerHenkan = new(*Defaults.MarkerHenkan)
	}
	if c.MarkerHenkanSelect == nil {
		c.MarkerHenkanSelect = new(*Defaults.MarkerHenkanSelect)
	}
	if c.ShowCandidatesCount <= 0 {
		c.ShowCandidatesCount = Defaults.ShowCandidatesCount
	}
	if c.SelectCandidateKeys == "" {
		c.SelectCandidateKeys = Defaults.SelectCandidateKeys
	}
	keys := map[rune]bool{}
	for _, r := range c.SelectCandidateKeys {
		keys[r] = true
	}
	if utf8.RuneCountInString(c.SelectCandidateKeys) != 7 || len(keys) != 7 {
		return c, fmt.Errorf("SelectCandidateKeys must be 7 distinct characters: %q", c.SelectCandidateKeys)
	}
	for _, p := range []**bool{&c.ImmediatelyCancel, &c.ImmediatelyOkuriConvert, &c.SetUndoPoint} {
		if *p == nil {
			*p = new(true)
		}
	}
	c.LowercaseMap = maps.Clone(c.LowercaseMap)
	for st, m := range c.Keymaps {
		if st != "input" && st != "henkan" {
			return c, fmt.Errorf("Keymaps: unknown state %q (input or henkan)", st)
		}
		for key, fn := range m {
			if fn != "" && functions[fn] == nil {
				return c, fmt.Errorf("Keymaps[%q][%q]: unknown function %q", st, key, fn)
			}
		}
	}
	for _, t := range []*KanaTable{c.KanaTable, c.ZenkakuTable} {
		for _, r := range t.rules {
			if r.Func != "" && functions[r.Func] == nil {
				return c, fmt.Errorf("kana table: %q: unknown function %q", r.From, r.Func)
			}
		}
	}
	return c, nil
}
