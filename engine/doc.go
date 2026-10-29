// Package engine is the SKK input method, a goda.vim plugin. A user's
// configuration is a main package that calls Main with a Config:
//
//	func main() {
//		engine.Main(engine.Config{
//			Dictionaries: []engine.Dictionary{engine.NewJisyo("~/.skk/SKK-JISYO.L", "")},
//		})
//	}
//
// The Vim script of goda-skk.vim sends every key to the plugin, which
// answers with the keys to type.
//
// This package is a port of skkeleton (https://github.com/vim-skk/skkeleton,
// Copyright (c) 2021 kuuote, zlib License; see NOTICE) from TypeScript to Go,
// altered: the configuration, the dictionaries and the interface with Vim
// differ from skkeleton's.
package engine
