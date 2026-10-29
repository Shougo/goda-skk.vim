" The list of candidates: a popup under the cursor (or above it when there is
" no room), or over the command line.

let s:windows = []

function! goda_skk#popup#open(lines) abort
  call goda_skk#popup#close()
  if empty(a:lines)
    return
  endif
  let width = max(map(copy(a:lines), 'strwidth(v:val)'))
  if mode() ==# 'c'
    let row = &lines + 1 - max([1, &cmdheight]) - len(a:lines)
    let col = max([strwidth(getcmdprompt()), 1])
    if has('nvim')
      call s:nvim_open(a:lines, #{relative: 'editor', row: row - 1, col: col,
            \ width: width, height: len(a:lines)})
    else
      call add(s:windows, popup_create(a:lines, #{line: row, col: col + 1}))
    endif
    redraw
    return
  endif
  let pos = screenpos(0, line('.'), col('.'))
  " Neovim does not put a window over the command line.
  let above = &lines - pos.row - (has('nvim') ? &cmdheight : 0) < len(a:lines)
  if has('nvim')
    call s:nvim_open(a:lines, #{relative: 'cursor', row: above ? 0 : 1, col: 0,
          \ anchor: above ? 'SW' : 'NW', width: width, height: len(a:lines)})
  else
    call add(s:windows, popup_create(a:lines, #{
          \ pos: above ? 'botleft' : 'topleft',
          \ line: above ? 'cursor-1' : 'cursor+1',
          \ col: 'cursor',
          \ }))
  endif
endfunction

function! s:nvim_open(lines, opts) abort
  let buf = nvim_create_buf(v:false, v:true)
  call nvim_buf_set_lines(buf, 0, -1, v:true, a:lines)
  call add(s:windows, nvim_open_win(buf, v:false,
        \ extend(#{style: 'minimal', border: 'none', focusable: v:false}, a:opts)))
endfunction

function! goda_skk#popup#close() abort
  if empty(s:windows)
    return
  endif
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
