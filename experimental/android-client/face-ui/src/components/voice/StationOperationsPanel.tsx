'use client';
import {
  forwardRef,
  useEffect,
  useLayoutEffect,
  useImperativeHandle,
  useRef,
  useState,
} from 'react';
import { PLACEHOLDER_SCHEMA } from '@/lib/labels/placeholderSchema';
import {fieldForLabel,labelGroups} from '../../lib/voice-guidance/labelEditor';
import {CatalogPicker} from './CatalogPicker';
import {individualData,productData,swipeDirection,scanQuery} from '../../catalog';
import {editLabelDraft,restoreLabelDraft} from '../../lib/voice-guidance/stationOperations';
import {LabelValueEditor} from './LabelValueEditor';
import {
  emptyLabelDraft,
  labelFields,
  labelIssues,
  LABEL_TEMPLATES,
  mergeLabelDraft,
  readOperation,
  type DeviceResult,
  type LabelDraft,
  type StationDevices,
  type StationOperation,
} from '@/lib/voice-guidance/stationOperations';
import { StationFieldEditor } from './StationFieldEditor';
import css from './VoiceWorkspace.module.css';
import {native} from '../../bridge';

export type StationOperationsHandle = {
  context: () => {operation:string};
  back:()=>boolean;
  show: (raw: unknown) => object;
  prepare: (raw: unknown) => object;
};
type Props = {
  active: boolean;
  devices?: StationDevices;
  onShow: () => void;
  onReceiving: () => void;
  onResult: (result: DeviceResult) => void;
  onWeight: (value: string) => void;
  onManualChange: () => void;
};
const titles = {
  receiving: '搬入入力',
  labels: 'ラベル印刷',
  weigh: '計量して登録',
  scan: 'スキャンして印刷',
  devices: '機器状態',
  menu: '作業を選ぶ',
};

