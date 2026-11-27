package engine

import (
	"strings"
	"unicode/utf8"
)

// A reading with no more candidates registers a word, typed in the pre-edit
// itself, as macSKK and AquaSKK do: "▼[登録:みかん]" and the word after it,
// converted as anywhere else (registering another word in it too). Until
// it is done, nothing goes into the buffer.

// registration is the registration of a word for a conversion.
type registration struct {
	// state is the conversion it is for; mode the input mode then.
	state state
	mode  string
	// text is the word typed so far, what was confirmed in it.
	text string
	// ascii: the keys are typed as they are ("l"; <C-j> goes back), as
	// ddskk registers "km/h".
	ascii bool
	// tags are the completions of <Tab> (registerTab), the one shown, and
	// the text they complete.
	tags    []string
	tag     int
	tagText string
}

// registering reports whether a word is being registered.
func (c *Context) registering() bool { return len(c.regs) > 0 }

// typed reports whether something is being typed in the registration (a
// reading, a romaji), which the keys that end it go to then.
func (s *state) typed() bool {
	return s.kind != stateInput || s.mode != modeDirect || s.feed != ""
}

// registerWord starts the registration of a word for the conversion, when
// there are no more candidates. In an expression (<C-r>=), whose command
// line has no room for it, the reading is confirmed as it is, with its
// okurigana (#244 of skkeleton.nvim).
func registerWord(c *Context) {
	s := &c.state
	if c.vimMode == "c" && c.cmdtype == "=" {
		s.kind = stateInput
		kakutei(c)
		return
	}
	c.regs = append(c.regs, &registration{state: s.clone(), mode: c.mode})
	c.state.reset(c.table, false, false)
	c.popup = nil
}

// bareText is the pre-edit without its markers, kept in insert mode when it
// is not the input any more: that of the conversion registered for, when
// registering (the registration is given up).
func (c *Context) bareText() string {
	if c.registering() {
		return c.regs[0].state.bareText()
	}
	return c.state.bareText()
}

// registrationText is the pre-edit of the registrations before what is
// typed in the innermost: "▼[登録:みかん]蜜" for each.
func (c *Context) registrationText() string {
	var b strings.Builder
	for _, r := range c.regs {
		b.WriteString(*c.cfg.MarkerHenkanSelect + "[登録:" + r.state.henkanFeed)
		if r.state.mode == modeOkuriAri {
			b.WriteString("*" + r.state.okuriFeed)
		}
		b.WriteString("]" + r.text)
	}
	return b.String()
}

// addRegistered adds s, confirmed, to the word being registered ("\b"
// deletes a character of it).
func (c *Context) addRegistered(s string) {
	r := c.regs[len(c.regs)-1]
	for _, ch := range s {
		if ch == '\b' {
			_, n := utf8.DecodeLastRuneInString(r.text)
			r.text = r.text[:len(r.text)-n]
			continue
		}
		r.text += string(ch)
	}
	r.tags = nil
}

// registerKey handles a key of the registration that ends it or edits it,
// when nothing is typed in it: fn is the function it is mapped to. It
// reports whether it did.
func registerKey(c *Context, fn, key string) bool {
	if !c.registering() || c.state.typed() {
		return false
	}
	r := c.regs[len(c.regs)-1]
	if r.ascii {
		switch fn {
		case "kakutei":
			r.ascii = false
			return true
		case "newline", "cancel", "deleteChar", "escape":
		default:
			ch := keyChar(key)
			if key == "<space>" {
				ch = " "
			}
			if utf8.RuneCountInString(ch) == 1 {
				c.addRegistered(ch)
			}
			return true
		}
	}
	switch fn {
	case "kakutei", "newline":
		registerDone(c, r.text)
	case "cancel":
		registerDone(c, "")
	case "deleteChar":
		if r.text == "" {
			registerDone(c, "")
		} else {
			c.addRegistered("\b")
		}
	case "disable":
		r.ascii = true
	case "escape":
		// Leaving: the conversions are as they were before, and what
		// leaving does is done to the outermost.
		for c.registering() {
			registerDone(c, "")
		}
		return false
	default:
		return false
	}
	return true
}

// registerDone ends the innermost registration: word is confirmed for its
// conversion and learned; "" takes the conversion back as it was.
func registerDone(c *Context, word string) {
	r := c.regs[len(c.regs)-1]
	c.regs = c.regs[:len(c.regs)-1]
	c.state = r.state
	if c.mode != r.mode {
		c.modeChange(r.mode)
	}
	if word == "" {
		if c.state.index >= *c.cfg.ShowCandidatesCount {
			showCandidates(c)
		}
		return
	}
	c.state.kind, c.state.candidates, c.state.index = stateHenkan, []string{word}, 0
	if t := c.state.trace; t != nil {
		c.state.trace = &lookupTrace{sources: t.sources, times: t.times,
			first: t.first, preferred: t.preferred, context: t.context, count: t.count, registered: true}
	}
	// Registered even past MaxLearnLength, which kakutei keeps to.
	c.lib.register(c.state.mode.henkanType(), c.state.word, c.state.okuri(), word)
	kakutei(c)
	// Taking it back would only choose the word just registered.
	c.invalidateKakutei()
}

// registerTab completes the word being registered for a reading with
// okurigana with the conjugations of a conjugation dictionary ("思" to
// "思;@verb-5-w"), the next one again. Not registering, it types the key.
func registerTab(c *Context, key string) {
	if !c.registering() || c.state.typed() {
		if !c.registering() {
			c.kakutei("\t")
		}
		return
	}
	r := c.regs[len(c.regs)-1]
	if r.state.mode != modeOkuriAri {
		return
	}
	if r.tags == nil || r.text != r.tags[r.tag] {
		r.tagText = r.text
		r.tags = conjugationCompletions(r.text, r.state.okuriFeed)
		r.tag = -1
	}
	if len(r.tags) == 0 {
		r.tags = nil
		return
	}
	r.tag = (r.tag + 1) % len(r.tags)
	r.text = r.tags[r.tag]
	c.popup = []string{r.text}
}
