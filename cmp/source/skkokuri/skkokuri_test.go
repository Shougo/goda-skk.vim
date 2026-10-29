package skkokuri

import (
	"slices"
	"testing"

	"github.com/Shougo/goda-skk.vim/cmp/source/internal/sourcetest"
)

func TestSource(t *testing.T) {
	e := sourcetest.New(t)
	s := New(e.P)
	if s.Name() != "skkokuri" {
		t.Errorf("name: %s", s.Name())
	}
	e.Type("Okuri")
	g := e.Gather(s)
	if got := sourcetest.Words(g); !slices.Equal(got, []string{"送り", "贈り"}) || g.Start != 0 {
		t.Errorf("gather: %v %+v", got, g)
	}
	// A romaji not yet kana: nothing.
	e.Key("cancel")
	e.Type("Okur")
	if got := sourcetest.Words(e.Gather(s)); len(got) != 0 {
		t.Errorf("with a feed: %v", got)
	}
	e.Type("i")
	e.Accept(s, e.Gather(s).Items[1], "贈り")
	if _, ok := e.P.Reading(); ok {
		t.Errorf("a reading after the completion")
	}
	e.Type("Okuri")
	if got := sourcetest.Words(e.Gather(s)); got[0] != "贈り" {
		t.Errorf("learned: %v", got)
	}
}
