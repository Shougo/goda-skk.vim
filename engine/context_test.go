package engine

import (
	"context"
	"slices"
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
	s.p.lib.lookup(context.Background(), OkuriNasi, "1かい", "第")
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
	if cs := s.p.lib.lookup(context.Background(), OkuriNasi, "かんじ", "前"); cs[0] != "漢字" {
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
