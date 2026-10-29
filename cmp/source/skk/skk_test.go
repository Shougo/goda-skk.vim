package skk

import (
	"slices"
	"testing"

	"github.com/Shougo/goda-skk.vim/cmp/source/internal/sourcetest"
)

func TestSource(t *testing.T) {
	e := sourcetest.New(t)
	s := New(e.P)
	if s.Name() != "skk" {
		t.Errorf("name: %s", s.Name())
	}
	e.Line = "x"
	e.Type("Kan")
	g := e.Gather(s)
	if got := sourcetest.Words(g); !slices.Equal(got, []string{"考え", "漢字", "感じ"}) || g.Start != 1 || !g.Incomplete {
		t.Errorf("gather: %v %+v", got, g)
	}
	if it := g.Items[2]; it.Info != "feeling" || it.FilterText != "▽かn" {
		t.Errorf("item: %+v", it)
	}
	// Not the line the reading is in.
	line := e.Line
	e.Line = "y"
	if g := e.Gather(s); len(g.Items) > 0 {
		t.Errorf("another line: %+v", g)
	}
	e.Line = line

	// Accepted: learned, and the reading is gone.
	e.Accept(s, g.Items[1], "x漢字")
	if !slices.Contains(e.Calls, "goda_skk#_complete_done") {
		t.Errorf("calls: %v", e.Calls)
	}
	if _, ok := e.P.Reading(); ok {
		t.Errorf("a reading after the completion")
	}
	e.Type("Kan")
	if got := sourcetest.Words(e.Gather(s)); got[0] != "漢字" {
		t.Errorf("learned: %v", got)
	}
	// Nothing for a reading without candidates.
	e.Key("cancel")
	e.Type("Okuri")
	if got := sourcetest.Words(e.Gather(s)); len(got) != 0 {
		t.Errorf("おくり: %v", got)
	}
}
