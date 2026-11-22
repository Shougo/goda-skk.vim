package engine

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func completeWords(t *testing.T, s *sim, keys string) []string {
	t.Helper()
	s.typ(keys)
	r, ok := s.p.Reading()
	if !ok {
		t.Fatalf("no reading after %q", keys)
	}
	var out []string
	for _, c := range s.p.Complete(context.Background(), r) {
		out = append(out, c.Candidate)
	}
	s.typ("<c-g>")
	return out
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func TestUserDictionaryOrder(t *testing.T) {
	// The lines are kept in their order; a reading learned goes up to the
	// top of its section, a reading purged away goes.
	path := filepath.Join(t.TempDir(), "jisyo")
	jisyo := okuriAriMarker + "\nかk /書/\nみr /見/\n" +
		okuriNasiMarker + "\nう /卯/\nあ /亜/\nい /伊/\n"
	if err := os.WriteFile(path, []byte(jisyo), 0o600); err != nil {
		t.Fatal(err)
	}
	u := NewUserDictionary(path)
	u.Register(OkuriNasi, "い", "胃")
	u.Register(OkuriAri, "みr", "見")
	u.Register(OkuriNasi, "え", "絵")
	u.Purge(OkuriNasi, "う", "卯")
	want := []string{
		okuriAriMarker, "みr /見/", "かk /書/",
		okuriNasiMarker, "え /絵/", "い /胃/伊/", "あ /亜/",
	}
	if got := readLines(t, path); !slices.Equal(got, want) {
		t.Errorf("file:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// orderDict has a reading that is the prefix of another: by reading, かくに
// comes first.
var orderDict = memDict{nasi: map[string][]string{
	"かくに":   {"角煮"},
	"かくにん":  {"確認"},
	"かんさい":  {"関西"},
	"かんじ":   {"漢字", "幹事", "監事"},
	"かんじょう": {"感情", "勘定"},
}}

func TestCompleteLearnedFirst(t *testing.T) {
	// The readings learned come first, the latest first, in the next Vim
	// too.
	path := filepath.Join(t.TempDir(), "jisyo")
	s := newSim(t, Config{Dictionaries: []Dictionary{orderDict}, UserDictionary: path})
	if got, want := completeWords(t, s, "Kakuni"), []string{"角煮", "確認"}; !slices.Equal(got, want) {
		t.Errorf("before learning: %v, want %v", got, want)
	}
	s.typ("Kakunin <c-j>")
	if got, want := completeWords(t, s, "Kakuni"), []string{"確認", "角煮"}; !slices.Equal(got, want) {
		t.Errorf("learned: %v, want %v", got, want)
	}
	s = newSim(t, Config{Dictionaries: []Dictionary{orderDict}, UserDictionary: path})
	if got, want := completeWords(t, s, "Kakuni"), []string{"確認", "角煮"}; !slices.Equal(got, want) {
		t.Errorf("next Vim: %v, want %v", got, want)
	}
}

func TestCompleteFirstCandidatesFirst(t *testing.T) {
	// The first candidate of each reading learned (the last learned of
	// it) comes before the other candidates: learning 漢字 does not bring
	// up 幹事 and 監事 (#27 of skkeleton).
	s := newSim(t, Config{Dictionaries: []Dictionary{orderDict}})
	u := s.p.lib.user
	u.Register(OkuriNasi, "かんじょう", "感情")
	u.Register(OkuriNasi, "かんじ", "漢字")
	want := []string{"漢字", "感情", "幹事", "監事", "勘定", "関西"}
	if got := completeWords(t, s, "Kan"); !slices.Equal(got, want) {
		t.Errorf("complete: %v, want %v", got, want)
	}
	// Another candidate of a reading learned takes its place.
	u.Register(OkuriNasi, "かんじょう", "勘定")
	want = []string{"勘定", "漢字", "感情", "幹事", "監事", "関西"}
	if got := completeWords(t, s, "Kan"); !slices.Equal(got, want) {
		t.Errorf("complete: %v, want %v", got, want)
	}
}

func TestCompleteOrderOfFile(t *testing.T) {
	// The order is that of the lines of the file, written by another Vim or
	// another SKK implementation.
	path := filepath.Join(t.TempDir(), "jisyo")
	s := newSim(t, Config{Dictionaries: []Dictionary{orderDict}, UserDictionary: path})
	jisyo := okuriAriMarker + "\n" + okuriNasiMarker + "\nかくにん /確認/\nかくに /角煮/\n"
	if err := os.WriteFile(path, []byte(jisyo), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, want := completeWords(t, s, "Kakuni"), []string{"確認", "角煮"}; !slices.Equal(got, want) {
		t.Errorf("complete: %v, want %v", got, want)
	}
}
