" goda-skk.vim: SKK input for Vim/Neovim on goda.vim.
"
" The input method is the "skk" plugin of a goda.vim binary (package engine,
" built from the user's configuration). While it is enabled, the keys of
" g:goda_skk#mapped_keys are mapped in the buffer; each one is sent to the
" plugin, which answers with the keys to type: backspaces over the old
" pre-edit, what was confirmed, and the new pre-edit (▽かな, ▼漢字).

" The version of this Vim script that the binary goes with (ScriptVersion of
" its Go side): goda.vim refuses a binary of another one, and says to build
" it again. Bumped with it when they no longer work together.
let s:version = #{version: 6, module: 'github.com/Shougo/goda-skk.vim'}

let g:goda_skk#enabled = v:false
let g:goda_skk#mode = ''
let g:goda_skk#state = #{phase: '', henkanFeed: ''}

" ---------------------------------------------------------------------------
" The binary

" (The binary has no "status" to ask.)
let s:goda = goda#configured#new('skk', extend(#{status: v:false}, s:version))

" Build the configuration (a Go module in g:goda_skk#config_dir) into
" g:goda_skk#binary with goda#build(): with the plugins installed
" (g:goda#workspace). Needs Go. A running binary is stopped, to be started
" again with the next key.
function! goda_skk#build() abort
  call goda_skk#disable()
  call s:goda.build()
  call s:goda.register()
endfunction

call s:goda.register()

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
  let [visible, selected, confirm, cancel] = [v:false, -1, v:false, v:false]
  if s:completion('visible', v:false)
    let [visible, selected] = [v:true, s:completion('selected', -1)]
    let confirm = s:completion('confirm', '') !=# '' ? v:true : v:false
    let cancel = s:completion('cancel', '') !=# '' ? v:true : v:false
  endif
  return #{mode: mode, prevInput: prev, bufnr: bufnr('%'), lnum: line('.'),
        \ pumVisible: visible, selected: selected, canConfirm: confirm,
        \ canCancel: cancel, cmdtype: mode ==# 'c' ? getcmdtype() : ''}
endfunction

" The popup menu of insert mode: g:goda_skk#completion_backend.
let s:native = #{
      \ visible: {-> mode() =~# '^i' && pumvisible()},
      \ selected: {-> complete_info(['selected']).selected},
      \ confirm: {-> "\<C-y>"},
      \ cancel: {-> "\<C-e>"},
      \ close: {-> "\<C-y>"},
      \ }

