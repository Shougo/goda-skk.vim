package engine

import (
	"context"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

// userJisyo is the user dictionary of s.
func userJisyo(t *testing.T, s *sim) string {
	t.Helper()
	b, err := os.ReadFile(s.p.cfg.UserDictionary)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return string(b)
}

func TestNoLearn(t *testing.T) {
	phrase := memDict{nasi: map[string][]string{
		"はいたつぎょうしゃ": {"配達業者"},
		"へんかん":      {"変換", "偏官"},
	}}
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict, NoLearn(phrase)}})
	s.typ("Haitatsugyousha <c-j>")
	if s.line != "配達業者" {
		t.Errorf("line: %q", s.line)
	}
	if j := userJisyo(t, s); strings.Contains(j, "はいたつぎょうしゃ") {
		t.Errorf("a candidate of NoLearn only is learned:\n%s", j)
	}
	// A candidate another dictionary gives too is learned.
	s.line = ""
	s.typ("Henkan <c-j>")
	if s.line != "変換" {
		t.Errorf("line: %q", s.line)
	}
	if j := userJisyo(t, s); !strings.Contains(j, "\nへんかん /変換/\n") {
		t.Errorf("a candidate of both is not learned:\n%s", j)
	}
	// One that only NoLearn gives is not, though the reading is in both.
	s.line = ""
	s.typ("Henkan   <c-j>")
	if s.line != "偏官" {
		t.Errorf("line: %q", s.line)
	}
	if j := userJisyo(t, s); strings.Contains(j, "偏官") {
		t.Errorf("a candidate of NoLearn only is learned:\n%s", j)
	}
}

func TestNoLearnComplete(t *testing.T) {
	phrase := memDict{nasi: map[string][]string{"かんじょう": {"勘定"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict, NoLearn(phrase)}})
	ctx := context.Background()
	s.typ("Kan")
	r, _ := s.p.Reading()
	if cs := s.p.Complete(ctx, r); !slices.Contains(cs, Completion{"かんじょう", "勘定"}) {
		t.Fatalf("complete: %v", cs)
	}
	s.line = "勘定"
	s.p.CompleteDone(ctx, s, OkuriNasi, "かんじょう", "勘定", "勘定")
	if j := userJisyo(t, s); strings.Contains(j, "勘定") {
		t.Errorf("a completion of NoLearn only is learned:\n%s", j)
	}
	// That of another dictionary is.
	s.typ("Kan")
	r, _ = s.p.Reading()
	s.p.Complete(ctx, r)
	s.line = "勘定感じ"
	s.p.CompleteDone(ctx, s, OkuriNasi, "かんじ", "感じ", "感じ")
	if j := userJisyo(t, s); !strings.Contains(j, "\nかんじ /感じ/\n") {
		t.Errorf("a completion is not learned:\n%s", j)
	}
}

func TestNoLearnRegister(t *testing.T) {
	// A word registered is learned, though NoLearn gives it.
	phrase := memDict{nasi: map[string][]string{"みかん": {"未完"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict, NoLearn(phrase)}})
	s.typ("Mikan ")
	s.register = func(string) string { return "蜜柑" }
	s.typ(" ")
	s.register = nil
	if s.line != "蜜柑" {
		t.Errorf("register: %q", s.line)
	}
	if j := userJisyo(t, s); !strings.Contains(j, "\nみかん /蜜柑/\n") {
		t.Errorf("not registered:\n%s", j)
	}
}

