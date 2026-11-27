package engine

import "testing"

func TestConversionAt(t *testing.T) {
	l := &library{}
	a := &kakuteiRecord{kakutei: "漢字", vimMode: "i", bufnr: 1, lnum: 2, bufferText: "あ漢字"}
	b := &kakuteiRecord{kakutei: "感じ", vimMode: "i", bufnr: 1, lnum: 2, bufferText: "あ漢字の感じ"}
	cmd := &kakuteiRecord{kakutei: "字", vimMode: "c", bufnr: 1, lnum: 2, bufferText: "字"}
	for _, r := range []*kakuteiRecord{a, b, cmd, a} {
		l.rememberConversion(r)
	}
	if len(l.conversions) != 2 {
		t.Fatalf("conversions: %d", len(l.conversions))
	}
	for _, tc := range []struct {
		bufnr, lnum, col int
		line             string
		want             *kakuteiRecord
	}{
		{1, 2, len("あ"), "あ漢字の感じだ", a},
		{1, 2, len("あ漢"), "あ漢字の感じだ", a},
		{1, 2, len("あ漢字の"), "あ漢字の感じだ", b},
		// Not on a conversion, another line or buffer, or what was before
		// it changed.
		{1, 2, 0, "あ漢字の感じだ", nil},
		{1, 2, len("あ漢字"), "あ漢字の感じだ", nil},
		{1, 3, len("あ"), "あ漢字の感じだ", nil},
		{2, 2, len("あ"), "あ漢字の感じだ", nil},
		{1, 2, len("い"), "い漢字の感じだ", nil},
	} {
		if _, got := l.conversionAt(tc.bufnr, tc.lnum, tc.col, tc.line); got != tc.want {
			t.Errorf("%d %d %d %q: %v, want %v", tc.bufnr, tc.lnum, tc.col, tc.line, got, tc.want)
		}
	}
	for range maxConversions + 5 {
		l.rememberConversion(&kakuteiRecord{vimMode: "i"})
	}
	if len(l.conversions) != maxConversions {
		t.Errorf("kept %d", len(l.conversions))
	}
}

func TestToHiragana(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		ok       bool
	}{
		{"かんじ", "かんじ", true},
		{"カンジ", "かんじ", true},
		{"ラーメン", "らーめん", true},
		{"ヴァ", "ゔぁ", true},
		{"いすゞ", "いすゞ", true},
		{"", "", false},
		{"漢字", "", false},
		{"かんじa", "", false},
		{"ｶﾝｼﾞ", "", false},
	} {
		if got, ok := toHiragana(tc.in); got != tc.want || ok != tc.ok {
			t.Errorf("%q: %q %v, want %q %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}

func TestConvertKana(t *testing.T) {
	s := newSim(t, Config{})
	s.call("convertKana", "かんじ")
	if got := s.preedit(); got != "▼漢字" {
		t.Errorf("convertKana: %q", got)
	}
	s.typ(" ")
	s.typ("<c-j>")
	if s.line != "感じ" {
		t.Errorf("confirmed: %q", s.line)
	}
	// Not while something is typed.
	s.typ("K")
	s.call("convertKana", "へんかん")
	if got := s.preedit(); got != "▽k" {
		t.Errorf("while typing: %q", got)
	}
}
