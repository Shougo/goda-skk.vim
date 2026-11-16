package engine

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// ctxDict is a ContextDictionary that records the text before the reading.
type ctxDict struct {
	memDict
	before *[]string
}

func (d ctxDict) LookupContext(ctx context.Context, typ HenkanType, word, before string) ([]string, error) {
	*d.before = append(*d.before, word+"|"+before)
	return d.Lookup(ctx, typ, word)
}

func TestContextDictionary(t *testing.T) {
	var got []string
	d := ctxDict{memDict{nasi: map[string][]string{"きしゃ": {"汽車", "記者"}}}, &got}
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict, d}})
	s.line = "駅のホームに"
	s.typ("Kisha <c-j>")
	if s.line != "駅のホームに汽車" {
		t.Errorf("line: %q", s.line)
	}
	// What was typed before it, and the keys that came together: the text
	// confirmed by them is not in the line Vim told.
	s.typ("ga")
	s.call("handleKey", "<space>", "K", "i", "s", "h", "a", "<space>")
	s.typ("<c-j>")
	// The okuri-ari reading and the "#" pattern of digits too.
	s.typ("Oku")
	s.typ("Ru")
	s.typ("<c-j>")
	s.p.lib.lookup(context.Background(), OkuriNasi, "1かい", func() string { return "第" })
	want := []string{
		"きしゃ|駅のホームに",
		"きしゃ|駅のホームに汽車が ",
		"おくr|駅のホームに汽車が 汽車",
		"1かい|第",
		"#かい|第",
	}
	if !slices.Equal(got, want) {
		t.Errorf("before:\n%q\nwant:\n%q", got, want)
	}
	// The other dictionaries are not told.
	if cs := s.p.lib.lookup(context.Background(), OkuriNasi, "かんじ", func() string { return "前" }); cs[0] != "漢字" {
		t.Errorf("lookup: %q", cs)
	}
}

func TestContextDictionaryTerminal(t *testing.T) {
	var got []string
	d := ctxDict{memDict{nasi: map[string][]string{"きしゃ": {"汽車"}}}, &got}
	s := newSim(t, Config{Dictionaries: []Dictionary{d}})
	s.mode = "t"
	s.typ("Kisha ")
	if !slices.Equal(got, []string{"きしゃ|"}) {
		t.Errorf("before: %q", got)
	}
}

func TestContextLines(t *testing.T) {
	var got []string
	d := ctxDict{memDict{nasi: map[string][]string{"きしゃ": {"汽車"}}}, &got}
	s := newSim(t, Config{Dictionaries: []Dictionary{d}})
	// The lines of the paragraph, at most contextLines, joined as one.
	s.line = "前の段落。\n\n一行目\n  駅の改札を出て、\nホームに\n"
	s.typ("Kisha <c-j>")
	s.line += "\n\nsee the\nREADME\n"
	s.typ("noKisha <c-j>")
	s.line += "\n\nThe train is\n"
	s.typ("Kisha <c-j>")
	want := []string{
		"きしゃ|一行目駅の改札を出て、ホームに",
		"きしゃ|see the README の",
		"きしゃ|The train is",
	}
	if !slices.Equal(got, want) {
		t.Errorf("before:\n%q\nwant:\n%q", got, want)
	}
	if want := []string{"1 3 5", "1 7 9", "1 10 12"}; !slices.Equal(s.getbufline, want) {
		t.Errorf("getbufline: %q, want %q", s.getbufline, want)
	}
}

func TestContextLinesNotAsked(t *testing.T) {
	// Without a ContextDictionary, or on the command line: no getbufline().
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict}})
	s.line = "前の行\n"
	s.typ("Kanji <c-j>")
	var got []string
	d := ctxDict{memDict{nasi: map[string][]string{"きしゃ": {"汽車"}}}, &got}
	c := newSim(t, Config{Dictionaries: []Dictionary{d}})
	c.mode = "c"
	c.line = "前の行\n"
	c.typ("Kisha ")
	if len(s.getbufline) != 0 || len(c.getbufline) != 0 || !slices.Equal(got, []string{"きしゃ|"}) {
		t.Errorf("getbufline: %q %q, before %q", s.getbufline, c.getbufline, got)
	}
}

