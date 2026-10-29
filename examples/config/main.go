// Command config is a user's goda-skk configuration: an ordinary Go program.
// Build it with :call goda_skk#build() (or go build) and goda-skk.vim runs
// the result.
package main

import "github.com/Shougo/goda-skk.vim/skk"

func main() {
	skk.Main(skk.Config{
		// Dictionary files, looked up in order after the user dictionary.
		// The encoding "" detects UTF-8 or EUC-JP.
		Dictionaries: []skk.Dictionary{
			skk.NewJisyo("~/.skk/SKK-JISYO.L", ""),
		},
		// Where conversions are learned (the default).
		UserDictionary: "~/.goda-skk-jisyo",

		// Rules added to the romaji table: "z " types a full-width space.
		KanaTable: skk.RomTable.With(
			skk.Rule{From: "z ", Kana: "　"},
		),

		// <CR> while converting confirms, without a new line.
		EggLikeNewline: true,
	})
}
