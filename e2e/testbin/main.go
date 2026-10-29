// The binary of the end-to-end tests: the input method with the dictionary
// $GODA_SKK_JISYO and the user dictionary $GODA_SKK_USER_JISYO.
package main

import (
	"os"

	"github.com/Shougo/goda-skk.vim/skk"
)

func main() {
	skk.Main(skk.Config{
		Dictionaries:   []skk.Dictionary{skk.NewJisyo(os.Getenv("GODA_SKK_JISYO"), "utf-8")},
		UserDictionary: os.Getenv("GODA_SKK_USER_JISYO"),
		KanaTable: skk.RomTable.With(
			skk.Rule{From: "jj", Func: "escape"},
			skk.Rule{From: "~", Kana: "〜"},
		),
		Keymaps: map[string]map[string]string{
			"input": {"<C-z>": "kakuteiUndo"},
		},
	})
}
