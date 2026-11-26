package engine

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func sahenJisyo(t *testing.T) Dictionary {
	t.Helper()
	path := filepath.Join(t.TempDir(), "SKK-JISYO.test")
	jisyo := okuriAriMarker + "\nかs /貸/\nあいs /愛/\n" +
		okuriNasiMarker + "\nべんきょう /勉強/\nか /蚊/\nあい /愛/藍/\n"
	if err := os.WriteFile(path, []byte(jisyo), 0o600); err != nil {
		t.Fatal(err)
	}
	return NewJisyo(path, "")
}

func TestSahenConvert(t *testing.T) {
	for _, tc := range []struct{ keys, want string }{
		{"BenkyouSu", "▼勉強す"},
		{"BenkyouShi", "▼勉強し"},
		{"BenkyouSa", "▼勉強さ"},
		{"BenkyouSe", "▼勉強せ"},
		// Not of the sa row.
		{"BenkyouKa", "▽べんきょう*か"},
	} {
		s := newSim(t, Config{Dictionaries: []Dictionary{sahenJisyo(t)}})
		s.typ(tc.keys)
		if got := s.preedit(); got != tc.want {
			t.Errorf("%q: %q, want %q", tc.keys, got, tc.want)
		}
	}
	s := newSim(t, Config{Dictionaries: []Dictionary{sahenJisyo(t)}})
	ctx := context.Background()
	for _, tc := range []struct {
		word, okuri string
		want        []string
	}{
		// After those with okurigana, each once.
		{"あいs", "す", []string{"愛", "藍"}},
		// A stem of one kana is no verb noun.
		{"かs", "す", []string{"貸"}},
		// Without the okurigana typed (Plugin.Lookup).
		{"べんきょうs", "", nil},
	} {
		if got := s.p.lib.lookupOkuri(ctx, OkuriAri, tc.word, tc.okuri, nil); !slices.Equal(got, tc.want) {
			t.Errorf("%q %q: %q, want %q", tc.word, tc.okuri, got, tc.want)
		}
	}
	// Learned with okurigana.
	s.typ("BenkyouSuru<c-j>")
	if got, want := readLines(t, s.p.cfg.UserDictionary)[1], "べんきょうs /勉強/[す/勉強/]/"; got != want {
		t.Errorf("user dictionary: %q, want %q", got, want)
	}
}

func TestSahenConvertOff(t *testing.T) {
	s := newSim(t, Config{Dictionaries: []Dictionary{sahenJisyo(t)}, SahenConvert: new(false)})
	if got := s.p.lib.lookupOkuri(context.Background(), OkuriAri, "べんきょうs", "す", nil); got != nil {
		t.Errorf("lookup: %q", got)
	}
}