" {name} of the completion backend called, or {default} without it.
function! s:completion(name, default) abort
  let backend = get(g:, 'goda_skk#completion_backend', s:native)
  return has_key(backend, a:name) ? call(backend[a:name], []) : a:default
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
  " A key typed in the command line of a pre-edit left by an inner one:
  " nothing was put in it.
  if !empty(get(s:, 'outer_stale', {})) && mode() ==# 'c'
        \ && getcmdtype() ==# s:outer_stale.at.cmdtype
    call s:clear_outer_stale()
  endif
  try
    let r = goda#request('skk', 'handle', a:func, a:keys, s:status(),
          \ g:goda_skk#enabled)
  catch
    echohl ErrorMsg | echomsg v:exception | echohl None
    return ''
  endtry

  " The completion of before is not of now: closed (#24 of skkeleton.nvim).
  let keys = (r.enable && !g:goda_skk#enabled || r.disable)
        \ && s:completion('visible', v:false) ? s:completion('close', '') : ''
  if r.enable && !g:goda_skk#enabled
    let s:mapped_keys = r.mappedKeys
    call s:enable(r.mappedKeys)
  endif
  let g:goda_skk#state = #{phase: r.phase, henkanFeed: r.henkanFeed}
  let keys .= r.keys
  if get(r, 'completionConfirm', v:false)
    " Not from the plugin: the keys may have special keys, which do not go
    " through JSON.
    let keys .= s:completion('confirm', '')
  endif
  if get(r, 'completionCancel', v:false)
    let keys .= s:completion('cancel', '')
  endif
  if get(r, 'completionClose', v:false)
    " After the text: whether it is still shown is known then. Vim's
    " popup menu closes on a newline, and its <C-y> would then type the
    " character above.
    let keys .= "\<Cmd>call goda_skk#_close_completion()\<CR>"
  endif
  if r.disable
    call s:disable(r.reenable)
  endif
  call s:set_mode(r.mode)
  if get(r, 'stale', '') !=# ''
    call s:remove_stale(r.stale, get(r, 'staleText', ''))
  endif
  let s:popup = r.popup
  let s:pre_edit = r.preEdit
  let s:pre_edit_text = get(r, 'preEditText', '')
  call s:register_tab(get(r, 'registering', v:false))
  if r.restore isnot v:null
    " After the text landed: where kakuteiUndo left from.
    let keys .= printf("\<Cmd>call goda_skk#_restore_point(%d, %d)\<CR>",
          \ r.restore.lnum, r.restore.col)
  endif
  " The pre-edit is deleted with <BS>, which 'backspace' without "start"
  " does not let past where insert mode started: back from <C-o>, the text
  " typed before. Until goda_skk#_handled().
  if stridx(keys, "\<C-h>") >= 0 && !has_key(s:, 'saved_backspace')
        \ && &backspace !~# 'start\|^[23]$'
    let s:saved_backspace = &backspace
    let &backspace = &backspace ==# '1' ? 'indent,eol,start'
          \ : join(filter(split(&backspace, ','), 'v:val !=# "0"') + ['start'], ',')
  endif
  " Not typed while a macro is recorded: the keys typed are, and they type
  " these again (#36 of skkeleton.nvim).
  call feedkeys(keys . "\<Cmd>call goda_skk#_handled()\<CR>",
        \ reg_recording() ==# '' ? 'nit' : 'ni')
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

" The pre-edit is text: when the input started over from {stale} (the cursor
" left it, or something else was typed after it), or was reset (another
" window, insert mode left with <C-c>), it is not the input any more. Where
" it still is, the command line loses it; insert mode keeps {text} of it,
" without the markers (what was typed after it, a completion that appended
" to it, say, belongs with it).
function! s:remove_stale(stale, text) abort
  let at = get(s:, 'pre_edit_at', {})
  let s:pre_edit_at = {}
  if a:stale ==# '' || get(at, 'text', '') !=# a:stale
    return
  endif
  let mode = mode()
  if at.mode ==# 'c' && mode ==# 'c' && getcmdtype() !=# at.cmdtype
    " A command line in the command line (<C-r>=): the pre-edit is in the
    " outer one, which loses it when it is back (#206 of skkeleton.nvim).
    call s:set_outer_stale(at, a:stale, a:text)
    return
  endif
  if at.mode ==# 'c' && mode ==# 'c' && getcmdtype() ==# at.cmdtype
    let [line, pos, keep] = [getcmdline(), getcmdpos(), '']
  elseif at.mode ==# 'i' && mode =~# '^[iRn]' && !pumvisible()
        \ && !s:completion('visible', v:false)
        \ && bufnr('%') == at.bufnr && line('.') == at.lnum
    " (Not under a completion, which knows where its text starts.) Insert
    " mode just left: the cursor is on the character before.
    let [line, pos, keep] = [getline('.'), col('.'), a:text]
    if mode ==# 'n'
      let pos += strlen(matchstr(line, '\%' . pos . 'c.'))
    endif
  else
    return
  endif
  let len = strlen(a:stale)
  if strpart(line, at.start - 1, len) !=# a:stale
    return
  endif
  if pos >= at.start + len
    let pos -= len - strlen(keep)
  elseif pos > at.start
    " Before what is kept of the rest.
    let rest = strpart(line, pos - 1, at.start + len - pos)
    let pos = keep =~# '\V' . escape(rest, '\') . '\$'
          \ ? at.start + strlen(keep) - strlen(rest) : at.start
  endif
  let line = strpart(line, 0, at.start - 1) . keep . strpart(line, at.start - 1 + len)
  if at.mode ==# 'c'
    call setcmdline(line, pos)
    return
  endif
  call setline('.', line)
  if mode ==# 'n' && pos > 1
    let pos -= strlen(matchstr(strpart(line, 0, pos - 1), '.$'))
  endif
  call cursor(line('.'), pos)
endfunction

" The pre-edit {stale} at {at} in the command line of an inner one goes
" when the outer one changes.
function! s:set_outer_stale(at, stale, text) abort
  let s:outer_stale = #{at: a:at, stale: a:stale, text: a:text}
  augroup goda_skk_outer_stale
    autocmd!
    autocmd CmdlineChanged * call s:remove_outer_stale()
    autocmd CmdlineLeave * call s:leave_outer_stale()
  augroup END
endfunction

" Back in the command line of the pre-edit left by an inner one: it goes.
function! s:remove_outer_stale() abort
  let outer = get(s:, 'outer_stale', {})
  if empty(outer) || getcmdtype() !=# outer.at.cmdtype
    return
  endif
  call s:clear_outer_stale()
  let s:pre_edit_at = outer.at
  call s:remove_stale(outer.stale, outer.text)
endfunction

" The command line of the pre-edit is left: nothing to remove any more.
function! s:leave_outer_stale() abort
  let outer = get(s:, 'outer_stale', {})
  if !empty(outer) && getcmdtype() ==# outer.at.cmdtype
    call s:clear_outer_stale()
  endif
endfunction

function! s:clear_outer_stale() abort
  let s:outer_stale = {}
  augroup goda_skk_outer_stale
    autocmd!
  augroup END
endfunction

" A command line in the command line of the input (<C-r>=, input()): its
" keys have an input of their own, and the input of the outer one is put
" aside meanwhile, to go on when it is back (#246 of skkeleton.nvim). The
" command lines that do not are counted too ({} in s:nested): they end
" first.
let s:nested = []
augroup goda_skk_nested
  autocmd!
  autocmd CmdlineEnter * call s:nested_enter()
  autocmd CmdlineLeave * call s:nested_leave()
augroup END

function! s:nested_enter() abort
  " (Leaving the command line resets the input: enabled there, it is still
  " the outer one.)
  if !g:goda_skk#enabled || get(s:, 'enabled_mode', '') !=# 'c'
    if !empty(s:nested)
      call add(s:nested, {})
    endif
    return
  endif
  try
    let r = goda#request('skk', 'push')
  catch
    call add(s:nested, {})
    return
  endtry
  call add(s:nested, #{mode: g:goda_skk#mode, state: g:goda_skk#state,
        \ popup: get(s:, 'popup', []), pre_edit: get(s:, 'pre_edit', ''),
        \ pre_edit_text: get(s:, 'pre_edit_text', ''),
        \ pre_edit_at: get(s:, 'pre_edit_at', {})})
  call goda_skk#popup#close()
  call goda_skk#highlight#clear()
  let g:goda_skk#state = #{phase: r.phase, henkanFeed: r.henkanFeed}
  let [s:popup, s:pre_edit, s:pre_edit_text, s:pre_edit_at] = [[], '', '', {}]
  call s:set_mode(r.mode)
endfunction

function! s:nested_leave() abort
  if empty(s:nested)
    return
  endif
  let outer = remove(s:nested, -1)
  if empty(outer)
    return
  endif
  if goda#is_running('skk')
    try
      call goda#request('skk', 'pop')
    catch
    endtry
  endif
  call goda_skk#popup#close()
  call goda_skk#highlight#clear()
  " Disabled in the inner one: enabled again, as it was.
  if !g:goda_skk#enabled
    call s:enable(get(s:, 'mapped_keys', []))
  endif
  let g:goda_skk#state = outer.state
  let [s:popup, s:pre_edit, s:pre_edit_text, s:pre_edit_at] =
        \ [outer.popup, outer.pre_edit, outer.pre_edit_text, outer.pre_edit_at]
  call s:set_mode(outer.mode)
  if !empty(s:popup)
    call goda_skk#popup#open(s:popup)
  endif
  call goda_skk#highlight#update(g:goda_skk#state.phase, s:pre_edit)
  if !get(v:event, 'abort', v:false) && s:pre_edit !=# '' && !empty(outer.pre_edit_at)
    " What it gives is put after the pre-edit, which goes then: it is not
    " the input any more (#206 of skkeleton.nvim).
    call s:set_outer_stale(outer.pre_edit_at, s:pre_edit, s:pre_edit_text)
  endif
endfunction

" After the keys were typed: the list of candidates, the highlight of the
" pre-edit, User goda-skk-handled.
function! goda_skk#_handled() abort
  if has_key(s:, 'saved_backspace')
    let &backspace = remove(s:, 'saved_backspace')
  endif
  " Where the pre-edit is, for s:remove_stale().
  let pre_edit = get(s:, 'pre_edit', '')
  let s:pre_edit_at = {}
  if mode() ==# 'c'
    let s:pre_edit_at = #{mode: 'c', text: pre_edit, cmdtype: getcmdtype(),
          \ start: getcmdpos() - strlen(pre_edit), wins: winnr('$')}
  elseif mode() =~# '^i'
    let s:pre_edit_at = #{mode: 'i', text: pre_edit, bufnr: bufnr('%'),
          \ lnum: line('.'), start: col('.') - strlen(pre_edit)}
  endif
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

