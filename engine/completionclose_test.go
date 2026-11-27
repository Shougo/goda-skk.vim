package engine

import "testing"

func TestCompletionClose(t *testing.T) {
	for _, tc := range []struct {
		typed, key string
		want       bool
	}{
		// The reading confirmed (EggLikeNewline: no item selected) or
		// canceled: its completion is closed.
		{"Kanji", "<cr>", true},
		{"Kanji", "<c-j>", true},
		{"Kanji", "<c-g>", true},
		{"Kanji", "q", true},
		// Still a reading, or converted (▼).
		{"Kan", "j", false},
		{"Kanji", "<space>", false},
		// No reading.
		{"k", "a", false},
	} {
		s := newSim(t, Config{Dictionaries: []Dictionary{testDict}, EggLikeNewline: true})
		s.typ(tc.typed)
		s.pum, s.selected = true, -1
		res := s.call("handleKey", tc.key)
		if res.CompletionClose != tc.want {
			t.Errorf("%q %q: CompletionClose %v, want %v", tc.typed, tc.key, res.CompletionClose, tc.want)
		}
	}
	// Not shown: nothing to close.
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict}})
	s.typ("Kanji")
	if res := s.call("handleKey", "<c-j>"); res.CompletionClose {
		t.Error("closed without a completion")
	}
}
