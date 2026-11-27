package engine

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
)

// slowDict is a dictionary read when told (ready).
type slowDict struct {
	memDict
	read *atomic.Bool
}

func (d slowDict) ready() bool { return d.read.Load() }

func TestNoWait(t *testing.T) {
	var read atomic.Bool
	main := memDict{nasi: map[string][]string{"かんじ": {"漢字"}}}
	large := slowDict{memDict{nasi: map[string][]string{"かんじ": {"幹事"}, "かんじゃ": {"患者"}}}, &read}
	s := newSim(t, Config{Dictionaries: []Dictionary{main, NoWait(NoLearn(large))}})
	ctx := context.Background()
	words := func(es []Entry) []string {
		var out []string
		for _, e := range es {
			out = append(out, e.Word)
		}
		return out
	}
	// Being read: skipped, in a conversion and a completion.
	if got, want := s.p.lib.lookup(ctx, OkuriNasi, "かんじ", nil), []string{"漢字"}; !slices.Equal(got, want) {
		t.Errorf("being read: %q, want %q", got, want)
	}
	if got, want := words(s.p.lib.complete(ctx, []string{"かんじ"})), []string{"かんじ"}; !slices.Equal(got, want) {
		t.Errorf("complete being read: %q, want %q", got, want)
	}
	// Read: looked up.
	read.Store(true)
	if got, want := s.p.lib.lookup(ctx, OkuriNasi, "かんじ", nil), []string{"漢字", "幹事"}; !slices.Equal(got, want) {
		t.Errorf("read: %q, want %q", got, want)
	}
	if got, want := words(s.p.lib.complete(ctx, []string{"かんじ"})), []string{"かんじ", "かんじゃ"}; !slices.Equal(got, want) {
		t.Errorf("complete read: %q, want %q", got, want)
	}
	// Without NoWait: looked up (Jisyo waits for its file).
	read.Store(false)
	s = newSim(t, Config{Dictionaries: []Dictionary{main, large}})
	if got, want := s.p.lib.lookup(ctx, OkuriNasi, "かんじ", nil), []string{"漢字", "幹事"}; !slices.Equal(got, want) {
		t.Errorf("waited: %q, want %q", got, want)
	}
}

func TestJisyoReady(t *testing.T) {
	path := filepath.Join(t.TempDir(), "SKK-JISYO.test")
	if err := os.WriteFile(path, []byte(okuriAriMarker+"\n"+okuriNasiMarker+"\nかんじ /幹事/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	d := NewJisyo(path, "")
	if d.ready() {
		t.Error("ready before Init")
	}
	if err := d.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !d.ready() {
		t.Error("not ready after Init")
	}
}
