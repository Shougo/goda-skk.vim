package engine

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

func TestConvertLog(t *testing.T) {
	jisyo := memDict{nasi: map[string][]string{"きしゃ": {"記者", "汽車"}}}
	var asked []string
	path := filepath.Join(t.TempDir(), "convert.log")
	s := newSim(t, Config{Dictionaries: []Dictionary{jisyo, Prefer(sureDict{asked: &asked})}, ConvertLog: path})
	s.register = func(string) string { return "騎射" }
	s.line = "駅の"
	s.typ("Kisha <c-j>")
	s.typ("Kisha <c-g>")
	s.typ("Kisha   ")

	var got []convertRecord
	for _, l := range readLines(t, path) {
		var r convertRecord
		if err := json.Unmarshal([]byte(l), &r); err != nil {
			t.Fatal(err)
		}
		got = append(got, r)
	}
	if len(got) != 3 {
		t.Fatalf("records: %+v", got)
	}
	both := []string{"1:engine.memDict", "2:engine.sureDict"}
	for i, want := range []convertRecord{
		// Prefer moved 汽車 first, with the text before.
		{Event: "confirm", Candidate: "汽車", Index: 0, Sources: both, First: "記者", Preferred: "汽車"},
		{Event: "cancel", Index: 0},
		{Event: "register", Candidate: "騎射", Index: 0},
	} {
		r := got[i]
		if r.Event != want.Event || r.Candidate != want.Candidate || r.Index != want.Index ||
			!slices.Equal(r.Sources, want.Sources) || r.First != want.First || r.Preferred != want.Preferred {
			t.Errorf("%d: %+v, want %+v", i, r, want)
		}
		if r.Type != OkuriNasi || r.Reading != "きしゃ" || r.Length != 3 || r.Count != 2 || r.Context <= 0 {
			t.Errorf("%d: %+v", i, r)
		}
		if _, ok := r.Times["2:engine.sureDict"]; !ok {
			t.Errorf("%d: times %v", i, r.Times)
		}
	}
	// Off: no trace.
	s = newSim(t, Config{Dictionaries: []Dictionary{jisyo}})
	s.typ("Kisha ")
	if s.p.lib.tracing || s.p.c.state.trace != nil {
		t.Errorf("traced without ConvertLog")
	}
}
