package source

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Shougo/goda-cmp.vim/cmp"
	"github.com/Shougo/goda.vim/sdk"

	"github.com/Shougo/goda-skk.vim/skk"
)

// editor is the line the keys go to, as Vim has it.
type editor struct {
	t     *testing.T
	p     *skk.Plugin
	line  string
	on    bool
	calls []string
}

func (e *editor) Meta() sdk.Meta { return sdk.Meta{} }
func (e *editor) Call(_ context.Context, fn string, _ ...any) (any, error) {
	e.calls = append(e.calls, fn)
	return nil, nil
}
func (e *editor) Notify(context.Context, string, ...any) error { return nil }
func (e *editor) Eval(context.Context, string) (any, error)    { return nil, nil }
func (e *editor) Cmd(context.Context, string) error            { return nil }

func (e *editor) key(fn string, keys ...string) {
	e.t.Helper()
	status := map[string]any{"mode": "i", "prevInput": e.line, "bufnr": 1, "lnum": 1, "selected": -1}
	v, err := e.p.Handlers()["handle"](context.Background(), e, []any{fn, keys, status, e.on})
	if err != nil {
		e.t.Fatal(err)
	}
	r := v.(skk.Result)
	for _, c := range strings.ReplaceAll(r.Keys, "\x07u", "") {
		if c == '\b' {
			rs := []rune(e.line)
			e.line = string(rs[:len(rs)-1])
		} else {
			e.line += string(c)
		}
	}
	e.on = (e.on || r.Enable) && !r.Disable
}

func (e *editor) typ(s string) {
	e.t.Helper()
	for _, c := range s {
		k := string(c)
		if k == " " {
			k = "<space>"
		}
		e.key("handleKey", k)
	}
}

func newEditor(t *testing.T) *editor {
	t.Helper()
	dir := t.TempDir()
	jisyo := filepath.Join(dir, "SKK-JISYO")
	if err := os.WriteFile(jisyo, []byte(";; okuri-ari entries.\nおくr /送/贈/\n;; okuri-nasi entries.\nかんじ /漢字/感じ;feeling/\nかんがえ /考え/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := skk.NewJisyo(jisyo, "utf-8")
	if err := d.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	p, err := skk.New(skk.Config{Dictionaries: []skk.Dictionary{d}, UserDictionary: filepath.Join(dir, "user")})
	if err != nil {
		t.Fatal(err)
	}
	e := &editor{t: t, p: p}
	e.key("enable")
	return e
}

func gather(t *testing.T, s cmp.Source, e *editor) cmp.Gathered {
	t.Helper()
	g, err := s.Gather(context.Background(), e, &cmp.Context{Input: e.line, Mode: "i"})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func words(g cmp.Gathered) []string {
	var out []string
	for _, it := range g.Items {
		out = append(out, it.Word)
	}
	return out
}

func TestSource(t *testing.T) {
	e := newEditor(t)
	s, so := New(e.p), NewOkuri(e.p)
	if s.Name() != "skk" || so.Name() != "skk_okuri" {
		t.Errorf("names: %s %s", s.Name(), so.Name())
	}
	e.line = "x"
	e.typ("Kan")
	g := gather(t, s, e)
	if got := words(g); !slices.Equal(got, []string{"考え", "漢字", "感じ"}) || g.Start != 1 || !g.Incomplete {
		t.Errorf("gather: %v %+v", got, g)
	}
	if it := g.Items[2]; it.Info != "feeling" || it.FilterText != "▽かn" {
		t.Errorf("item: %+v", it)
	}
	// Not the line the reading is in.
	if g, _ := s.Gather(context.Background(), e, &cmp.Context{Input: "y", Mode: "i"}); len(g.Items) > 0 {
		t.Errorf("another line: %+v", g)
	}

	// Accepted: learned, and the reading is gone.
	it := g.Items[1]
	e.line = "x漢字"
	if err := s.(cmp.CompleteDoner).OnCompleteDone(context.Background(), e, cmp.DoneArgs{Item: it}); err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(e.calls, "goda_skk#_complete_done") {
		t.Errorf("calls: %v", e.calls)
	}
	if _, ok := e.p.Reading(); ok {
		t.Errorf("a reading after the completion")
	}
	e.typ("Kan")
	if got := words(gather(t, s, e)); got[0] != "漢字" {
		t.Errorf("learned: %v", got)
	}

	// Okurigana.
	e.key("cancel")
	e.typ("Okuri")
	if got := words(gather(t, so, e)); !slices.Equal(got, []string{"送り", "贈り"}) {
		t.Errorf("okuri: %v", got)
	}
	if got := words(gather(t, s, e)); len(got) != 0 {
		t.Errorf("okuri-nasi of おくり: %v", got)
	}
}
