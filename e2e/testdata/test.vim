" The end-to-end scenario: typing in insert mode and on the command line of
" a real Vim or Neovim. Failures go to $GODA_SKK_RESULT; it says "ok" at
" the end.

set nocompatible
set encoding=utf-8
let s:errors = []
let s:result = $GODA_SKK_RESULT

execute 'set runtimepath^=' . fnameescape($GODA_VIM_DIR)
execute 'set runtimepath^=' . fnameescape($GODA_SKK_DIR)
runtime plugin/goda_skk.vim
let g:goda_skk#binary = $GODA_SKK_BINARY

map! <C-j> <Plug>(goda-skk-enable)

let s:events = []
augroup test
  autocmd!
  autocmd User goda-skk-enable-post call add(s:events, 'enable')
  autocmd User goda-skk-disable-post call add(s:events, 'disable')
augroup END

function! s:check(name, got, want) abort
  if a:got !=# a:want
    call add(s:errors, printf('%s: got %s, want %s', a:name, string(a:got), string(a:want)))
  endif
endfunction

" Type {keys} in insert mode in an empty buffer, and leave it.
function! s:type(keys) abort
  %delete _
  call feedkeys('i' . a:keys . "\<Esc>", 'xt')
  return getline(1, '$')
endfunction

function! s:run() abort
  call s:check('kana', s:type("\<C-j>aiueo"), ['あいうえお'])
  call s:check('mode after', g:goda_skk#mode, '')
  call s:check('enabled after', g:goda_skk#enabled, v:false)
  call s:check('events', s:events, ['enable', 'disable'])

  call s:check('pre-edit', s:type("\<C-j>Kanji"), ['かんじ'])
  call s:check('henkan', s:type("\<C-j>Kanji \<C-j>"), ['漢字'])
  call s:check('next candidate', s:type("\<C-j>Kanji  \<C-j>"), ['感じ'])
  call s:check('okuri', s:type("\<C-j>OkuRi\<C-j>"), ['送り'])
  call s:check('newline', s:type("\<C-j>Nihon \<CR>x"), ['日本', 'x'])
  call s:check('backspace', s:type("\<C-j>kak\<BS>i"), ['かい'])
  call s:check('katakana', s:type("\<C-j>qaiu"), ['アイウ'])
  call s:check('disable', s:type("\<C-j>alabc"), ['あabc'])
  call s:check('enable again', s:type("\<C-j>a\<C-j>i"), ['あい'])
  call s:check('text before', s:type("xyz\<C-j>aiu"), ['xyzあいう'])

  " The popup lists the candidates after the fourth.
  %delete _
  let s:popup = []
  call feedkeys("i\<C-j>Kanji     \<Cmd>let s:popup = s:popup_lines()\<CR>s\<Esc>", 'xt')
  call s:check('popup', s:popup, ['a: 完治', 's: 関知'])
  call s:check('popup choice', getline(1), '関知')

  " The user dictionary learned: 関知 comes first now.
  call s:check('learned', s:type("\<C-j>Kanji \<C-j>"), ['関知'])
  call s:check('user dictionary', readfile($GODA_SKK_USER_JISYO)->filter('v:val =~# "^かんじ"'),
        \ ['かんじ /関知/感じ/漢字/'])

  " Registering a word, converting in the prompt; then it is learned.
  call s:check('register', s:type("\<C-j>Mikan Kanji \<C-j>\<CR>"), ['関知'])
  call s:check('registered', s:type("\<C-j>Mikan \<C-j>"), ['関知'])
  call s:check('register cancelled', s:type("\<C-j>Ringo \<Esc>a"), ['りんごあ'])
  call s:check('register <C-g>', s:type("\<C-j>Ringo \<C-g>a"), ['りんごあ'])
  call s:check('register maps', maparg('a', 'c'), '')
  let s:events = []
  call s:type("\<C-j>Suika \<CR>")
  call s:check('register events', s:events, ['enable', 'enable', 'disable', 'enable', 'disable'])

  " kakuteiUndo after typing on, and the cursor back at the end.
  call s:check('kakuteiUndo', s:type("\<C-j>Nihon \<C-j>aiu\<C-z> \<C-j>lx"), ['二本あいうx'])
  " Rules of the kana table.
  call s:check('rule', s:type("\<C-j>~"), ['〜'])
  call s:check('jj', s:type("\<C-j>ajj"), ['あ'])
  call s:check('jj disabled', [mode(), g:goda_skk#enabled], ['n', v:false])

  " The highlight of the pre-edit.
  highlight GodaSkkHenkan cterm=underline
  highlight GodaSkkHenkanSelect cterm=reverse
  %delete _
  call feedkeys("i\<C-j>Kanji\<Cmd>let s:hl = s:highlights()\<CR>"
        \ . " \<Cmd>let s:hl2 = s:highlights()\<CR>\<C-j>\<Cmd>let s:hl3 = s:highlights()\<CR>\<Esc>", 'xt')
  call s:check('highlight', s:hl, [[1, 12, 'GodaSkkHenkan']])
  call s:check('highlight select', s:hl2, [[1, 9, 'GodaSkkHenkanSelect']])
  call s:check('highlight cleared', s:hl3, [])

  " The state popup.
  call goda_skk#state_popup#config(#{labels: #{input: #{hira: 'あ'}, henkan: #{hira: '▼'}}})
  call goda_skk#state_popup#enable()
  %delete _
  call feedkeys("i\<C-j>\<Cmd>let s:sp = s:popup_lines()\<CR>Kanji \<Cmd>let s:sp2 = s:popup_lines()\<CR>\<Esc>", 'xt')
  call s:check('state popup', s:sp, ['あ'])
  call s:check('state popup henkan', s:sp2, ['▼'])
  call s:check('state popup closed', s:popup_lines(), [])
  call goda_skk#state_popup#disable()

  " The command line.
  let g:cmdline = ''
  call feedkeys(":let g:cmdline = '\<C-j>nihon\<C-j>'\<CR>", 'xt')
  call s:check('cmdline', g:cmdline, 'にほん')
  call s:check('cmdline enabled', g:goda_skk#enabled, v:false)

  " A terminal job (no cat on MS-Windows; Neovim does not enter Terminal
  " mode within feedkeys() with "x").
  if !has('win32') && !has('nvim')
    call s:terminal()
  endif

  " The buffer mappings come back.
  inoremap <buffer> a A
  call s:check('buffer map', s:type("\<C-j>ala"), ['あA'])
  iunmap <buffer> a
endfunction

function! s:terminal() abort
  tmap <C-j> <Plug>(goda-skk-enable)
  new
  call term_start(['cat'], #{curwin: v:true})
  let buf = bufnr('%')
  call feedkeys("\<C-j>aiu\<CR>"
        \ . "\<Cmd>let s:tmode = [mode(), g:goda_skk#mode]\<CR>"
        \ . "\<C-\>\<C-n>", 'xt')
  call s:check('terminal mode', s:tmode, ['t', 'hira'])
  call s:check('terminal disabled', g:goda_skk#enabled, v:false)
  for _ in range(100)
    if match(getbufline(buf, 1, '$'), 'あいう') >= 0
      break
    endif
    sleep 20m
  endfor
  call s:check('terminal', match(getbufline(buf, 1, '$'), 'あいう') >= 0, v:true)
  bwipeout!
  tunmap <C-j>
endfunction

" The highlights of line 1: [[col, length, group]].
function! s:highlights() abort
  if has('nvim')
    return map(nvim_buf_get_extmarks(0, -1, 0, -1, #{details: v:true}),
          \ {_, m -> [m[2] + 1, m[3].end_col - m[2], m[3].hl_group]})
  endif
  return map(prop_list(1), {_, p -> [p.col, p.length, p.type]})
endfunction

function! s:popup_lines() abort
  if has('nvim')
    let wins = filter(nvim_list_wins(), 'nvim_win_get_config(v:val).relative !=# ""')
    return empty(wins) ? [] : nvim_buf_get_lines(winbufnr(wins[0]), 0, -1, v:true)
  endif
  let ids = popup_list()
  return empty(ids) ? [] : getbufline(winbufnr(ids[0]), 1, '$')
endfunction

try
  call s:run()
catch
  call add(s:errors, v:throwpoint . ': ' . v:exception)
endtry
call writefile(s:errors + (empty(s:errors) ? ['ok'] : []) + goda#log(), s:result)
call goda#stop()
qall!
