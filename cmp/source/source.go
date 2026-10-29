// Package source is the goda-cmp.vim completion source of goda-skk.vim: it
// completes the reading being typed (▽) with the candidates of the readings
// that start with it ("skk"), and with okurigana ("skk_okuri"), as
// skkeleton's ddc.vim sources did.
//
// The sources ask the input method directly, so the skk plugin must be in
// the same binary as goda-cmp:
//
//	p, err := skk.New(skk.Config{...})
//	b := cmp.New()
//	b.AddSource(source.New(p))
//	b.AddSource(source.NewOkuri(p))
//	...
//	engine, err := b.Plugin()
//	host.Main(engine, p)
//
// The items replace the pre-edit, marker included. Give the sources no
// matchers and no sorters: they are ordered already (what was learned
// first), and do not start with what is typed.
package source

import (
	"context"
	"strings"

	"github.com/Shougo/goda-cmp.vim/cmp"
	"github.com/Shougo/goda-skk.vim/skk"
	"github.com/Shougo/goda.vim/sdk"
)

// Name is the name of the source of New; NameOkuri that of NewOkuri.
const (
	Name      = "skk"
	NameOkuri = "skk_okuri"
)

type data struct {
	typ       skk.HenkanType
	reading   string
	candidate string
	inserted  string
}

type source struct {
	p     *skk.Plugin
	okuri bool
}

// New is the source of the candidates of the readings that start with the
// one typed.
func New(p *skk.Plugin) cmp.Source { return &source{p: p} }

// NewOkuri is the source of the candidates with okurigana of the reading
// typed ("たべる" gives 食べる).
func NewOkuri(p *skk.Plugin) cmp.Source { return &source{p: p, okuri: true} }

func (s *source) Name() string {
	if s.okuri {
		return NameOkuri
	}
	return Name
}

// split is a candidate without its annotation, and the annotation.
func split(c string) (string, string) {
	word, note, _ := strings.Cut(c, ";")
	return word, note
}

func (s *source) Gather(ctx context.Context, _ sdk.Host, c *cmp.Context) (cmp.Gathered, error) {
	none := cmp.Gathered{Start: len(c.Input), Incomplete: true}
	r, ok := s.p.Reading()
	if !ok || r.PreEdit == "" || r.VimMode != c.Mode || !strings.HasSuffix(c.Input, r.PreEdit) {
		return none, nil
	}
	if s.okuri && r.Feed != "" {
		return none, nil
	}
	g := cmp.Gathered{
		Start: len(c.Input) - len(r.PreEdit),
		// The reading changes with every key: ask again each time.
		Incomplete: true,
	}
	add := func(d data, note string) {
		g.Items = append(g.Items, cmp.Item{
			Word: d.inserted,
			Info: note,
			Dup:  1,
			// Matchers that compare with what is typed keep the items.
			FilterText: r.PreEdit,
			UserData:   d,
		})
	}
	if s.okuri {
		for _, oc := range s.p.CompleteOkuri(ctx, r) {
			word, note := split(oc.Candidate)
			add(data{skk.OkuriAri, oc.Reading, oc.Candidate, word + oc.Okuri}, note)
		}
		return g, nil
	}
	for _, co := range s.p.Complete(ctx, r) {
		word, note := split(co.Candidate)
		if word == "" {
			continue
		}
		add(data{skk.OkuriNasi, co.Reading, co.Candidate, word}, note)
	}
	return g, nil
}

// OnCompleteDone learns the candidate, which kakuteiUndo can take back.
func (s *source) OnCompleteDone(ctx context.Context, h sdk.Host, a cmp.DoneArgs) error {
	d, ok := a.Item.UserData.(data)
	if !ok {
		return nil
	}
	s.p.CompleteDone(ctx, h, d.typ, d.reading, d.candidate, d.inserted)
	return nil
}

var (
	_ cmp.Source        = (*source)(nil)
	_ cmp.CompleteDoner = (*source)(nil)
)
