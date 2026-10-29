" The vimrc of the end-to-end test of the completion (cmp_test.go): Ctrl-]
" records the line and Vim's popup menu to $GODA_SKK_RESULT.
set nocompatible noswapfile encoding=utf-8 ttimeout ttimeoutlen=10
set completeopt=menuone,noinsert,noselect
execute 'set runtimepath^=' . fnameescape($GODA_VIM_DIR)
execute 'set runtimepath^=' . fnameescape($GODA_CMP_DIR)
execute 'set runtimepath^=' . fnameescape($GODA_SKK_DIR)
let g:goda_cmp#binary = $GODA_SKK_BINARY
let g:goda_skk#binary = $GODA_SKK_BINARY
map! <C-j> <Plug>(goda-skk-enable)

function! s:dump() abort
  let words = []
  if pumvisible()
    let words = map(copy(complete_info(['items']).items), 'v:val.word')
  endif
  call writefile([json_encode(#{line: getline('.'), words: words,
        \ phase: g:goda_skk#state.phase})], $GODA_SKK_RESULT, 'a')
endfunction
inoremap <C-]> <Cmd>call <SID>dump()<CR>

autocmd VimEnter * call goda_cmp#enable()
