// Package skkserv is a dictionary of goda-skk.vim that asks an SKK server
// (skkserv, yaskkserv2, ...) over TCP, as skkeleton's source skk_server:
//
//	Dictionaries: []engine.Dictionary{
//		engine.NewJisyo("~/.skk/SKK-JISYO.L", ""),
//		skkserv.New(skkserv.Params{}), // localhost:1178, EUC-JP
//	},
//
// One connection is kept and used by one request at a time. A request that
// gets no answer in time gives nothing, and the connection is made again
// for the next one; while the server cannot be reached, it is tried again
// every few seconds only, so that typing does not wait for it.
package skkserv

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/text/encoding/japanese"

	"github.com/Shougo/goda-skk.vim/engine"
)

// Params are the settings of the dictionary.
type Params struct {
	// Addr is the server (default "localhost:1178").
	Addr string
	// Encoding of the requests and the answers: "euc-jp" (default) or
	// "utf-8".
	Encoding string
	// Timeout bounds a request, connecting included (default 1 second).
	Timeout time.Duration
	// MaxCompletions bounds the readings a completion looks up, one
	// request each (default 30).
	MaxCompletions int
}

// retryAfter is how long a server that could not be reached is left alone.
const retryAfter = 5 * time.Second

// Dictionary is the SKK server as a dictionary.
type Dictionary struct {
	p Params

	mu       sync.Mutex
	conn     net.Conn
	r        *bufio.Reader
	failedAt time.Time
	dialer   net.Dialer
}

// New returns the dictionary of the server; it connects with the first
// request (or Init).
func New(p Params) *Dictionary {
	if p.Addr == "" {
		p.Addr = "localhost:1178"
	}
	if p.Encoding == "" {
		p.Encoding = "euc-jp"
	}
	if p.Timeout <= 0 {
		p.Timeout = time.Second
	}
	if p.MaxCompletions <= 0 {
		p.MaxCompletions = 30
	}
	return &Dictionary{p: p}
}

// Init connects, when the plugin starts.
func (d *Dictionary) Init(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.connect(ctx)
}

// Lookup asks the server for the candidates of word ("1word ").
func (d *Dictionary) Lookup(ctx context.Context, typ engine.HenkanType, word string) ([]string, error) {
	if word == "" || strings.ContainsAny(word, " \n") {
		return nil, nil
	}
	ans, err := d.request(ctx, "1"+word+" ")
	if err != nil || !strings.HasPrefix(ans, "1") {
		return nil, err
	}
	return engine.Candidates(ans[1:], typ), nil
}

// Complete asks the server for the readings that start with prefix ("4"),
// and their candidates. Servers that do not complete give nothing.
func (d *Dictionary) Complete(ctx context.Context, prefix string) ([]engine.Entry, error) {
	if prefix == "" || strings.ContainsAny(prefix, " \n") {
		return nil, nil
	}
	ans, err := d.request(ctx, "4"+prefix+" ")
	if err != nil || !strings.HasPrefix(ans, "1") {
		return nil, err
	}
	var out []engine.Entry
	for _, w := range strings.FieldsFunc(ans[1:], func(r rune) bool { return r == '/' || r == ' ' }) {
		if !strings.HasPrefix(w, prefix) {
			continue
		}
		if len(out) == d.p.MaxCompletions {
			break
		}
		cs, err := d.Lookup(ctx, engine.OkuriNasi, w)
		if err != nil {
			return out, err
		}
		if len(cs) > 0 {
			out = append(out, engine.Entry{Word: w, Candidates: cs})
		}
	}
	return out, nil
}

// errUnreachable: the server could not be reached a moment ago.
var errUnreachable = errors.New("not reached a moment ago")

// connect makes the connection, unless there is one. d.mu is held.
func (d *Dictionary) connect(ctx context.Context) error {
	if d.conn != nil {
		return nil
	}
	if !d.failedAt.IsZero() && time.Since(d.failedAt) < retryAfter {
		return errUnreachable
	}
	ctx, cancel := context.WithTimeout(ctx, d.p.Timeout)
	defer cancel()
	conn, err := d.dialer.DialContext(ctx, "tcp", d.p.Addr)
	if err != nil {
		d.failedAt = time.Now()
		return err
	}
	d.failedAt = time.Time{}
	d.conn, d.r = conn, bufio.NewReader(conn)
	return nil
}

// drop closes the connection: an answer still on its way would be taken
// for that of the next request.
func (d *Dictionary) drop() {
	if d.conn != nil {
		d.conn.Close()
		d.conn, d.r = nil, nil
	}
}

// request sends cmd and returns the answer line, without its newline.
func (d *Dictionary) request(ctx context.Context, cmd string) (string, error) {
	req, err := encode(cmd, d.p.Encoding)
	if err != nil {
		return "", err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	// A connection the server closed (it does after a while) fails at the
	// first request: once more with a new one.
	for try := 0; ; try++ {
		if err := d.connect(ctx); err != nil {
			if errors.Is(err, errUnreachable) {
				return "", nil
			}
			return "", fmt.Errorf("skkserv %s: %w", d.p.Addr, err)
		}
		line, err := d.roundTrip(ctx, req)
		if err == nil {
			return decode(line, d.p.Encoding)
		}
		d.drop()
		var ne net.Error
		if try > 0 || (errors.As(err, &ne) && ne.Timeout()) {
			return "", fmt.Errorf("skkserv %s: %w", d.p.Addr, err)
		}
	}
}

func (d *Dictionary) roundTrip(ctx context.Context, req []byte) ([]byte, error) {
	deadline := time.Now().Add(d.p.Timeout)
	if dl, ok := ctx.Deadline(); ok && dl.Before(deadline) {
		deadline = dl
	}
	if err := d.conn.SetDeadline(deadline); err != nil {
		return nil, err
	}
	if _, err := d.conn.Write(req); err != nil {
		return nil, err
	}
	line, err := d.r.ReadBytes('\n')
	if err != nil {
		return nil, err
	}
	return bytes.TrimRight(line, "\r\n"), nil
}

// Close says goodbye to the server ("0") and closes the connection.
func (d *Dictionary) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn != nil {
		d.conn.SetDeadline(time.Now().Add(d.p.Timeout))
		d.conn.Write([]byte("0"))
	}
	d.drop()
	return nil
}

func isEUC(encoding string) bool {
	switch strings.ToLower(strings.ReplaceAll(encoding, "_", "-")) {
	case "utf-8", "utf8":
		return false
	}
	return true
}

func encode(s, encoding string) ([]byte, error) {
	if !isEUC(encoding) {
		return []byte(s), nil
	}
	// The wave dash typed (U+301C) is the one of JIS X 0208.
	return japanese.EUCJP.NewEncoder().Bytes([]byte(strings.ReplaceAll(s, "〜", "～")))
}

func decode(b []byte, encoding string) (string, error) {
	if !isEUC(encoding) {
		return string(b), nil
	}
	out, err := japanese.EUCJP.NewDecoder().Bytes(b)
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(string(out), "～", "〜"), nil
}

var (
	_ engine.Dictionary  = (*Dictionary)(nil)
	_ engine.Initializer = (*Dictionary)(nil)
)