" The reading a completion completed is gone: the completion is closed, if
" it is still shown.
function! goda_skk#_close_completion() abort
  if s:completion('visible', v:false)
    call feedkeys(s:completion('close', ''), 'ni')
  endif
endfunction

" A completion (goda-skk.vim/cmp/source/...) replaced the pre-edit.
function! goda_skk#_complete_done(phase) abort
  let g:goda_skk#state = #{phase: a:phase, henkanFeed: ''}
  let [s:popup, s:pre_edit, s:pre_edit_text] = [[], '', '']
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

" A key of a terminal job not for the input method (<C-a>, <Left>): the job
" moves or edits its line, which the input method cannot see, so the
" pre-edit is confirmed before it gets the key (#121 of skkeleton.nvim).
function! goda_skk#_pass(i) abort
  " First in the typeahead, after what the confirmation types.
  call feedkeys(eval('"\' . b:goda_skk_pass[a:i] . '"'), 'ni')
  if get(s:, 'pre_edit', '') !=# ''
    call goda_skk#handle('kakutei')
  endif
endfunction

" Reconversion (Normal mode): the conversion under the cursor, confirmed in
" insert mode before, goes back to choosing its candidates, as kakuteiUndo
" does right after it. Insert mode is entered after its text.
function! goda_skk#reconvert() abort
  if !goda#is_running('skk')
    return
  endif
  let r = goda#request('skk', 'reconvertAt', bufnr('%'), line('.'),
        \ col('.') - 1, getline('.'))
  if empty(r)
    echohl WarningMsg | echomsg 'goda-skk: no conversion here' | echohl None
    return
  endif
  call s:insert_at(r.end, 'reconvert', string(r.id))
endfunction

" Reconversion (Visual mode, characters of a line): a conversion the
" selection is in, as goda_skk#reconvert(); or kana, converted as if typed.
function! goda_skk#reconvert_selection() abort
  if !goda#is_running('skk')
    return
  endif
  let [lnum, c1] = getpos('v')[1:2]
  let [lnum2, c2] = getpos('.')[1:2]
  execute "normal! \<Esc>"
  if mode() !=# 'n' || lnum != lnum2 || visualmode() !=# 'v'
    echohl WarningMsg | echomsg 'goda-skk: select characters of a line' | echohl None
    return
  endif
  let [c1, c2] = [min([c1, c2]), max([c1, c2])]
  let line = getline(lnum)
  " The byte after the last character selected.
  let end = c2 - 1 + len(matchstr(line, '.', c2 - 1))
  let r = goda#request('skk', 'reconvertRange', bufnr('%'), lnum,
        \ c1 - 1, end, line)
  if has_key(r, 'id')
    call cursor(lnum, c1)
    call s:insert_at(r.end, 'reconvert', string(r.id))
  elseif has_key(r, 'kana')
    call setline(lnum, strpart(line, 0, c1 - 1) . strpart(line, end))
    call cursor(lnum, c1)
    call s:insert_at(c1 - 1, 'convertKana', r.kana)
  else
    echohl WarningMsg | echomsg 'goda-skk: no conversion nor kana here' | echohl None
  endif
