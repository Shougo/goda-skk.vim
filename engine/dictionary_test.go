package engine

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLookupAnnotation(t *testing.T) {
	// The candidates of the same word are one, the first, whatever their
	// annotations: in an entry and between dictionaries. Without one, it
	// has that of a later one.
	a := memDict{nasi: map[string][]string{"あ": {"A;x", "B", "A"}}}
	b := memDict{nasi: map[string][]string{"あ": {"B;y", "C", "A;z", "C;w"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{a, b}})
	got := s.p.Lookup(context.Background(), OkuriNasi, "あ")
	if want := []string{"A;x", "B;y", "C;w"}; !slices.Equal(got, want) {
		t.Errorf("lookup: %v, want %v", got, want)
	}
}

func TestCompleteAnnotation(t *testing.T) {
	a := memDict{nasi: map[string][]string{"ああ": {"A;x", "A", "B"}}}
	b := memDict{nasi: map[string][]string{"ああ": {"A;z", "B;y"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{a, b}})
	ctx := context.Background()
	s.typ("A")
	r, _ := s.p.Reading()
	var got []string
	for _, c := range s.p.Complete(ctx, r) {
		if c.Reading == "ああ" {
			got = append(got, c.Candidate)
		}
	}
	if want := []string{"A;x", "B;y"}; !slices.Equal(got, want) {
		t.Errorf("complete: %v, want %v", got, want)
	}
}

func TestNoLearnAnnotation(t *testing.T) {
	// A candidate is learned if a dictionary that is not NoLearn gives its
	// word, though with another annotation.
	phrase := memDict{nasi: map[string][]string{"へんかん": {"変換;x"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{NoLearn(phrase), testDict}})
	s.typ("Henkan <c-j>")
	if s.line != "変換" {
		t.Errorf("line: %q", s.line)
	}
	if j := userJisyo(t, s); !strings.Contains(j, "\nへんかん /変換;x/\n") {
		t.Errorf("a candidate of both is not learned:\n%s", j)
	}
}

func TestRegisterAnnotation(t *testing.T) {
	// Learning a candidate takes the place of those of the same word,
	// whatever their annotations.
	path := filepath.Join(t.TempDir(), "jisyo")
	jisyo := okuriAriMarker + "\n" + okuriNasiMarker + "\nあ /B/A;x/A/\n"
	if err := os.WriteFile(path, []byte(jisyo), 0o600); err != nil {
		t.Fatal(err)
	}
	u := NewUserDictionary(path)
	u.Register(OkuriNasi, "あ", "A;x")
	lookup := func() []string {
		got, _ := u.Lookup(context.Background(), OkuriNasi, "あ")
		return got
	}
	if got, want := lookup(), []string{"A;x", "B"}; !slices.Equal(got, want) {
		t.Errorf("register: %v, want %v", got, want)
	}
	u.Register(OkuriNasi, "あ", "B;y")
	if got, want := lookup(), []string{"B;y", "A;x"}; !slices.Equal(got, want) {
		t.Errorf("register: %v, want %v", got, want)
	}
}

func TestPurgeAnnotation(t *testing.T) {
	// Purging a candidate removes those of the same word too, whatever
	// their annotations: they would come up in its place.
	path := filepath.Join(t.TempDir(), "jisyo")
	jisyo := okuriAriMarker + "\n" + okuriNasiMarker + "\nあ /A;x/B/A/\n"
	if err := os.WriteFile(path, []byte(jisyo), 0o600); err != nil {
		t.Fatal(err)
	}
	u := NewUserDictionary(path)
	u.Purge(OkuriNasi, "あ", "A;x")
	got, _ := u.Lookup(context.Background(), OkuriNasi, "あ")
	if want := []string{"B"}; !slices.Equal(got, want) {
		t.Errorf("purge: %v, want %v", got, want)
	}
}

func TestLearnedWithoutAnnotation(t *testing.T) {
	// A word learned without its annotation (from Google, say) does not
	// hide that of the dictionary file, and is learned again with it.
	jisyo := memDict{nasi: map[string][]string{"いぜん": {"依然;still", "已然;already", "以前"}}}
	google := memDict{nasi: map[string][]string{"いぜん": {"以前", "依然", "已然"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{jisyo, google}})
	s.p.lib.user.Register(OkuriNasi, "いぜん", "已然")
	ctx := context.Background()
	got := s.p.Lookup(ctx, OkuriNasi, "いぜん")
	if want := []string{"已然;already", "依然;still", "以前"}; !slices.Equal(got, want) {
		t.Errorf("lookup: %v, want %v", got, want)
	}
	s.typ("Izen <c-j>")
	b, err := os.ReadFile(s.p.cfg.UserDictionary)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\nいぜん /已然;already/\n") {
		t.Errorf("user dictionary:\n%s", b)
	}
}
