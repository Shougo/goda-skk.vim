package engine

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func conjugationJisyo(t *testing.T) Dictionary {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SKK-JISYO.verb-adjective")
	jisyo := okuriAriMarker + "\n" + okuriNasiMarker + "\n" +
		"か /書;@verb-5-k/買;@verb-5-w/勝;@verb-5-t/\n" +
		"たか /高;@adj/\n" +
		"たべ /食べ;@verb-1/\n" +
		"たす /助;@verb-bungo-2-shimo-k/\n" +
		"かんじ /漢字/\n"
	if err := os.WriteFile(path, []byte(jisyo), 0o600); err != nil {
		t.Fatal(err)
	}
	return NewJisyo(path, "")
}

func TestConjugation(t *testing.T) {
	for _, tc := range []struct{ keys, want string }{
		// The stems whose conjugation takes the okurigana.
		{"KaKu", "▼書く"},
		{"KaKu ", "▼書く"}, // 買 and 勝 do not take く: registered next
		{"KaTta", "▼買った"},
		{"KaTta ", "▼勝った"},
		{"KaI", "▼書い"},
		{"KaWa", "▼買わ"},
		{"TakaI", "▼高い"},
		{"TakaKu", "▼高く"},
		// The okurigana after the kana a stem ends with, or with it.
		{"TabeRu", "▼食べる"},
		{"TaBe", "▼食べ"},
		{"TabeTa", "▼食べた"},
		{"TabeKu", "▽たべ*く"},
		// Classical conjugations are not converted.
		{"TasuKe", "▽たす*け"},
	} {
		s := newSim(t, Config{Dictionaries: []Dictionary{conjugationJisyo(t)}, ShowCandidatesCount: new(9)})
		s.typ(tc.keys)
		if got := s.preedit(); got != tc.want && !(tc.keys == "KaKu " && s.res.Registering) {
			t.Errorf("%q: %q, want %q", tc.keys, got, tc.want)
		}
	}
	// The stems are no words of their own.
	s := newSim(t, Config{Dictionaries: []Dictionary{conjugationJisyo(t)}})
	ctx := context.Background()
	if got := s.p.lib.lookup(ctx, OkuriNasi, "か", nil); got != nil {
		t.Errorf("okuri-nasi: %q", got)
	}
	var words []string
	for _, e := range s.p.lib.complete(ctx, []string{"か", "た"}) {
		words = append(words, e.Word)
	}
	if want := []string{"かんじ"}; !slices.Equal(words, want) {
		t.Errorf("complete: %q, want %q", words, want)
	}
}

func TestRegisterConjugation(t *testing.T) {
	s := newSim(t, Config{Dictionaries: []Dictionary{conjugationJisyo(t)}})
	for _, tc := range []struct{ word, okuri, cand, stemLine, wordLine string }{
		// Godan: the stem at the consonant, for every okurigana.
		{"おもu", "う", "思;@verb-5-w", "おも /思;@verb-5-w/", "おもu /思/[う/思/]/"},
		// Ichidan: the kana of the okurigana is the stem's.
		{"にb", "べ", "似;@verb-1", "にべ /似べ;@verb-1/", "にb /似/[べ/似/]/"},
		{"みr", "る", "見;@verb-1", "み /見;@verb-1/", "みr /見/[る/見/]/"},
	} {
		s.p.lib.register(OkuriAri, tc.word, tc.okuri, tc.cand)
		lines := readLines(t, s.p.cfg.UserDictionary)
		if !slices.Contains(lines, tc.stemLine) || !slices.Contains(lines, tc.wordLine) {
			t.Errorf("%q %q: %q", tc.word, tc.cand, lines)
		}
	}
	// Then converted with any okurigana it takes.
	s.typ("OmoWanai")
	if got, want := s.line, "思わない"; got != want {
		t.Errorf("after registering: %q, want %q", got, want)
	}
}

func TestConjugationCompletions(t *testing.T) {
	got := conjugationCompletions("思", "う")
	if len(got) != len(conjugationOrder) || got[0] != "思;@verb-5-w" || got[1] != "思;@verb-5-w-tou" {
		t.Errorf("思 う: %q", got)
	}
	if got, want := conjugationCompletions("思;@verb-5-r", "う"), []string{"思;@verb-5-r", "思;@verb-5-r-aru"}; !slices.Equal(got, want) {
		t.Errorf("思;@verb-5-r: %q, want %q", got, want)
	}
	if got, want := conjugationCompletions("高;adj", "い"), []string{"高;@adj"}; !slices.Equal(got, want) {
		t.Errorf("高;adj: %q, want %q", got, want)
	}
	if got := conjugationCompletions("", "う"); got != nil {
		t.Errorf("empty: %q", got)
	}
}
