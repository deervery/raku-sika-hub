'use client';
import { useEffect, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from 'react';
import type { FormDefinition } from '@/types/forms';
import { startRealtime } from '@/lib/voice-guidance/realtimeClient';
import {
  RECEIVING_INPUT_KEYS,
  validateInputPatch,
  type ChoiceOverrides,
  type InputPatch,
} from '@/lib/voice-guidance/receivingInput';
import {
  fieldChoices,
  missingValue,
  reviewEntries,
  validateManualPatch,
  type ReviewMode,
} from '@/lib/voice-guidance/stationUi';
import {receivingEntries,healthEntries,healthSummary,labelOptional,HEALTH_GROUP} from '../../receivingLayout';
import {HealthEditor} from './HealthEditor';
import {swipeDirection} from '../../catalog';
import { receivingReviewValue } from './ReceivingReview';
import { StationFieldEditor } from './StationFieldEditor';
import css from './VoiceWorkspace.module.css';
import { StationOperationsPanel, type StationOperationsHandle } from './StationOperationsPanel';
import type { StationDevices } from '@/lib/voice-guidance/stationOperations';

export type AddStationChoice = (
  field: string,
  name: string,
) => Promise<{ value: string; label: string }>;
type Line = { id: string; role: 'user' | 'assistant'; text: string; order: number };
export function VoiceStation({
  form,
  values,
  enteredKeys,
  defaultKeys = [],
  options = {},
  onApply,
  onReview,
  onManualPatch,
  onAddChoice,
  devices,
  disabled = false,
  connect = startRealtime,
}: {
  form: FormDefinition;
  values: Record<string, unknown>;
  enteredKeys?: string[];
  defaultKeys?: string[];
  options?: ChoiceOverrides;
  onApply: (patch: InputPatch, base: Record<string, unknown>) => void;
  onReview: () => void;
  onManualPatch?: (patch: Record<string, unknown>, base: Record<string, unknown>) => void;
  onAddChoice?: AddStationChoice;
  devices?: StationDevices;
  disabled?: boolean;
  connect?: typeof startRealtime;
}) {
  const [pending, setPending] = useState<Record<string, unknown>>({});
  const pendingRef = useRef(pending);
  const [localOptions, setLocalOptions] = useState<ChoiceOverrides>({});
  const combinedOptions = useMemo(() => ({ ...options, ...localOptions }), [options, localOptions]);
  const current = useRef({ form, values, options: combinedOptions, disabled });
  useLayoutEffect(() => {
    current.current = { form, values, options: combinedOptions, disabled };
  }, [form, values, combinedOptions, disabled]);
  const [mode, setMode] = useState<ReviewMode>('all');
  const [focus, setFocus] = useState<string[] | undefined>();
  const [board, setBoard] = useState(false);
  const [operationVisible, setOperationVisible] = useState(true);
  const operationShown=useRef(true);
  const operationPanel = useRef<StationOperationsHandle>(null);
  const [page, setPage] = useState(0);
  const swipe=useRef<{x:number;y:number}|null>(null);
  const [editor, setEditor] = useState<{ key: string; query?: string } | null>(null);
  const [chatStarted,setChatStarted]=useState(false);
  const [home,setHome]=useState(true);
  const [voice, setVoice] = useState('off');
  const [history, setHistory] = useState(false);
  const historyRef = useRef(false);
  const chat = useRef<HTMLDivElement>(null);
  const counter = useRef(0);
  const [lines, setLines] = useState<Line[]>([
    { id: 'welcome', role: 'assistant', text: '分かっていることから、教えてください。', order: 0 },
  ]);
  const session = useRef<ReturnType<typeof startRealtime> | null>(null);
  const generation = useRef(0);
  const merged = { ...values, ...pending };
  // Form defaults are not facts supplied by the operator (especially health checks).
  const displayValues = Object.fromEntries(
    [...new Set([...(enteredKeys ?? Object.keys(values)), ...Object.keys(pending)])].map((key) => [
      key,
      merged[key],
    ]),
  );
  const allEntries=receivingEntries(form,displayValues,mode,focus);
  // Keep primary information on one viewport; overflow uses explicit pages instead of tiny text.
  const [pageUnits, setPageUnits] = useState(12);
  useEffect(() => {
    const resize = () => {
      const columns = window.innerWidth <= 600 ? 2 : 4;
      const rows = Math.max(1, Math.floor((window.innerHeight - 210) / 100));
      setPageUnits(Math.max(columns, Math.min(12, rows * columns)));
    };
    resize();
    window.addEventListener('resize', resize);
    return () => window.removeEventListener('resize', resize);
  }, []);
  const pages: (typeof allEntries)[] = [];
  let units = 0;
  for (const entry of allEntries) {
    const label = receivingReviewValue(
      entry[1],
      displayValues[entry[0]],
      combinedOptions[entry[0]],
    );
    const cost = label.length > 12 || entry[1].label.length > 14 ? 2 : 1;
    if (!pages.length || units + cost > pageUnits) {
      pages.push([]);
      units = 0;
    }
    pages[pages.length - 1].push(entry);
    units += cost;
  }
  const pageIndex = Math.min(page, Math.max(0, pages.length - 1));
  const entries = pages[pageIndex] ?? [];
  const say = (text: string) => {
    const order = ++counter.current;
    setLines((previous) => [
      ...previous.slice(-59),
      { id: `local-${order}`, role: 'assistant', text, order },
    ]);
  };
  function stop() {
    generation.current++;
    session.current?.stop();
    session.current = null;
    setVoice('off');setChatStarted(false);
  }
  useEffect(() => {
    const activeGeneration = generation;
    const pause = () => {
      activeGeneration.current++;
      session.current?.stop();
      session.current = null;
      setVoice('off');
    };
    const hide = () => {
      if (document.hidden) pause();
    };
    document.addEventListener('visibilitychange', hide);
    window.addEventListener('station-pause', pause);
    return () => {
      activeGeneration.current++;
      session.current?.stop();
      document.removeEventListener('visibilitychange', hide);
      window.removeEventListener('station-pause', pause);
    };
  }, []);
  useEffect(() => {
    if (!historyRef.current && chat.current) chat.current.scrollTop = chat.current.scrollHeight;
  }, [lines]);
  const ownSave=useRef(false);
  const sessionBase = useRef(values);
  useEffect(() => {
    if(ownSave.current&&sessionBase.current!==values&&!disabled){ownSave.current=false;sessionBase.current=values;return;}
    if (sessionBase.current !== values || disabled) {
      generation.current++;
      session.current?.stop();
      session.current = null;
      setVoice('off');
      if (sessionBase.current !== values) {
        pendingRef.current = {};
        setPending({});
        sessionBase.current = values;
      }
    }
  }, [values, disabled]);
  function changeMode(next: ReviewMode, fields?: string[]) {
    operationShown.current=false;
    setOperationVisible(false);
    setMode(next);
    setFocus(fields);
    setPage(0);
    setBoard(true);
    setEditor(null);
  }
  function storePatch(patch: Record<string, unknown>) {
    operationShown.current=false;
    setOperationVisible(false);
    pendingRef.current = { ...pendingRef.current, ...patch };
    setPending(pendingRef.current);
    setBoard(true);
    setMode('filled');
    setFocus(undefined);
    setPage(0);
  }
  function openChoices(raw: unknown) {
    if (!raw || typeof raw !== 'object') throw new Error('項目を教えてください。');
    const { field, query } = raw as { field?: unknown; query?: unknown };
    const state = current.current;
    if (
      typeof field !== 'string' ||
      !state.form.fields[field] ||
      !reviewEntries(state.form, { ...state.values, ...pendingRef.current }, 'all').some(
        ([key]) => key === field,
      )
    )
      throw new Error('その項目は今は選べません。');
    if (query != null && (typeof query !== 'string' || query.length > 100))
      throw new Error('名前をもう一度教えてください。');
    setEditor({ key: field, query: typeof query === 'string' ? query : '' });
    setBoard(true);
    const choices = fieldChoices(state.form.fields[field], state.options[field]);
    return {
      needsChoice: true,
      question:
        field === 'hunterName' &&
        query &&
        !choices.some((item) => item.label === query || item.value === query)
          ? `${query}さんを新しく追加しますか？`
          : `${state.form.fields[field].label}を選んでください。`,
    };
  }
  function propose(raw: unknown) {
    if(operationShown.current&&operationPanel.current?.context().operation.startsWith('business:'))return operationPanel.current.form(raw);
    const state = current.current;
    if (state.disabled) throw new Error('今は入力できません。');
    if (!raw || typeof raw !== 'object' || Array.isArray(raw))
      throw new Error('内容をもう一度教えてください。');
    const valid: Record<string, unknown> = {};
    let unresolved: { field: string; query: string } | undefined;
    for (const [key, value] of Object.entries(raw)) {
      if (
        !RECEIVING_INPUT_KEYS.includes(key as (typeof RECEIVING_INPUT_KEYS)[number]) ||
        typeof value !== 'string' ||
        value.length > 100
      )
        throw new Error('対応する項目を教えてください。');
      const field = state.form.fields[key];
      if (!field) throw new Error('その項目はありません。');
      if (
        ['select', 'radio'].includes(field.type) &&
        !fieldChoices(field, state.options[key]).some((option) => option.value === value)
      ) {
        unresolved ??= { field: key, query: value };
        continue;
      }
      valid[key] = value;
    }
    if (Object.keys(valid).length)
      storePatch(
        validateInputPatch(
          valid,
          state.form,
          { ...state.values, ...pendingRef.current },
          state.options,
        ),
      );
    return unresolved ? openChoices(unresolved) : { ok: true };
  }
  function start(interrupt = false) {
    if (disabled || (voice !== 'off' && !interrupt)) return;
    const ticket = ++generation.current;
    session.current?.stop();
    session.current = null;
    const definition = current.current.form;
    const orderBase = counter.current + 1;
    try {
      session.current = connect(Object.fromEntries(Object.entries(displayValues).filter(([key]) => !defaultKeys.includes(key) || key in pendingRef.current)), current.current.options, {
        context:()=>operationShown.current?(operationPanel.current?.context()||{operation:'menu'}):{operation:'receiving'},
        state: (state) => {
          if (ticket === generation.current) {
            setVoice(state);
            if (state === 'off') session.current = null;
          }
        },
        error: (message) => {
          if (ticket === generation.current) {
            session.current = null;
            say(message);
          }
        },
        message: (id, role, text, order = 0) => {
          if (ticket !== generation.current) return;
          if(role==='user'&&text.trim())setChatStarted(true);
          const position = orderBase + order;
          counter.current = Math.max(counter.current, position);
          const key = `${ticket}-${id}`;
          setLines((previous) =>
            [
              ...previous.filter((line) => line.id !== key),
              { id: key, role, text, order: position },
            ]
              .sort((a, b) => a.order - b.order)
              .slice(-60),
          );
        },
        propose: (raw) => {
          if (
            ticket !== generation.current ||
            (sessionBase.current !== current.current.values && !ownSave.current) ||
            definition !== current.current.form
          )
            throw new Error('入力が変わりました。');
          return propose(raw);
        },
        choices: (raw) => {
          if (ticket !== generation.current) throw new Error('会話は終了しました');
          return openChoices(raw);
        },
        operation: (raw) => {
          if (ticket !== generation.current || !operationPanel.current)
            throw new Error('会話は終了しました');
          return operationPanel.current.show(raw);
        },
        label: (raw) => {
          if (ticket !== generation.current || !operationPanel.current)
            throw new Error('会話は終了しました');
          return operationPanel.current.prepare(raw);
        },
        review: (raw) => {
          if (ticket !== generation.current) throw new Error('会話は終了しました');
          if(operationShown.current&&operationPanel.current?.context().operation.startsWith('business:'))return operationPanel.current.form({__review:true});
          const args = (raw ?? {}) as { mode?: unknown; fields?: unknown };
          if (args.mode && !['filled', 'missing', 'all'].includes(String(args.mode)))
            throw new Error('表示方法を確認してください');
          if (
            args.fields != null &&
            (!Array.isArray(args.fields) ||
              args.fields.length > 12 ||
              args.fields.some(
                (key) => typeof key !== 'string' || !current.current.form.fields[key],
              ))
          )
            throw new Error('表示する項目を確認してください');
          changeMode((args.mode as ReviewMode) || 'filled', args.fields as string[] | undefined);
          return { ok: true };
        },
      });
    } catch {
      session.current = null;
      setVoice('off');
      say('音声を開始できませんでした。アプリを開き直してください。');
    }
  }
  function voiceAction() {
    if (voice !== 'off') stop();
    else start(); // Speech itself interrupts; the visible mic always ends capture.
  }
  useEffect(()=>{
    const host=window as Window & {stationBack?:()=>boolean};
    host.stationBack=()=>{if(editor){setEditor(null);return true;}if(operationVisible){if(operationPanel.current?.back())return true;if(chatStarted){stop();setChatStarted(false);return true;}return false;}if(board){setBoard(false);chooseWork();setChatStarted(false);return true;}chooseWork();return true;};
    return()=>{delete host.stationBack;};
  },[editor,operationVisible,board,chatStarted]);
  function chooseWork() {
    session.current?.inform({});
    try {operationPanel.current?.show({operation:'menu'});}
    catch (error) {say(error instanceof Error ? error.message : '作業を選んでください。');}
  }
  function manual(key: string, value: unknown) {
    const state = current.current;
    if (state.disabled) throw new Error('今は変更できません。');
    const patch = validateManualPatch(
      { [key]: value },
      {...state.form,fields:{...state.form.fields,[key]:{...state.form.fields[key],digits:undefined}}},
      { ...state.values, ...pendingRef.current },
      state.options,
    );
    storePatch(patch);
    if(onManualPatch){ownSave.current=true;onManualPatch(patch,state.values);}
    session.current?.inform(patch);
  }
  const latest = {
    user: [...lines].reverse().find((line) => line.role === 'user')?.id,
    assistant: [...lines].reverse().find((line) => line.role === 'assistant')?.id,
  };
  function finish() {
    try {
      const patch = validateManualPatch(pendingRef.current, form, values, combinedOptions);
      if (onManualPatch) onManualPatch(patch, values);
      else if (Object.keys(patch).length)
        onApply(validateInputPatch(patch, form, values, combinedOptions), values);
      stop();
      onReview();
    } catch (e) {
      say(e instanceof Error ? e.message : '入力を確認してください。');
    }
  }
  return (
    <section
      className={css.workspace}
      data-board={board || (operationVisible&&!home)}
      data-home={home&&!board}
      data-chatting={chatStarted}
      data-voice={voice}
      aria-label="会話で搬入入力"
    >
      <StationOperationsPanel
        ref={operationPanel}
        active={operationVisible}
        onHome={setHome}
        devices={devices}
        onShow={() => {
          operationShown.current=true;
          setBoard(false);
          setOperationVisible(true);
          setEditor(null);
        }}
        onReceiving={() => changeMode('all')}
        onManualChange={()=>session.current?.inform({})}
        onWeight={(value) => manual('weight', value)}
        onResult={(result) => session.current?.inform({ deviceResult: result })}
      />
      {board && !operationVisible && (
        <div className={css.board} aria-label="フォーム内容の確認">
          <nav className={css.modes} aria-label="表示する項目">
            {(['filled', 'missing', 'all'] as const).map((item) => (
              <button
                type="button"
                key={item}
                aria-pressed={mode === item && !focus?.length}
                onClick={() => changeMode(item)}
              >
                {item === 'filled' ? '入力済み' : item === 'missing' ? '未入力' : '一覧'}
              </button>
            ))}
          </nav>
          <div className={css.cards} data-count={entries.length} data-receiving="true" aria-label="下書きの内容"
            onTouchStart={e=>{const t=e.touches[0];swipe.current={x:t.clientX,y:t.clientY};}}
            onTouchEnd={e=>{const start=swipe.current;swipe.current=null;if(start){const t=e.changedTouches[0];const direction=swipeDirection(t.clientX-start.x,t.clientY-start.y);if(direction)setPage(Math.max(0,Math.min(pages.length-1,pageIndex+direction)));}}}
            onTouchCancel={()=>{swipe.current=null;}}>
            {entries.map(([key, field]) => {
              const grouped=key===HEALTH_GROUP;
              const value = grouped?healthSummary(form,merged):receivingReviewValue(field, displayValues[key], combinedOptions[key]);
              const initial=grouped?healthEntries(form,merged).every(([k])=>defaultKeys.includes(k)&&!(k in pending)):defaultKeys.includes(key)&&!(key in pending);
              const wide = value.length > 12 || field.label.length > 14;
              return (
                <button
                  type="button"
                  key={key}
                  className={css.card}
                  style={{ '--basis': wide ? '46%' : '21%' } as CSSProperties}
                  data-default={initial}
                  aria-label={`${field.label}：${value}`}
                  data-missing={!grouped&&missingValue(displayValues[key])}
                  data-warning={grouped?value.startsWith('異常あり'):field.warnValues?.includes(String(displayValues[key]))}
                  onClick={() => setEditor({ key })}
                >
                  {!labelOptional(key,displayValues[key])&&<span className={css.label}>
                    {field.label}
                    {field.required && <small>必須</small>}
                  </span>}
                  <strong>
                    {value}
                    {!missingValue(displayValues[key]) && field.suffix && (
                      <small>{field.suffix}</small>
                    )}
                  </strong>
                  {initial&&<small className={css.defaultNote}>初期値</small>}
                </button>
              );
            })}
            {!entries.length && (
              <p className={css.empty}>
                {mode === 'filled'
                  ? '教えてもらった内容が、ここに並びます。'
                  : '表示する項目はありません。'}
              </p>
            )}
          </div>
          <div className={css.boardFooter}>
            {pages.length > 1 && (
              <nav aria-label="項目のページ">
                <button
                  type="button"
                  disabled={pageIndex === 0}
                  onClick={() => setPage(pageIndex - 1)}
                >
                  ←
                </button>
                <span>
                  {pageIndex + 1}/{pages.length}
                </span>
                <button
                  type="button"
                  disabled={pageIndex + 1 === pages.length}
                  onClick={() => setPage(pageIndex + 1)}
                >
                  →
                </button>
              </nav>
            )}
            <button type="button" onClick={() => {setBoard(false);chooseWork();setChatStarted(false);}}>
              ホームに戻る
            </button>
            <button type="button" onClick={finish} disabled={disabled}>
              搬入登録を確認
            </button>
          </div>
        </div>
      )}
      <div
        ref={chat}
        className={css.chat}
        data-history={history}
        aria-label="会話履歴"
        onScroll={() => {
          const el = chat.current;
          if (el) {
            const older = el.scrollHeight - el.clientHeight - el.scrollTop > 36;
            historyRef.current = older;
            setHistory(older);
          }
        }}
      >
        {lines
          .filter((line) => line.text)
          .map((line) => (
            <div
              key={line.id}
              className={css.bubble}
              data-role={line.role}
              data-old={latest[line.role] !== line.id}
            >
              <p>{line.text}</p>
            </div>
          ))}
      </div>
      <div className={css.controls}>
      {chatStarted&&home&&<button type="button" onClick={()=>{stop();chooseWork();}}>メニュー</button>}
      <button
        className={css.faceButton}
        type="button"
        aria-pressed={voice!=='off'}
        disabled={disabled}
        aria-label="マイクをオン・オフ"
        onClick={voiceAction}

      >

        <svg viewBox="0 0 320 180" role="img" aria-label="案内する表情">
          <g className={css.eyes}>
            <ellipse cx="108" cy="69" rx="11" ry="16" />
            <ellipse cx="212" cy="69" rx="11" ry="16" />
          </g>
          <ellipse className={css.cheek} cx="67" cy="104" rx="21" ry="9" />
          <ellipse className={css.cheek} cx="253" cy="104" rx="21" ry="9" />
          <path
            className={css.mouth}
            d="M142 109Q160 129 178 109"
            fill="none"
            stroke="currentColor"
            strokeWidth="6"
            strokeLinecap="round"
          />
        </svg>
      </button>
        <span className={css.status} role="status">
          {voice === 'connecting'
            ? '考えています…'
            : voice === 'off'
              ? ''
              : voice === 'speaking'
                ? '話しています'
                : '聞いています'}
        </span>
      </div>
      {editor && (
        <div className={css.overlay}>
          {editor.key===HEALTH_GROUP?<HealthEditor form={form} values={merged} defaults={defaultKeys.filter(k=>!(k in pending))} onSave={manual} onClose={()=>setEditor(null)}/>:<StationFieldEditor
            key={`${editor.key}-${editor.query ?? ''}`}
            fieldKey={editor.key}
            field={form.fields[editor.key]}
            value={merged[editor.key]}
            options={combinedOptions[editor.key]}
            query={editor.query}
            onClose={() => setEditor(null)}
            onSave={(value) => manual(editor.key, value)}
            onAdd={
              editor.key === 'hunterName' && onAddChoice
                ? async (name) => {
                    if (current.current.disabled) throw new Error('今は追加できません');
                    const choice = await onAddChoice('hunterName', name);
                    const choices = [
                      ...fieldChoices(
                        form.fields.hunterName,
                        current.current.options.hunterName,
                      ).filter((item) => item.value !== choice.value),
                      choice,
                    ];
                    setLocalOptions((previous) => ({ ...previous, hunterName: choices }));
                    current.current.options = { ...current.current.options, hunterName: choices };
                    manual('hunterName', choice.value);
                  }
                : undefined
            }
          />}
        </div>
      )}
    </section>
  );
}
