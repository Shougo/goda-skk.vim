package engine

import (
	"context"
	"strings"

	"github.com/Shougo/goda.vim/sdk"
)

// Modes of the input, as g:goda_skk#mode tells them.
const (
	ModeHira    = "hira"
	ModeKata    = "kata"
	ModeHankata = "hankata"
	ModeZenkaku = "zenkaku"
	ModeAbbrev  = "abbrev"
)

// registration is a word to register: the prompt, and the conversion.
type registration struct {
	prompt string
	state  state
}

// candidateRef is a candidate that was confirmed, for purgeCandidate.
type candidateRef struct {
	typ       HenkanType
	word      string
	candidate string
}

// Context is the state of the input in the editor: one per plugin (the
// editor has one cursor).
type Context struct {
	cfg *Config
	lib *library

	state state
	mode  string
	// table is the kana table of the mode: the zenkaku mode has its own.
	table *KanaTable
	pre   preEdit

	keymaps map[string]keymap

	vimMode   string // "i", "c" or "t"
	prevInput string // the text before the cursor
	// Where the cursor is: the buffer and the line.
	bufnr, lnum int

	lastCandidate candidateRef
	// inHenkanFirst guards against a key mapped to henkanFirst alone (it
	// would call itself).
	inHenkanFirst bool

	// What the handling of keys asks the editor for, besides the keys to
	// feed.
	popup   []string
	disable bool
	// reenable: enable again at the next insert mode (KeepState).
	reenable bool

	undo undoState

	// register is the registration of a word to ask Vim for.
	register *registration

	ctx  context.Context
	host sdk.Host
}

func newContext(cfg *Config, lib *library) *Context {
	c := &Context{cfg: cfg, lib: lib, mode: ModeHira, table: cfg.KanaTable, keymaps: defaultKeymaps()}
	c.state.reset(c.table, false, false)
	return c
}

// resetState puts the state back to direct input; keep is "converter" and
// maybe "table". In the abbrev mode, the mode goes back to hiragana and
// nothing is kept.
func (c *Context) resetState(keepConverter, keepTable bool) {
	if c.mode == ModeAbbrev {
		c.modeChange(ModeHira)
		keepConverter, keepTable = false, false
	}
	c.state.reset(c.table, keepConverter, keepTable)
}

func (c *Context) modeChange(mode string) { c.mode = mode }

// kakutei writes s to the buffer, before the pre-edit.
func (c *Context) kakutei(s string) {
	// What the handling writes after a kakutei to take back, which is not
	// part of it.
	if p := c.undo.pending; p != nil {
		p.tail += s
	}
	c.pre.doKakutei(s)
}

// kakuteiWithUndoPoint is kakutei, also starting a new step of undo in
// insert mode (<C-g>u).
func (c *Context) kakuteiWithUndoPoint(s string) {
	c.kakutei(s)
	if *c.cfg.SetUndoPoint && c.vimMode == "i" {
		c.pre.doKakutei("\x07u")
	}
}

func (c *Context) text() string { return c.state.text(c.cfg) }

// keyChar is the character a key notation types.
func keyChar(key string) string {
	switch strings.ToLower(key) {
	case "<space>":
		return " "
	case "<lt>":
		return "<"
	case "<bar>":
		return "|"
	case "<bs>", "<c-h>":
		return "\b"
	case "<cr>", "<c-m>":
		return "\r"
	case "<c-j>", "<nl>":
		return "\n"
	case "<tab>", "<c-i>":
		return "\t"
	case "<esc>":
		return "\x1b"
	case "<c-g>":
		return "\x07"
	case "<c-q>":
		return "\x11"
	}
	return key
}

// keyNotation is the notation of the key that types char (the inverse of
// keyChar).
func keyNotation(char string) string {
	switch char {
	case " ":
		return "<space>"
	case "\b":
		return "<bs>"
	case "\r":
		return "<cr>"
	case "\n":
		return "<c-j>"
	case "\x1b":
		return "<esc>"
	case "\x07":
		return "<c-g>"
	case "\x11":
		return "<c-q>"
	}
	return char
}
