package engine

import (
	"slices"
	"testing"
)

func TestShowAnnotation(t *testing.T) {
	dict := memDict{nasi: map[string][]string{
		"かんじ": {"漢字", "感じ;feeling", "幹事", "監事", "完治;cure", "寛治"},
	}}
	s := newSim(t, Config{Dictionaries: []Dictionary{dict}})
	for _, tc := range []struct {
		keys string
		want []string
	}{
		{"Kanji ", nil},
		// The annotation of the candidate shown.
		{" ", []string{"feeling"}},
		{" ", nil},
		// Back to it.
		{"x", []string{"feeling"}},
		// The list: after their candidates.
		{"   ", []string{"a: 完治;cure", "s: 寛治"}},
	} {
		s.typ(tc.keys)
		if !slices.Equal(s.res.Popup, tc.want) {
			t.Errorf("%q: popup %q, want %q", tc.keys, s.res.Popup, tc.want)
		}
	}
	if s.preedit() != "▼完治" {
		t.Errorf("pre-edit %q", s.preedit())
	}

	s = newSim(t, Config{Dictionaries: []Dictionary{dict}, ShowAnnotation: new(false)})
	s.typ("Kanji  ")
	if s.res.Popup != nil {
		t.Errorf("off: popup %q", s.res.Popup)
	}
	s.typ("   ")
	if want := []string{"a: 完治", "s: 寛治"}; !slices.Equal(s.res.Popup, want) {
		t.Errorf("off: popup %q, want %q", s.res.Popup, want)
	}
}
