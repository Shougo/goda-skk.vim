// Package skkokuri is the goda-cmp.vim source "skkokuri" of goda-skk.vim: it
// completes the reading being typed (▽) with the candidates with okurigana
// of its splits ("おくり" gives 送り and 贈り, from "おくr"), as skkeleton's
// ddc.vim source skkeleton_okuri did. See package skk (its sibling) for the
// configuration.
package skkokuri

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

// New returns the source, named "skkokuri", of the input method p.
func New(p *engine.Plugin) cmp.Source { return source{p} }

func (source) Name() string { return "skkokuri" }

func (s source) Gather(ctx context.Context, _ sdk.Host, c *cmp.Context) (cmp.Gathered, error) {
	r, ok := s.p.Reading()
	// The reading changes with every key: ask again each time.
	g := cmp.Gathered{Start: len(c.Input), Incomplete: true}
	if !ok || r.PreEdit == "" || r.Feed != "" || r.VimMode != c.Mode || !strings.HasSuffix(c.Input, r.PreEdit) {
		return g, nil
	}
	g.Start = len(c.Input) - len(r.PreEdit)
	for _, oc := range s.p.CompleteOkuri(ctx, r) {
		word, note, _ := strings.Cut(oc.Candidate, ";")
		g.Items = append(g.Items, cmp.Item{
			Word: word + oc.Okuri,
			Info: note,
			Dup:  1,
			// Matchers that compare with what is typed keep the items.
			FilterText: r.PreEdit,
			UserData:   data{oc.Reading, oc.Candidate, word + oc.Okuri},
		})
	}
	return g, nil
}

// OnCompleteDone learns the candidate, which kakuteiUndo can take back.
func (s source) OnCompleteDone(ctx context.Context, h sdk.Host, a cmp.DoneArgs) error {
	if d, ok := a.Item.UserData.(data); ok {
		s.p.CompleteDone(ctx, h, engine.OkuriAri, d.reading, d.candidate, d.inserted)
	}
	return nil
}

var (
	_ cmp.Source        = source{}
	_ cmp.CompleteDoner = source{}
)
