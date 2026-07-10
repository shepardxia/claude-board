package main

// page is the single-page Brutalist master-detail UI, served as-is. It uses
// relative URLs (/events, /clear, /push) so it is port- and host-agnostic.
// Detail pane: input and output are two tappable regions — tap either to give
// it most of the height; the other collapses to a scrollable strip.
const page = `<!doctype html><html lang=en><head><meta charset=utf-8>
<meta name=viewport content="width=device-width, initial-scale=1, viewport-fit=cover">
<title>Claude Board</title>
<link rel=preconnect href="https://fonts.googleapis.com">
<link rel=preconnect href="https://fonts.gstatic.com" crossorigin>
<link href="https://fonts.googleapis.com/css2?family=Geist+Mono:wght@400;500;600;700&display=swap" rel="stylesheet">
<style>
:root{
  --bg:#eceae3; --panel:#f7f6f2; --ink:#0b0b0b; --line:#0b0b0b;
  --faint:#7b786f; --fail:#c8352a; --run:#0b0b0b; --ok:#7b786f;
  --mono:'Geist Mono',ui-monospace,SFMono-Regular,Menlo,monospace;
}
*{box-sizing:border-box}
html,body{margin:0;height:100%}
body{background:var(--bg);font-family:var(--mono);color:var(--ink);-webkit-font-smoothing:antialiased;
  overflow:hidden;overscroll-behavior:none}
.frame{height:100vh;height:100dvh;overflow:hidden}
.board{width:100%;height:100%;min-height:0;background:var(--bg);color:var(--ink);
  display:flex;flex-direction:column;overflow:hidden}

/* top bar */
.bar{display:flex;align-items:center;gap:16px;padding:13px 18px;border-bottom:1px solid var(--line);flex-wrap:wrap;flex:none}
.cg{display:flex;gap:15px;flex-wrap:wrap}
.chip{background:none;border:0;padding:0;cursor:pointer;font-family:var(--mono);font-size:15px;
  color:var(--faint);border-bottom:1px solid transparent}
.chip.on{color:var(--ink);border-bottom:1px solid var(--ink)}
.chip .cc{opacity:.5}
.vdiv{width:1px;height:16px;background:var(--line)}
.clear{margin-left:auto;border:1px solid var(--line);background:none;cursor:pointer;
  font-family:var(--mono);font-size:15px;color:var(--faint);padding:6px 16px;white-space:nowrap}

/* panes */
.panes{flex:1;min-height:0;display:flex;flex-direction:row}
.listpane{width:360px;flex:none;overflow-y:auto;border-right:1px solid var(--line);background:var(--panel);-webkit-overflow-scrolling:touch}
.detailpane{flex:1;min-width:0;min-height:0;display:flex;flex-direction:column;background:var(--bg)}

/* list rows */
.row{display:flex;flex-direction:column;align-items:flex-start;width:100%;text-align:left;cursor:pointer;
  font-family:var(--mono);padding:13px 16px;border:0;border-bottom:1px solid var(--line);background:var(--panel)}
.row.sel{background:var(--bg)}
.rtop{display:flex;align-items:baseline;gap:10px;width:100%}
.acc{width:2px;align-self:stretch;flex:none;margin:-13px 0 -13px -16px}
.abbr{font-weight:600;font-size:16px;flex:none}
.rtag{font-size:15px;color:var(--faint);flex:none}
.mr{margin-left:auto;font-size:13.5px;color:var(--faint);white-space:nowrap;flex:none}
.sw{font-size:13px;white-space:nowrap;flex:none}
.rsub{font-size:15.5px;color:var(--ink);opacity:.82;margin-top:7px;width:100%;text-align:left;
  white-space:nowrap;overflow:hidden;text-overflow:ellipsis}
.pfx{color:var(--faint)}
.empty{padding:40px 18px;text-align:center;font-size:15px;color:var(--faint)}

/* detail */
.dhead{padding:20px 24px;border-bottom:1px solid var(--line);flex:none}
.dtitle{display:flex;align-items:baseline;gap:12px;flex-wrap:wrap}
.dlabel{font-weight:600;font-size:19px}
.dtag{font-size:14.5px;color:var(--faint)}
.dstatus{margin-left:auto;font-size:14px;white-space:nowrap}
.dmeta{display:flex;gap:16px;margin-top:7px;font-size:14px;color:var(--faint)}

/* split: input (sub) over output (body); tap either to expand it */
.dsplit{flex:1;min-height:0;display:flex;flex-direction:column}
.dsubwrap{overflow:auto;-webkit-overflow-scrolling:touch;border-bottom:1px solid var(--line);cursor:pointer}
.dsub{padding:14px 24px;font-size:15.5px;line-height:1.55;color:var(--ink);white-space:pre-wrap;word-break:break-word}
.dbodywrap{overflow:auto;-webkit-overflow-scrolling:touch;cursor:pointer}
.zone-primary{flex:1 1 0;min-height:30%}   /* main pane: keeps >=30%, grows into slack, scrolls */
.zone-secondary{flex:0 1 auto;min-height:0} /* other: natural height; shrinks + scrolls only when tight */
.dbody{margin:0;padding:20px 24px;font-size:15px;line-height:1.6;color:var(--ink);white-space:pre-wrap;word-break:break-word}
.dbody .add{color:#2f6f3a}
.dbody .del{color:var(--fail)}
.dbody .hunk{color:var(--faint)}
.dfoot{flex:none;padding:9px 24px;border-top:1px solid var(--line);font-size:13px;color:var(--faint)}
.center{padding:40px 18px;text-align:center;font-size:15px;color:var(--faint)}

/* status colors */
.st-fail{color:var(--fail)}
.st-run{color:var(--ink)}
.st-ok{color:var(--faint)}

@media (max-width:720px){
  .panes{flex-direction:column}
  .listpane{width:auto;flex:none;height:250px;border-right:0;border-bottom:1px solid var(--line)}
}
::-webkit-scrollbar{width:8px;height:8px}
::-webkit-scrollbar-thumb{background:rgba(128,128,128,.4)}
::-webkit-scrollbar-track{background:transparent}
</style></head>
<body>
<div class=frame><div class=board data-skin=br>
  <div class=bar id=bar></div>
  <div class=panes>
    <div class=listpane id=list></div>
    <div class=detailpane id=detail></div>
  </div>
</div></div>
<script>
const state={data:[],activeTools:[],activeProjects:[],selectedId:null,idc:0,expand:'body'};
let renderedSelId=null, raf=0;

const META={Bash:{a:'BASH',p:'$ '},Task:{a:'TASK',p:'» '},Read:{a:'READ'},Grep:{a:'GREP'},
  Glob:{a:'GLOB'},Write:{a:'WRITE'},Edit:{a:'EDIT'},MultiEdit:{a:'EDIT'},NotebookEdit:{a:'EDIT'},
  WebFetch:{a:'FETCH'},WebSearch:{a:'SEARCH'},TodoWrite:{a:'TODO'}};
function meta(l){return META[l]||{a:(l||'?').toUpperCase().slice(0,6)};}
const FAILRE=/\b(error|failed|fatal|traceback|exception|no such|not found|command not found|assertionerror|exit code [1-9])\b/i;
function status(it){ if(it.status)return it.status; const b=(it.body||'')+' '+(it.sub||''); return FAILRE.test(b)?'fail':'ok'; }
function rel(ts){const d=Math.max(0,Date.now()/1000-ts);
  if(d<5)return'just now'; if(d<60)return Math.floor(d)+'s ago';
  if(d<3600)return Math.floor(d/60)+'m ago'; if(d<86400)return Math.floor(d/3600)+'h ago';
  return Math.floor(d/86400)+'d ago';}
function clock(ts){const d=new Date(ts*1000),p=n=>String(n).padStart(2,'0');
  return p(d.getHours())+':'+p(d.getMinutes())+':'+p(d.getSeconds());}
function dur(s){if(s==null)return''; if(s<1)return Math.round(s*1000)+'ms';
  if(s<60)return(Math.round(s*10)/10)+'s'; return Math.floor(s/60)+'m'+Math.round(s%60)+'s';}
function esc(s){return(s==null?'':(''+s)).replace(/[&<>"]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;'}[c]));}
function pj(v,ind){
  const pad='  '.repeat(ind), pad1='  '.repeat(ind+1);
  if(v===null) return 'null';
  if(typeof v==='number'||typeof v==='boolean') return String(v);
  if(typeof v==='string'){
    if(v.indexOf('\n')>=0) return '\n'+v.split('\n').map(l=>pad1+l).join('\n');
    return JSON.stringify(v);
  }
  if(Array.isArray(v)){
    if(v.length===0) return '[]';
    return '[\n'+v.map(x=>pad1+pj(x,ind+1)).join(',\n')+'\n'+pad+']';
  }
  const keys=Object.keys(v);
  if(keys.length===0) return '{}';
  return '{\n'+keys.map(k=>pad1+JSON.stringify(k)+': '+pj(v[k],ind+1)).join(',\n')+'\n'+pad+'}';
}
function bodyOf(it){let b=it.body||''; if(it.json){try{b=pj(JSON.parse(it.body),0);}catch(e){}} return b;}
function fmtBody(b){
  if(!/(^|\n)@@/.test(b)) return esc(b);
  return b.split('\n').map(l=>{const e=esc(l);
    if(l.startsWith('@@')) return '<span class=hunk>'+e+'</span>';
    if(l.startsWith('+')) return '<span class=add>'+e+'</span>';
    if(l.startsWith('-')) return '<span class=del>'+e+'</span>';
    return e;}).join('\n');
}

function chipHTML(c,kind){return '<button class="chip'+(c.active?' on':'')+'" data-'+kind+'="'+esc(c.label)+'">'
  +esc(c.label)+'<span class=cc> '+c.count+'</span></button>';}

function rowHTML(it,selected){
  const m=meta(it.label), st=status(it);
  const accent = st==='fail'?'var(--fail)': selected?'var(--ink)':'transparent';
  const durStr = st==='run'?'':dur(it.dur);
  const mr=(durStr?durStr+' · ':'')+rel(it.ts);
  const sw = st==='fail'?'failed': st==='run'?'running':'';
  const swHTML = st!=='ok' ? '<span class="sw st-'+st+'">'+sw+'</span>' : '';
  return '<button class="row'+(selected?' sel':'')+'" data-id="'+it.id+'">'
    +'<div class=rtop>'
      +'<span class=acc style="background:'+accent+'"></span>'
      +'<span class=abbr>'+esc(m.a)+'</span>'
      +'<span class=rtag>'+esc(it.tag)+'</span>'
      +'<span class=mr data-ts="'+it.ts+'" data-dur="'+esc(durStr)+'">'+esc(mr)+'</span>'
      +swHTML
    +'</div>'
    +'<div class=rsub><span class=pfx>'+esc(m.p||'')+'</span>'+esc(it.sub)+'</div>'
  +'</button>';
}

function detailHTML(it){
  if(!it) return '<div class="center">select an event</div>';
  const m=meta(it.label), st=status(it);
  const b=bodyOf(it);
  const durStr = st==='run'?'':dur(it.dur);
  const hasBody=b.trim().length>0;
  const lc=b.split('\n').length;
  const statusLabel={ok:'ok',run:'running',fail:'failed'}[st];
  const headPrimary = state.expand==='head';
  const subCls = headPrimary?'zone-primary':'zone-secondary';
  const bodyCls = headPrimary?'zone-secondary':'zone-primary';
  let h='<div class=dhead>'
    +'<div class=dtitle>'
      +'<span class=dlabel>'+esc(it.label)+'</span>'
      +'<span class=dtag>'+esc(it.tag)+'</span>'
      +'<span class="dstatus st-'+st+'">'+statusLabel+'</span>'
    +'</div>'
    +'<div class=dmeta><span>'+clock(it.ts)+'</span>'
      +(durStr?'<span>'+durStr+'</span>':'')
      +'<span class=ta data-ts="'+it.ts+'">'+rel(it.ts)+'</span></div>'
  +'</div>'
  +'<div class=dsplit>'
    +'<div class="dsubwrap '+subCls+'" data-zone=head>'
      +'<div class=dsub><span class=pfx>'+esc(m.p||'')+'</span>'+esc(it.sub)+'</div>'
    +'</div>';
  if(hasBody) h+='<div class="dbodywrap '+bodyCls+'" data-zone=body><pre class=dbody>'+fmtBody(b)+'</pre></div>';
  else h+='<div class="dbodywrap '+bodyCls+'" data-zone=body><div class="center">no output</div></div>';
  h+='</div>';
  if(hasBody) h+='<div class=dfoot>'+lc+(lc===1?' line':' lines')+'</div>';
  return h;
}

const barEl=document.getElementById('bar'),
      listEl=document.getElementById('list'),
      detailEl=document.getElementById('detail');

function render(){
  raf=0;
  const listScroll=listEl.scrollTop;
  const dwOld=detailEl.querySelector('.dbodywrap');
  const oldDetScroll=dwOld?dwOld.scrollTop:0;

  const seenT=[],seenP=[];
  state.data.forEach(it=>{ if(!seenT.includes(it.label))seenT.push(it.label);
                           if(!seenP.includes(it.tag))seenP.push(it.tag); });
  const toolChips=seenT.map(l=>({label:l,count:state.data.filter(x=>x.label===l).length,active:state.activeTools.includes(l)}));
  const projChips=seenP.map(t=>({label:t,count:state.data.filter(x=>x.tag===t).length,active:state.activeProjects.includes(t)}));

  const items=state.data.filter(it=>{
    if(state.activeTools.length&&!state.activeTools.includes(it.label))return false;
    if(state.activeProjects.length&&!state.activeProjects.includes(it.tag))return false;
    return true;
  });
  let sel=items.find(x=>x.id===state.selectedId)||items[0]||null;
  const selChanged=(sel?sel.id:null)!==renderedSelId;

  barEl.innerHTML =
    '<div class=cg>'+toolChips.map(c=>chipHTML(c,'tool')).join('')+'</div>'
    +(projChips.length?'<span class=vdiv></span>':'')
    +'<div class=cg>'+projChips.map(c=>chipHTML(c,'project')).join('')+'</div>'
    +'<button class=clear data-clear>clear</button>';

  listEl.innerHTML = items.length
    ? items.map(it=>rowHTML(it, sel&&it.id===sel.id)).join('')
    : '<div class=empty>no matching records</div>';

  detailEl.innerHTML = detailHTML(sel);

  renderedSelId = sel?sel.id:null;
  listEl.scrollTop=listScroll;
  const dwNew=detailEl.querySelector('.dbodywrap');
  if(dwNew && !selChanged) dwNew.scrollTop=oldDetScroll;
}
function schedule(){ if(!raf) raf=requestAnimationFrame(render); }

function applyExpand(){
  const sw=detailEl.querySelector('.dsubwrap'), bw=detailEl.querySelector('.dbodywrap');
  if(!sw||!bw)return;
  const hp=state.expand==='head';
  sw.classList.toggle('zone-primary',hp); sw.classList.toggle('zone-secondary',!hp);
  bw.classList.toggle('zone-primary',!hp); bw.classList.toggle('zone-secondary',hp);
}

function toggle(arr,v){const i=arr.indexOf(v); if(i>=0)arr.splice(i,1); else arr.push(v);}
barEl.addEventListener('click',e=>{const b=e.target.closest('button'); if(!b)return;
  if(b.hasAttribute('data-clear')){fetch('/clear'+location.search,{method:'POST'}).catch(()=>{});state.data=[];state.selectedId=null;render();return;}
  if(b.dataset.tool!=null){toggle(state.activeTools,b.dataset.tool);render();return;}
  if(b.dataset.project!=null){toggle(state.activeProjects,b.dataset.project);render();}
});
listEl.addEventListener('click',e=>{const r=e.target.closest('.row'); if(!r)return;
  state.selectedId=r.dataset.id; render();});
detailEl.addEventListener('click',e=>{const z=e.target.closest('[data-zone]'); if(!z)return;
  const zone=z.getAttribute('data-zone');
  if(state.expand!==zone){ state.expand=zone; applyExpand(); }});

function updateTimes(){
  document.querySelectorAll('.mr').forEach(el=>{const d=el.dataset.dur;
    el.textContent=(d?d+' · ':'')+rel(+el.dataset.ts);});
  document.querySelectorAll('.ta').forEach(el=>el.textContent=rel(+el.dataset.ts));
}
setInterval(updateTimes,4000);

function addCard(ev){
  if(ev.kind!=='card')return;
  state.data.unshift({id:'c'+(state.idc++),label:ev.label,tag:ev.tag||'?',sub:ev.sub||'',
    body:ev.body||'',sid:ev.sid||'',json:!!ev.json,ts:ev.ts||Date.now()/1000,dur:ev.dur});
  if(state.data.length>400) state.data.length=400;
  schedule();
}
const es=new EventSource('/events'+location.search);
es.onmessage=e=>{const ev=JSON.parse(e.data);
  if(ev.kind==='clear'){state.data=[];state.selectedId=null;render();return;}
  addCard(ev);};
render();
</script>
</body></html>`
