" The highlight of the pre-edit in insert mode: GodaSkkHenkan for a reading
" (▽), GodaSkkHenkanSelect for a candidate (▼). Nothing for a group that is
" not defined.

let s:groups = #{henkan: 'GodaSkkHenkan', select: 'GodaSkkHenkanSelect'}
let s:namespace = has('nvim') ? nvim_create_namespace('goda-skk-henkan') : 0
" Where the highlight is: [bufnr, lnum].
let s:at = []

" {pre_edit} ends at the cursor.
function! goda_skk#highlight#update(phase, pre_edit) abort
  if mode() !=# 'i' || a:pre_edit ==# '' || a:phase !~# '^input:\|^henkan$'
    return
  endif
  let group = a:phase ==# 'henkan' ? s:groups.select : s:groups.henkan
  if !hlexists(group)
    return
  endif
  let end = col('.')
  let start = end - strlen(a:pre_edit)
  if start < 1 || getline('.')[start - 1 : end - 2] !=# a:pre_edit
    return
  endif
  let s:at = [bufnr('%'), line('.')]
  if has('nvim')
    call nvim_buf_set_extmark(0, s:namespace, line('.') - 1, start - 1,
          \ #{end_col: end - 1, hl_group: group})
  else
    if empty(prop_type_get(group))
      call prop_type_add(group, #{highlight: group})
    endif
    call prop_add(line('.'), start, #{end_col: end, type: group, bufnr: bufnr('%')})
  endif
endfunction

function! goda_skk#highlight#clear() abort
  if empty(s:at)
    return
  endif
  let [bufnr, lnum] = s:at
  let s:at = []
  if !bufexists(bufnr)
    return
  endif
  if has('nvim')
    call nvim_buf_clear_namespace(bufnr, s:namespace, 0, -1)
    return
  endif
  for group in values(s:groups)
    if !empty(prop_type_get(group))
      call prop_remove(#{type: group, bufnr: bufnr, all: v:true})
    endif
  endfor
endfunction
