package karukan

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Shougo/goda-skk.vim/engine"
)

func TestLookup(t *testing.T) {
	var (
		mu  sync.Mutex
		got []request
	)
	requests := func() []request {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/kanji/convert" ||
			r.Header.Get("Content-Type") != "application/json" {
			http.NotFound(w, r)
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		mu.Lock()
		got = append(got, req)
		mu.Unlock()
		switch req.Hiragana {
		case "かんじへんかん":
			w.Write([]byte(`{"candidates":["漢字変換","感じ変換","かんじへんかん"],"katakana":"カンジヘンカン"}`))
		case "おそい":
			time.Sleep(300 * time.Millisecond)
			w.Write([]byte(`{"candidates":["遅い"]}`))
		default:
			w.Write([]byte(`{"candidates":[]}`))
		}
	}))
	defer srv.Close()
	d := New(Params{
		URL:        srv.URL + "/",
		Model:      "jinen-v2-xsmall-q8",
		Candidates: 3,
		BeamSearch: "d1_greedy",
		Timeout:    100 * time.Millisecond,
	})
	ctx := context.Background()
	cs, err := d.Lookup(ctx, engine.OkuriNasi, "かんじへんかん")
	if err != nil || !slices.Equal(cs, []string{"漢字変換", "感じ変換"}) {
		t.Errorf("lookup: %q %v", cs, err)
	}
	want := request{
		Hiragana:      "かんじへんかん",
		NumCandidates: 3,
		Model:         "jinen-v2-xsmall-q8",
		BeamSearch:    "d1_greedy",
	}
	if r := requests(); !slices.Equal(r, []request{want}) {
		t.Errorf("requests: %+v", r)
	}
	// Too slow: nothing, no error.
	if cs, err := d.Lookup(ctx, engine.OkuriNasi, "おそい"); err != nil || cs != nil {
		t.Errorf("slow: %q %v", cs, err)
	}
	// Not asked: okuri-ari, and the pattern of the readings with digits.
	for _, c := range []struct {
		typ  engine.HenkanType
		word string
	}{{engine.OkuriAri, "おくr"}, {engine.OkuriNasi, "#かい"}, {engine.OkuriNasi, ""}} {
		if cs, err := d.Lookup(ctx, c.typ, c.word); err != nil || cs != nil || len(requests()) != 2 {
			t.Errorf("%q: %q %v %d", c.word, cs, err, len(requests()))
		}
	}
	if cs, err := d.Lookup(ctx, engine.OkuriNasi, "ない"); err != nil || cs != nil {
		t.Errorf("none: %q %v", cs, err)
	}
	if es, err := d.Complete(ctx, "かん"); err != nil || es != nil {
		t.Errorf("complete: %v %v", es, err)
	}
}