func TestContextTerminalText(t *testing.T) {
	var got []string
	d := ctxDict{memDict{nasi: map[string][]string{"えき": {"駅"}, "きしゃ": {"汽車"}}}, &got}
	s := newSim(t, Config{Dictionaries: []Dictionary{d}})
	s.mode = "t"
	// What was confirmed in the buffer before, keys of the same call too.
	s.typ("Eki <c-j>no")
	s.call("handleKey", "K", "i", "s", "h", "a", "<space>")
	s.typ("<c-j>")
	// A backspace takes a character back; a newline is dropped.
	s.typ("<bs><cr>Kisha <c-j>")
	// Another buffer has its own; insert mode has its line.
	s.bufnr = 2
	s.typ("Kisha <c-j>")
	s.mode, s.bufnr, s.line = "i", 1, "x"
	s.typ("Kisha <c-j>")
	want := []string{"えき|", "きしゃ|駅の", "きしゃ|駅の汽", "きしゃ|", "きしゃ|x"}
	if !slices.Equal(got, want) {
		t.Errorf("before:\n%q\nwant:\n%q", got, want)
	}
}

func TestRecentText(t *testing.T) {
	tt := recentText{}
	tt.add(1, strings.Repeat("あ", recentLength)+"い\x07u\b\bう\r\n")
	if want := strings.Repeat("あ", recentLength-1) + "う"; tt[1] != want {
		t.Errorf("text: %q", tt[1])
	}
	for i := range recentBuffers {
		tt.add(i+2, "x")
	}
	if len(tt) != 1 || tt[recentBuffers+1] != "x" {
		t.Errorf("buffers: %v", len(tt))
	}
	// Without a dictionary told the text, nothing is kept.
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict}})
	s.mode = "t"
	s.typ("Kanji <c-j>")
	if s.p.recent != nil {
		t.Errorf("kept: %v", s.p.recent)
	}
}

func TestContextCommandLine(t *testing.T) {
	var got []string
	d := ctxDict{memDict{nasi: map[string][]string{"えき": {"駅"}, "きしゃ": {"汽車"}}}, &got}
	s := newSim(t, Config{Dictionaries: []Dictionary{d}})
	s.typ("Eki <c-j>no<esc>")
	// On the command line: the text written in the buffer, then the
	// command.
	s.mode, s.line = "c", "s/"
	s.typ("Kisha <c-j>")
	// What the command line confirmed is not the buffer's.
	s.line = ""
	s.typ("Kisha <c-j>")
	// Another buffer has its own.
	s.bufnr, s.line = 2, ""
	s.typ("Kisha <c-j>")
	want := []string{"えき|", "きしゃ|駅の s/", "きしゃ|駅の", "きしゃ|"}
	if !slices.Equal(got, want) {
		t.Errorf("before:\n%q\nwant:\n%q", got, want)
	}
}

func TestContextOnlyWhenAsked(t *testing.T) {
	var got []string
	d := ctxDict{memDict{nasi: map[string][]string{"きしゃ": {"汽車"}}}, &got}
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict, Fallback(d)}})
	s.line = "前の行\n"
	// The dictionary file has it: the text before it is not asked for.
	s.typ("Kanji <c-j>")
	if len(s.getbufline) != 0 || len(got) != 0 {
		t.Errorf("asked: %q %q", s.getbufline, got)
	}
	s.typ("Kisha <c-j>")
	if len(s.getbufline) != 1 || !slices.Equal(got, []string{"きしゃ|前の行漢字"}) {
		t.Errorf("not asked: %q %q", s.getbufline, got)
	}
	// Looked up twice for a reading with digits, computed once.
	n := 0
	s.p.lib.lookup(context.Background(), OkuriNasi, "1かい", func() string { n++; return "第" })
	if n != 1 || !slices.Equal(got[1:], []string{"1かい|第", "#かい|第"}) {
		t.Errorf("before: %d %q", n, got)
	}
}
