package engine

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// recentLength is how many characters of the text confirmed in a buffer are
// kept, and recentBuffers for how many buffers.
const (
	recentLength  = 200
	recentBuffers = 16
)

// recentText is the text the input method confirmed in each buffer (by
// buffer number), in Insert mode or in a terminal job: the text before the
// reading where the buffer does not show it (ContextDictionary). In a
// terminal job, what the job shows is not what is typed; on the command
// line, the text is the command, after that of the buffer. It is kept in
// memory only, and only when a dictionary is told the text.
type recentText map[int]string

// add appends what keys confirmed in bufnr: a backspace takes a character
// back, control characters (a newline) are dropped.
func (t recentText) add(bufnr int, confirmed string) {
	if t == nil || confirmed == "" {
		return
	}
	s := t[bufnr]
	for _, r := range cleanKakutei(confirmed) {
		switch {
		case r == '\b':
			_, size := utf8.DecodeLastRuneInString(s)
			s = s[:len(s)-size]
		case unicode.IsControl(r):
		default:
			s += string(r)
		}
	}
	if rs := []rune(s); len(rs) > recentLength {
		s = string(rs[len(rs)-recentLength:])
	}
	if _, ok := t[bufnr]; !ok && len(t) >= recentBuffers {
		clear(t) // the buffers of long ago
	}
	t[bufnr] = s
}

// cleanKakutei is confirmed text without the undo points.
func cleanKakutei(s string) string {
	return strings.ReplaceAll(s, "\x07u", "")
}
