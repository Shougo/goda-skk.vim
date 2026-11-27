package engine

import "testing"

func TestAffixKey(t *testing.T) {
	// ";;" for ">" (a rule of the kana table): a prefix after a reading,
	// the reading of a suffix elsewhere.
	dict := memDict{nasi: map[string][]string{
		"ちょう>": {"超"}, "ちょう": {"長", "超"}, ">てき": {"的"}, "てき": {"敵"}, "かんじ": {"漢字"},
	}}
	for _, tc := range []struct{ keys, line, preedit string }{
		{"Chou;;", "▼超", "▼超"},
		{"Chou;;<c-j>", "超", ""},
		{"Chou;;;;teki ", "超▼的", "▼的"},
		{"Kanji ;;teki ", "漢字▼的", "▼的"},
		{";;teki ", "▼的", "▼的"},
		{"a;;", "あ▽>", "▽>"},
		// ";" alone is still the henkanPoint.
		{";kanji ", "▼漢字", "▼漢字"},
		{"Kan;ji", "▽かん*じ", "▽かん*じ"},
		// And ">".
		{"Chou>>teki ", "超▼的", "▼的"},
	} {
		s := newSim(t, Config{Dictionaries: []Dictionary{dict},
			KanaTable: RomTable.With(Rule{From: ";;", Func: "prefix"})})
		s.typ(tc.keys)
		if s.line != tc.line || s.preedit() != tc.preedit {
			t.Errorf("%q: line %q pre-edit %q, want %q %q", tc.keys, s.line, s.preedit(), tc.line, tc.preedit)
		}
	}
}
