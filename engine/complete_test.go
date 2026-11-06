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

func TestCompleteDoneAfterKey(t *testing.T) {
	s := newSim(t, Config{})
	ctx := context.Background()
	s.typ("Kan")
	// The completion is accepted by typing on: the key is handled before
	// the completion tells.
	s.line = "感じ"
	s.pum = true
	s.typ("d")
	s.pum = false
	s.p.CompleteDone(ctx, s, OkuriNasi, "かんじ", "感じ", "感じ")
	s.typ("esu")
	if s.line != "感じです" {
		t.Errorf("typing on: %q", s.line)
	}
	if got := s.p.lib.lookup(ctx, OkuriNasi, "かんじ"); got[0] != "感じ" {
		t.Errorf("learned: %v", got)
	}
	// Typing on with a new reading.
	s.typ("Kan")
	s.line = "感じです感じ"
	s.pum = true
	s.typ("O")
	s.pum = false
	s.p.CompleteDone(ctx, s, OkuriNasi, "かんじ", "感じ", "感じ")
	s.typ("ku")
	if s.line != "感じです感じ▽おく" {
		t.Errorf("a new reading: %q", s.line)
	}
}

// EggLikeNewline: <CR> with an item selected has Vim choose it, without
// a new line.
func TestEggLikeNewlineConfirm(t *testing.T) {
	s := newSim(t, Config{EggLikeNewline: true})
	s.typ("Kan")
	s.line = "感じ"
	s.pum, s.selected = true, 0
	s.typ("<cr>")
	s.pum, s.selected = false, -1
	if !s.res.CompletionConfirm || s.res.Keys != "" || s.line != "感じ" || s.preedit() != "" {
		t.Errorf("confirm: %+v %q %q", s.res, s.line, s.preedit())
	}
	// Nothing selected: <CR> as before.
	s.typ("Kan")
	s.pum = true
	s.typ("<cr>")
	s.pum = false
	if s.res.CompletionConfirm || s.line != "感じかん" {
		t.Errorf("not selected: %v %q", s.res.CompletionConfirm, s.line)
	}
}

func TestCompleteDoneAfterKeys(t *testing.T) {
	s := newSim(t, Config{})
	ctx := context.Background()
	s.typ("Kan")
	s.p.Reading()
	s.line = "感じ"
	// Two keys before the completion tells: a reading the completed one
	// starts with.
	s.pum = true
	s.typ("K")
	s.pum = false
	s.typ("a")
	s.p.CompleteDone(ctx, s, OkuriNasi, "かんじ", "感じ", "感じ")
	s.typ("ku")
	if s.line != "感じ▽かく" || s.preedit() != "▽かく" {
		t.Errorf("the reading is lost: %q %q", s.line, s.preedit())
	}
}

// The key typed on has the completion gather again before it tells it was
// done: what the key typed stays (jj after a completion was "jj").
func TestCompleteDoneAfterGather(t *testing.T) {
	s := newSim(t, Config{KanaTable: RomTable.With(Rule{From: "jj", Func: "escape"})})
	ctx := context.Background()
	s.typ("Ni")
	s.p.Reading()
	s.line = "日本"
	s.pum = true
	s.typ("j")
	s.pum = false
	if _, ok := s.p.Reading(); ok {
		t.Error("a reading")
	}
	s.p.CompleteDone(ctx, s, OkuriNasi, "にほん", "日本", "日本")
	s.typ("j")
	if s.line != "日本" || s.res.Phase != "escape" {
		t.Errorf("jj: %q %q", s.line, s.res.Phase)
	}
	// A completion accepted without typing on resets the reading.
	s = newSim(t, Config{})
	s.typ("Ni")
	s.p.Reading()
	s.line = "日本"
	s.p.CompleteDone(ctx, s, OkuriNasi, "にほん", "日本", "日本")
	if got := s.p.c.state.phase(); got != "input" {
		t.Errorf("accepted: %q", got)
	}
}
