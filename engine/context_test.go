package engine

import (
	"context"
	"fmt"
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
	if want := []string{"1 1 5", "1 1 9", "1 1 12"}; !slices.Equal(s.getbufline, want) {
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

func TestContextLinesLimit(t *testing.T) {
	// At most contextLines lines before the cursor's.
	var got []string
	d := ctxDict{memDict{nasi: map[string][]string{"きしゃ": {"汽車"}}}, &got}
	s := newSim(t, Config{Dictionaries: []Dictionary{d}})
	for i := range contextLines + 5 {
		s.line += fmt.Sprintf("行%d番\n", i)
	}
	s.typ("Kisha ")
	want := ""
	for i := 5; i < contextLines+5; i++ {
		want += fmt.Sprintf("行%d番", i)
	}
	if !slices.Equal(got, []string{"きしゃ|" + want}) {
		t.Errorf("before: %q, want %q", got, want)
	}
}

// okuriDict is an OkuriDictionary that records what it is told.
type okuriDict struct {
	ctxDict
	okuri *[]string
}

func (d okuriDict) LookupOkuri(_ context.Context, word, okuri, before string) ([]string, error) {
	*d.okuri = append(*d.okuri, word+"|"+okuri+"|"+before)
	switch word {
	case "おおi":
		return []string{"多"}, nil
	case "いっt":
		return []string{"言っ"}, nil
	}
	return nil, nil
}

func TestOkuriDictionary(t *testing.T) {
	var got, okuri []string
	d := okuriDict{ctxDict{memDict{nasi: map[string][]string{"おおいに": {"大いに"}}}, &got}, &okuri}
	s := newSim(t, Config{Dictionaries: []Dictionary{d}})
	s.line = "人が"
	// With okurigana: the okurigana typed and the text before.
	s.typ("OoI<c-j>")
	s.typ("ItTa<c-j>")
	// Without: as a ContextDictionary.
	s.typ("Ooini <c-j>")
	if s.line != "人が多い言った大いに" {
		t.Errorf("line: %q", s.line)
	}
	if want := []string{"おおi|い|人が", "いっt|た|人が多い"}; !slices.Equal(okuri, want) {
		t.Errorf("okuri: %q, want %q", okuri, want)
	}
	if want := []string{"おおいに|人が多い言った"}; !slices.Equal(got, want) {
		t.Errorf("context: %q, want %q", got, want)
	}
}

// pickDict answers by the text before: a karukan that knows a few words.
type pickDict struct{ memDict }

func (d pickDict) LookupContext(_ context.Context, typ HenkanType, word, before string) ([]string, error) {
	if typ == OkuriNasi && word == "きしゃ" && strings.Contains(before, "駅") {
		return []string{"汽車", "記者"}, nil
	}
	if typ == OkuriNasi && word == "きしゃ" {
		return []string{"記者", "汽車"}, nil
	}
	return nil, nil
}

func (d pickDict) LookupOkuri(_ context.Context, word, okuri, before string) ([]string, error) {
	if word == "かk" && strings.Contains(before, "恥") {
		return []string{"掻", "書"}, nil
	}
	if word == "かk" {
		return []string{"書", "掻"}, nil
	}
	return nil, nil
}

func TestPrefer(t *testing.T) {
	jisyo := memDict{
		nasi: map[string][]string{"きしゃ": {"記者", "汽車", "帰社"}},
		ari:  map[string][]string{"かk": {"書", "描", "掻"}},
	}
	for _, c := range []struct {
		name, line string
		dict       Dictionary
		typ        HenkanType
		word       string
		learned    bool
		want       []string
	}{
		// Before the order of the dictionary file, with the text before.
		{"nasi", "駅のホームに", Prefer(pickDict{}), OkuriNasi, "きしゃ", false, []string{"汽車", "記者", "帰社"}},
		{"ari", "恥を", Prefer(pickDict{}), OkuriAri, "かk", false, []string{"掻", "書", "描"}},
		// Not before the first learned (記者, 書): it is what was chosen.
		{"nasi learned", "駅のホームに", Prefer(pickDict{}), OkuriNasi, "きしゃ", true, []string{"記者", "汽車", "帰社"}},
		{"ari learned", "恥を", Prefer(pickDict{}), OkuriAri, "かk", true, []string{"書", "描", "掻"}},
		// Without the text: the order of the dictionaries.
		{"no text", "", Prefer(pickDict{}), OkuriNasi, "きしゃ", false, []string{"記者", "汽車", "帰社"}},
		// Not Prefer, or Fallback: the order of the dictionaries.
		{"not prefer", "駅のホームに", pickDict{}, OkuriNasi, "きしゃ", false, []string{"記者", "汽車", "帰社"}},
		{"fallback", "駅のホームに", Prefer(Fallback(pickDict{})), OkuriNasi, "きしゃ", false, []string{"記者", "汽車", "帰社"}},
	} {
		s := newSim(t, Config{Dictionaries: []Dictionary{jisyo, c.dict}})
		if c.learned {
			s.p.lib.user.Register(OkuriNasi, "きしゃ", "記者")
			s.p.lib.user.Register(OkuriAri, "かk", "書")
		}
		before := func() string { return c.line }
		okuri := ""
		if c.typ == OkuriAri {
			okuri = "く"
		}
		got := s.p.lib.lookupOkuri(context.Background(), c.typ, c.word, okuri, before)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	// Through a conversion: confirmed, it is learned first.
	s := newSim(t, Config{Dictionaries: []Dictionary{jisyo, Prefer(pickDict{})}})
	s.line = "恥を"
	s.typ("KaKu<c-j>")
	if s.line != "恥を掻く" {
		t.Errorf("line: %q", s.line)
	}
	s.line = ""
	if got := s.p.lib.lookupOkuri(context.Background(), OkuriAri, "かk", "く", nil); got[0] != "掻" {
		t.Errorf("learned: %q", got)
	}
}

// sureDict is a SureDictionary sure of its first after "駅".
type sureDict struct {
	memDict
	asked *[]string
}

func (d sureDict) LookupSure(_ context.Context, typ HenkanType, word, okuri, before string) ([]string, bool, error) {
	*d.asked = append(*d.asked, string(typ)+"|"+word+"|"+okuri+"|"+before)
	if typ == OkuriAri {
		return []string{"掻", "書"}, strings.Contains(before, "恥"), nil
	}
	return []string{"汽車", "記者"}, strings.Contains(before, "駅"), nil
}

func TestPreferSure(t *testing.T) {
	jisyo := memDict{
		nasi: map[string][]string{"きしゃ": {"記者", "汽車"}},
		ari:  map[string][]string{"かk": {"書", "掻"}},
	}
	var asked []string
	d := Prefer(sureDict{asked: &asked})
	for _, c := range []struct {
		line       string
		typ        HenkanType
		word, okur string
		want       string
	}{
		// Sure: first. Not sure: the order learned.
		{"駅のホームに", OkuriNasi, "きしゃ", "", "汽車"},
		{"新聞社の", OkuriNasi, "きしゃ", "", "記者"},
		{"恥を", OkuriAri, "かk", "く", "掻"},
		{"記録を", OkuriAri, "かk", "く", "書"},
	} {
		s := newSim(t, Config{Dictionaries: []Dictionary{jisyo, d}})
		got := s.p.lib.lookupOkuri(context.Background(), c.typ, c.word, c.okur, func() string { return c.line })
		if got[0] != c.want {
			t.Errorf("%s %s: %q, want %q first", c.line, c.word, got, c.want)
		}
	}
	want := []string{"okurinasi|きしゃ||駅のホームに", "okurinasi|きしゃ||新聞社の", "okuriari|かk|く|恥を", "okuriari|かk|く|記録を"}
	if !slices.Equal(asked, want) {
		t.Errorf("asked: %q, want %q", asked, want)
	}
	// Not Prefer: LookupSure is not called.
	asked = nil
	s := newSim(t, Config{Dictionaries: []Dictionary{jisyo, sureDict{asked: &asked}}})
	s.p.lib.lookup(context.Background(), OkuriNasi, "きしゃ", func() string { return "駅" })
	if len(asked) != 0 {
		t.Errorf("asked without Prefer: %q", asked)
	}
}
