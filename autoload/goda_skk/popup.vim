" The list of candidates: a popup under the cursor (or above it when there is
" no room), or over the command line.

let s:windows = []
let s:opts = {}

" {config}: {opts: options of popup_create() (Vim) or nvim_open_win()
" (Neovim)}, over those that place the popup.
function! goda_skk#popup#config(config) abort
  let s:opts = get(a:config, 'opts', {})
endfunction

function! goda_skk#popup#open(lines) abort
  call goda_skk#popup#close()
  if empty(a:lines)
    return
  endif
  let width = max(map(copy(a:lines), 'strwidth(v:val)'))
  " The rows on the screen, with the border and the padding of the options:
  " Vim moves a popup that does not fit by itself, Neovim does not.
  let height = len(a:lines) + s:extra_rows()
  if mode() ==# 'c'
    let row = &lines + 1 - max([1, &cmdheight]) - height
    let col = max([strwidth(getcmdprompt()), 1])
    if has('nvim')
      call s:nvim_open(a:lines, #{relative: 'editor', row: row - 1, col: col,
            \ width: width, height: len(a:lines)})
    else
      call add(s:windows, popup_create(a:lines,
            \ extend(#{line: row, col: col + 1}, s:opts)))
    endif
    redraw
    return
  endif
  " Scrolled: where the cursor is now (#30 of skkeleton.nvim, for the
  " state popup).
  let s:lines = a:lines
  augroup goda_skk_popup
    autocmd!
    autocmd WinScrolled * call s:reopen()
  augroup END
  let pos = screenpos(0, line('.'), col('.'))
  " Neovim does not put a window over the command line.
  let above = &lines - pos.row - (has('nvim') ? &cmdheight : 0) < height
  if has('nvim')
    call s:nvim_open(a:lines, #{relative: 'cursor', row: above ? 0 : 1, col: 0,
          \ anchor: above ? 'SW' : 'NW', width: width, height: len(a:lines)})
  else
    call add(s:windows, popup_create(a:lines, extend(#{
          \ pos: above ? 'botleft' : 'topleft',
          \ line: above ? 'cursor-1' : 'cursor+1',
          \ col: 'cursor',
          \ }, s:opts)))
  endif
endfunction

" {place}: where it is, which the options may change, and its size, which
" they do not (as in the state popup).
function! s:nvim_open(lines, place) abort
  let buf = nvim_create_buf(v:false, v:true)
  call nvim_buf_set_lines(buf, 0, -1, v:true, a:lines)
  let opts = extend(extend(#{style: 'minimal', border: 'none', focusable: v:false},
        \ a:place), s:opts)
  let [opts.width, opts.height] = [a:place.width, a:place.height]
  call add(s:windows, nvim_open_win(buf, v:false, opts))
endfunction

" The rows the border (and in Vim the padding) of the options add above and
" below the lines.
function! s:extra_rows() abort
  if has('nvim')
    let border = get(s:opts, 'border', 'none')
    if type(border) == v:t_string
      return border ==# 'none' ? 0 : border ==# 'shadow' ? 1 : 2
    endif
    " Clockwise from the top left, repeated when fewer than eight; a
    " character may be [char, highlight].
    let chars = map(copy(border), {_, c -> type(c) == v:t_list ? c[0] : c})
    let n = len(chars)
    return n == 0 ? 0 : (chars[1 % n] !=# '') + (chars[5 % n] !=# '')
  endif
  let rows = 0
  if has_key(s:opts, 'border')
    " [top, right, bottom, left], zero for none; [] all around.
    let b = s:opts.border
    let rows += empty(b) ? 2 : (get(b, 0, 1) != 0) + (get(b, 2, get(b, 0, 1)) != 0)
  endif
  if has_key(s:opts, 'padding')
    let p = s:opts.padding
    let rows += empty(p) ? 2 : get(p, 0, 0) + get(p, 2, get(p, 0, 0))
  endif
  return rows
endfunction

function! s:reopen() abort
  if mode() =~# '^i' && !empty(s:windows)
    call goda_skk#popup#open(s:lines)
  endif
endfunction

function! goda_skk#popup#close() abort
  if empty(s:windows)
    return
  endif
  augroup goda_skk_popup
    autocmd!
  augroup END
  for w in s:windows
    if has('nvim')
      if nvim_win_is_valid(w)
        call nvim_win_close(w, v:true)
      endif
    else
      call popup_close(w)
    endif
  endfor
  let s:windows = []
  if !has('nvim')
    redraw
  endif
endfunction
