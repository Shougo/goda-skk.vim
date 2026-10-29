package engine

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/Shougo/goda.vim/host"
	"github.com/Shougo/goda.vim/sdk"
)

// PluginName is the name of the plugin: goda#request('skk', ...).
const PluginName = "skk"

// Plugin is the input method, a goda.vim plugin.
type Plugin struct {
	cfg     Config
	lib     *library
	keymaps map[string]keymap

	// mappedKeys are the keys Vim maps while enabled.
	mappedKeys []string

	mu       sync.Mutex
	c        *Context
	lastMode string
	// outer are the contexts put aside while Vim asks for a word to
	// register (registerWord): the keys of the prompt have their own.
	outer []outerContext
}

type outerContext struct {
	c        *Context
	lastMode string
}

// New returns the plugin with the configuration, or an error for an
// invalid one. The dictionaries that read files start reading them now, in
// the background.
func New(cfg Config) (*Plugin, error) {
	cfg, err := cfg.fill()
	if err != nil {
		return nil, fmt.Errorf("goda-skk: %w", err)
	}
	lib := &library{dicts: cfg.Dictionaries, logf: log.Printf}
	if cfg.UserDictionary != "-" {
		lib.user = NewUserDictionary(cfg.UserDictionary)
		lib.user.logf = log.Printf
	}
	for _, d := range cfg.Dictionaries {
		if in, ok := d.(Initializer); ok {
			go func() {
				if err := in.Init(context.Background()); err != nil {
					log.Printf("goda-skk: %v", err)
				}
			}()
		}
	}
	p := &Plugin{cfg: cfg, lib: lib, keymaps: buildKeymaps(cfg.Keymaps), lastMode: ModeHira}
	p.mappedKeys = mappedKeys(&p.cfg, p.keymaps)
	p.c = p.newContext()
	return p, nil
}

// Main runs the plugin as the whole program (the last line of a main
// function); with an invalid configuration, it exits with the error.
func Main(cfg Config) {
	p, err := New(cfg)
	if err != nil {
		log.Fatal(err)
	}
	host.Main(p)
}

func (p *Plugin) newContext() *Context {
	c := newContext(&p.cfg, p.lib)
	c.keymaps = p.keymaps
	return c
}

func (p *Plugin) Name() string { return PluginName }

func (p *Plugin) Handlers() map[string]sdk.Handler {
	return map[string]sdk.Handler{
		"handle":  p.handle,
		"reset":   p.reset,
		"preedit": p.preedit,
	}
}

// Status is what Vim tells with every key: where the input goes and what
// is before the cursor, and the completion popup.
type Status struct {
	Mode       string `json:"mode"`
	PrevInput  string `json:"prevInput"`
	Bufnr      int    `json:"bufnr"`
	Lnum       int    `json:"lnum"`
	PumVisible bool   `json:"pumVisible"`
	Selected   int    `json:"selected"`
	ConfirmKey string `json:"confirmKey"`
}

// Result is the answer to handle.
type Result struct {
	// Keys to feed: backspaces over the old pre-edit, what was confirmed,
	// and the new pre-edit.
	Keys string `json:"keys"`
	// Mode of the input after the keys ("" when disabled).
	Mode string `json:"mode"`
	// Phase is the state ("input", "input:okurinasi", "input:okuriari",
	// "henkan" or "escape"), HenkanFeed the reading.
	Phase      string `json:"phase"`
	HenkanFeed string `json:"henkanFeed"`
	// Popup lists candidates to show; Enable and Disable turn the input
	// method on and off.
	Popup   []string `json:"popup"`
	Enable  bool     `json:"enable"`
	Disable bool     `json:"disable"`
	// Reenable: enable again at the next insert mode (Config.KeepState).
	Reenable bool `json:"reenable"`
	// PreEdit is the pre-edit after the keys: it ends at the cursor.
	PreEdit string `json:"preEdit"`
	// Restore is where to put the cursor after the keys (kakuteiUndo).
	Restore *cursorPoint `json:"restore"`
	// MappedKeys are the keys to map, with Enable.
	MappedKeys []string `json:"mappedKeys"`
	// Register is the prompt of a word to register: Vim asks for it, then
	// runs registerDone with it.
	Register string `json:"register"`
}

