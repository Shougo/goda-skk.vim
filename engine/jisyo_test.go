package engine

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/text/encoding/japanese"
)

const sampleJisyo = `;; -*- mode: fundamental; coding: euc-jp -*-
;; okuri-ari entries.
おくr /送/贈/
はしr /走/[る/走/]/
;; okuri-nasi entries.
かっこ /[/]/「/
かんじ /漢字/感じ;feeling/
かんじょう /感情/勘定/
`

func TestParseJisyo(t *testing.T) {
	j := parseJisyo(sampleJisyo)
	for _, tc := range []struct {
		typ  HenkanType
		word string
		want []string
	}{
		{OkuriAri, "おくr", []string{"送", "贈"}},
		{OkuriAri, "はしr", []string{"走"}}, // the okuri block is no candidate
		{OkuriNasi, "かっこ", []string{"[", "]", "「"}},
		{OkuriNasi, "かんじ", []string{"漢字", "感じ;feeling"}},
		{OkuriNasi, "ない", nil},
	} {
		if got := j.lookup(tc.typ, tc.word); !slices.Equal(got, tc.want) {
			t.Errorf("%s %q: %q, want %q", tc.typ, tc.word, got, tc.want)
		}
	}
	var words []string
	for _, e := range j.complete("かんじ") {
		words = append(words, e.Word)
	}
	if !slices.Equal(words, []string{"かんじ", "かんじょう"}) {
		t.Errorf("complete: %q", words)
	}
}

func TestJisyoEncodings(t *testing.T) {
	dir := t.TempDir()
	text := okuriNasiMarker + "\nから /〜/\nかんじ /漢字/\n"
	// EUC-JP has the wave dash where Go's decoder puts the full-width
	// tilde.
	euc, err := japanese.EUCJP.NewEncoder().String(strings.ReplaceAll(text, "〜", "～"))
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{"euc": euc, "utf8": text}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct{ file, enc string }{
		{"euc", "euc-jp"}, {"euc", ""}, {"utf8", "utf-8"}, {"utf8", ""},
	} {
		d := NewJisyo(filepath.Join(dir, tc.file), tc.enc)
		got, err := d.Lookup(context.Background(), OkuriNasi, "かんじ")
		if err != nil || !slices.Equal(got, []string{"漢字"}) {
			t.Errorf("%s (%q): %q, %v", tc.file, tc.enc, got, err)
		}
		// The wave dash stays the wave dash.
		if got, _ := d.Lookup(context.Background(), OkuriNasi, "から"); !slices.Equal(got, []string{"〜"}) {
			t.Errorf("%s (%q): wave dash %q", tc.file, tc.enc, got)
		}
	}
	if _, err := NewJisyo(filepath.Join(dir, "euc"), "sjis").Lookup(context.Background(), OkuriNasi, "x"); err == nil {
		t.Error("an unsupported encoding was accepted")
	}
	if _, err := NewJisyo(filepath.Join(dir, "none"), "").Lookup(context.Background(), OkuriNasi, "x"); err == nil {
		t.Error("a missing file gave no error")
	}
}

func TestConvertNumber(t *testing.T) {
	for _, tc := range []struct{ pattern, word, want string }{
		{"#0年", "2026ねん", "2026年"},
		{"#1年", "2026ねん", "２０２６年"},
		{"#2年", "2026ねん", "二〇二六年"},
		{"#3年", "2026ねん", "二千二十六年"},
		{"#3円", "10000えん", "一万円"},
		{"#3円", "1234えん", "千二百三十四円"},
		{"#5円", "1234えん", "阡弐佰参拾肆円"},
		{"#8", "2026", "MMXXVI"},
		{"#9歩", "34ふ", "３四歩"},
		{"#1月#1日", "10がつ26にち", "１０月２６日"},
	} {
		if got := convertNumber(tc.pattern, tc.word); got != tc.want {
			t.Errorf("%q %q: %q, want %q", tc.pattern, tc.word, got, tc.want)
		}
	}
	if got := numberWord("10がつ26にち"); got != "#がつ#にち" {
		t.Errorf("numberWord: %q", got)
	}
}

func TestGraphemes(t *testing.T) {
	for s, want := range map[string]int{"": 0, "▽かな": 3, "が": 1, "が": 1, "👍🏻": 1, "é": 1} {
		if got := graphemes(s); got != want {
			t.Errorf("%q: %d, want %d", s, got, want)
		}
	}
}