func TestDefaults(t *testing.T) {
	var got request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{"candidates":["変換"]}`))
	}))
	defer srv.Close()
	d := New(Params{URL: srv.URL})
	if cs, err := d.Lookup(context.Background(), engine.OkuriNasi, "へんかん"); err != nil || !slices.Equal(cs, []string{"変換"}) {
		t.Errorf("lookup: %q %v", cs, err)
	}
	if want := (request{Hiragana: "へんかん", NumCandidates: 5}); got != want {
		t.Errorf("request: %+v", got)
	}
	if d := New(Params{}); d.p.URL != DefaultURL || d.p.Timeout != time.Second {
		t.Errorf("defaults: %+v", d.p)
	}
}

func TestLoading(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "No models loaded", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	if cs, err := New(Params{URL: srv.URL}).Lookup(context.Background(), engine.OkuriNasi, "かんじ"); err != nil || cs != nil {
		t.Errorf("loading: %q %v", cs, err)
	}
}

func TestError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusNotFound)
	}))
	defer srv.Close()
	if _, err := New(Params{URL: srv.URL}).Lookup(context.Background(), engine.OkuriNasi, "かんじ"); err == nil {
		t.Errorf("no error")
	}
}

func TestUnreachable(t *testing.T) {
	// A port nobody listens on.
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	url := "http://" + l.Addr().String()
	l.Close()
	d := New(Params{URL: url})
	ctx := context.Background()
	if _, err := d.Lookup(ctx, engine.OkuriNasi, "かんじ"); err == nil {
		t.Errorf("no error")
	}
	// Left alone for a while: nothing, no error.
	if cs, err := d.Lookup(ctx, engine.OkuriNasi, "かんじ"); err != nil || cs != nil {
		t.Errorf("again: %q %v", cs, err)
	}
	d.mu.Lock()
	d.failedAt = time.Now().Add(-retryAfter)
	d.mu.Unlock()
	if _, err := d.Lookup(ctx, engine.OkuriNasi, "かんじ"); err == nil {
		t.Errorf("retry: no error")
	}
}

func TestContext(t *testing.T) {
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req request
		json.NewDecoder(r.Body).Decode(&req)
		got = append(got, req.Context)
		w.Write([]byte(`{"candidates":["汽車"]}`))
	}))
	defer srv.Close()
	ctx := context.Background()
	before := "あの日の朝、駅のホームに"
	for _, n := range []*int{nil, new(3), new(0)} {
		d := New(Params{URL: srv.URL, ContextLength: n})
		if cs, err := d.LookupContext(ctx, engine.OkuriNasi, "きしゃ", before); err != nil || !slices.Equal(cs, []string{"汽車"}) {
			t.Errorf("lookup: %q %v", cs, err)
		}
	}
	New(Params{URL: srv.URL}).Lookup(ctx, engine.OkuriNasi, "きしゃ")
	if want := []string{"あの日の朝、駅のホームに", "ームに", "", ""}; !slices.Equal(got, want) {
		t.Errorf("context: %q, want %q", got, want)
	}
}

func TestContextLocal(t *testing.T) {
	for _, c := range []struct {
		url  string
		n    *int
		want int
	}{
		{"", nil, 40},
		{"http://127.0.0.1:3000", nil, 40},
		{"http://127.8.9.1:3000", nil, 40},
		{"http://localhost:3000/", nil, 40},
		{"http://LOCALHOST", nil, 40},
		{"http://[::1]:3000", nil, 40},
		{"http://192.168.1.2:3000", nil, 0},
		{"http://karukan.example.com", nil, 0},
		{"http://localhost.example.com", nil, 0},
		{"http://192.168.1.2:3000", new(20), 20},
		{"http://127.0.0.1:3000", new(0), 0},
	} {
		if got := *New(Params{URL: c.url, ContextLength: c.n}).p.ContextLength; got != c.want {
			t.Errorf("%q %v: %d, want %d", c.url, c.n, got, c.want)
		}
	}
}

func TestLookupOkuri(t *testing.T) {
	var got []request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req request
		json.NewDecoder(r.Body).Decode(&req)
		got = append(got, req)
		switch req.Hiragana {
		case "おおい":
			w.Write([]byte(`{"candidates":["多い","覆い","おおい","多(タ)い","大井","多い"]}`))
		case "いった":
			w.Write([]byte(`{"candidates":["言った","行った","いった"]}`))
		default:
			w.Write([]byte(`{"candidates":[]}`))
		}
	}))
	defer srv.Close()
	ctx := context.Background()
	// Off by default: the server is not asked.
	off := New(Params{URL: srv.URL})
	if cs, err := off.LookupOkuri(ctx, "おおi", "い", "人が"); err != nil || cs != nil || len(got) != 0 {
		t.Errorf("off: %q %v %d", cs, err, len(got))
	}
	d := New(Params{URL: srv.URL, OkuriAri: true})
	for _, c := range []struct {
		word, okuri string
		want        []string
	}{
		// The stems of the candidates that end with the okurigana, with a
		// kanji: not "おお", "多(タ)" or "大井".
		{"おおi", "い", []string{"多", "覆"}},
		// The stem typed with a sokuon ("ItTa").
		{"いっt", "た", []string{"言っ", "行っ"}},
	} {
		cs, err := d.LookupOkuri(ctx, c.word, c.okuri, "人が")
		if err != nil || !slices.Equal(cs, c.want) {
			t.Errorf("%s: %q %v, want %q", c.word, cs, err, c.want)
		}
	}
	if want := []request{
		{Hiragana: "おおい", Context: "人が", NumCandidates: 5},
		{Hiragana: "いった", Context: "人が", NumCandidates: 5},
	}; !slices.Equal(got, want) {
		t.Errorf("requests: %+v", got)
	}
	// Not asked: no okurigana, a pattern of digits, a reading with
	// okurigana as a ContextDictionary.
	n := len(got)
	d.LookupOkuri(ctx, "おおi", "", "")
	d.LookupOkuri(ctx, "#かi", "い", "")
	d.LookupContext(ctx, engine.OkuriAri, "おおi", "人が")
	if len(got) != n {
		t.Errorf("asked: %+v", got[n:])
	}
}
