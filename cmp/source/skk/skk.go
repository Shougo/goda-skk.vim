// Package skk is the goda-cmp.vim source "skk" of goda-skk.vim: it completes
// the reading being typed (▽) with the candidates of the readings that start
// with it, as skkeleton's ddc.vim source skkeleton did. Those learned in this
// Vim come first. Package skkokuri has the source with okurigana.
//
// The source asks the input method directly, so the input method must be
// in the binary of goda-cmp.vim:
//
//	import (
//		"github.com/Shougo/goda-skk.vim/cmp/source/skk"
//		"github.com/Shougo/goda-skk.vim/cmp/source/skkokuri"
//		"github.com/Shougo/goda-skk.vim/engine"
//	)
//
//	p, err := engine.New(engine.Config{...})
//	b := cmp.New()
//	b.AddSource(skk.New(p))
//	b.AddSource(skkokuri.New(p))
//	...
//	cmpPlugin, err := b.Plugin()
//	host.Main(cmpPlugin, p)
//
// The items replace the pre-edit, marker included. Give the source no
// matchers and no sorters: the items are ordered already, and do not start
// with what is typed. The source says where the reading starts
// (cmp.CompletePositioner), whatever the keywordPattern.
//
//	"skk": {Matchers: []string{}, Sorters: []string{}},
package skk

import (
	"context"
	"strings"

	"github.com/Shougo/goda-cmp.vim/cmp"
	"github.com/Shougo/goda.vim/sdk"

	"github.com/Shougo/goda-skk.vim/engine"
)

type data struct {
	reading   string
	candidate string
	inserted  string
}

type source struct{ p *engine.Plugin }

// New returns the source, named "skk", of the input method p.
func New(p *engine.Plugin) cmp.Source { return source{p} }

func (source) Name() string { return "skk" }

// CompletePosition is where the pre-edit starts, -1 without a reading.
func (s source) CompletePosition(_ context.Context, _ sdk.Host, c *cmp.Context) int {
	r, ok := s.p.Reading()
	if !ok || r.PreEdit == "" || r.VimMode != c.Mode || !strings.HasSuffix(c.Input, r.PreEdit) {
		return -1
	}
	return len(c.Input) - len(r.PreEdit)
}

func (s source) Gather(ctx context.Context, _ sdk.Host, c *cmp.Context) (cmp.Gathered, error) {
	r, ok := s.p.Reading()
	// The reading changes with every key: ask again each time.
	g := cmp.Gathered{Start: len(c.Input), Incomplete: true}
	if !ok || r.PreEdit == "" || r.VimMode != c.Mode || !strings.HasSuffix(c.Input, r.PreEdit) {
		return g, nil
	}
	g.Start = len(c.Input) - len(r.PreEdit)
	for _, co := range s.p.Complete(ctx, r) {
		word, note, _ := strings.Cut(co.Candidate, ";")
		if word == "" {
			continue
		}
		g.Items = append(g.Items, cmp.Item{
			Word: word,
			Info: note,
			Dup:  1,
			// Matchers that compare with what is typed keep the items.
			FilterText: r.PreEdit,
			UserData:   data{co.Reading, co.Candidate, word},
		})
	}
	return g, nil
}

// OnCompleteDone learns the candidate, which kakuteiUndo can take back.
func (s source) OnCompleteDone(ctx context.Context, h sdk.Host, a cmp.DoneArgs) error {
	if d, ok := a.Item.UserData.(data); ok {
		s.p.CompleteDone(ctx, h, engine.OkuriNasi, d.reading, d.candidate, d.inserted)
	}
	return nil
}

var (
	_ cmp.Source             = source{}
	_ cmp.CompleteDoner      = source{}
	_ cmp.CompletePositioner = source{}
)
