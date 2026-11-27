" The vimrc of the end-to-end test of the completion (cmp_test.go): Ctrl-]
" records the line and Vim's popup menu to $GODA_SKK_RESULT.
set nocompatible noswapfile encoding=utf-8 ttimeout ttimeoutlen=10
set completeopt=menuone,noinsert,noselect
execute 'set runtimepath^=' . fnameescape($GODA_VIM_DIR)
execute 'set runtimepath^=' . fnameescape($GODA_CMP_DIR)
execute 'set runtimepath^=' . fnameescape($GODA_CMP_UI_DIR)
execute 'set runtimepath^=' . fnameescape($GODA_SKK_DIR)
let g:goda_cmp#binary = $GODA_SKK_BINARY
let g:goda_skk#binary = $GODA_SKK_BINARY
map! <C-j> <Plug>(goda-skk-enable)
" <F2> opens a file as vim-guise does: in a callback, without leaving Insert
" mode.
inoremap <F2> <Cmd>call timer_start(50, {-> execute('silent noswapfile tab drop ' . tempname())})<CR>

function! s:dump() abort
  " The rows of the popup, as the user sees them: complete_info() gives the
  " items in reverse order in some versions of Vim (9.1).
  let words = []
  if pumvisible()
    " Neovim answers with Floats.
    let pos = map(pum_getpos(), {_, v -> type(v) == v:t_float ? float2nr(v) : v})
    for i in range(pos.height)
      let row = join(map(range(pos.col + 1, pos.col + pos.width),
            \ {_, c -> screenstring(pos.row + 1 + i, c)}), '')
      call add(words, split(trim(row))[0])
    endfor
  endif
  call writefile([json_encode(#{line: getline('.'), words: words,
        \ phase: g:goda_skk#state.phase,
        \ enabled: g:goda_skk#enabled, mapped: maparg('a', 'i', 0, 1)->get('buffer', 0)})], $GODA_SKK_RESULT, 'a')
endfunction
inoremap <C-]> <Cmd>call <SID>dump()<CR>

autocmd VimEnter * call goda_cmp#enable()