endfunction

" Insert mode at the byte {col} (0-based) of the line, the input method
" enabled, and the function {func} of the plugin with {arg}. The keys go
" before what is typed after the mapping.
function! s:insert_at(col, func, arg) abort
  let keys = 'A'
  if a:col < len(getline('.'))
    call cursor(line('.'), a:col + 1)
    let keys = 'i'
  endif
  call feedkeys(keys . "\<Cmd>call goda_skk#handle('enable')\<CR>"
        \ . printf("\<Cmd>call goda_skk#handle(%s, [%s])\<CR>",
        \   string(a:func), string(a:arg)), 'ni')
endfunction

" <Tab> completes the word being registered in the pre-edit (registerTab of
" the plugin): mapped only meanwhile, as it is the completion's otherwise.
function! s:register_tab(on) abort
  let mode = get(s:, 'enabled_mode', 'i')
  if a:on && !get(s:, 'tab_mapped', v:false)
    let s:tab_saved = maparg('<Tab>', mode, 0, 1)
    execute mode . "noremap <buffer> <nowait> <Tab> <Cmd>call goda_skk#handle('registerTab')<CR>"
    let s:tab_mapped = v:true
  elseif !a:on && get(s:, 'tab_mapped', v:false)
    let s:tab_mapped = v:false
    silent! execute mode . 'unmap <buffer> <Tab>'
    if get(s:tab_saved, 'buffer', 0)
      call mapset(mode, 0, s:tab_saved)
    endif
  endif
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
  if mode ==# 't'
    " The other keys that move or edit the line of the job, unless mapped
    " (by the input method or the user): goda_skk#_pass(). Not those of
    " Vim after CTRL-\ (CTRL-\ CTRL-N, CTRL-\ CTRL-O) and of 'termwinkey',
    " which a mapping would break.
    let b:goda_skk_pass = []
    let ctrl = map(range(char2nr('a'), char2nr('z')), {_, n -> printf('<C-%s>', nr2char(n))})
    let skip = '<C-[jmno]>' . (has('nvim') ? '' : '\|<C-w>')
    if !has('nvim') && &termwinkey !=# ''
      let skip .= '\|\c' . escape(&termwinkey, '\')
    endif
    for lhs in filter(ctrl, {_, k -> k !~# skip})
          \ + ['<Left>', '<Right>', '<Up>', '<Down>', '<Home>', '<End>', '<Del>']
      if maparg(lhs, 't') !=# ''
        continue
      endif
      execute printf('tnoremap <buffer> <nowait> %s <Cmd>call goda_skk#_pass(%d)<CR>',
            \ lhs, len(b:goda_skk_pass))
      call add(b:goda_skk_pass, lhs)
    endfor
  endif
  let g:goda_skk#enabled = v:true
  let s:enabled_mode = mode
  let s:enabled_buf = bufnr('%')
  call s:autocmds()
  call s:doautocmd('goda-skk-enable-post')
endfunction

function! s:autocmds() abort
  augroup goda_skk_enabled
    autocmd!
    " Leaving insert mode, the command line or the terminal job ends the
    " input.
    autocmd ModeChanged *:n,*:nt call s:reset()
    " Replace mode (<Insert>) is not for the input method (#29 of
    " skkeleton.nvim). Not by the pattern "i*:R*": with 'fileignorecase'
    " (macOS, Windows) it matches "r?" too, the confirm() of purgeCandidate.
    autocmd ModeChanged i*:* if v:event.new_mode =~# '^R' | call s:reset() | endif
    " Back from a command of <C-o>, which may have moved the cursor: the
    " pre-edit left there starts over (#117 of skkeleton.nvim).
    autocmd ModeChanged i*:ni* call s:ctrl_o_enter()
    autocmd ModeChanged ni*:i* call s:check_stale()
    autocmd WinLeave * call s:win_leave()
    autocmd BufEnter * call s:buf_enter()
  augroup END
endfunction

function! s:disable(reenable = v:false) abort
  if !g:goda_skk#enabled
    return
  endif
  call s:doautocmd('goda-skk-disable-pre')
  " The buffer mappings go back as they were, <Tab> of a registration too.
  let s:tab_mapped = v:false
  call goda_skk#popup#close()
  call goda_skk#highlight#clear()
  call s:restore(get(s:, 'enabled_mode', 'i'), get(s:, 'enabled_buf', bufnr('%')))
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

" Where <C-o> started its command. At the end of the line the cursor is put
" on the last character meanwhile, and back after the return to insert mode
" (ModeChanged comes before): the same place and line are "not moved".
function! s:ctrl_o_enter() abort
  let s:ctrl_o = [bufnr('%'), line('.'), col('.'), getline('.')]
endfunction

function! s:check_stale() abort
  let before = get(s:, 'ctrl_o', [])
  unlet! s:ctrl_o
  if before == [bufnr('%'), line('.'), col('.'), getline('.')]
    return
  endif
  if g:goda_skk#enabled && get(s:, 'pre_edit', '') !=# ''
    " No key: only what the buffer says now.
    call goda_skk#handle('handleKey', [])
  endif
endfunction

" Terminal mode left (CTRL-\ CTRL-N, another window): the pre-edit in the
" line of the job, which the input method cannot change any more, is
" confirmed there, as <Esc> confirms it (#56 of skkeleton.nvim).
function! s:confirm_job() abort
  if get(s:, 'enabled_mode', '') !=# 't' || get(s:, 'pre_edit', '') ==# ''
        \ || &buftype !=# 'terminal'
    return
  endif
  try
    let r = goda#request('skk', 'handle', 'kakutei', [],
          \ #{mode: 't', prevInput: '', bufnr: bufnr('%'), lnum: line('.'),
          \   pumVisible: v:false, selected: -1, canConfirm: v:false,
          \   canCancel: v:false}, v:true)
  catch
    return
  endtry
  if r.keys ==# ''
    return
  endif
  if has('nvim')
    call chansend(b:terminal_job_id, r.keys)
  else
    call term_sendkeys(bufnr('%'), r.keys)
  endif
endfunction

" Another window: the input ends. Vim opening the command-line window
" (<C-f>) has made it already, and copies the command line into it after
" this: the pre-edit is left there, as text (as in Neovim, which does not
" leave the window then; #205 of skkeleton.nvim).
function! s:win_leave() abort
  let at = get(s:, 'pre_edit_at', {})
  if mode() ==# 'c' && get(at, 'mode', '') ==# 'c'
        \ && winnr('$') > get(at, 'wins', winnr('$'))
    let s:pre_edit_at = {}
  endif
  call s:reset()
endfunction

" Another buffer is entered without leaving Insert mode or the terminal job
" (a callback opening a file, as vim-guise does, or <Cmd>): no autocommand
" of leaving may come before (":tab drop" has only TabLeave), and the
" mappings are in the buffer left.
function! s:buf_enter() abort
  if bufnr('%') == get(s:, 'enabled_buf', bufnr('%')) || mode() =~# '^c'
    return
  endif
  call s:reset()
  call s:disable()
endfunction

" goda_skk#internal#restore({mode}) in the buffer {bufnr}, the one enabled:
" in a window of it, or when it is entered next.
function! s:restore(mode, bufnr) abort
  if a:bufnr == bufnr('%')
    call goda_skk#internal#restore(a:mode)
    return
  endif
  if !bufexists(a:bufnr)
    return
  endif
  let cmd = printf('call goda_skk#internal#restore(%s)', string(a:mode))
  let wins = win_findbuf(a:bufnr)
  if !empty(wins)
    call win_execute(wins[0], cmd)
  else
    execute printf('autocmd BufEnter <buffer=%d> ++once %s', a:bufnr, cmd)
  endif
endfunction

function! s:reset() abort
  " The outer command lines have no input to go on with either.
  call map(s:nested, '{}')
  if goda#is_running('skk')
    call s:confirm_job()
    call goda#notify('skk', 'reset')
  endif
  " The pre-edit is not the input any more (#27, #29, #46 of
  " skkeleton.nvim).
  call s:remove_stale(get(s:, 'pre_edit', ''), get(s:, 'pre_edit_text', ''))
  " What reads the state sees it reset too (#15 of skkeleton.nvim).
  let g:goda_skk#state = #{phase: '', henkanFeed: ''}
  let [s:popup, s:pre_edit, s:pre_edit_text, s:pre_edit_at] = [[], '', '', {}]
  call s:disable()
endfunction
