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

	"github.com/Shougo/goda-skk.vim/cmp/source/skk"
	"github.com/Shougo/goda-skk.vim/cmp/source/skkokuri"
	"github.com/Shougo/goda-skk.vim/engine"
)

func main() {
	jisyo := "/usr/share/skk/SKK-JISYO.L"
	if runtime.GOOS == "windows" {
		jisyo = "~/.config/SKK-JISYO.L"
	}
	p, err := engine.New(engine.Config{
		// globalDictionaries
		Dictionaries: []engine.Dictionary{engine.NewJisyo(jisyo, "")},
		// No markers: the highlights GodaSkkHenkan and
		// GodaSkkHenkanSelect show the pre-edit (vimrc.vim).
		MarkerHenkan:          new(""),
		MarkerHenkanSelect:    new(""),
		EggLikeNewline:        true,
		RegisterConvertResult: true,

		// register_kanatable('rom', {'jj': 'escape', '~': ['〜', '']})
		KanaTable: engine.RomTable.With(
			engine.Rule{From: "jj", Func: "escape"},
			engine.Rule{From: "~", Kana: "〜"},
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
	b.AddSource(skk.New(p))
	b.AddSource(skkokuri.New(p))
	b.AddFilter(matcherhead.New(matcherhead.Params{}))
	b.AddFilter(sorterrank.New())
	b.PatchGlobal(cmp.Config{
		UI:      "pum",
		Sources: []string{"around"},
		SourceOptions: map[string]cmp.SourceOption{
			"_": {
				Matchers: []string{"matcherhead"},
				Sorters:  []string{"sorterrank"},
			},
			// In the order of the input method, all of them.
			"skk":      {Mark: "skk", Matchers: []string{}, Sorters: []string{}},
			"skkokuri": {Mark: "skk", Matchers: []string{}, Sorters: []string{}},
		},
	})

	cmpPlugin, err := b.Plugin()
	if err != nil {
		log.Fatal(err)
	}
	// goda-cmp.vim and the input method, in one binary.
	host.Main(cmpPlugin, p)
}
