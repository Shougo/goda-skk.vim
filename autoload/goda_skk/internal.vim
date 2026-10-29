" The buffer mappings and options goda-skk changes while it is enabled, and
" puts back.

let s:maps = {}     " bufnr -> {mode: [mappings]}
let s:options = {}  " bufnr -> {textwidth, virtualedit}

augroup goda_skk_internal
  autocmd!
  autocmd BufWipeout * call s:forget(str2nr(expand('<abuf>')))
augroup END

function! s:forget(bufnr) abort
  silent! unlet s:maps[a:bufnr]
  silent! unlet s:options[a:bufnr]
endfunction

" Save the buffer mappings of {mode} and the options, and set the options
" for the input.
function! goda_skk#internal#save(mode) abort
  let bufnr = bufnr('%')
  let saved = get(s:maps, bufnr, {})
  if !has_key(saved, a:mode)
    " maplist() gives what mapset() takes, in Vim and Neovim.
    let saved[a:mode] = filter(maplist(), {_, m -> m.buffer && m.mode ==# a:mode})
  endif
  let s:maps[bufnr] = saved
  if a:mode !=# 'i' || has_key(s:options, bufnr)
    return
  endif
  let s:options[bufnr] = #{textwidth: &l:textwidth, virtualedit: &l:virtualedit}
  " A line break from 'textwidth' would break the pre-edit, and the cursor
  " must be able to stand after the end of the line (okurigana there).
  setlocal textwidth=0
  setlocal virtualedit=onemore
endfunction

" Put back what goda_skk#internal#save({mode}) changed.
function! goda_skk#internal#restore(mode) abort
  let bufnr = bufnr('%')
  let saved = get(s:maps, bufnr, {})
  if has_key(saved, a:mode)
    execute a:mode . 'mapclear <buffer>'
    for m in remove(saved, a:mode)
      call mapset(a:mode, v:false, m)
    endfor
  endif
  if a:mode ==# 'i' && has_key(s:options, bufnr)
    let o = remove(s:options, bufnr)
    let &l:textwidth = o.textwidth
    let &l:virtualedit = o.virtualedit
  endif
endfunction