export const StationOperationsPanel = forwardRef<StationOperationsHandle, Props>(
  function StationOperationsPanel(
    { active, devices, onShow, onReceiving, onResult, onWeight, onManualChange },
    ref,
  ) {
    const [operation, setOperation] = useState<StationOperation>('menu');
    const [draft, setDraft] = useState<LabelDraft>(()=>{try{return devices?.draftStorageKey?restoreLabelDraft(JSON.parse(localStorage.getItem(devices.draftStorageKey)||'{}')):emptyLabelDraft();}catch{return emptyLabelDraft();}});
    const [group,setGroup]=useState('');
    const [capacity,setCapacity]=useState(()=>window.innerHeight<720?2:4);
    useEffect(()=>{const resize=()=>setCapacity(window.innerHeight<720?2:4);window.addEventListener('resize',resize);return ()=>window.removeEventListener('resize',resize);},[]);
    const draftRef = useRef(draft);
    const [page, setPage] = useState(0);
    const swipe=useRef<{x:number;y:number}|null>(null);
    const [scanSeed,setScanSeed]=useState('');
    const [editing, setEditing] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const busyRef = useRef(false);
    const [results, setResults] = useState<Partial<Record<StationOperation, DeviceResult>>>({});
    const [printLocked, setPrintLocked] = useState(false);
    const printLockedRef = useRef(false);
    const [confirm, setConfirm] = useState<'print' | 'test' | 'tare' | 'zero' | null>(null);
    const [error, setError] = useState('');
    const [facility,setFacility]=useState<Record<string,string>>({});
    const [facilityError,setFacilityError]=useState('');
    useEffect(()=>{if(confirm!=='print')return;let current=true;setFacility({});setFacilityError('');native('catalog',{kind:'facility'}).then(r=>{if(current){setFacility(r.item||{});if(!Object.keys(r.item||{}).length)setFacilityError('表示できる施設情報がありません。');}}).catch(e=>{if(current)setFacilityError(e.message);});return()=>{current=false;};},[confirm,devices?.identity]);
    const generation = useRef(0);
    const identity = useRef(devices?.identity);
    useEffect(()=>{if(!active){generation.current++;busyRef.current=false;setBusy(false);}},[active]);
    useEffect(() => {
      const currentGeneration = generation;
      return () => {
        currentGeneration.current++;
      };
    }, []);
    // Changing account/facility/device invalidates the preview and any late result.
    useLayoutEffect(() => {
      identity.current = devices?.identity;
      generation.current++;
      busyRef.current = false;
      setBusy(false);
      setConfirm(null);
      setEditing(null);
      setResults({});
      // Keep a previously dispatched print locked until the operator checks output.
    }, [devices?.identity]);
    async function run(
      action: 'print' | 'test' | 'weigh' | 'scan' | 'health' | 'tare' | 'zero',
      target: StationOperation = operation,
    ) {
      if (busyRef.current || !devices) return;
      const isPrint = action === 'print' || action === 'test';
      if (isPrint && printLockedRef.current) return;
      if (action === 'print' && (labelIssues(draftRef.current).length || devices.labelUnavailable))
        return;
      if (['print', 'test', 'tare', 'zero'].includes(action) && confirm !== action) return;
      busyRef.current = true;
      setBusy(true);
      setError('');
      setConfirm(null);
      if (isPrint) {
        printLockedRef.current = true;
        setPrintLocked(true);
      }
      const ticket = ++generation.current;
      const scope = devices.identity;
      setResults((previous) => ({ ...previous, [target]: undefined }));
      let result: DeviceResult;
      try {
        result =
          action === 'print'
            ? await devices.printLabel({ ...draftRef.current, data: { ...draftRef.current.data } })
            : action === 'test'
              ? await devices.testPrint()
              : await devices[action]();
      } catch {
        result = {
          state: isPrint || action === 'tare' || action === 'zero' ? 'unknown' : 'error',
          message: isPrint
            ? '送信結果が不明です。出力を確認してください。自動再送はしません。'
            : '機器に接続できません。接続を確認してください。',
        };
      }
      if (ticket !== generation.current || scope !== identity.current) return;
      setResults((previous) => ({ ...previous, [target]: result }));
      busyRef.current = false;
      setBusy(false);
      onResult(result);
    }
    function update(raw: unknown) {
      if (busyRef.current || printLockedRef.current)
        throw new Error('先ほどの印刷結果を確認してから変更してください。');
      const next = mergeLabelDraft(draftRef.current, raw);
      commit(next);
    }
    function commit(next:LabelDraft){
      draftRef.current = next;
      setDraft(next);
      if(devices?.draftStorageKey)localStorage.setItem(devices.draftStorageKey,JSON.stringify(next));
      setConfirm(null);
      setError('');
    }
    function show(next: StationOperation) {
      if(next==='devices')next='menu';
      if(next==='weigh'){void native('work',{path:'/input/meat-inbound'}).catch(e=>setError(e.message));return;}
      if(next==='scan'){setOperation('labels');setConfirm(null);setPage(0);setGroup('individual');setEditing('individualNumber');onShow();return;}
      if (busyRef.current) throw new Error('機器の応答を待っています。');
      setConfirm(null);
      setEditing(null);
      setError('');
      setPage(0);
      if (next === 'receiving') {
        onReceiving();
        return;
      }
      setOperation(next);
      onShow();
      if (devices && ['weigh'].includes(next))
        void run(next === 'devices' ? 'health' : next === 'scan' ? 'scan' : 'weigh', next);
    }
    useImperativeHandle(ref, () => ({
      context:()=>({operation}),
      back:()=>{if(editing){setEditing(null);return true;}if(confirm){setConfirm(null);return true;}if(operation!=='menu'){show('menu');return true;}return false;},
      show: (raw) => {
        const next = readOperation(raw);
        show(next);
        return {
          shown: next,
          mutationExecuted: false,
          reading: !!devices && ['weigh'].includes(next),
        };
      },
      prepare: (raw) => {
        if(editing)throw new Error('画面の入力を保存するか閉じてから、訂正を教えてください。');
        const next=mergeLabelDraft(draftRef.current,raw);
        const changedKeys=Object.keys((raw as {data?:object}).data||{});
        const unsupported=changedKeys.find(key=>!labelFields(next).includes(key as (ReturnType<typeof labelFields>)[number]));
        if(unsupported)throw new Error(`${fieldForLabel(unsupported,next).label}はこのラベルでは使いません。ラベルの種類を選び直してください。`);
        const data=(raw as {data?:Record<string,string>}).data;
        if(data?.individualNumber!==undefined&&data.individualNumber!==draftRef.current.data.individualNumber)update({...raw as object,data:{...individualData({id:'',individualNumber:data.individualNumber}),...data}});else update(raw);
        show('labels');
        if(changedKeys.includes('individualNumber')){setScanSeed(data?.individualNumber||'');setEditing('individualNumber');}
        else if(changedKeys.includes('productName'))setEditing('productName');
        const target=labelGroups(next).find(group=>group.keys.includes(changedKeys[0]));
        if(target){setGroup(target.id);setPage(Math.floor(target.keys.indexOf(changedKeys[0])/capacity));}
        return { shown: 'labels', executed: false, missing: labelIssues(draftRef.current) };
      },
    }));
    if (!active) return null;
    const result = results[operation];
    const fields = labelFields(draft);
    const groups=labelGroups(draft);
    const currentGroup=groups.find(item=>item.id===group)||groups[0];
    const groupKeys=currentGroup?.keys||[];
    const fieldCapacity=currentGroup?.id==='basic'?Math.max(1,groupKeys.length):groupKeys.some(key=>(draft.data[key]?.length||0)>12)?Math.max(1,capacity/2):capacity;
    const pageIndex=Math.min(page,Math.max(0,Math.ceil(groupKeys.length/fieldCapacity)-1));
    const shownFields=groupKeys.slice(pageIndex*fieldCapacity,pageIndex*fieldCapacity+fieldCapacity);
    const issues=labelIssues(draft);
    const allGroups=groups.map(g=>g.id);
    const advance=(direction:number)=>{if(!direction||editing||confirm)return;const max=Math.max(0,Math.ceil(groupKeys.length/fieldCapacity)-1);if(direction>0&&pageIndex<max)setPage(pageIndex+1);else if(direction<0&&pageIndex>0)setPage(pageIndex-1);else {const index=allGroups.indexOf(currentGroup.id);const next=allGroups[index+direction];if(next){setGroup(next);setPage(0);}}};
    async function scanLabel(camera:boolean){const ticket=++generation.current;setError('');onManualChange();try{await native('stop');const result=scanQuery((await native(camera?'cameraScan':'scan')).value);if(ticket!==generation.current)return;if('query'in result){setScanSeed(result.query);setEditing('individualNumber');}else {const row=(await native('catalog',{kind:result.trace?'trace':'individual',id:result.id})).item;if(ticket!==generation.current)return;update({template:'traceable',data:individualData(row)});show('labels');setGroup('individual');}}catch(e){if(ticket===generation.current)setError(e instanceof Error?e.message:'読み取れませんでした');}}

    const fieldFor=(key:string)=>fieldForLabel(key,draft);
    const edit = (key: string) => {
      if (!printLocked && !busy) {
        setConfirm(null);
        setEditing(key);
      }
    };
    const requestConfirm = (value: typeof confirm) => {
      setError('');
      setConfirm(value);
    };
    return (
      <div className={css.board} data-operation={operation} aria-label="現場の作業">
        <nav className={css.operationHeader}>
          <strong>{titles[operation]}</strong>
        </nav>
        {operation === 'menu' ? (
          <div className={css.cards} data-count="5">
            {(['receiving', 'labels', 'weigh', 'scan'] as const).map((key) => (
              <button className={css.card} key={key} type="button" onClick={() => show(key)}>
                <strong>{titles[key]}</strong>
              </button>
            ))}
            <button type="button" className={css.card} onClick={()=>void native('work',{path:'/input/meat-outbound'}).catch(e=>setError(e.message))}><strong>スキャンして出庫</strong></button>
          </div>
        ) : operation === 'labels' ? (
          <>
            <div className={css.operationToolbar}>
              <button type="button" disabled={busy || printLocked} aria-haspopup="dialog" onClick={() => edit('template')}>
                {LABEL_TEMPLATES[draft.template]}
              </button>
              <button type="button" disabled={busy || printLocked} onClick={() => edit('copies')}>
                {draft.copies}枚
              </button>
              <button type="button" aria-label="カメラスキャン" disabled={busy||printLocked} onClick={()=>void scanLabel(true)}>カメラ</button>
            </div>
            <nav className={css.labelGroups} role="tablist" aria-label="ラベルの入力項目">
              {groups.map(item=><button type="button" key={item.id} role="tab" aria-selected={currentGroup?.id===item.id} onClick={()=>{setGroup(item.id);setPage(0);}}>{item.label}</button>)}

            </nav>
            <div
              className={css.cards}
              data-count={shownFields.length}
              aria-label="ラベルの印刷内容" role="tabpanel" data-group={currentGroup?.id}
              onTouchStart={e=>{const t=e.touches[0];swipe.current={x:t.clientX,y:t.clientY};}}
              onTouchEnd={e=>{const start=swipe.current;swipe.current=null;if(start){const t=e.changedTouches[0];advance(swipeDirection(t.clientX-start.x,t.clientY-start.y));}}}
              onTouchCancel={()=>{swipe.current=null;}}
            >
              {shownFields.map((key) => (
                <button
                  type="button"
                  key={key}
                  className={css.card}
                  disabled={busy || printLocked}
                  data-missing={!draft.data[key]}
                  data-long={(draft.data[key]?.length ?? 0) > 12}
                  onClick={() => edit(key)}
                >
                  <span className={css.label}>{fieldFor(key).label}<small>{fieldFor(key).required?'必須':'任意'}</small></span>
                  <strong>{draft.data[key] || '未入力'}</strong>

                </button>
              ))}
            </div>
            <div className={css.operationToolbar}>
              {groupKeys.length > fieldCapacity && (
                <>
                  <button type="button" data-page="true" disabled={pageIndex === 0} onClick={() => setPage(pageIndex - 1)}>
                    前へ
                  </button>
                  <span aria-label="ページ">{Array.from({length:Math.ceil(groupKeys.length/fieldCapacity)},(_,i)=>i===pageIndex?'●':'○').join(' ')}</span>
                  <button
                    type="button"
                    data-page="true" disabled={(pageIndex + 1) * fieldCapacity >= groupKeys.length}
                    onClick={() => setPage(pageIndex + 1)}
                  >
                    次へ
                  </button>
                </>
              )}
              <button
                type="button"
                disabled={
                  busy || printLocked
                }
                onClick={() => requestConfirm('print')}
              >
                入力内容を確認
              </button>

            </div>
            {!printLocked && (
              <p className={css.operationHint}>
                {issues.length?`確認が必要な項目：${issues.length}件`:'入力内容を確認できます'}
              </p>
            )}
            {printLocked && !busy && (
              <button
                type="button"
                className={css.operationReset}
                onClick={() => {
                  printLockedRef.current = false;
                  setPrintLocked(false);
                  setResults((previous) => ({ ...previous, labels: undefined }));
                }}
              >
                出力を確認した・次のラベルへ
              </button>
            )}
          </>
        ) : (
          <>
            <div className={css.cards} data-count={result?.cards?.length || 1}>
              {result?.cards?.length ? (
                result.cards.map((card) => (
                  <div key={card.label} className={css.card} data-long={card.value.length > 12}>
                    <span className={css.label}>{card.label}</span>
                    <strong>{card.value}</strong>
                  </div>
                ))
              ) : (
                <div className={css.card}>
                  <strong>
                    {busy
                      ? operation === 'scan'
                        ? 'コードを読み取ってください'
                        : '機器に確認中…'
                      : titles[operation]}
                  </strong>
                </div>
              )}
            </div>
            <div className={css.operationToolbar}>
              <button
                type="button"
                disabled={!devices || busy}
                onClick={() =>
                  void run(
                    operation === 'devices' ? 'health' : operation === 'scan' ? 'scan' : 'weigh',
                  )
                }
              >
                {operation === 'devices'
                  ? '状態を更新'
                  : operation === 'scan'
                    ? '読み取り開始'
                    : '重量を取得'}
              </button>
              {operation === 'weigh' && (
                <>
                  <button
                    type="button"
                    disabled={!devices || busy}
                    onClick={() => requestConfirm('tare')}
                  >
                    風袋引き
                  </button>
                  <button
                    type="button"
                    disabled={!devices || busy}
                    onClick={() => requestConfirm('zero')}
                  >
                    ゼロ点
                  </button>
                  {result?.weightKg != null && (
                    <>
                      <button
                        type="button"
                        disabled={busy}
                        onClick={() => onWeight(String(result.weightKg))}
                      >
                        搬入重量に使う
                      </button>
                      <button
                        type="button"
                        disabled={busy || printLocked}
                        onClick={() => {
                          update({
                            template: draft.template === 'carcass' ? 'traceable' : draft.template,
                            data: { productQuantity: `${result.weightKg} kg` },
                          });
                          show('labels');
                        }}
                      >
                        ラベルの内容量に使う
                      </button>
                    </>
                  )}
                </>
              )}
              {operation==='scan'&&<><button type="button" disabled={busy} onClick={()=>void scanLabel(true)}>カメラスキャン</button>{result?.scanValue&&<button type="button" onClick={()=>{try{const value=scanQuery(result.scanValue!);if('query'in value){setScanSeed(value.query);setEditing('individualNumber');}else{void native('catalog',{kind:value.trace?'trace':'individual',id:value.id}).then(r=>{update({template:'traceable',data:individualData(r.item)});show('labels');}).catch(e=>setError(e.message));}}catch(e){setError(e instanceof Error?e.message:'コードを確認してください');}}}>個体データを読み取る</button>}</>}

            </div>
          </>
        )}
        {result && (
          <p className={css.operationHint} role="status" data-state={result.state}>
            {result.message}
          </p>
        )}
        {busy && (
          <p role="status">
            {operation === 'labels' ? '印刷を送信中…' : '機器の応答を待っています…'}
          </p>
        )}
        {error && <p role="alert">{error}</p>}
        {confirm && (
          <div className={css.overlay}>
            <div className={css.editor} role="dialog" aria-modal="true" aria-label="機器操作の確認">
              <h2>
                {confirm === 'print'
                  ? `${LABEL_TEMPLATES[draft.template]}を${draft.copies}枚印刷`
                  : confirm === 'test'
                    ? '接続確認用のテストラベルを印刷'
                    : confirm === 'tare'
                      ? '風袋引きを実行'
                      : 'ゼロ点を合わせる'}
              </h2>
              {confirm==='print'?<>
                <div className={css.confirmFields}>{fields.map(key=><button type="button" key={key} onClick={()=>{setConfirm(null);setEditing(key);}}><span>{fieldFor(key).label}</span><strong>{draft.data[key]||'未入力'}</strong></button>)}</div>
                {!!issues.length&&<p role="status">未入力・確認が必要な項目があります。項目を押して編集できます。</p>}
                <section className={css.facilitySummary}><h3>印刷元の施設</h3>{Object.entries(facility).map(([key,value])=><p key={key}>{value}</p>)}{!Object.keys(facility).length&&<p>{facilityError||'施設情報を読み取り中…'}</p>}<button type="button" onClick={()=>void native('work',{path:'/input/settings/facility'})}>施設情報を編集する</button></section>
              </>:<p>計量器に載っている物を確認してください。</p>}
              <p>{devices?.printer}</p>
              {confirm==='print'&&devices?.labelUnavailable&&<p>下書きはこの端末に保存されています。業務画面への自動転記はないため、登録済みの記録と照合して印刷してください。</p>}
              <button
                type="button"
                disabled={busy}
                className={css.primary}
                onClick={() => {if(confirm==='print'&&devices?.labelUnavailable){void native('work',{path:'/reports/labels'});}else void run(confirm);}}
              >
                {confirm==='print'&&devices?.labelUnavailable?'業務画面で印刷する':'この内容で実行する'}
              </button>
              <button type="button" onClick={() => setConfirm(null)}>
                戻る
              </button>
            </div>
          </div>
        )}
        {editing && (
          <div className={css.overlay}>
            {editing==='individualNumber'||editing==='productName'?<CatalogPicker key={editing} kind={editing==='individualNumber'?'individuals':'products'} initial={editing==='individualNumber'?(scanSeed||draft.data.individualNumber||''):''}
              onClose={()=>{setEditing(null);setScanSeed('');}}
              onNumber={editing==='individualNumber'?value=>{commit({...draftRef.current,data:{...draftRef.current.data,...individualData({id:'',individualNumber:value})}});onManualChange();}:undefined}
              onSelect={row=>{const data=editing==='individualNumber'?individualData(row):productData(row);let next=draftRef.current;for(const [key,value] of Object.entries(data))next=editLabelDraft(next,key,value);commit(next);onManualChange();setEditing(null);setScanSeed('');if(operation!=='labels'){show('labels');setGroup('individual');}}}
            />:editing==='productQuantity'||['receivingDate','deadlineDate'].includes(editing)?<LabelValueEditor
              kind={editing==='productQuantity'?'quantity':'date'} label={fieldFor(editing).label} value={draft.data[editing]||''}
              onClose={()=>setEditing(null)} onSave={value=>{commit(editLabelDraft(draftRef.current,editing,value));onManualChange();}}

            />:<StationFieldEditor
              key={editing}
              fieldKey={editing}
              field={fieldFor(editing)}
              value={
                editing === 'template'
                  ? draft.template
                  : editing === 'copies'
                    ? draft.copies
                    : draft.data[editing]
              }
              onClose={() => setEditing(null)}
              onSave={(value) => {
                commit(editLabelDraft(draftRef.current,editing,value));
                onManualChange();
                if(fieldFor(editing).type==='select')setEditing(null);
                if(editing==='template'){setPage(0);setGroup('');}
              }}
            />}
          </div>
        )}
      </div>
    );
  },
);
