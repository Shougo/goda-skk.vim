package skk

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/Shougo/goda.vim/sdk"
)

// The completion of readings, for a completion source (package
// github.com/Shougo/goda-skk.vim/cmp/source): the source runs in the same
// binary as the plugin, and asks it.

// Reading is the reading being typed (▽): Kana, and Feed the romaji not yet
// kana. PreEdit is how it shows before the cursor; VimMode is "i" or "c".
type Reading struct {
	PreEdit string
	Kana    string
	Feed    string
	VimMode string
}

// Completion is a candidate of a reading that starts with the one typed.
type Completion struct {
	Reading   string
	Candidate string
}

// Reading returns the reading being typed, or false when none is (or the
// input method is choosing a candidate, or is not enabled).
func (p *Plugin) Reading() (Reading, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	c := p.c
	s := &c.state
	if s.kind != stateInput || s.mode != modeOkuriNasi || c.disable {
		return Reading{}, false
	}
	return Reading{PreEdit: c.pre.current, Kana: s.henkanFeed, Feed: s.feed, VimMode: c.vimMode}, true
}

// Complete returns the candidates of the readings that start with r (and go
// on with the kana of its feed): those learned in this process first, the
// latest first, then by reading.
func (p *Plugin) Complete(ctx context.Context, r Reading) []Completion {
	prefixes := []string{r.Kana}
	if r.Feed != "" {
		prefixes = nil
		seen := map[string]bool{}
		for _, rule := range p.cfg.KanaTable.withPrefix(r.Feed) {
			if rule.Kana != "" && !seen[rule.Kana] {
				seen[rule.Kana] = true
				prefixes = append(prefixes, r.Kana+rule.Kana)
			}
		}
	}
	if len(prefixes) == 0 || r.Kana == "" && r.Feed == "" {
		return nil
	}
	var out []Completion
	for _, e := range p.lib.complete(ctx, prefixes) {
		for _, c := range e.Candidates {
			out = append(out, Completion{e.Word, c})
		}
	}
	if u := p.lib.user; u != nil {
		ranks := map[string]int64{}
		for _, c := range out {
			ranks[c.Candidate] = u.rank(c.Candidate)
		}
		slices.SortStableFunc(out, func(a, b Completion) int {
			return cmp.Compare(ranks[b.Candidate], ranks[a.Candidate])
		})
	}
	return out
}

// OkuriCompletion is a candidate with okurigana for the reading typed: the
// reading split into a stem and Okuri ("たべる": "たべr" with "る").
type OkuriCompletion struct {
	Reading   string
	Candidate string
	Okuri     string
}

// CompleteOkuri returns the candidates with okurigana of r, for each way
// to split it, the longest stem first.
func (p *Plugin) CompleteOkuri(ctx context.Context, r Reading) []OkuriCompletion {
	kana := []rune(r.Kana)
	var out []OkuriCompletion
	for i := len(kana) - 1; i >= 1; i-- {
		okuri := string(kana[i:])
		stem := string(kana[:i])
		word := okuriWord(stem, okuri)
		if word == stem {
			continue
		}
		for _, c := range p.lib.lookup(ctx, OkuriAri, word) {
			out = append(out, OkuriCompletion{word, c, okuri})
		}
	}
	return out
}

// Lookup returns the candidates of word, from every dictionary.
func (p *Plugin) Lookup(ctx context.Context, typ HenkanType, word string) []string {
	return p.lib.lookup(ctx, typ, word)
}

// CompleteDone is told that the completion inserted inserted, candidate of
// word: it is learned, the reading typed is gone, and kakuteiUndo can take
// it back.
func (p *Plugin) CompleteDone(ctx context.Context, h sdk.Host, typ HenkanType, word, candidate, inserted string) {
	p.lib.learn(typ, word, candidate)
	p.mu.Lock()
	c := p.c
	c.lastCandidate = candidateRef{typ, word, candidate}
	c.invalidateKakutei()
	if c.state.kind == stateInput && inserted != "" &&
		strings.HasPrefix(inserted, candidateText(candidate, affixNone)) {
		cands := p.lib.lookup(ctx, typ, word)
		if i := slices.Index(cands, candidate); i >= 0 {
			st := state{
				kind: stateHenkan, mode: modeOkuriNasi, table: c.state.table,
				converter: c.state.converter, word: word, henkanFeed: word,
				candidates: cands, index: i,
				okuriFeed: inserted[len(candidateText(candidate, affixNone)):],
			}
			if typ == OkuriAri {
				st.mode = modeOkuriAri
				_, size := utf8.DecodeLastRuneInString(word)
				st.henkanFeed = word[:len(word)-size]
			}
			c.recordKakutei(inserted, st)
		}
	}
	// The pre-edit was replaced.
	c.resetState(true, false)
	c.pre.output("")
	phase := c.state.phase()
	p.mu.Unlock()
	if h != nil {
		_, _ = h.Call(ctx, "goda_skk#_complete_done", phase)
	}
}
