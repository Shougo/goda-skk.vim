# goda-skk.vim

SKK input method for Vim and Neovim, built on
[goda.vim](https://github.com/Shougo/goda.vim). The successor of
[skkeleton](https://github.com/vim-skk/skkeleton): the same way of typing,
with the input method written in Go and configured in Go — a small program
you own.

Status: early. Conversion with okurigana, registering words, the user
dictionary and dictionary files, kakuteiUndo, insert mode, the command line
and terminal jobs work; the highlight of the pre-edit and the state popup
(skkeleton-henkan-highlight and skkeleton-state-popup) are built in, and
[goda-cmp.vim](https://github.com/Shougo/goda-cmp.vim) completes readings
(the sources `cmp/source/skk` and `cmp/source/skkokuri`, the ddc.vim
sources of skkeleton). skkserv and Google
Japanese Input are to come.

A skkeleton configuration translated, with the completion of goda-cmp.vim,
is in [examples/skkeleton](examples/skkeleton).

## Try it

You need Go 1.26 or newer, Vim 9.1 or newer (with `+job +channel +popupwin`)
or Neovim 0.10 or newer, and an SKK dictionary (SKK-JISYO.L, say). Install
`Shougo/goda.vim` and `Shougo/goda-skk.vim` with your plugin manager, then
create your configuration, a Go module of your own (keep it in your
dotfiles; `examples/config` is only a template to copy):

```sh
mkdir -p ~/.vim/goda-skk && cd ~/.vim/goda-skk        # Neovim: ~/.config/nvim/goda-skk
cp /path/to/goda-skk.vim/examples/config/main.go .
go mod init goda-skk-config
go get github.com/Shougo/goda-skk.vim@latest
go mod tidy
```

The configuration:

```go
package main

import "github.com/Shougo/goda-skk.vim/engine"

func main() {
	engine.Main(engine.Config{
		Dictionaries: []engine.Dictionary{
			engine.NewJisyo("~/.skk/SKK-JISYO.L", ""),
		},
		EggLikeNewline: true,
	})
}
```

Add to your vimrc / init.vim:

```vim
map! <C-j> <Plug>(goda-skk-enable)
tmap <C-j> <Plug>(goda-skk-enable)
```

Then build it once (and after every change of the configuration):

```vim
:call goda_skk#build()
```

In insert mode, <C-j> and type: `Kanji<Space>` gives ▼漢字, <C-j> confirms.
If nothing happens, `:echo goda#log()` shows what the plugin said.

Everything else is in `:help goda_skk` ([doc/goda_skk.txt](doc/goda_skk.txt)).

## Tests

```sh
go test ./...        # unit tests; also real Vim and Neovim if present
go test -short ./... # unit tests only
```

## Credits

goda-skk.vim is a port of [skkeleton](https://github.com/vim-skk/skkeleton)
by kuuote: its kana tables and much of its logic come from skkeleton, under
the zlib license (see [NOTICE](NOTICE)). Thanks!

## License

MIT; see [LICENSE](LICENSE). The parts from skkeleton: zlib, see
[NOTICE](NOTICE).
