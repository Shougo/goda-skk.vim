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

  " A key of no candidate on the page: the list stays.
  %delete _
  call feedkeys("i\<C-j>Kanji     d\<Cmd>let s:popup = s:popup_lines()\<CR>\<C-g>\<C-g>\<Esc>", 'xt')
  call s:check('popup stays', s:popup, ['a: 完治', 's: 関知'])
  " Scrolled: it is under the cursor still (WinScrolled comes when Vim
  " waits for a key).
  call setline(1, ['a', 'b', 'c'])
  call feedkeys("2GA\<C-j>Kanji     \<Cmd>let s:popup = [s:popup_row()]\<CR>"
        \ . "\<C-x>\<C-e>\<Cmd>doautocmd <nomodeline> WinScrolled\<CR>"
        \ . "\<Cmd>let s:popup += [s:popup_row(), s:popup_lines()]\<CR>\<C-g>\<C-g>\<Esc>", 'xt')
  call s:check('popup scrolled', s:popup, [3, 2, ['a: 完治', 's: 関知']])

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
  " A candidate of a completion inserted: it follows the cursor.
  call setline(1, ['ab cd', ''])
  call feedkeys("2Gi\<C-j>\<C-x>\<C-n>\<Cmd>sleep 20m\<CR>"
        \ . "\<Cmd>let s:sp = [s:popup_col(), screenpos(0, 2, col('.')).col]\<CR>\<C-e>\<Esc>", 'xt')
  call s:check('state popup completion', s:sp, [3, 3])
  " Scrolled (WinScrolled comes when Vim waits for a key).
  call setline(1, ['a', 'b', 'c'])
  call feedkeys("2GA\<C-j>\<C-x>\<C-e>\<Cmd>doautocmd <nomodeline> WinScrolled\<CR>"
        \ . "\<Cmd>let s:sp = [s:popup_row(), screenpos(0, 2, col('.')).row]\<CR>\<C-j>\<Esc>", 'xt')
  call s:check('state popup scrolled', s:sp, [2, 1])
  " Replace mode: neither the input method nor the popup.
  %delete _
  call feedkeys("i\<C-j>Kan\<Insert>a"
        \ . "\<Cmd>let s:sp = [mode(), g:goda_skk#enabled, s:popup_lines()]\<CR>\<Esc>", 'xt')
  call s:check('replace mode', s:sp, ['R', v:false, []])
  call s:check('replace mode line', getline(1), 'かna')
  call goda_skk#state_popup#disable()

  " The command line.
  let g:cmdline = ''
  call feedkeys(":let g:cmdline = '\<C-j>nihon\<C-j>'\<CR>", 'xt')
  call s:check('cmdline', g:cmdline, 'にほん')
  call s:check('cmdline enabled', g:goda_skk#enabled, v:false)
  " The cursor left the pre-edit: it is not left in the command line.
  let s:cl = ''
  call feedkeys(":\<C-j>A\<Left>i\<Cmd>let s:cl = getcmdline()\<CR>\<C-u>\<Esc>", 'xt')
  call s:check('cmdline stale', s:cl, 'い')
  " Another window: the input is reset, and the pre-edit goes.
  new
  call feedkeys(":\<C-j>K\<Cmd>wincmd w\<CR>\<Cmd>let s:cl = getcmdline()\<CR>\<C-u>\<Esc>", 'xt')
  call s:check('cmdline other window', [s:cl, g:goda_skk#enabled], ['', v:false])
  only!

  " The pre-edit left in insert mode: its markers go (and an expression
  " typed after it does not take it back).
  call s:check('insert stale', s:type("\<C-j>A\<Left>i"), ['いあ'])
  call s:check('insert stale inside', s:type("\<C-j>Kanj\<Left>i"), ['かんいj'])
  call setline(1, 'ab cd')
  call feedkeys("0i\<C-j>O\<C-r>=1\<CR>a\<Esc>", 'xt')
  call s:check('insert stale expression', getline(1), 'お1あab cd')
  " Insert mode left with <C-c>: the input is reset.
  call setline(1, 'ab')
  call feedkeys("A\<C-j>Kan\<C-c>", 'xt')
  call s:check('<C-c>', [getline(1), col('.'), g:goda_skk#enabled], ['abかn', 6, v:false])

  " Enabling and disabling close the popup menu of a completion.
  set completeopt=menuone,noinsert,noselect
  call setline(1, ['Kanji', ''])
  call feedkeys("2GiKan\<C-n>\<C-j>\<Cmd>let s:pm = [pumvisible(), g:goda_skk#enabled]\<CR>"
        \ . " \<C-n>\<Cmd>let s:pm += [pumvisible()]\<CR>"
        \ . "l\<Cmd>let s:pm += [pumvisible(), g:goda_skk#enabled]\<CR>\<Esc>", 'xt')
  call s:check('completion closed', [s:pm, getline(2)], [[0, v:true, 1, 0, v:false], 'Kan '])
  " Another completion.
  let g:closed = 0
  let g:goda_skk#completion_backend = #{visible: {-> v:true},
        \ close: {-> "\<Cmd>let g:closed += 1\<CR>"}}
  call s:type("\<C-j>a\<C-j>")
  " (<Esc> disables too.)
  call s:check('completion backend', g:closed, 2)
  call s:type("\<C-j>la")
  call s:check('completion backend disabled', g:closed, 4)
  unlet g:goda_skk#completion_backend
  set completeopt&

  " A macro: the keys typed, not what they typed.
  call setline(1, ['ab', 'cd'])
  call feedkeys("ggqaA\<C-j>Kanji \<C-j>\<Esc>qj@a", 'xt')
  call s:check('macro', getline(1, '$'), ['ab関知', 'cd関知'])
  call s:check('macro register', getreg('a'), "A\<C-j>Kanji \<C-j>\<Esc>")
  %delete _

  " Another window: the input is reset, and so is the state.
  %delete _
  new
  wincmd p
  call feedkeys("i\<C-j>Kanji\<Cmd>wincmd w\<CR>"
        \ . "\<Cmd>let s:st = [g:goda_skk#enabled, g:goda_skk#state]\<CR>\<Esc>", 'xt')
  call s:check('other window', s:st, [v:false, #{phase: '', henkanFeed: ''}])
  only!
  " Not while registering: the prompt belongs to the window.
  call setline(1, 'ab cd')
  new
  call setline(1, 'xy')
  wincmd p
  let win = win_getid()
  call feedkeys("A\<C-j>Ringo \<Cmd>wincmd w\<CR>\<Esc>"
        \ . "\<Cmd>let s:st = [win_getid() == win ? 'same' : 'other', g:goda_skk#enabled, g:goda_skk#mode]\<CR>a\<Esc>", 'xt')
  call s:check('register window', s:st, ['same', v:true, 'hira'])
  call setline(1, 'ab cd')
  call feedkeys("A\<C-j>Ringo Kanji \<C-j>\<Cmd>wincmd w\<CR>\<CR>"
        \ . "\<Cmd>let s:st = win_getid() == win ? 'same' : 'other'\<CR>\<Esc>", 'xt')
  call s:check('register window confirmed', s:st, 'same')
  call s:check('register window lines', [getline(1), getbufline(winbufnr(winnr('#')), 1)],
        \ ['ab cd関知', ['xy']])
  only!
  enew!

  " A terminal job (no cat on MS-Windows; Neovim does not enter Terminal
  " mode within feedkeys() with "x").
  if !has('win32') && !has('nvim')
    call s:terminal()
  endif

  " The binary stopped (or exited): the next key starts it again.
  call goda#stop('skk')
  call s:check('restarted', s:type("\<C-j>aiu\<C-j>"), ['あいう'])

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
  " The job echoes it when it gets to it: wait for that (5 seconds at
  " most, for a loaded machine).
  for _ in range(250)
    if match(getbufline(buf, 1, '$'), 'あいう') >= 0
      break
    endif
    call term_wait(buf, 20)
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

" The screen row of the popup.
function! s:popup_row() abort
  redraw
  if has('nvim')
    let wins = filter(nvim_list_wins(), 'nvim_win_get_config(v:val).relative !=# ""')
    return empty(wins) ? 0 : nvim_win_get_position(wins[0])[0] + 1
  endif
  let ids = popup_list()
  return empty(ids) ? 0 : popup_getpos(ids[0]).line
endfunction

" The screen column of the popup.
function! s:popup_col() abort
  redraw
  if has('nvim')
    let wins = filter(nvim_list_wins(), 'nvim_win_get_config(v:val).relative !=# ""')
    return empty(wins) ? 0 : nvim_win_get_position(wins[0])[1] + 1
  endif
  let ids = popup_list()
  return empty(ids) ? 0 : popup_getpos(ids[0]).col
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