// handle runs fn ("handleKey", "enable", "disable", "toggle",
// "registerDone" or the name of a function) for keys, with the status of
// Vim.
func (p *Plugin) handle(ctx context.Context, h sdk.Host, params []any) (any, error) {
	var args struct {
		Fn      string
		Keys    []string
		Status  Status
		Enabled bool
	}
	if len(params) < 3 {
		return nil, fmt.Errorf("handle: want fn, keys and status")
	}
	if err := sdk.Decode(params[0], &args.Fn); err != nil {
		return nil, err
	}
	if err := sdk.Decode(params[1], &args.Keys); err != nil {
		return nil, err
	}
	if err := sdk.Decode(params[2], &args.Status); err != nil {
		return nil, err
	}
	if len(params) > 3 {
		_ = sdk.Decode(params[3], &args.Enabled)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.run(ctx, h, args.Fn, args.Keys, args.Status, args.Enabled)
}

func (p *Plugin) run(ctx context.Context, h sdk.Host, fn string, keys []string, st Status, enabled bool) (Result, error) {
	if fn == "registerDone" {
		// Back from the prompt: the context that asked.
		if len(p.outer) == 0 {
			return Result{}, fmt.Errorf("goda-skk: registerDone without registerWord")
		}
		o := p.outer[len(p.outer)-1]
		p.outer = p.outer[:len(p.outer)-1]
		p.c, p.lastMode = o.c, o.lastMode
	}
	c := p.c
	c.ctx, c.host = ctx, h
	c.popup, c.disable, c.reenable = nil, false, false
	c.vimMode, c.prevInput = st.Mode, st.PrevInput
	c.bufnr, c.lnum = st.Bufnr, st.Lnum
	// Where the cursor goes back to after kakuteiUndo is only good while
	// something is typed there.
	if !c.hasPendingInput() {
		c.undo.remembered = nil
	}
	// Only now is it known where the last conversion was typed.
	resolved := c.resolvePendingKakutei()
	// The buffer is not what the pre-edit says (a completion changed it,
	// or the cursor moved): start over from there.
	if st.Mode != "t" && !strings.HasSuffix(st.PrevInput, c.text()) {
		if !resolved {
			c.invalidateKakutei()
		}
		c.resetState(true, false)
		c.pre.output("")
	}
	if fn == "toggle" {
		fn = "enable"
		if enabled {
			fn = "disable"
		}
	}
	// Converting, or a romaji typed: the enabling key confirms.
	converting := c.state.kind != stateInput || c.state.mode != modeDirect || c.state.feed != ""
	switch {
	case fn == "enable" && (!enabled || !converting):
		// Enabling starts afresh; while converting, the key is typed.
		c = p.newContext()
		c.ctx, c.host = ctx, h
		c.vimMode, c.prevInput = st.Mode, st.PrevInput
		c.bufnr, c.lnum = st.Bufnr, st.Lnum
		p.c = c
		if p.cfg.KeepMode && p.lastMode != ModeHira {
			if f := modeFunctions[p.lastMode]; f != nil {
				f(c, "")
			}
		}
		res := p.result("")
		res.Enable = true
		res.MappedKeys = p.mappedKeys
		return res, nil
	case fn == "disable" && !(converting && len(keys) > 0):
		disableFunc(c, "")
		return p.result(c.pre.output(c.text())), nil
	case fn == "enable" || fn == "disable":
		fn = "handleKey"
	}

	if fn == "registerDone" {
		word := ""
		if len(keys) > 0 {
			word = keys[0]
		}
		registerDone(c, word)
		return p.result(c.pre.output(c.text())), nil
	}

	before := c.mode
	if st.PumVisible && fn == "handleKey" && strings.Join(keys, "") == "<cr>" &&
		st.Selected >= 0 && p.cfg.EggLikeNewline && st.ConfirmKey != "" {
		// <CR> accepts the completion.
		c.resetState(true, false)
		c.pre.output("")
		return p.result(st.ConfirmKey), nil
	}
	if fn == "handleKey" {
		for _, k := range keys {
			handleKey(c, k)
			if c.register != nil {
				break
			}
		}
	} else {
		f := functions[fn]
		if f == nil {
			return Result{}, fmt.Errorf("goda-skk: unknown function %q", fn)
		}
		for _, k := range keys {
			f(c, k)
		}
		if len(keys) == 0 {
			f(c, "")
		}
	}
	out := c.pre.output(c.text())
	if out == "" && before != c.mode {
		// Something to feed, so that the mode shown is redrawn.
		out = " \b"
	}
	res := p.result(out)
	if r := c.register; r != nil {
		// Vim asks for the word next; meanwhile, keys go to a new context.
		res.Register, res.Popup = r.prompt, nil
		p.outer = append(p.outer, outerContext{c, p.lastMode})
		p.c = p.newContext()
	}
	return res, nil
}

// result is the Result of the handling, with keys to feed.
func (p *Plugin) result(keys string) Result {
	c := p.c
	if p.cfg.KeepMode {
		p.lastMode = c.mode
	}
	mode := c.mode
	if c.disable {
		mode = ""
	}
	// The cursor belongs to what is typed now, if anything.
	restore := c.undo.restore
	c.undo.restore = nil
	if c.hasPendingInput() {
		restore = nil
	}
	pre := c.pre.current
	if c.state.kind == stateEscape {
		pre = ""
	}
	return Result{
		PreEdit:    pre,
		Restore:    restore,
		Keys:       keys,
		Mode:       mode,
		Phase:      c.state.phase(),
		HenkanFeed: c.state.henkanFeed,
		Popup:      c.popup,
		Disable:    c.disable,
		Reenable:   c.reenable,
	}
}

// reset forgets the input (Vim left insert mode).
func (p *Plugin) reset(context.Context, sdk.Host, []any) (any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.c, p.outer = p.newContext(), nil
	return nil, nil
}

// preedit is the pre-edit: what is being typed.
func (p *Plugin) preedit(context.Context, sdk.Host, []any) (any, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.c.text(), nil
}

// modeFunctions start the modes, for KeepMode.
var modeFunctions = map[string]Func{
	ModeHira:    hirakana,
	ModeKata:    katakana,
	ModeHankata: hankatakana,
	ModeZenkaku: zenkaku,
}

var _ sdk.Plugin = (*Plugin)(nil)
