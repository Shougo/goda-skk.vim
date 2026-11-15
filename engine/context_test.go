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
