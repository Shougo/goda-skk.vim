package engine

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestPinDemote(t *testing.T) {
	dict := memDict{
		ari:  map[string][]string{"みt": {"見", "観"}},
		nasi: map[string][]string{"かんじ": {"漢字", "感じ", "幹事"}},
	}
	keymaps := map[string]map[string]string{
		"henkan": {"<c-p>": "pin", "<c-d>": "demote"},
		"input":  {"<c-p>": "pin"},
	}
	s := newSim(t, Config{Dictionaries: []Dictionary{dict}, Keymaps: keymaps})
	steps := []struct {
		keys, preedit string
		popup         []string
	}{
		// Pinned: first, whatever is learned after.
		{"MiTa<c-p>", "▼見た", []string{"pin: 見"}},
		{"<c-j>MiTa <c-j>", "", nil}, // 観 confirmed: learned
		{"MiTa", "▼見た", nil},
		// Again: taken back, and the order learned is back.
		{"<c-p>", "▼見た", []string{"unpin: 見"}},
		{"<c-j>MiTa <c-j>MiTa", "▼観た", nil},
		// Demoted: last.
		{"<c-j>Kanji <c-d>", "▼漢字", []string{"demote: 漢字"}},
		{"<c-j>Kanji ", "▼感じ", nil},
		// The candidate confirmed last, out of a conversion.
		{"<c-j><c-p>", "", []string{"pin: 感じ"}},
	}
	for _, st := range steps {
		s.typ(st.keys)
		if s.preedit() != st.preedit || !slices.Equal(s.res.Popup, st.popup) {
			t.Errorf("%q: pre-edit %q popup %q, want %q %q", st.keys, s.preedit(), s.res.Popup, st.preedit, st.popup)
		}
	}
	ctx := context.Background()
	if got, want := s.p.lib.lookup(ctx, OkuriNasi, "かんじ", nil), []string{"感じ", "幹事", "漢字"}; !slices.Equal(got, want) {
		t.Errorf("lookup: %q, want %q", got, want)
	}
	es := s.p.lib.complete(ctx, []string{"かんじ"})
	if len(es) != 1 || !slices.Equal(es[0].Candidates, []string{"感じ", "幹事", "漢字"}) {
		t.Errorf("complete: %+v", es)
	}
	lines := readLines(t, s.p.cfg.OrderFile)
	want := []string{
		`{"type":"okurinasi","reading":"かんじ","word":"感じ","order":"pin"}`,
		`{"type":"okurinasi","reading":"かんじ","word":"漢字","order":"demote"}`,
	}
	if !slices.Equal(lines, want) {
		t.Errorf("file:\n%q\nwant\n%q", lines, want)
	}
	// Read by another plugin (another Vim).
	o := newCandidateOrder(s.p.cfg.OrderFile)
	if got := o.apply(OkuriNasi, "かんじ", []string{"漢字", "感じ", "幹事"}); !slices.Equal(got, []string{"感じ", "幹事", "漢字"}) {
		t.Errorf("read again: %q", got)
	}

	// As vim-caskk: "p" and "P" while choosing.
	s = newSim(t, Config{Dictionaries: []Dictionary{dict},
		Keymaps: map[string]map[string]string{"henkan": {"p": "pin", "P": "demote"}}})
	for _, st := range []struct {
		keys, preedit string
		popup         []string
	}{
		{"Kanji p", "▼漢字", []string{"pin: 漢字"}},
		{" P", "▼感じ", []string{"demote: 感じ"}},
		{"<c-j>Kanji ", "▼漢字", nil},
		{"  ", "▼感じ", nil},
		// Out of a conversion, "p" is typed.
		{"<c-j>pa", "", nil},
	} {
		s.typ(st.keys)
		if s.preedit() != st.preedit || !slices.Equal(s.res.Popup, st.popup) {
			t.Errorf("%q: pre-edit %q popup %q, want %q %q", st.keys, s.preedit(), s.res.Popup, st.preedit, st.popup)
		}
	}
	if !strings.HasSuffix(s.line, "感じぱ") {
		t.Errorf("line %q", s.line)
	}

	// Marked in the list.
	s = newSim(t, Config{Dictionaries: []Dictionary{memDict{nasi: map[string][]string{
		"かんじ": {"漢字", "感じ", "幹事"},
	}}}, ShowCandidatesCount: new(0)})
	s.p.lib.order.toggle(OkuriNasi, "かんじ", "幹事", orderPin)
	s.p.lib.order.toggle(OkuriNasi, "かんじ", "漢字", orderDemote)
	s.typ("Kanji ")
	if want := []string{"a: ★幹事", "s: 感じ", "d: ↓漢字"}; !slices.Equal(s.res.Popup, want) {
		t.Errorf("list: %q, want %q", s.res.Popup, want)
	}

	// No file: nothing to pin.
	s = newSim(t, Config{Dictionaries: []Dictionary{dict}, Keymaps: keymaps, OrderFile: "-"})
	s.typ("Kanji <c-p>")
	if s.res.Popup != nil || s.p.lib.order != nil {
		t.Errorf("no file: %q", s.res.Popup)
	}
}
