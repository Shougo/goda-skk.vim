// Command skkeleton is the configuration of a skkeleton user, translated:
// skkeleton#config(), register_kanatable() and register_keymap(), and the
// ddc.vim sources skkeleton and skkeleton_okuri, as goda-cmp.vim sources.
// The input method and goda-cmp.vim are one binary (build it with
// goda_cmp#build()); its Vim script is in vimrc.vim.
package main

import (
	"log"
	"runtime"

	"github.com/Shougo/goda-cmp.vim/cmp"
	"github.com/Shougo/goda-cmp.vim/ext/filter/matcherhead"
	"github.com/Shougo/goda-cmp.vim/ext/filter/sorterrank"
	"github.com/Shougo/goda-cmp.vim/ext/source/around"
	"github.com/Shougo/goda-cmp.vim/ext/source/line"
	"github.com/Shougo/goda-cmp.vim/ext/ui/pum"
	"github.com/Shougo/goda.vim/host"

	"github.com/Shougo/goda-skk.vim/cmp/source"
	"github.com/Shougo/goda-skk.vim/skk"
)

func main() {
	jisyo := "/usr/share/skk/SKK-JISYO.L"
	if runtime.GOOS == "windows" {
		jisyo = "~/.config/SKK-JISYO.L"
	}
	p, err := skk.New(skk.Config{
		// globalDictionaries
		Dictionaries: []skk.Dictionary{skk.NewJisyo(jisyo, "")},
		// No markers: the highlights GodaSkkHenkan and
		// GodaSkkHenkanSelect show the pre-edit (vimrc.vim).
		MarkerHenkan:          new(""),
		MarkerHenkanSelect:    new(""),
		EggLikeNewline:        true,
		RegisterConvertResult: true,

		// register_kanatable('rom', {'jj': 'escape', '~': ['〜', '']})
		KanaTable: skk.RomTable.With(
			skk.Rule{From: "jj", Func: "escape"},
			skk.Rule{From: "~", Kana: "〜"},
		),
		// register_keymap('input', '<C-z>', 'kakuteiUndo'); <C-z> is
		// mapped by itself.
		Keymaps: map[string]map[string]string{
			"input": {"<C-z>": "kakuteiUndo"},
		},
	})
	if err != nil {
		log.Fatal(err)
	}

	b := cmp.New()
	b.AddUI(pum.New())
	b.AddSource(around.New(around.Params{}))
	b.AddSource(line.New(line.Params{}))
	// The ddc.vim sources skkeleton and skkeleton_okuri.
	b.AddSource(source.New(p))
	b.AddSource(source.NewOkuri(p))
	b.AddFilter(matcherhead.New(matcherhead.Params{}))
	b.AddFilter(sorterrank.New())
	none := []string{}
	b.PatchGlobal(cmp.Config{
		UI:      "pum",
		Sources: []string{"around"},
		SourceOptions: map[string]cmp.SourceOption{
			"_": {
				Matchers: []string{"matcherhead"},
				Sorters:  []string{"sorterrank"},
			},
			// In the order of the input method, all of them.
			source.Name:      {Mark: "skk", Matchers: none, Sorters: none},
			source.NameOkuri: {Mark: "skk", Matchers: none, Sorters: none},
		},
	})

	engine, err := b.Plugin()
	if err != nil {
		log.Fatal(err)
	}
	// goda-cmp.vim and the input method, in one binary.
	host.Main(engine, p)
}
