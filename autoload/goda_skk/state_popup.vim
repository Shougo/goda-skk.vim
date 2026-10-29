" A popup by the cursor that shows the mode and the phase in insert mode:
" the labels of goda_skk#state_popup#config().

let s:config = #{labels: {}, opts: {}}
let s:win = -1
let s:buf = -1

" {config}: {labels: {phase: {mode: label}}, opts: options of popup_create()
" (Vim) or nvim_open_win() (Neovim)}. The phases are those of
" g:goda_skk#state; the label "latin" (not in a phase) is shown while the
" input method is disabled.
function! goda_skk#state_popup#config(config) abort
  let s:config.labels = get(a:config, 'labels', {})
  let s:config.opts = get(a:config, 'opts', {})
endfunction

function! goda_skk#state_popup#enable() abort
  augroup goda_skk_state_popup
    autocmd!
    autocmd User goda-skk-handled,goda-skk-disable-post call s:update()
    autocmd InsertEnter * call s:update()
    if has_key(s:config.labels, 'latin')
      autocmd CursorMovedI * call s:update()
    endif
    autocmd ModeChanged i*:n* call s:close()
  augroup END
endfunction

function! goda_skk#state_popup#disable() abort
  call s:close()
  augroup goda_skk_state_popup
    autocmd!
  augroup END
endfunction

function! s:label() abort
  if !g:goda_skk#enabled || g:goda_skk#mode ==# ''
    return get(s:config.labels, 'latin', '')
  endif
  let phase = get(g:goda_skk#state, 'phase', '')
  return get(get(s:config.labels, phase, {}), g:goda_skk#mode, '')
endfunction

function! s:update() abort
  " After the keys of the event were typed: the cursor is where they left
  " it.
  if mode() !~# '^i'
    return
  endif
  let label = s:label()
  if label ==# ''
    call s:close()
    return
  endif
  if has('nvim')
    if !bufexists(s:buf)
      let s:buf = nvim_create_buf(v:false, v:true)
    endif
    call nvim_buf_set_lines(s:buf, 0, -1, v:true, [label])
    let opts = extend(#{relative: 'cursor', row: 1, col: 0, style: 'minimal',
          \ focusable: v:false}, s:config.opts)
    let opts.width = strwidth(label)
    let opts.height = 1
    if s:win > 0 && nvim_win_is_valid(s:win)
      call nvim_win_set_config(s:win, opts)
    else
      let s:win = nvim_open_win(s:buf, v:false, opts)
    endif
  else
    let opts = extend(#{line: 'cursor+1', col: 'cursor'}, s:config.opts)
    if s:win > 0 && index(popup_list(), s:win) >= 0
      call popup_settext(s:win, label)
      call popup_move(s:win, opts)
    else
      let s:win = popup_create(label, opts)
    endif
  endif
endfunction

function! s:close() abort
  if has('nvim')
    if s:win > 0 && nvim_win_is_valid(s:win)
      call nvim_win_close(s:win, v:true)
    endif
  elseif s:win > 0
    call popup_close(s:win)
  endif
  let s:win = -1
endfunction
