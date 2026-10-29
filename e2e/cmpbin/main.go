// The binary of the end-to-end test of the completion: goda-cmp.vim with
// the sources of goda-skk.vim, and the input method, in one binary.
package main

import (
	"log"
	"os"

	"github.com/Shougo/goda-cmp.vim/cmp"
	"github.com/Shougo/goda-cmp.vim/ext/ui/native"
	"github.com/Shougo/goda.vim/host"

	"github.com/Shougo/goda-skk.vim/cmp/source"
	"github.com/Shougo/goda-skk.vim/skk"
)

func main() {
	p, err := skk.New(skk.Config{
		Dictionaries:   []skk.Dictionary{skk.NewJisyo(os.Getenv("GODA_SKK_JISYO"), "utf-8")},
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
	b.AddSource(source.New(p))
	b.AddSource(source.NewOkuri(p))
	none := []string{}
	b.PatchGlobal(cmp.Config{
		UI:      "native",
		Sources: []string{source.Name, source.NameOkuri},
		SourceOptions: map[string]cmp.SourceOption{
			"_": {Matchers: none, Sorters: none},
		},
		AutoCompleteDelay: new(0),
	})
	engine, err := b.Plugin()
	if err != nil {
		log.Fatal(err)
	}
	host.Main(engine, p)
}