func TestFallback(t *testing.T) {
	ctx := context.Background()
	phrase := memDict{nasi: map[string][]string{
		"かんじ":  {"幹字"},
		"はいたつ": {"配達"},
		"#かい":  {"#1回"},
		"すいとう": {"水筒"},
	}}
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict, Fallback(phrase)}})
	// The dictionaries before it give candidates: it is not looked up.
	if cs := s.p.lib.lookup(ctx, OkuriNasi, "かんじ", ""); slices.Contains(cs, "幹字") {
		t.Errorf("looked up after candidates: %v", cs)
	}
	// They give none.
	if cs := s.p.lib.lookup(ctx, OkuriNasi, "はいたつ", ""); !slices.Equal(cs, []string{"配達"}) {
		t.Errorf("not looked up: %v", cs)
	}
	if cs := s.p.lib.lookup(ctx, OkuriNasi, "1かい", ""); !slices.Equal(cs, []string{"１回"}) {
		t.Errorf("digits: %v", cs)
	}
	// The user dictionary is before it.
	s.p.lib.register(OkuriNasi, "すいとう", "出納")
	if cs := s.p.lib.lookup(ctx, OkuriNasi, "すいとう", ""); !slices.Equal(cs, []string{"出納"}) {
		t.Errorf("after the user dictionary: %v", cs)
	}
	// Completion: only when the others complete nothing.
	if es := s.p.lib.complete(ctx, []string{"かん"}); slices.ContainsFunc(es, func(e Entry) bool { return slices.Contains(e.Candidates, "幹字") }) {
		t.Errorf("completed after candidates: %v", es)
	}
	if es := s.p.lib.complete(ctx, []string{"はい"}); len(es) != 1 || es[0].Word != "はいたつ" {
		t.Errorf("not completed: %v", es)
	}
}

func TestFallbackChain(t *testing.T) {
	ctx := context.Background()
	a := memDict{nasi: map[string][]string{"はいたつ": {"配達"}}}
	b := memDict{nasi: map[string][]string{"はいたつ": {"拝達"}, "ほげ": {"補下"}}}
	s := newSim(t, Config{Dictionaries: []Dictionary{testDict, Fallback(a), Fallback(b)}})
	if cs := s.p.lib.lookup(ctx, OkuriNasi, "はいたつ", ""); !slices.Equal(cs, []string{"配達"}) {
		t.Errorf("the second is looked up: %v", cs)
	}
	if cs := s.p.lib.lookup(ctx, OkuriNasi, "ほげ", ""); !slices.Equal(cs, []string{"補下"}) {
		t.Errorf("the second is not looked up: %v", cs)
	}
}

// initDict is an Initializer.
type initDict struct {
	memDict
	done chan struct{}
}

func (d initDict) Init(context.Context) error {
	close(d.done)
	return nil
}

// closeDict is an io.Closer.
type closeDict struct {
	memDict
	closed *bool
}

func (d closeDict) Close() error {
	*d.closed = true
	return nil
}

func TestOptionClose(t *testing.T) {
	var closed bool
	s := newSim(t, Config{Dictionaries: []Dictionary{NoLearn(closeDict{closed: &closed})}})
	if err := s.p.Close(context.Background()); err != nil || !closed {
		t.Errorf("Close: %v, closed %v", err, closed)
	}
}

func TestOptionKeepsInterfaces(t *testing.T) {
	var got []string
	d := ctxDict{memDict{nasi: map[string][]string{"きしゃ": {"記者"}}}, &got}
	in := initDict{done: make(chan struct{})}
	s := newSim(t, Config{Dictionaries: []Dictionary{
		testDict, Fallback(NoLearn(d)), NoLearn(Fallback(in)),
	}})
	select {
	case <-in.done:
	case <-time.After(5 * time.Second):
		t.Fatal("Init is not called")
	}
	if !s.p.lib.hasContext() {
		t.Errorf("hasContext: false")
	}
	s.line = "新聞の"
	s.typ("Kisha <c-j>")
	if s.line != "新聞の記者" || !slices.Equal(got, []string{"きしゃ|新聞の"}) {
		t.Errorf("line %q, before %q", s.line, got)
	}
	// Both options, in either order.
	for _, o := range []Dictionary{Fallback(NoLearn(d)), NoLearn(Fallback(d))} {
		if _, opt := unwrap(o); !opt.noLearn || !opt.fallback {
			t.Errorf("options: %+v", opt)
		}
	}
	if j := userJisyo(t, s); strings.Contains(j, "記者") {
		t.Errorf("learned:\n%s", j)
	}
}
