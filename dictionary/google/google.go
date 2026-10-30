// Package google is a dictionary of goda-skk.vim that asks Google Japanese
// Input (the transliteration API of Google CGI API for Japanese Input), as
// skkeleton's source google_japanese_input:
//
//	Dictionaries: []engine.Dictionary{
//		engine.NewJisyo("~/.skk/SKK-JISYO.L", ""),
//		google.New(google.Params{}),
//	},
//
// It converts readings without okurigana only, and does not complete. Its
// candidates come after those of the dictionaries before it. A request that
// is not answered in time (Timeout) gives nothing: typing does not wait for
// the network.
package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/Shougo/goda-skk.vim/engine"
)

// DefaultURL is the API.
const DefaultURL = "https://www.google.com/transliterate"

// Params are the settings of the dictionary.
type Params struct {
	// URL of the API (default DefaultURL).
	URL string
	// Timeout bounds a request (default 500 milliseconds).
	Timeout time.Duration
	// Client makes the requests (default http.DefaultClient).
	Client *http.Client
}

// Dictionary is Google Japanese Input as a dictionary.
type Dictionary struct{ p Params }

// New returns the dictionary.
func New(p Params) *Dictionary {
	if p.URL == "" {
		p.URL = DefaultURL
	}
	if p.Timeout <= 0 {
		p.Timeout = 500 * time.Millisecond
	}
	if p.Client == nil {
		p.Client = http.DefaultClient
	}
	return &Dictionary{p}
}

// Lookup asks for the candidates of word, as one phrase.
func (d *Dictionary) Lookup(ctx context.Context, typ engine.HenkanType, word string) ([]string, error) {
	if typ != engine.OkuriNasi || word == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(ctx, d.p.Timeout)
	defer cancel()
	q := url.Values{
		"langpair": {"ja-Hira|ja"},
		// The comma keeps the reading one phrase.
		"text": {word + ","},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, d.p.URL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := d.p.Client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil // too slow: nothing this time
		}
		return nil, fmt.Errorf("google: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google: %s", resp.Status)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil
		}
		return nil, fmt.Errorf("google: %w", err)
	}
	// [["reading", ["candidate", ...]], ...]: one phrase.
	var phrases [][]json.RawMessage
	if err := json.Unmarshal(b, &phrases); err != nil {
		return nil, fmt.Errorf("google: %w", err)
	}
	if len(phrases) == 0 || len(phrases[0]) < 2 {
		return nil, nil
	}
	var cs []string
	if err := json.Unmarshal(phrases[0][1], &cs); err != nil {
		return nil, fmt.Errorf("google: %w", err)
	}
	return cs, nil
}

// Complete gives nothing: the API does not complete.
func (d *Dictionary) Complete(context.Context, string) ([]engine.Entry, error) {
	return nil, nil
}

var _ engine.Dictionary = (*Dictionary)(nil)
