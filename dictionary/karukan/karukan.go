// Package karukan is a dictionary of goda-skk.vim that asks karukan-server
// (https://github.com/togatoga/karukan), a neural kana-kanji conversion
// engine running a small language model with llama.cpp on the local
// machine:
//
//	Dictionaries: []engine.Dictionary{
//		engine.NewJisyo("~/.skk/SKK-JISYO.L", ""),
//		karukan.New(karukan.Params{}),
//	},
//
// It converts readings without okurigana only, as one phrase, and does not
// complete. Its candidates come after those of the dictionaries before it;
// wrapped as engine.Fallback(engine.NoLearn(karukan.New(...))), it converts
// only the readings they do not have, and its phrases are not learned.
// It is an engine.ContextDictionary: the end of the text before the reading
// (ContextLength) goes to the server with it, as the model converts better
// knowing it; by default only to a server on the local machine (a URL of
// the loopback address or localhost).
// A request that is not answered in time (Timeout) gives nothing, a server
// still loading its models gives nothing, and a server that cannot be
// reached is tried again every few seconds only: typing does not wait for
// it.
package karukan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Shougo/goda-skk.vim/engine"
)

// DefaultURL is where karukan-server listens by default.
const DefaultURL = "http://127.0.0.1:3000"

// retryAfter is how long a server that could not be reached is left alone.
const retryAfter = 5 * time.Second

// Params are the settings of the dictionary.
type Params struct {
	// URL of karukan-server (default DefaultURL).
	URL string
	// Model is the key of the model in the [models] table of karukan's
	// config.toml (default: the server's default model).
	Model string
	// Candidates is the number of candidates asked for (default 5).
	Candidates int
	// BeamSearch is the beam search of the server: "" (the server's
	// default, the full beam search) or "d1_greedy" (faster).
	BeamSearch string
	// ContextLength is how many characters of the text before the reading
	// go to the server with it (new(0) for none). The default is 40 for a
	// server on the local machine (the model is as fast with it, and a
	// little better than with 10, karukan's input method's), and none for
	// another one: the text may be anything in the buffer. Set it to send
	// the text to another machine.
	ContextLength *int
	// Timeout bounds a request (default 1 second).
	Timeout time.Duration
	// Client makes the requests (default http.DefaultClient).
	Client *http.Client
}

// Dictionary is karukan-server as a dictionary.
type Dictionary struct {
	p Params

	mu       sync.Mutex
	failedAt time.Time // when the server could not be reached
}

// New returns the dictionary.
func New(p Params) *Dictionary {
	if p.URL == "" {
		p.URL = DefaultURL
	}
	p.URL = strings.TrimSuffix(p.URL, "/")
	if p.Candidates <= 0 {
		p.Candidates = 5
	}
	if p.ContextLength == nil || *p.ContextLength < 0 {
		n := 0
		if isLocal(p.URL) {
			n = 40
		}
		p.ContextLength = &n
	}
	if p.Timeout <= 0 {
		p.Timeout = time.Second
	}
	if p.Client == nil {
		p.Client = http.DefaultClient
	}
	return &Dictionary{p: p}
}

type request struct {
	Hiragana      string `json:"hiragana"`
	Context       string `json:"context,omitempty"`
	NumCandidates int    `json:"num_candidates"`
	Model         string `json:"model,omitempty"`
	BeamSearch    string `json:"beam_search_type,omitempty"`
}

type response struct {
	Candidates []string `json:"candidates"`
}

// Lookup asks for the candidates of word, as one phrase.
func (d *Dictionary) Lookup(ctx context.Context, typ engine.HenkanType, word string) ([]string, error) {
	return d.LookupContext(ctx, typ, word, "")
}

// LookupContext asks for the candidates of word, as one phrase, after
// before.
func (d *Dictionary) LookupContext(ctx context.Context, typ engine.HenkanType, word, before string) ([]string, error) {
	// "#" is the pattern of the readings with digits: the model converts
	// the digits as typed.
	if typ != engine.OkuriNasi || word == "" || strings.Contains(word, "#") {
		return nil, nil
	}
	if d.resting() {
		return nil, nil
	}
	body, err := json.Marshal(request{
		Hiragana:      word,
		Context:       tail(before, *d.p.ContextLength),
		NumCandidates: d.p.Candidates,
		Model:         d.p.Model,
		BeamSearch:    d.p.BeamSearch,
	})
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, d.p.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.p.URL+"/api/kanji/convert", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.p.Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil // too slow: nothing this time
		}
		d.fail()
		return nil, fmt.Errorf("karukan %s: %w", d.p.URL, err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil
		}
		return nil, fmt.Errorf("karukan %s: %w", d.p.URL, err)
	}
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusServiceUnavailable:
		return nil, nil // the models are still loading
	default:
		return nil, fmt.Errorf("karukan %s: %s: %s", d.p.URL, resp.Status, bytes.TrimSpace(b))
	}
	var r response
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, fmt.Errorf("karukan %s: %w", d.p.URL, err)
	}
	var cs []string
	for _, c := range r.Candidates {
		// The reading itself is no conversion.
		if c != "" && c != word {
			cs = append(cs, c)
		}
	}
	return cs, nil
}

// Complete gives nothing: the server does not complete.
func (d *Dictionary) Complete(context.Context, string) ([]engine.Entry, error) {
	return nil, nil
}

// isLocal reports whether rawURL is on the local machine: its host is
// localhost or a loopback address (not resolved).
func isLocal(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Hostname()
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip, err := netip.ParseAddr(host)
	return err == nil && ip.IsLoopback()
}

// tail is the last n characters of s.
func tail(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}

// resting reports whether the server could not be reached a moment ago.
func (d *Dictionary) resting() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return !d.failedAt.IsZero() && time.Since(d.failedAt) < retryAfter
}

func (d *Dictionary) fail() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.failedAt = time.Now()
}

var _ engine.ContextDictionary = (*Dictionary)(nil)
