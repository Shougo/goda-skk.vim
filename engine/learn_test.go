package engine

import (
	"os"
	"strings"
	"testing"
)

func TestMaxLearnLength(t *testing.T) {
	s := newSim(t, Config{
		Dictionaries: []Dictionary{testDict, memDict{nasi: map[string][]string{
			"かんじへんかん": {"漢字変換"},
		}}},
		MaxLearnLength: 3,
	})
	s.typ("Kanji  <c-j>Kanjihenkan <c-j>")
	if s.line != "感じ漢字変換" {
		t.Errorf("line: %q", s.line)
	}
	b, err := os.ReadFile(s.p.cfg.UserDictionary)
	if err != nil {
		t.Fatal(err)
	}
	// "かんじ" (3 characters) is learned, "かんじへんかん" is not.
	if !strings.Contains(string(b), "\nかんじ /感じ/\n") || strings.Contains(string(b), "かんじへんかん") {
		t.Errorf("user dictionary:\n%s", b)
	}
	if _, err := New(Config{MaxLearnLength: -1}); err == nil {
		t.Errorf("negative: no error")
	}
}

func TestMaxLearnLengthRegister(t *testing.T) {
	// A word registered is asked for: it is learned whatever its length.
	s := newSim(t, Config{MaxLearnLength: 3})
	s.register = func(string) string { return "蜜柑缶詰" }
	s.typ("Mikankandume ")
	s.register = nil
	if s.line != "蜜柑缶詰" {
		t.Errorf("register: %q", s.line)
	}
	s.typ("Mikankandume ")
	if s.line != "蜜柑缶詰▼蜜柑缶詰" {
		t.Errorf("registered: %q", s.line)
	}
}
