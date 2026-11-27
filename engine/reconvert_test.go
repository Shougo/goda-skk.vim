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
