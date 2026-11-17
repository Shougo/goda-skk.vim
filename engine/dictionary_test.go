package engine

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func TestLookupAnnotation(t *testing.T) {
	// The candidates of the same word are one, the first, whatever their
	// annotations: in an entry and between dictionaries.
	a := memDict{nasi: map[string][]string{"あ": {"A;x", "B", "A"}}}
	b := memDict{nasi: map[string][]string{"あ": {"B;y", "C", "A;z"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{a, b}})
	got := s.p.Lookup(context.Background(), OkuriNasi, "あ")
	if want := []string{"A;x", "B", "C"}; !slices.Equal(got, want) {
		t.Errorf("lookup: %v, want %v", got, want)
	}
}

func TestCompleteAnnotation(t *testing.T) {
	a := memDict{nasi: map[string][]string{"ああ": {"A;x", "A"}}}
	b := memDict{nasi: map[string][]string{"ああ": {"A;z", "B"}}}
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
	if want := []string{"A;x", "B"}; !slices.Equal(got, want) {
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
