// Package sourcetest is what the tests of the completion sources share: an
// editor that types into a line through the input method.
package sourcetest

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Shougo/goda-cmp.vim/cmp"
	"github.com/Shougo/goda.vim/sdk"

	"github.com/Shougo/goda-skk.vim/engine"
)

// Editor is the line the keys go to, as Vim has it, and the functions of
// Vim the plugin called.
type Editor struct {
	t     *testing.T
	P     *engine.Plugin
	Line  string
	Calls []string
	on    bool
}

func (e *Editor) Meta() sdk.Meta { return sdk.Meta{} }
func (e *Editor) Call(_ context.Context, fn string, _ ...any) (any, error) {
	e.Calls = append(e.Calls, fn)
	return nil, nil
}
func (e *Editor) Notify(context.Context, string, ...any) error { return nil }
func (e *Editor) Eval(context.Context, string) (any, error)    { return nil, nil }
func (e *Editor) Cmd(context.Context, string) error            { return nil }

// Key runs fn of the plugin for keys and types its answer.
func (e *Editor) Key(fn string, keys ...string) {
	e.t.Helper()
	status := map[string]any{"mode": "i", "prevInput": e.Line, "bufnr": 1, "lnum": 1, "selected": -1}
	v, err := e.P.Handlers()["handle"](context.Background(), e, []any{fn, keys, status, e.on})
	if err != nil {
		e.t.Fatal(err)
	}
	r := v.(engine.Result)
	for _, c := range strings.ReplaceAll(r.Keys, "\x07u", "") {
		if c == '\b' {
			rs := []rune(e.Line)
			e.Line = string(rs[:len(rs)-1])
		} else {
			e.Line += string(c)
		}
	}
	e.on = (e.on || r.Enable) && !r.Disable
}

// Type types the characters of s (" " is <Space>).
func (e *Editor) Type(s string) {
	e.t.Helper()
	for _, c := range s {
		k := string(c)
		if k == " " {
			k = "<space>"
		}
		e.Key("handleKey", k)
	}
}

// New returns an editor with the input method enabled, with a dictionary
// of a few words.
func New(t *testing.T) *Editor {
	t.Helper()
	dir := t.TempDir()
	jisyo := filepath.Join(dir, "SKK-JISYO")
	if err := os.WriteFile(jisyo, []byte(";; okuri-ari entries.\nおくr /送/贈/\n;; okuri-nasi entries.\nかんじ /漢字/感じ;feeling/\nかんがえ /考え/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := engine.NewJisyo(jisyo, "utf-8")
	if err := d.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	p, err := engine.New(engine.Config{Dictionaries: []engine.Dictionary{d}, UserDictionary: filepath.Join(dir, "user")})
	if err != nil {
		t.Fatal(err)
	}
	e := &Editor{t: t, P: p}
	e.Key("enable")
	return e
}

// Gather asks s for the items of the line.
func (e *Editor) Gather(s cmp.Source) cmp.Gathered {
	e.t.Helper()
	g, err := s.Gather(context.Background(), e, &cmp.Context{Input: e.Line, Mode: "i"})
	if err != nil {
		e.t.Fatal(err)
	}
	return g
}

// Words are the words of the items.
func Words(g cmp.Gathered) []string {
	var out []string
	for _, it := range g.Items {
		out = append(out, it.Word)
	}
	return out
}

// Accept tells s that the item was accepted, with the line it gave.
func (e *Editor) Accept(s cmp.Source, it cmp.Item, line string) {
	e.t.Helper()
	e.Line = line
	if err := s.(cmp.CompleteDoner).OnCompleteDone(context.Background(), e, cmp.DoneArgs{Item: it}); err != nil {
		e.t.Fatal(err)
	}
}
