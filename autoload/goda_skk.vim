" goda-skk.vim: SKK input for Vim/Neovim on goda.vim.
"
" The input method is the "skk" plugin of a goda.vim binary (package engine,
" built from the user's configuration). While it is enabled, the keys of
" g:goda_skk#mapped_keys are mapped in the buffer; each one is sent to the
" plugin, which answers with the keys to type: backspaces over the old
" pre-edit, what was confirmed, and the new pre-edit (▽かな, ▼漢字).

let g:goda_skk#enabled = v:false
let g:goda_skk#mode = ''
let g:goda_skk#state = #{phase: '', henkanFeed: ''}

" ---------------------------------------------------------------------------
" The binary

" The binary is machine specific, so it is built into a cache directory, not
" into the configuration directory (which belongs in your dotfiles).
function! s:cache_dir() abort
  if has('nvim')
    let base = stdpath('cache')
  elseif !empty($XDG_CACHE_HOME)
    let base = $XDG_CACHE_HOME . '/vim'
  elseif has('win32') && !empty($LOCALAPPDATA)
    let base = $LOCALAPPDATA . '/vim'
  else
    let base = expand('~/.cache/vim')
  endif
  return base . '/goda-skk'
endfunction

function! s:binary() abort
  if exists('g:goda_skk#binary')
    return fnamemodify(expand(g:goda_skk#binary), ':p')
  endif
  return s:cache_dir() . '/goda-skk' . (has('win32') ? '.exe' : '')
endfunction

function! s:config_dir() abort
  if exists('g:goda_skk#config_dir')
    return expand(g:goda_skk#config_dir)
  endif
  return (has('nvim') ? stdpath('config') : expand('~/.vim')) . '/goda-skk'
endfunction

" Build the configuration (a Go module in g:goda_skk#config_dir) into
" g:goda_skk#binary. Needs Go. A running binary is stopped, to be started
" again with the next key.
function! goda_skk#build() abort
  let dir = s:config_dir()
  if !isdirectory(dir)
    throw 'goda-skk: no config directory: ' . dir
  endif
  let bin = s:binary()
  call mkdir(fnamemodify(bin, ':h'), 'p')
  call goda_skk#disable()
  " "go -C" runs go in that directory: no cd, which differs between shells.
  let out = system('go -C ' . shellescape(dir) . ' build -o ' . shellescape(bin) . ' .')
  if v:shell_error
    throw 'goda-skk: go build failed:' . "\n" . out
  endif
  if goda#is_running('skk') && get(goda#plugins(), 'skk', '') ==# bin
    call goda#stop('skk')
  endif
  echomsg 'goda-skk: built ' . bin
endfunction

" Start the binary unless a running one serves "skk" already (another
" binary can contain the plugin).
function! s:start() abort
  if goda#is_running('skk')
    return
  endif
  let bin = s:binary()
  if !executable(bin)
    throw 'goda-skk: ' . bin . ' does not exist yet; build it with: call goda_skk#build()'
  endif
  let plugins = goda#start(bin)
  if index(plugins, 'skk') < 0
    if !empty(plugins)
      call goda#stop(plugins[0])
    endif
    throw 'goda-skk: ' . bin . ' does not contain the "skk" plugin'
  endif
endfunction

" ---------------------------------------------------------------------------
" Keys

" The notation of a key for the plugin: special keys in lower case.
function! s:notation(lhs) abort
  if a:lhs ==? '<lt>'
    return '<'
  elseif a:lhs ==? '<Bar>'
    return '|'
  elseif a:lhs =~# '^<.\+>$'
    return tolower(a:lhs)
  endif
  return a:lhs
endfunction

" ---------------------------------------------------------------------------
" Handling

" Where the input goes: "i" (insert mode), "c" (the command line) or "t"
" (a terminal job).
function! s:vim_mode() abort
  let mode = mode()
  return mode ==# 'c' || mode ==# 't' ? mode : 'i'
endfunction

" What Vim tells the plugin with every key.
function! s:status() abort
  let mode = s:vim_mode()
  if mode ==# 'c'
    let prev = getcmdline()[: getcmdpos() - 2]
  elseif mode ==# 't'
    " What the job shows is not what is typed.
    let prev = ''
  else
    let prev = getline('.')[: col('.') - 2]
  endif
  let [visible, selected, confirm] = [v:false, -1, '']
  if exists('*pum#visible') && pum#visible()
    let info = pum#complete_info()
    let [visible, selected] = [v:true, get(info, 'selected', -1)]
    let confirm = "\<Cmd>call pum#map#confirm()\<CR>"
  elseif mode ==# 'i' && pumvisible()
    let [visible, selected] = [v:true, complete_info(['selected']).selected]
    let confirm = "\<C-y>"
  endif
  return #{mode: mode, prevInput: prev, bufnr: bufnr('%'), lnum: line('.'),
        \ pumVisible: visible, selected: selected, confirmKey: confirm}
endfunction

" Send {keys} to the plugin with {func} ("handleKey", "enable", "disable",
" "toggle", or a function of the plugin) and type its answer.
function! goda_skk#handle(func, keys = []) abort
  if a:func ==# 'disable' && !g:goda_skk#enabled
    return ''
  endif
  if mode() ==# 'R'
    echohl WarningMsg | echomsg 'goda-skk: not in Replace mode' | echohl None
    return ''
  endif
  try
    call s:start()
    let r = goda#request('skk', 'handle', a:func, a:keys, s:status(),
          \ g:goda_skk#enabled)
  catch
    echohl ErrorMsg | echomsg v:exception | echohl None
    return ''
  endtry

  if r.enable && !g:goda_skk#enabled
    call s:enable(r.mappedKeys)
  endif
  let g:goda_skk#state = #{phase: r.phase, henkanFeed: r.henkanFeed}
  let keys = r.keys
  if r.disable
    call s:disable(r.reenable)
  endif
  call s:set_mode(r.mode)
  let s:popup = r.popup
  let s:pre_edit = r.preEdit
  if r.register !=# ''
    " After the keys: ask for the word to register.
    let s:register_prompt = r.register
    let keys .= "\<Cmd>call goda_skk#_register_word()\<CR>"
  endif
  if r.restore isnot v:null
    " After the text landed: where kakuteiUndo left from.
    let keys .= printf("\<Cmd>call goda_skk#_restore_point(%d, %d)\<CR>",
          \ r.restore.lnum, r.restore.col)
  endif
  call feedkeys(keys . "\<Cmd>call goda_skk#_handled()\<CR>", 'nit')
  return ''
endfunction

" Only when there is one: an event without autocommands says so, and in
" Vim that keeps TextChangedI from coming.
function! s:doautocmd(name) abort
  if exists('#User#' . a:name)
    execute 'doautocmd <nomodeline> User' a:name
  endif
endfunction

function! s:set_mode(mode) abort
  if a:mode ==# g:goda_skk#mode
    return
  endif
  let g:goda_skk#mode = a:mode
  call s:doautocmd('goda-skk-mode-changed')
endfunction

" After the keys were typed: the list of candidates, the highlight of the
" pre-edit, User goda-skk-handled.
function! goda_skk#_handled() abort
  call goda_skk#popup#close()
  call goda_skk#highlight#clear()
  if g:goda_skk#enabled
    if !empty(get(s:, 'popup', []))
      call goda_skk#popup#open(s:popup)
    endif
    call goda_skk#highlight#update(g:goda_skk#state.phase, get(s:, 'pre_edit', ''))
  endif
  call s:doautocmd('goda-skk-handled')
endfunction

" For kakuteiUndo: put the cursor right after {kakutei} when the line still
" starts with {before} and it.
function! goda_skk#_locate_kakutei(bufnr, lnum, before, kakutei) abort
  if mode() !=# 'i' || a:bufnr != bufnr('%') || a:lnum != line('.')
    return v:false
  endif
  let head = a:before . a:kakutei
  if strpart(getline('.'), 0, strlen(head)) !=# head
    return v:false
  endif
  call cursor(a:lnum, strlen(head) + 1)
  return v:true
endfunction

" A completion (goda-skk.vim/cmp/source/...) replaced the pre-edit.
function! goda_skk#_complete_done(phase) abort
  let g:goda_skk#state = #{phase: a:phase, henkanFeed: ''}
  let [s:popup, s:pre_edit] = [[], '']
  call goda_skk#popup#close()
  call goda_skk#highlight#clear()
  if g:goda_skk#enabled
    call s:doautocmd('goda-skk-handled')
  endif
endfunction

function! goda_skk#_restore_point(lnum, col) abort
  if mode() ==# 'i' && line('.') == a:lnum
    call cursor(a:lnum, a:col)
  endif
endfunction

function! goda_skk#_key(mode, i) abort
  let [func, key] = b:goda_skk_keys[a:mode][a:i]
  return goda_skk#handle(func, [key])
endfunction

" The registration of a word: ask for it on the command line, where the
" input method is enabled, and tell the plugin ('' when cancelled). The
" input method of where it was asked from is put aside meanwhile.
function! goda_skk#_register_word() abort
  let outer = #{enabled: g:goda_skk#enabled, mode: g:goda_skk#mode,
        \ state: g:goda_skk#state, enabled_mode: get(s:, 'enabled_mode', 'i'),
        \ popup: get(s:, 'popup', []), pre_edit: get(s:, 'pre_edit', '')}
  call goda_skk#popup#close()
  let g:goda_skk#enabled = v:false
  augroup goda_skk_enabled
    autocmd!
    autocmd CmdlineEnter * ++once call goda_skk#enable()
  augroup END
  " The cursor stays where it is at the end of the line.
  let virtualedit = &l:virtualedit
  setlocal virtualedit=all
  let word = ''
  try
    let word = input(s:register_prompt)
  catch /^Vim:Interrupt$/
  finally
    if g:goda_skk#enabled
      call s:disable()
    endif
    let &l:virtualedit = virtualedit
    let g:goda_skk#enabled = outer.enabled
    let g:goda_skk#state = outer.state
    let s:enabled_mode = outer.enabled_mode
    let [s:popup, s:pre_edit] = [outer.popup, outer.pre_edit]
    if outer.enabled
      call s:autocmds()
      " What the prompt's disabling undid (the sources of a completion,
      " say) is done again.
      call s:doautocmd('goda-skk-enable-pre')
      call s:doautocmd('goda-skk-enable-post')
    else
      augroup goda_skk_enabled
        autocmd!
      augroup END
    endif
    call s:set_mode(outer.mode)
    echo '' | redraw
  endtry
  call goda_skk#handle('registerDone', [word])
endfunction

function! goda_skk#enable() abort
  return goda_skk#handle('enable')
endfunction

function! goda_skk#disable() abort
  return goda_skk#handle('disable')
endfunction

function! goda_skk#toggle() abort
  return goda_skk#handle('toggle')
endfunction

function! goda_skk#mode() abort
  return g:goda_skk#enabled ? g:goda_skk#mode : ''
endfunction

function! goda_skk#is_enabled() abort
  return g:goda_skk#enabled
endfunction

" ---------------------------------------------------------------------------
" Enabling: buffer mappings and options

" {keys}: the keys the configuration uses, from the plugin.
function! s:enable(keys) abort
  call s:doautocmd('goda-skk-enable-pre')
  let mode = s:vim_mode()
  call goda_skk#internal#save(mode)
  let keys = get(g:, 'goda_skk#mapped_keys', []) + a:keys
  " The keys are not written into the mappings: quotes, backslashes and bars
  " would need escaping, differently in each.
  let b:goda_skk_keys = get(b:, 'goda_skk_keys', {})
  let b:goda_skk_keys[mode] = []
  for lhs in uniq(sort(keys))
    " A key the user mapped to <Plug>(goda-skk-...) keeps doing that.
    let func = 'handleKey'
    let m = matchlist(maparg(lhs, mode), '<Plug>(goda-skk-\(\a\+\))')
    if !empty(m)
      let func = m[1]
    endif
    execute printf('%snoremap <buffer> <nowait> %s <Cmd>call goda_skk#_key(%s, %d)<CR>',
          \ mode, lhs, string(mode), len(b:goda_skk_keys[mode]))
    call add(b:goda_skk_keys[mode], [func, s:notation(lhs)])
  endfor
  let g:goda_skk#enabled = v:true
  let s:enabled_mode = mode
  call s:autocmds()
  call s:doautocmd('goda-skk-enable-post')
endfunction

function! s:autocmds() abort
  augroup goda_skk_enabled
    autocmd!
    " Leaving insert mode, the command line or the terminal job ends the
    " input.
    autocmd ModeChanged *:n,*:nt call s:reset()
    autocmd WinLeave * call s:reset()
  augroup END
endfunction

function! s:disable(reenable = v:false) abort
  if !g:goda_skk#enabled
    return
  endif
  call s:doautocmd('goda-skk-disable-pre')
  call goda_skk#popup#close()
  call goda_skk#highlight#clear()
  call goda_skk#internal#restore(get(s:, 'enabled_mode', 'i'))
  let g:goda_skk#enabled = v:false
  augroup goda_skk_enabled
    autocmd!
  augroup END
  if a:reenable
    autocmd goda_skk_enabled InsertEnter <buffer> ++once
          \ call feedkeys("\<Cmd>call goda_skk#enable()\<CR>", 'n')
  endif
  call s:set_mode('')
  call s:doautocmd('goda-skk-disable-post')
endfunction

function! s:reset() abort
  if goda#is_running('skk')
    call goda#notify('skk', 'reset')
  endif
  call s:disable()
endfunction
