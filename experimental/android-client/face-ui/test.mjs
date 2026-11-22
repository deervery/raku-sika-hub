import assert from 'node:assert/strict';
import {readFile} from 'node:fs/promises';
import {transform,build} from 'esbuild';
const {code}=await transform(await readFile('src/localIntent.ts','utf8'),{loader:'ts',format:'esm'});
const {localIntent}=await import('data:text/javascript;base64,'+Buffer.from(code).toString('base64'));
assert.deepEqual(localIntent('シカのオスで、個体番号001、重さは0.07キロです').patch,{species:'シカ',sex:'オス',individualNumber:'001',weight:'0.07'});
assert.equal(localIntent('70グラム').patch.weight,'0.07');
assert.equal(localIntent('個体0001').patch.individualNumber,'0001');
for(const text of ['0.07','0.07kgかもしれない','シカじゃない','0.07kgと0.08kg','メスかオス','個体12と個体13'])assert.equal(localIntent(text),null,text);
for(const text of ['はい','いいね','もういい','確定'])assert.ok(localIntent(text).question);
assert.equal(localIntent('未入力').review,'missing');
console.log('14 intent assertions passed');
const bundled=async(entry)=>{
  const result=await build({entryPoints:[entry],bundle:true,write:false,format:'esm',platform:'node'});
  return import('data:text/javascript;base64,'+Buffer.from(result.outputFiles[0].text).toString('base64'));
};
const {draftValues,defaultFieldKeys}=await bundled('src/draftDefaults.ts');
const form={fields:{species:{type:'select',defaultValue:'シカ'},health:{type:'select',defaultValue:'異常なし'},weight:{type:'number'},empty:{type:'text',defaultValue:'x'}}};
assert.equal(draftValues(form,{}).species,'シカ');
assert.equal(draftValues(form,{species:'クマ',empty:''}).species,'クマ');
assert.equal(draftValues(form,{empty:''}).empty,'');
assert.ok(defaultFieldKeys(form,{}).includes('health'));
assert.ok(!defaultFieldKeys(form,{health:'異常なし'}).includes('health'));
assert.ok(!defaultFieldKeys(form,{}).includes('weight'));
const {createConversation}=await bundled('src/conversation.ts');
const tick=()=>new Promise(r=>setTimeout(r,0));
for(const phase of ['interpret','speak']){
  const pending=[],calls=[],applied=[],states=[],messages=[];let session='',receive=()=>{},unsubscribed=false;
  const native=async(op,args)=>{calls.push(op);if(op==='voiceStart'){session=args.session;return {continuous:true};}if(op===phase)return new Promise(resolve=>pending.push(resolve));if(op==='interpret')return {question:'こんにちは'};return {};};
  const connect=createConversation(native,()=>true,fn=>{receive=fn;return ()=>{unsubscribed=true;};});
  const callbacks={message:(...args)=>messages.push(args),propose:p=>applied.push(p),review:()=>{},state:s=>states.push(s),error:e=>{throw new Error(e);}};
  const voice=connect({}, {},callbacks);
  const send=(type,turn,text='')=>receive({session,type,turn,text});
  send('ready',0);send('speech',1);send('text',1,'こんにちは');await tick();
  assert.equal(pending.length,1,phase);const stale=pending.shift();
  voice.inform({});voice.inform({}); // Repeated manual edits must not suppress the next speech turn.
  send('speech',2);send('text',2,'0.08kg');await tick();
  assert.deepEqual(applied,[{weight:'0.08'}]);
  const before=states.length;stale(phase==='interpret'?{patch:{weight:'0.07'}}:{});await tick();
  assert.equal(states.length,before,'late response must not change the new turn');
  send('speech',3);send('text',3,'個体番号0001');await tick();
  assert.equal(applied[1].individualNumber,'0001');
  assert.equal(calls.filter(op=>op==='voiceStart').length,1,'one microphone session across turns');
  assert.ok(messages.every((msg,i)=>!i||msg[3]>messages[i-1][3]),'bubbles keep chronological order');
  voice.stop();assert.ok(unsubscribed);const count=applied.length;
  send('speech',4);send('text',4,'1kg');await tick();assert.equal(applied.length,count);
}
const {labelGroups,fieldForLabel,validDate,validQuantity,labelIntent}=await bundled('src/lib/voice-guidance/labelEditor.ts');
const {labelFields,emptyLabelDraft,mergeLabelDraft,labelIssues,LABEL_TEMPLATES,LABEL_INPUT_KEYS}=await bundled('src/lib/voice-guidance/stationOperations.ts');
for(const template of Object.keys(LABEL_TEMPLATES)){
  const draft={...emptyLabelDraft(),template};
  assert.deepEqual(labelGroups(draft).flatMap(g=>g.keys).sort(),labelFields(draft).sort(),template+' must expose every operator field exactly once');
}
assert.equal(validDate('2028-02-29'),true);assert.equal(validDate('2026-02-29'),false);assert.equal(validDate('2026-04-31'),false);assert.equal(validDate('2026-11'),false);
for(const value of ['0.07kg','70g','2個','3袋','2包'])assert.equal(validQuantity(value),true,value);
for(const value of ['0kg','-1kg','1.5袋','1','NaNkg'])assert.equal(validQuantity(value),false,value);
assert.ok(labelFields({...emptyLabelDraft(),template:'traceable'}).includes('receivingDate'));
assert.ok(labelFields({...emptyLabelDraft(),template:'processed'}).includes('isHeatedMeatProducts'));
assert.equal(fieldForLabel('attentionText',emptyLabelDraft()).required,false);
assert.equal(mergeLabelDraft(emptyLabelDraft(),{data:{individualNumber:'0001'}}).data.individualNumber,'0001');
assert.deepEqual(labelIntent('内容量は70グラムです'),{label:{data:{productQuantity:'70g'}}});
assert.deepEqual(labelIntent('消費期限は2026年12月3日'),{label:{data:{deadlineDate:'2026-12-03',deadlineLabel:'消費期限'}}});
assert.ok(labelIntent('印刷して').question,'speech alone must not execute printing');
// The palette is centralized, including the formerly green review/operations styles.
for(const file of ['src/components/voice/Station.module.css','src/components/voice/VoiceWorkspace.module.css'])assert.equal(/#[0-9a-f]{3,8}\b|rgba?\(/i.test(await readFile(file,'utf8')),false,file);
console.log('Continuous turns, interruption, label field coverage, units/dates and theme checks passed');

const legacyStates=[],legacyPatches=[];
const legacy=createConversation(async op=>op==='voiceStart'?{continuous:false}:op==='listen'?{text:'0.07kg'}:{},()=>false,()=>()=>{});
legacy({}, {},{message:()=>{},propose:patch=>legacyPatches.push(patch),review:()=>{},state:s=>legacyStates.push(s),error:e=>{throw new Error(e);}});
await tick();await tick();assert.deepEqual(legacyPatches,[{weight:'0.07'}]);assert.equal(legacyStates.at(-1),'off');
for(const data of [{productQuantity:'-1kg'},{productQuantity:'1.2個'},{deadlineDate:'2026-02-29'},{individualNumber:'1'.repeat(16)}])assert.throws(()=>mergeLabelDraft(emptyLabelDraft(),{data}));
console.log('Manual-edit cancellation, Android 12 fallback and invalid label rejection passed');

const {individualData,productData,scanQuery,swipeDirection}=await bundled('src/catalog.ts');
assert.equal(individualData({id:'i',individualNumber:'0001',species:'シカ'}).individualNumber,'0001');
assert.equal(individualData({id:'i',individualNumber:'0002'}).species,'','old individual values are cleared');
assert.equal(productData({id:'p',name:'ロース',defaultContentQuantity:'70',unit:'g'}).productQuantity,'70g');
assert.equal(productData({id:'p',name:'ロース',defaultContentQuantity:70}).productQuantity,'','never invent units');
assert.deepEqual(scanQuery('0001'),{query:'0001'});
assert.deepEqual(scanQuery('https://rakusika.com/t/abc'),{id:'abc',trace:true});
for(const url of ['https://evil.example/t/abc','https://rakusika.com.evil.test/t/abc','javascript:alert(1)','https://x@rakusika.com/t/abc'])assert.throws(()=>scanQuery(url));
assert.equal(swipeDirection(-90,10),1);assert.equal(swipeDirection(90,10),-1);assert.equal(swipeDirection(10,90),0);
const {editLabelDraft,restoreLabelDraft}=await bundled('src/lib/voice-guidance/stationOperations.ts');
let partial=editLabelDraft({...emptyLabelDraft(),data:{deadlineDate:'2028-02-29',productQuantity:'70g'}},'deadlineDate','2028-02-');
partial=editLabelDraft(partial,'productQuantity','70');assert.ok(labelIssues(partial).length);assert.equal(restoreLabelDraft(partial).data.productQuantity,'70');assert.equal(partial.data.deadlineDate,'2028-02-');
const {interpretationInput}=await bundled('src/interpretationInput.ts');
const input=interpretationInput('😀'.repeat(1000),{weight:'0.07',secret:'never'},{hunterName:Array.from({length:200},()=>({value:'名'.repeat(100),label:'名'.repeat(100)}))},Array.from({length:10},()=>({role:'user',text:'話'.repeat(400)})));
assert.ok(Buffer.byteLength(JSON.stringify(input))<=12000);assert.equal(input.values.secret,undefined);assert.ok(input.options.hunterName.length<=10);
console.log('Catalog mapping, QR allowlist, swipe, incomplete autosave and bounded Gemini context passed');
