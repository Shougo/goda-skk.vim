package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"
	"unicode/utf8"
)

// lookupTrace is what a lookup of a conversion found, for ConvertLog.
type lookupTrace struct {
	// sources: the dictionaries that gave each word.
	sources map[string][]string
	// times: how long each dictionary took, in milliseconds.
	times map[string]float64
	// first: the first word before Prefer moved preferred first ("": it
	// moved none).
	first, preferred string
	// context: the characters of the text before the reading the
	// ContextDictionaries got (-1: none was asked).
	context int
	count   int
	// registered: the word confirmed was registered.
	registered bool
}

// dictionaryName names d, of index i in library.all() (0: the user
// dictionary), for ConvertLog: "1:*engine.Jisyo".
func dictionaryName(i int, d Dictionary) string {
	return fmt.Sprintf("%d:%T", i, d)
}

// convertRecord is a line of ConvertLog.
type convertRecord struct {
	Time string `json:"time"`
	// Event: "confirm", "register" (a word registered and confirmed) or
	// "cancel".
	Event   string     `json:"event"`
	Type    HenkanType `json:"type"`
	Reading string     `json:"reading"`
	Okuri   string     `json:"okuri,omitempty"`
	// Length: the characters of the reading.
	Length    int    `json:"length"`
	Candidate string `json:"candidate,omitempty"`
	// Index: that of the candidate confirmed, or the last one shown.
	Index int `json:"index"`
	Count int `json:"count"`
	// Sources: the dictionaries that gave the candidate.
	Sources   []string           `json:"sources,omitempty"`
	First     string             `json:"first,omitempty"`
	Preferred string             `json:"preferred,omitempty"`
	Context   int                `json:"context"`
	Times     map[string]float64 `json:"times_ms"`
}

// logConversion appends the conversion of s to ConvertLog, as event.
func (c *Context) logConversion(event string, s *state) {
	t := s.trace
	if c.cfg.ConvertLog == "" || t == nil {
		return
	}
	r := convertRecord{
		Time:    time.Now().Format(time.RFC3339),
		Event:   event,
		Type:    s.mode.henkanType(),
		Reading: s.word,
		Okuri:   s.okuri(),
		Length:  utf8.RuneCountInString(s.word),
		Index:   s.index,
		Count:   t.count,
		First:   t.first, Preferred: t.preferred,
		Context: t.context,
		Times:   t.times,
	}
	if event == "confirm" && t.registered {
		r.Event = "register"
	}
	if event != "cancel" && s.index >= 0 && s.index < len(s.candidates) {
		r.Candidate = s.candidates[s.index]
		r.Sources = t.sources[candidateWord(r.Candidate)]
	}
	b, err := json.Marshal(r)
	if err == nil {
		err = appendLine(expandHome(c.cfg.ConvertLog), b)
	}
	if err != nil {
		log.Printf("goda-skk: ConvertLog: %v", err)
	}
}

func appendLine(path string, b []byte) error {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	_, err = f.Write(append(b, '\n'))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// took records how long the dictionary name took since start.
func (t *lookupTrace) took(name string, start time.Time) {
	if t != nil {
		t.times[name] += float64(time.Since(start).Microseconds()) / 1000
	}
}
