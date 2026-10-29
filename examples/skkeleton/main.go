// Command skkeleton is the configuration of a skkeleton user, translated:
// skkeleton#config(), register_kanatable() and register_keymap(). Its Vim
// script is in vimrc.vim.
package main

import (
	"runtime"

	"github.com/Shougo/goda-skk.vim/skk"
)

func main() {
	jisyo := "/usr/share/skk/SKK-JISYO.L"
	if runtime.GOOS == "windows" {
		jisyo = "~/.config/SKK-JISYO.L"
	}
	skk.Main(skk.Config{
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
}
