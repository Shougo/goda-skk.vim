package engine

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// keymap is what keys do in a state, and the function of the other keys.
type keymap struct {
	def  string
	keys map[string]string
}

// defaultKeymaps are the keymaps of the states.
func defaultKeymaps() map[string]keymap {
	return map[string]keymap{
		"input": {def: "kanaInput", keys: map[string]string{
			"<bs>":  "deleteChar",
			"<c-h>": "deleteChar",
			"<c-g>": "cancel",
			"<cr>":  "newline",
			"<esc>": "escape",
			"<c-j>": "kakutei",
			"<c-q>": "hankata",
			">":     "prefix",
		}},
		"henkan": {def: "henkanInput", keys: map[string]string{
			"<c-g>":   "cancel",
			"<cr>":    "newline",
			"<c-j>":   "kakutei",
			"<space>": "henkanForward",
			"x":       "henkanBackward",
			"X":       "purgeCandidate",
			">":       "suffix",
		}},
	}
}

// normalizeKey writes a key notation as the keymaps do: special keys in
// lower case ("<C-J>" is "<c-j>", "<NL>" is "<c-j>"), a character as it is.
func normalizeKey(key string) string {
	if len(key) > 2 && key[0] == '<' && key[len(key)-1] == '>' {
		k := strings.ToLower(key)
		switch k {
		case "<nl>":
			return "<c-j>"
		case "<c-m>":
			return "<cr>"
		case "<c-[>":
			return "<esc>"
		case "<lt>":
			return "<"
		case "<bar>":
			return "|"
		case " ":
			return "<space>"
		}
		return k
	}
	if key == " " {
		return "<space>"
	}
	return key
}

// buildKeymaps applies the keymaps of the configuration.
func buildKeymaps(cfg map[string]map[string]string) map[string]keymap {
	kms := defaultKeymaps()
	for st, m := range cfg {
		km := kms[st]
		for key, fn := range m {
			key = normalizeKey(key)
			if fn == "" {
				delete(km.keys, key)
			} else {
				km.keys[key] = fn
			}
		}
	}
	return kms
}

// handleKey runs what key does in the state.
func handleKey(c *Context, key string) {
	key = normalizeKey(key)
	var km keymap
	switch c.state.kind {
	case stateInput:
		km = c.keymaps["input"]
	case stateHenkan:
		km = c.keymaps["henkan"]
	default:
		return
	}
	fn, ok := km.keys[key]
	if !ok {
		fn = km.def
	}
	functions[fn](c, key)
}

// defaultMappedKeys are the keys Vim maps while enabled, besides those of
// the keymaps and the kana tables.
var defaultMappedKeys = strings.Split("abcdefghijklmnopqrstuvwxyz"+
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ"+
	"1234567890"+
	"!\"#$%&'()"+
	",./;:]@[-^\\"+
	">?_+*}`{=~<|", "")

// mappedKeys are the keys Vim maps while enabled, in the notation of a
// mapping: the default ones, and the first keys of the keymaps and of the
// rules of the kana tables.
func mappedKeys(cfg *Config, kms map[string]keymap) []string {
	keys := map[string]bool{}
	for _, k := range defaultMappedKeys {
		keys[k] = true
	}
	for _, km := range kms {
		for k := range km.keys {
			keys[firstKey(k)] = true
		}
	}
	for _, t := range []*KanaTable{cfg.KanaTable, cfg.ZenkakuTable} {
		for _, r := range t.rules {
			if r.From != "" {
				keys[firstKey(r.From)] = true
			}
		}
	}
	ret := make([]string, 0, len(keys))
	for k := range keys {
		ret = append(ret, mapNotation(k))
	}
	slices.Sort(ret)
	return slices.Compact(ret)
}

// firstKey is the first key of keys: a notation ("<c-z>") or a character.
func firstKey(keys string) string {
	if keys[0] == '<' {
		if i := strings.IndexByte(keys, '>'); i > 1 {
			return normalizeKey(keys[:i+1])
		}
	}
	_, size := utf8.DecodeRuneInString(keys)
	return keys[:size]
}

// mapNotation is the left-hand side of a mapping of key.
func mapNotation(key string) string {
	switch key {
	case "<":
		return "<lt>"
	case "|":
		return "<Bar>"
	case " ", "<space>":
		return "<Space>"
	}
	// <s-l>: Vim gets "L".
	if len(key) == 5 && strings.HasPrefix(key, "<s-") && key[4] == '>' &&
		'a' <= key[3] && key[3] <= 'z' {
		return strings.ToUpper(key[3:4])
	}
	return key
}
