package google

import (
	"context"
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
		got []string
	)
	requests := func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(got)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.URL.Query().Get("langpair")+" "+r.URL.Query().Get("text"))
		mu.Unlock()
		switch r.URL.Query().Get("text") {
		case "かんじ,":
			w.Write([]byte(`[["かんじ",["漢字","感じ","幹事"]]]`))
		case "おそい,":
			time.Sleep(300 * time.Millisecond)
			w.Write([]byte(`[["おそい",["遅い"]]]`))
		default:
			w.Write([]byte(`[]`))
		}
	}))
	defer srv.Close()
	d := New(Params{URL: srv.URL, Timeout: 100 * time.Millisecond})
	ctx := context.Background()
	cs, err := d.Lookup(ctx, engine.OkuriNasi, "かんじ")
	if err != nil || !slices.Equal(cs, []string{"漢字", "感じ", "幹事"}) {
		t.Errorf("lookup: %q %v", cs, err)
	}
	if r := requests(); !slices.Equal(r, []string{"ja-Hira|ja かんじ,"}) {
		t.Errorf("requests: %q", r)
	}
	// Too slow: nothing, no error.
	if cs, err := d.Lookup(ctx, engine.OkuriNasi, "おそい"); err != nil || cs != nil {
		t.Errorf("slow: %q %v", cs, err)
	}
	// Not asked: okuri-ari.
	if cs, err := d.Lookup(ctx, engine.OkuriAri, "おくr"); err != nil || cs != nil || len(requests()) != 2 {
		t.Errorf("okuri-ari: %q %v %d", cs, err, len(requests()))
	}
	if cs, err := d.Lookup(ctx, engine.OkuriNasi, "ない"); err != nil || cs != nil {
		t.Errorf("none: %q %v", cs, err)
	}
	if es, err := d.Complete(ctx, "かん"); err != nil || es != nil {
		t.Errorf("complete: %v %v", es, err)
	}
}

func TestError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no", http.StatusForbidden)
	}))
	defer srv.Close()
	if _, err := New(Params{URL: srv.URL}).Lookup(context.Background(), engine.OkuriNasi, "かんじ"); err == nil {
		t.Errorf("no error")
	}
}
