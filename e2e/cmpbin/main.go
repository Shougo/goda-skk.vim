// The binary of the end-to-end test of the completion: goda-cmp.vim with
// the sources of goda-skk.vim, and the input method, in one binary.
package main

import (
	"log"
	"os"

	"github.com/Shougo/goda-cmp.vim/cmp"
	"github.com/Shougo/goda-cmp.vim/ext/ui/native"
	"github.com/Shougo/goda.vim/host"

	"github.com/Shougo/goda-skk.vim/cmp/source/skk"
	"github.com/Shougo/goda-skk.vim/cmp/source/skkokuri"
	"github.com/Shougo/goda-skk.vim/engine"
)

func main() {
	p, err := engine.New(engine.Config{
		Dictionaries:   []engine.Dictionary{engine.NewJisyo(os.Getenv("GODA_SKK_JISYO"), "utf-8")},
		UserDictionary: os.Getenv("GODA_SKK_USER_JISYO"),
		Keymaps: map[string]map[string]string{
			"input": {"<C-z>": "kakuteiUndo"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	b := cmp.New()
	b.AddUI(native.New())
	b.AddSource(skk.New(p))
	b.AddSource(skkokuri.New(p))
	b.PatchGlobal(cmp.Config{
		UI:      "native",
		Sources: []string{"skk", "skkokuri"},
		SourceOptions: map[string]cmp.SourceOption{
			// As examples/skkeleton/vimrc.vim patches it: katakana only, which
			// the reading does not match (CompletePosition says where it is).
			"_": {Matchers: []string{}, Sorters: []string{}, KeywordPattern: "[ァ-ヮア-ンー]+"},
		},
		AutoCompleteDelay: new(0),
	})
	cmpPlugin, err := b.Plugin()
	if err != nil {
		log.Fatal(err)
	}
	host.Main(cmpPlugin, p)
}
