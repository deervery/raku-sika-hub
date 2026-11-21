'use strict';
const $ = id => document.getElementById(id);
const names = {product:'商品',cut:'部位',lot:'ロット',weight:'重量',unit:'単位',copies:'枚数',amount:'金額'};
const kinds = {print:'ラベル印刷',inventory:'計量・在庫登録',sale:'販売記録',stock:'在庫照会'};
let current, busy = false, mode = 'filled', renderKey = '';
async function api(path, body) {
  const r = await fetch(path, body === undefined ? {} : {method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  const data = await r.json(); if (!r.ok) throw Error(data.error || '接続できません'); return data;
}
function button(text, action, parent) { const b=document.createElement('button');b.type='button';b.textContent=text;b.onclick=action;parent.append(b);return b; }
function commandBase(state=current.state){return {session_id:state.session_id,revision:state.revision,media_epoch:state.media_epoch};}
async function send(action, extra={}, base=commandBase()) {
  if(busy)return;busy=true;
  try{current=await api('/api/commands',{...base,action,...extra});renderKey='';render();}
  catch(e){$('bubble').textContent=e.message;try{current=await api('/api/sessions/'+current.state.session_id);}catch{}renderKey='';}
  finally{busy=false;}
}
function render(){
  $('analysis-notice').hidden=!(current?.jobs||[]).some(j=>j.status==='done');
  if(!current||$('editor').open)return;
  const s=current.state,key=`${s.revision}:${s.media_epoch}:${mode}`;if(renderKey===key)return;renderKey=key;
  $('bubble').textContent=s.reply;
  // Reuse keyed cards so a changing value doesn't move focus to another field.
  let keys=s.view.fields.filter(k=>mode==='all'||(mode==='missing'?s.missing.includes(k):s.fields[k]));
  if(!keys.length)keys=s.missing.slice(0,1);
  const cards=$('cards');cards.dataset.count=String(keys.length);
  for(const b of [...cards.children])if(!keys.includes(b.dataset.key))b.remove();
  keys.forEach((k,index)=>{
    let b=[...cards.children].find(el=>el.dataset.key===k);
    if(!b){b=button('',()=>edit(k),cards);b.dataset.key=k;b.append(document.createElement('label'),document.createElement('strong'));}
    b.firstChild.textContent=names[k];b.lastChild.textContent=s.fields[k]||'未入力';b.classList.toggle('changed',(s.view.changed||[]).includes(k));
    b.disabled=['executing','unknown','done','cancelled','failed'].includes(s.status);
    if(cards.children[index]!==b)cards.insertBefore(b,cards.children[index]||null);
  });
  const actions=$('actions');actions.replaceChildren();
  if(s.confirmation)button('この内容で確定',()=>send('confirm',{confirmation_id:s.confirmation.id}),actions);
  if(s.status==='unknown')button('結果を照会',()=>send('reconcile'),actions);
  button(mode==='filled'?'未入力を見る':mode==='missing'?'すべて見る':'入力済みを見る',()=>{mode=mode==='filled'?'missing':mode==='missing'?'all':'filled';render();},actions);
  if(!['executing','unknown','done','cancelled','failed'].includes(s.status))button('取り消す',()=>send('cancel'),actions);
  if(['done','cancelled','failed'].includes(s.status))button('次の作業',menu,actions);
}
function openEditor(title){$('editor-title').textContent=title;$('entry').textContent='';$('choices').replaceChildren();$('editor').showModal();}
function closeEditor(){$('editor').close();renderKey='';render();}
function edit(key){
  const base=commandBase(), old=current.state.fields[key]||'';
  openEditor(names[key]);
  const options={product:['鹿肉','猪肉'],cut:['ロース','肩ロース','ヒレ','モモ','バラ','ミンチ'],unit:['kg','g']};
  const save=value=>{closeEditor();send('patch',{fields:{[key]:value}},base);};
  if(options[key]){for(const v of options[key])button(v,()=>save(v),$('choices'));return;}
  let value=old;$('entry').textContent=value||'—';
  for(const digit of ['1','2','3','4','5','6','7','8','9','0',...(key==='weight'?['.']:[]),'⌫'])button(digit,()=>{value=digit==='⌫'?value.slice(0,-1):value+digit;$('entry').textContent=value||'—';},$('choices'));
  button('クリア',()=>{value='';$('entry').textContent='—';},$('choices'));
  button('反映',()=>save(value),$('choices'));
}
function menu(){
  if(busy||!current||['executing','unknown'].includes(current.state.status))return;
  openEditor('作業を選ぶ');
  for(const [kind,label]of Object.entries(kinds))button(label,()=>{closeEditor();mode='filled';send('begin',{kind});},$('choices'));
}
$('face').onclick=menu;$('close-editor').onclick=closeEditor;
$('analysis-notice').onclick=()=>{
  const jobs=(current?.jobs||[]).filter(j=>j.status==='done').sort((a,b)=>b.created_at.localeCompare(a.created_at));
  if(!jobs.length)return;const j=jobs[0];openEditor('分析結果（検証用データ）');
  $('entry').textContent=`対象 ${j.current} ${j.unit} ／ 比較 ${j.comparison} ${j.unit} ／ 差 ${j.difference} ${j.unit}`;
};
$('editor').addEventListener('cancel',()=>setTimeout(()=>{renderKey='';render();},0));
(async()=>{
  try{
    const id=localStorage.getItem('station-lab-session');
    if(id){try{current=await api('/api/sessions/'+id);}catch{}}
    if(!current)current=await api('/api/sessions',{});
    localStorage.setItem('station-lab-session',current.state.session_id);render();
    setInterval(async()=>{if(busy)return;try{current=await api('/api/sessions/'+current.state.session_id);render();}catch{renderKey='';$('bubble').textContent='接続を確認しています。操作は再送しません。';}},1000);
  }catch(e){$('bubble').textContent=e.message;}
})();
