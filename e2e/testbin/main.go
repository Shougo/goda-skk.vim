// The binary of the end-to-end tests: the input method with the dictionary
// $GODA_SKK_JISYO and the user dictionary $GODA_SKK_USER_JISYO.
package main

import (
	"os"

	"github.com/Shougo/goda-skk.vim/engine"
)

func main() {
	engine.Main(engine.Config{
		Dictionaries:   []engine.Dictionary{engine.NewJisyo(os.Getenv("GODA_SKK_JISYO"), "utf-8")},
		UserDictionary: os.Getenv("GODA_SKK_USER_JISYO"),
		KanaTable: engine.RomTable.With(
			engine.Rule{From: "jj", Func: "escape"},
			engine.Rule{From: "~", Kana: "〜"},
		),
		Keymaps: map[string]map[string]string{
			"input": {"<C-z>": "kakuteiUndo"},
		},
	})
}
