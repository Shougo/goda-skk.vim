package engine

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLookupOkuriBlock(t *testing.T) {
	j := parseJisyo(okuriAriMarker+"\n"+
		"いk /行;go/生/[く/生/行/]/[っ/行/]/\n"+
		"はしr /走/[る/走/奔/]/\n"+
		okuriNasiMarker+"\n", false)
	for _, tc := range []struct {
		word, okuri string
		want        []string
	}{
		// The block of the okurigana first, with the annotations of
		// the others.
		{"いk", "く", []string{"生", "行;go"}},
		// No block: the candidates as they are.
		{"いk", "き", []string{"行;go", "生"}},
		{"いk", "", []string{"行;go", "生"}},
		// The block of the first kana ("ITta").
		{"いk", "った", []string{"行;go", "生"}},
		// A candidate only in the block.
		{"はしr", "る", []string{"走", "奔"}},
		{"ない", "る", nil},
	} {
		if got := j.lookupOkuri(tc.word, tc.okuri); !slices.Equal(got, tc.want) {
			t.Errorf("%q %q: %q, want %q", tc.word, tc.okuri, got, tc.want)
		}
	}
}

func TestRegisterOkuriBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jisyo")
	jisyo := okuriAriMarker + "\nいk /行/生/[く/生/行/]/\n" + okuriNasiMarker + "\n"
	if err := os.WriteFile(path, []byte(jisyo), 0o600); err != nil {
		t.Fatal(err)
	}
	u := NewUserDictionary(path)
	for _, tc := range []struct {
		okuri, candidate, want string
	}{
		// First in the block of its okurigana.
		{"く", "行", "いk /行/生/[く/行/生/]/"},
		// A new block, last.
		{"き", "生", "いk /生/行/[く/行/生/]/[き/生/]/"},
		// No okurigana: no block.
		{"", "行", "いk /行/生/[く/行/生/]/[き/生/]/"},
		// "[" or "]" would end the block.
		{"く", "[", "いk /[/行/生/[く/行/生/]/[き/生/]/"},
	} {
		u.register(OkuriAri, "いk", tc.okuri, tc.candidate)
		if got := readLines(t, path)[1]; got != tc.want {
			t.Errorf("%q %q: %q, want %q", tc.okuri, tc.candidate, got, tc.want)
		}
	}
	// Purged from the blocks too, the blocks left empty with it.
	u.Purge(OkuriAri, "いk", "生")
	if got, want := readLines(t, path)[1], "いk /[/行/[く/行/]/"; got != want {
		t.Errorf("purge: %q, want %q", got, want)
	}
	u.Register(OkuriNasi, "かっこ", "[")
	if got := readLines(t, path); !slices.Contains(got, "かっこ /[/") {
		t.Errorf("okuri-nasi: %q", got)
	}
}

func TestOkuriBlockConversion(t *testing.T) {
	// 行く and 生きる share the reading いk: each okurigana its own word.
	dict := memDict{ari: map[string][]string{"いk": {"行", "生", "逝"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{dict}})
	for _, tc := range []struct{ keys, want string }{
		{"IKi", "▼行き"},
		{" <c-j>ruIKu", "▼生く"},
		{" <c-j>IKi", "▼生き"},
		{"<c-j>IKu", "▼行く"},
	} {
		s.typ(tc.keys)
		if got := s.preedit(); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.keys, got, tc.want)
		}
	}
	s.typ("<c-j>")
	if want := "生きる行く生き行く"; s.line != want {
		t.Errorf("line %q, want %q", s.line, want)
	}
	lines := readLines(t, s.p.cfg.UserDictionary)
	if want := "いk /行/生/[き/生/]/[く/行/]/"; lines[1] != want {
		t.Errorf("user dictionary:\n%s", strings.Join(lines, "\n"))
	}
}

func TestJisyoOkuriBlock(t *testing.T) {
	// A dictionary file has its blocks too.
	path := filepath.Join(t.TempDir(), "SKK-JISYO.test")
	jisyo := okuriAriMarker + "\nはしr /奔/走/[る/走/]/\n" + okuriNasiMarker + "\n"
	if err := os.WriteFile(path, []byte(jisyo), 0o600); err != nil {
		t.Fatal(err)
	}
	s := newSim(t, Config{Dictionaries: []Dictionary{NewJisyo(path, "")}})
	ctx := context.Background()
	if got, want := s.p.lib.lookupOkuri(ctx, OkuriAri, "はしr", "る", nil), []string{"走", "奔"}; !slices.Equal(got, want) {
		t.Errorf("る: %q, want %q", got, want)
	}
	if got, want := s.p.lib.lookupOkuri(ctx, OkuriAri, "はしr", "", nil), []string{"奔", "走"}; !slices.Equal(got, want) {
		t.Errorf("none: %q, want %q", got, want)
	}
}

func TestCompleteDoneOkuriBlock(t *testing.T) {
	// The okurigana inserted after the candidate is that of its block.
	s := newSim(t, Config{})
	s.typ("Oku")
	s.line = "送り"
	s.p.CompleteDone(context.Background(), s, OkuriAri, "おくr", "送", "送り")
	if got, want := readLines(t, s.p.cfg.UserDictionary)[1], "おくr /送/[り/送/]/"; got != want {
		t.Errorf("user dictionary: %q, want %q", got, want)
	}
}

func TestCompleteOkuriBlock(t *testing.T) {
	// The okuri block of each split first, and no dictionary asked to
	// convert with the okurigana: the completion splits at every key.
	var got, okuri, asked []string
	od := okuriDict{ctxDict{memDict{}, &got}, &okuri}
	sd := Prefer(sureDict{memDict{}, &asked})
	dict := memDict{ari: map[string][]string{"いk": {"行", "生"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{dict, od, sd}})
	s.p.lib.register(OkuriAri, "いk", "き", "生")
	s.p.lib.register(OkuriAri, "いk", "く", "行")
	ctx := context.Background()
	for _, tc := range []struct{ keys, want string }{
		{"Iku", "行"},
		{"<c-g>Iki", "生"},
	} {
		s.typ(tc.keys)
		r, _ := s.p.Reading()
		oc := s.p.CompleteOkuri(ctx, r)
		if len(oc) == 0 || oc[0].Candidate != tc.want {
			t.Errorf("%q: %+v, want %q first", tc.keys, oc, tc.want)
		}
	}
	if len(okuri) != 0 {
		t.Errorf("LookupOkuri: %q", okuri)
	}
	for _, a := range asked {
		if strings.Count(a, "|") == 3 && strings.Split(a, "|")[2] != "" {
			t.Errorf("LookupSure with okurigana: %q", a)
		}
	}
}
