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
// It converts readings without okurigana as one phrase (with OkuriAri,
// those with okurigana too: an engine.OkuriDictionary), and does not
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
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

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
	// OkuriAri converts readings with okurigana too (off by default): the
	// stem and the okurigana typed as a phrase after the text before them
	// ("おおい" after "人が"), and the stems of the candidates that end with
	// the okurigana ("多"). The text before tells "書く" from "掻く"; the
	// text after ("大いに") is not typed yet. A conversion with okurigana
	// asks the server then.
	OkuriAri bool
	// Margin is how much the score of the model for the first candidate
	// must exceed that of the second for engine.Prefer to put it first
	// (default 2; new(0.0) for always). Of 37 words of the same reading
	// after a text that tells them, the first is right for 32: with 2, 26
	// come first and 1 of them is wrong; with 0, 37 and 5.
	Margin *float64
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
	if p.Margin == nil {
		p.Margin = new(2.0)
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
	// CandidateTokens have the scores of the candidates (log
	// probabilities), when the server gives them.
	CandidateTokens []struct {
		Text  string  `json:"text"`
		Score float64 `json:"score"`
	} `json:"candidate_tokens"`
}

// scored is a candidate with the score of the model, when known.
type scored struct {
	text  string
	score float64
	known bool
}

// texts are the candidates of cs.
func texts(cs []scored) []string {
	var out []string
	for _, c := range cs {
		out = append(out, c.text)
	}
	return out
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
	cs, err := d.convert(ctx, word, before)
	return texts(cs), err
}

// LookupOkuri asks for the candidates of the stem of word with okuri, the
// okurigana typed, after before (Params.OkuriAri).
func (d *Dictionary) LookupOkuri(ctx context.Context, word, okuri, before string) ([]string, error) {
	cs, err := d.stems(ctx, word, okuri, before)
	return texts(cs), err
}

// LookupSure is LookupContext or LookupOkuri, and whether the score of the
// first candidate exceeds that of the second by Margin (engine.Prefer).
func (d *Dictionary) LookupSure(ctx context.Context, typ engine.HenkanType, word, okuri, before string) ([]string, bool, error) {
	var cs []scored
	var err error
	switch {
	case typ == engine.OkuriAri:
		cs, err = d.stems(ctx, word, okuri, before)
	case word != "" && !strings.Contains(word, "#"):
		cs, err = d.convert(ctx, word, before)
	}
	sure := len(cs) < 2 || !cs[0].known || !cs[1].known ||
		cs[0].score-cs[1].score >= *d.p.Margin
	return texts(cs), sure, err
}

// stems are the stems of the candidates of the stem of word with okuri,
// after before (Params.OkuriAri), each with the score of its first one.
func (d *Dictionary) stems(ctx context.Context, word, okuri, before string) ([]scored, error) {
	if !d.p.OkuriAri || okuri == "" || strings.Contains(word, "#") {
		return nil, nil
	}
	// "おおi": the stem, without the consonant of the okurigana.
	stem := strings.TrimRightFunc(word, func(r rune) bool { return r < utf8.RuneSelf })
	if stem == "" {
		return nil, nil
	}
	cs, err := d.convert(ctx, stem+okuri, before)
	var out []scored
	for _, c := range cs {
		s, ok := strings.CutSuffix(c.text, okuri)
		if ok && s != stem && isStem(s) && !slices.ContainsFunc(out, func(o scored) bool { return o.text == s }) {
			c.text = s
			out = append(out, c)
		}
	}
	return out, err
}

// isStem reports whether s may be the stem of a word: Japanese, with a
// kanji ("多", not "多(タ)" or "おお").
func isStem(s string) bool {
	kanji := false
	for _, r := range s {
		switch {
		case unicode.Is(unicode.Han, r) || r == '々':
			kanji = true
		case unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || r == 'ー':
		default:
			return false
		}
	}
	return kanji
}

// convert asks the server for the candidates of reading after before.
func (d *Dictionary) convert(ctx context.Context, word, before string) ([]scored, error) {
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
	scores := map[string]float64{}
	for _, t := range r.CandidateTokens {
		if _, ok := scores[t.Text]; !ok {
			scores[t.Text] = t.Score
		}
	}
	var cs []scored
	for _, c := range r.Candidates {
		// The reading itself is no conversion.
		if c != "" && c != word {
			s, ok := scores[c]
			cs = append(cs, scored{text: c, score: s, known: ok})
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

var (
	_ engine.OkuriDictionary = (*Dictionary)(nil)
	_ engine.SureDictionary  = (*Dictionary)(nil)
)
