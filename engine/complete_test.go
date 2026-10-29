package engine

import (
	"context"
	"slices"
	"testing"
)

func TestComplete(t *testing.T) {
	s := newSim(t, Config{})
	ctx := context.Background()
	if _, ok := s.p.Reading(); ok {
		t.Errorf("a reading before typing one")
	}
	s.typ("Ka")
	r, ok := s.p.Reading()
	if !ok || r.PreEdit != "▽か" || r.Kana != "か" || r.Feed != "" || r.VimMode != "i" {
		t.Errorf("reading: %+v %v", r, ok)
	}
	words := func(cs []Completion) []string {
		var out []string
		for _, c := range cs {
			out = append(out, c.Candidate)
		}
		return out
	}
	got := words(s.p.Complete(ctx, r))
	want := []string{"漢字", "感じ", "幹事", "監事", "完治", "寛治", "莞爾", "冠辞", "環時"}
	if !slices.Equal(got, want) {
		t.Errorf("complete: %v", got)
	}
	// The kana the feed goes on with.
	s.typ("<c-g>Hen")
	r, _ = s.p.Reading()
	if r.Kana != "へ" || r.Feed != "n" {
		t.Errorf("reading: %+v", r)
	}
	if got := words(s.p.Complete(ctx, r)); !slices.Equal(got, []string{"変換", "返還"}) {
		t.Errorf("complete with a feed: %v", got)
	}
	// What was learned comes first.
	s.typ("<c-g>Kanji     a")
	s.typ("Ka")
	r, _ = s.p.Reading()
	if got := words(s.p.Complete(ctx, r)); got[0] != "完治" {
		t.Errorf("learned first: %v", got)
	}
	// Okurigana.
	s.typ("<c-g>Okuri")
	r, _ = s.p.Reading()
	oc := s.p.CompleteOkuri(ctx, r)
	if len(oc) != 1 || oc[0] != (OkuriCompletion{"おくr", "送", "り"}) {
		t.Errorf("okuri: %+v", oc)
	}
	// Choosing a candidate: no reading.
	s.typ("<c-g>Kanji ")
	if _, ok := s.p.Reading(); ok {
		t.Errorf("a reading while choosing")
	}
}

func TestCompleteDone(t *testing.T) {
	s := newSim(t, Config{Keymaps: map[string]map[string]string{"input": {"<c-z>": "kakuteiUndo"}}})
	ctx := context.Background()
	s.typ("Kan")
	// The completion replaces the pre-edit, and tells.
	s.line = "感じ"
	s.p.CompleteDone(ctx, s, OkuriNasi, "かんじ", "感じ", "感じ")
	if r, ok := s.p.Reading(); ok {
		t.Errorf("reading after the completion: %+v", r)
	}
	if got := s.p.lib.lookup(ctx, OkuriNasi, "かんじ"); got[0] != "感じ" {
		t.Errorf("learned: %v", got)
	}
	// kakuteiUndo takes it back.
	s.typ("<c-z>")
	if s.line != "▼感じ" {
		t.Errorf("undo: %q", s.line)
	}
	s.typ(" <c-j>")
	if s.line != "漢字" {
		t.Errorf("another: %q", s.line)
	}
	// With okurigana.
	s.typ("Oku")
	s.line = "漢字送り"
	s.p.CompleteDone(ctx, s, OkuriAri, "おくr", "送", "送り")
	s.typ("<c-z>")
	if s.line != "漢字▼送り" || s.res.HenkanFeed != "おく" {
		t.Errorf("okuri undo: %q %+v", s.line, s.res)
	}
}
