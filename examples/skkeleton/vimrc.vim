" The Vim script of a skkeleton user, translated (see main.go for
" skkeleton#config() and the completion). skkeleton-state-popup and
" skkeleton-henkan-highlight are built in; pum.vim is found by itself.

" The input method is in the binary of goda-cmp.vim (main.go is the
" goda-cmp.vim configuration, built with goda_cmp#build()).
let g:goda_skk#binary = has('nvim')
      \ ? stdpath('cache') . '/goda-cmp/goda-cmp'
      \ : expand('~/.cache/vim/goda-cmp/goda-cmp')

inoremap <C-j> <Plug>(goda-skk-toggle)
cnoremap <C-j> <Plug>(goda-skk-toggle)
tnoremap <C-j> <Plug>(goda-skk-toggle)
nnoremap <C-j> i<Plug>(goda-skk-enable)

augroup MyAutoCmd
  autocmd!
augroup END

" skkeleton-henkan-highlight
highlight GodaSkkHenkan gui=underline term=underline cterm=underline
highlight GodaSkkHenkanSelect
      \ gui=underline,reverse term=underline,reverse cterm=underline,reverse

autocmd MyAutoCmd User goda-skk-enable-pre call s:skk_pre()
function s:skk_pre() abort
  if (!has('nvim') || $DISPLAY !=# '') && has('clipboard')
    " Copy to clipboard to use Vim as IME
    autocmd ModeChanged *:n ++once
      \ let @* = '.'->getline() | let @+ = @*
  endif

  const hl_name = s:is_cmdline() ? 'CmdlineCursor' : 'Cursor'
  const hl = has('nvim')
        \ ? nvim_get_hl(0, #{ name: hl_name })
        \ : hl_name->hlget()
  if !hl->empty()
    let s:hl_cursor = hl
  endif
endfunction

function s:is_cmdline() abort
  return '*cmdline#_get'->exists() && !cmdline#_get().pos->empty()
endfunction

" Change the cursor color by the mode.
autocmd MyAutoCmd User goda-skk-mode-changed call s:skk_changed()
function s:skk_changed() abort
  if !'s:hl_cursor'->exists()
    return
  endif
  const color = get(#{
        \   hira: '#80403f',
        \   kata: '#f04060',
        \   hankata: '#60a060',
        \   zenkaku: '#60c060',
        \   abbrev: '#60f060',
        \ }, g:goda_skk#mode, '#606060')
  call s:highlight_cursor(color)
endfunction

" And while converting.
autocmd MyAutoCmd User goda-skk-handled call s:skk_handled()
function s:skk_handled() abort
  if g:goda_skk#mode ==# '' || !'s:hl_cursor'->exists()
    return
  endif
  if g:goda_skk#state.phase =~# '^henkan$\|^input:'
    call s:highlight_cursor('#a0f0a0')
  else
    call s:skk_changed()
  endif
endfunction

function s:highlight_cursor(color) abort
  let hl = s:hl_cursor->deepcopy()
  const name = s:is_cmdline() ? 'CmdlineCursor' : 'Cursor'
  if has('nvim')
    let hl.bg = a:color
    call nvim_set_hl(0, name, hl)
  else
    let hl[0].guibg = a:color
    let hl[0].name = name
    call hlset(hl)
  endif
  " NOTE: redraw is needed
  redraw
endfunction

" skkeleton-state-popup
call goda_skk#state_popup#config(#{
      \   labels: {
      \     'input': #{ hira: 'あ', kata: 'ア', hankata: 'ｶﾅ', zenkaku: 'Ａ' },
      \     'input:okurinasi': #{ hira: '▽', kata: '▽', hankata: '▽', abbrev: 'ab' },
      \     'input:okuriari': #{ hira: '▽', kata: '▽', hankata: '▽' },
      \     'henkan': #{ hira: '▼', kata: '▼', hankata: '▼', abbrev: 'ab' },
      \   },
      \   opts: has('nvim')
      \     ? #{ relative: 'cursor', col: 0, row: 1, anchor: 'NW', style: 'minimal' }
      \     : #{ pos: 'botleft', line: 'cursor+1', col: 'cursor', highlight: 'WildMenu' },
      \ })
call goda_skk#state_popup#enable()

" The sources of the input method while it is enabled.
autocmd MyAutoCmd User goda-skk-enable-pre call s:skk_pre_cmp()
function s:skk_pre_cmp() abort
  if 'b:prev_buffer_skk_config'->exists()
    return
  endif

  " Overwrite sources
  let b:prev_buffer_skk_config = goda_cmp#get_buffer()

  call goda_cmp#patch_buffer(#{
        \   cmdlineSources: {
        \     ':': ['skk', 'skk_okuri'],
        \     '@': ['skk', 'skk_okuri'],
        \   },
        \   sources: ['around', 'skk', 'skk_okuri', 'line'],
        \   sourceOptions: #{
        \     _: #{
        \       keywordPattern: '[ァ-ヮア-ンー]+',
        \     },
        \   },
        \ })
endfunction

autocmd MyAutoCmd User goda-skk-disable-post call s:skk_post_cmp()
function s:skk_post_cmp() abort
  if 'b:prev_buffer_skk_config'->exists()
    " Restore sources
    call goda_cmp#set_buffer(b:prev_buffer_skk_config)
    unlet b:prev_buffer_skk_config
  endif
endfunction
