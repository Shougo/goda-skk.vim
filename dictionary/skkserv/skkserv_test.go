package skkserv

import (
	"bufio"
	"context"
	"net"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/text/encoding/japanese"

	"github.com/Shougo/goda-skk.vim/engine"
)

// server is an SKK server in EUC-JP: entries by reading; stall makes it
// answer nothing; it hangs up after hangup requests.
type server struct {
	t       *testing.T
	ln      net.Listener
	entries map[string]string
	mu      sync.Mutex
	stall   bool
	hangup  int
	conns   int
}

func newServer(t *testing.T, entries map[string]string) *server {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	s := &server{t: t, ln: ln, entries: entries}
	t.Cleanup(func() { ln.Close() })
	go s.serve()
	return s
}

func (s *server) serve() {
	for {
		c, err := s.ln.Accept()
		if err != nil {
			return
		}
		s.mu.Lock()
		s.conns++
		s.mu.Unlock()
		go s.handle(c)
	}
}

func (s *server) handle(c net.Conn) {
	defer c.Close()
	r := bufio.NewReader(c)
	n := 0
	for {
		b, err := r.ReadBytes(' ')
		if err != nil {
			return
		}
		cmd, err := japanese.EUCJP.NewDecoder().String(strings.TrimSuffix(string(b), " "))
		if err != nil {
			return
		}
		s.mu.Lock()
		stall, hangup := s.stall, s.hangup
		s.mu.Unlock()
		if stall {
			continue
		}
		n++
		if hangup > 0 && n > hangup {
			return
		}
		var ans string
		switch {
		case strings.HasPrefix(cmd, "1"):
			if e, ok := s.entries[cmd[1:]]; ok {
				ans = "1" + e
			} else {
				ans = "4" + cmd[1:] + " "
			}
		case strings.HasPrefix(cmd, "4"):
			var ws []string
			for w := range s.entries {
				if strings.HasPrefix(w, cmd[1:]) {
					ws = append(ws, w)
				}
			}
			slices.Sort(ws)
			ans = "1/" + strings.Join(ws, "/") + "/"
		}
		out, err := japanese.EUCJP.NewEncoder().String(ans + "\n")
		if err != nil {
			s.t.Error(err)
			return
		}
		if _, err := c.Write([]byte(out)); err != nil {
			return
		}
	}
}

var entries = map[string]string{
	"かんじ":  "/漢字/感じ;feeling/",
	"かんがえ": "/考え/",
	"おくr":  "/送/[る/送/]/",
	"から":   "/空/～/",
}

func TestLookup(t *testing.T) {
	s := newServer(t, entries)
	d := New(Params{Addr: s.ln.Addr().String(), Timeout: 200 * time.Millisecond})
	ctx := context.Background()
	if err := d.Init(ctx); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		typ  engine.HenkanType
		word string
		want []string
	}{
		{engine.OkuriNasi, "かんじ", []string{"漢字", "感じ;feeling"}},
		{engine.OkuriAri, "おくr", []string{"送"}},
		// The wave dash of JIS X 0208.
		{engine.OkuriNasi, "から", []string{"空", "〜"}},
		{engine.OkuriNasi, "ない", nil},
	} {
		got, err := d.Lookup(ctx, c.typ, c.word)
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("%s: %q, %v; want %q", c.word, got, err, c.want)
		}
	}
	es, err := d.Complete(ctx, "かん")
	if err != nil || len(es) != 2 || es[0].Word != "かんがえ" || !slices.Equal(es[1].Candidates, []string{"漢字", "感じ;feeling"}) {
		t.Errorf("complete: %+v %v", es, err)
	}
	s.mu.Lock()
	conns := s.conns
	s.mu.Unlock()
	if conns != 1 {
		t.Errorf("connections: %d", conns)
	}
	d.Close()
}

// No answer in time: nothing, and a new connection for the next request.
func TestTimeoutAndHangup(t *testing.T) {
	s := newServer(t, entries)
	d := New(Params{Addr: s.ln.Addr().String(), Timeout: 100 * time.Millisecond})
	ctx := context.Background()
	s.mu.Lock()
	s.stall = true
	s.mu.Unlock()
	if got, err := d.Lookup(ctx, engine.OkuriNasi, "かんじ"); err == nil || got != nil {
		t.Errorf("stalled: %q %v", got, err)
	}
	s.mu.Lock()
	s.stall = false
	s.hangup = 1 // the server closes connections after one request
	s.mu.Unlock()
	for range 3 {
		if got, err := d.Lookup(ctx, engine.OkuriNasi, "かんじ"); err != nil || len(got) != 2 {
			t.Errorf("after: %q %v", got, err)
		}
	}
}

// A server that cannot be reached is left alone for a while.
func TestUnreachable(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	ln.Close()
	d := New(Params{Addr: addr, Timeout: 100 * time.Millisecond})
	ctx := context.Background()
	if _, err := d.Lookup(ctx, engine.OkuriNasi, "かんじ"); err == nil {
		t.Errorf("no error")
	}
	start := time.Now()
	if got, err := d.Lookup(ctx, engine.OkuriNasi, "かんじ"); err != nil || got != nil || time.Since(start) > 50*time.Millisecond {
		t.Errorf("again: %q %v", got, err)
	}
}

func TestUTF8(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		if b, _ := r.ReadString(' '); b == "1かんじ " {
			c.Write([]byte("1/漢字/\n"))
		}
	}()
	d := New(Params{Addr: ln.Addr().String(), Encoding: "utf-8"})
	if got, err := d.Lookup(context.Background(), engine.OkuriNasi, "かんじ"); err != nil || !slices.Equal(got, []string{"漢字"}) {
		t.Errorf("utf-8: %q %v", got, err)
	}
}

// The plugin closes the connection when it stops.
func TestPluginClose(t *testing.T) {
	s := newServer(t, entries)
	d := New(Params{Addr: s.ln.Addr().String()})
	p, err := engine.New(engine.Config{Dictionaries: []engine.Dictionary{d}, UserDictionary: "-"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := d.Lookup(context.Background(), engine.OkuriNasi, "かんじ"); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(context.Background()); err != nil {
		t.Fatal(err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn != nil {
		t.Errorf("still connected")
	}
}
