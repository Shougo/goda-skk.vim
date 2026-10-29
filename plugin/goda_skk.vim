if exists('g:loaded_goda_skk')
  finish
endif
let g:loaded_goda_skk = 1

" Insert mode, the command line and terminal jobs.
for s:mode in ['noremap!', 'tnoremap']
  for s:name in ['enable', 'disable', 'toggle']
    execute printf('%s <Plug>(goda-skk-%s) <Cmd>call goda_skk#%s()<CR>',
          \ s:mode, s:name, s:name)
  endfor
endfor
unlet s:mode s:name
