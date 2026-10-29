package engine

import (
	"slices"
	"strings"
)

// kakuteiUndo takes the last conversion back: what was confirmed is
// deleted and its candidates are shown again, so that another can be
// chosen. The cursor may have moved on along the line since then: it is
// moved back to the conversion (when the line still starts with what it was
// then), and put back after the next one is confirmed.

// kakuteiRecord is a conversion that kakuteiUndo can take back.
type kakuteiRecord struct {
	// kakutei is what it typed; state the conversion just before.
	kakutei string
	state   state
	mode    string
	// Where it was typed.
	vimMode     string
	bufnr, lnum int
	// bufferText is the line before the cursor right after it (the line
	// starts with it; it ends with kakutei).
	bufferText string
	// tail is what the handling typed after it (pending records).
	tail string
}

// rememberedPoint is where the cursor was when kakuteiUndo moved it back to
// the conversion; bytes the length of what was taken back.
type rememberedPoint struct {
	lnum, col, bytes int
}

// cursorPoint is where Vim puts the cursor after the keys (byte column).
type cursorPoint struct {
	Lnum int `json:"lnum"`
	Col  int `json:"col"`
}

type undoState struct {
	// last can be taken back. pending was confirmed by this handling: only
	// the next one knows where it ended up in the buffer.
	last, pending *kakuteiRecord
	remembered    *rememberedPoint
	// restore is where to put the cursor after this handling.
	restore *cursorPoint
}

func (s *state) clone() state {
	t := *s
	t.candidates = slices.Clone(s.candidates)
	return t
}

// hasPendingInput: something is being typed where the cursor is.
func (c *Context) hasPendingInput() bool {
	s := &c.state
	return s.kind != stateInput || s.mode != modeDirect || s.feed != "" || c.pre.current != ""
}

func (c *Context) recordKakutei(kakutei string, st state) {
	c.undo.last = nil
	c.undo.pending = &kakuteiRecord{
		kakutei: kakutei, state: st, mode: c.mode,
		vimMode: c.vimMode, bufnr: c.bufnr, lnum: c.lnum,
	}
}

// invalidateKakutei: the last conversion cannot be taken back any more.
func (c *Context) invalidateKakutei() {
	c.undo.last, c.undo.pending = nil, nil
}

func (c *Context) isAt(r *kakuteiRecord) bool {
	return c.vimMode == r.vimMode && c.bufnr == r.bufnr && c.lnum == r.lnum
}

// resolvePendingKakutei completes the record of the conversion of the last
// handling, now that the line before the cursor is known. It reports
// whether that conversion explains the buffer.
func (c *Context) resolvePendingKakutei() bool {
	p := c.undo.pending
	if p == nil {
		return false
	}
	c.undo.pending = nil
	if !c.isAt(p) {
		return false
	}
	// What was typed after it is not part of it: the key that confirmed
	// it by typing on, and the pre-edit.
	tail := p.tail + c.pre.current
	text := c.prevInput
	if tail != "" && strings.HasSuffix(text, tail) {
		text = text[:len(text)-len(tail)]
	}
	if !strings.HasSuffix(text, p.kakutei) {
		return false
	}
	p.bufferText, p.tail = text, ""
	c.undo.last = p
	return true
}

func (c *Context) recordPointRestore(kakutei, bufferText string) {
	col := len(c.prevInput) + 1
	c.undo.remembered = nil
	// A cursor inside the conversion has no place after another one.
	if col >= len(bufferText)+1 {
		c.undo.remembered = &rememberedPoint{c.lnum, col, len(kakutei)}
	}
}

// usePointRestore is where the cursor goes back to, now that kakutei
// replaced what kakuteiUndo took back.
func (c *Context) usePointRestore(kakutei string) *cursorPoint {
	r := c.undo.remembered
	c.undo.remembered = nil
	if r == nil {
		return nil
	}
	return &cursorPoint{r.lnum, r.col + len(kakutei) - r.bytes}
}

// locateKakutei puts the cursor right after the conversion.
func (c *Context) locateKakutei(r *kakuteiRecord) bool {
	if c.prevInput == r.bufferText {
		return true
	}
	if c.host == nil || c.vimMode != "i" {
		return false
	}
	before := r.bufferText[:len(r.bufferText)-len(r.kakutei)]
	v, err := c.host.Call(c.ctx, "goda_skk#_locate_kakutei", r.bufnr, r.lnum, before, r.kakutei)
	if err != nil {
		return false
	}
	ok, _ := v.(bool)
	return ok || toInt(v) == 1
}

// kakuteiUndo takes the last conversion back to choosing its candidate.
func kakuteiUndo(c *Context, _ string) {
	s := &c.state
	last := c.undo.last
	// A key before this one in the same handling typed something already.
	if last == nil || !c.isAt(last) || c.pre.kakutei.Len() > 0 ||
		s.kind != stateInput || s.mode != modeDirect || s.feed != "" {
		return
	}
	if !c.locateKakutei(last) {
		return
	}
	c.kakutei(strings.Repeat("\b", graphemes(last.kakutei)))
	c.state = last.state.clone()
	c.recordPointRestore(last.kakutei, last.bufferText)
	c.invalidateKakutei()
	if c.mode != last.mode {
		c.modeChange(last.mode)
	}
	if c.state.index >= c.cfg.ShowCandidatesCount {
		showCandidates(c)
	}
}
