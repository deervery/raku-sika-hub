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
import { type FormField } from '@/types/forms';
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
};
const titles = {
  receiving: '搬入入力',
  labels: 'ラベル印刷',
  weigh: '計量',
  scan: 'スキャン',
  devices: '機器状態',
  menu: '作業を選ぶ',
};

export const StationOperationsPanel = forwardRef<StationOperationsHandle, Props>(
  function StationOperationsPanel(
    { active, devices, onShow, onReceiving, onResult, onWeight },
    ref,
  ) {
    const [operation, setOperation] = useState<StationOperation>('menu');
    const [draft, setDraft] = useState<LabelDraft>(emptyLabelDraft);
    const draftRef = useRef(draft);
    const [page, setPage] = useState(0);
    const [editing, setEditing] = useState<string | null>(null);
    const [busy, setBusy] = useState(false);
    const busyRef = useRef(false);
    const [results, setResults] = useState<Partial<Record<StationOperation, DeviceResult>>>({});
    const [printLocked, setPrintLocked] = useState(false);
    const printLockedRef = useRef(false);
    const [confirm, setConfirm] = useState<'print' | 'test' | 'tare' | 'zero' | null>(null);
    const [error, setError] = useState('');
    const generation = useRef(0);
    const identity = useRef(devices?.identity);
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
      draftRef.current = next;
      setDraft(next);
      setConfirm(null);
      setError('');
    }
    function show(next: StationOperation) {
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
      if (devices && ['weigh', 'scan', 'devices'].includes(next))
        void run(next === 'devices' ? 'health' : next === 'scan' ? 'scan' : 'weigh', next);
    }
    useImperativeHandle(ref, () => ({
      show: (raw) => {
        const next = readOperation(raw);
        show(next);
        return {
          shown: next,
          mutationExecuted: false,
          reading: !!devices && ['weigh', 'scan', 'devices'].includes(next),
        };
      },
      prepare: (raw) => {
        update(raw);
        show('labels');
        return { shown: 'labels', executed: false, missing: labelIssues(draftRef.current) };
      },
    }));
    if (!active) return null;
    const result = results[operation];
    const fields = labelFields(draft);
    const shownFields = fields.slice(page * 6, page * 6 + 6);
    const issues = labelIssues(draft);
    const fieldFor = (key: string): FormField => ({
      label:
        key === 'copies'
          ? '枚数'
          : key === 'template'
            ? 'ラベルの種類'
            : (PLACEHOLDER_SCHEMA[key as keyof typeof PLACEHOLDER_SCHEMA]?.label ?? key),
      type:
        key === 'copies'
          ? 'number'
          : ['receivingDate', 'deadlineDate'].includes(key)
            ? 'date'
            : ['template', 'species', 'sex', 'storageTemperature'].includes(key)
              ? 'select'
              : 'text',
      required: true,
      order: 0,
      options:
        key === 'template'
          ? Object.entries(LABEL_TEMPLATES).map(([value, label]) => `${value}:${label}`)
          : key === 'species'
            ? ['シカ', 'クマ', 'イノシシ']
            : key === 'sex'
              ? ['オス', 'メス']
              : key === 'storageTemperature'
                ? ['-18℃以下', '4℃以下', '10℃以下']
                : undefined,
      selectValueLabel: key === 'template',
    });
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
      <div className={css.board} aria-label="現場の作業">
        <nav className={css.operationHeader}>
          <strong>{titles[operation]}</strong>
          <button type="button" disabled={busy} onClick={() => show('menu')}>
            作業を選ぶ
          </button>
          <button type="button" disabled={busy} onClick={onReceiving}>
            搬入入力へ
          </button>
        </nav>
        {operation === 'menu' ? (
          <div className={css.cards} data-count="5">
            {(['receiving', 'labels', 'weigh', 'scan', 'devices'] as const).map((key) => (
              <button className={css.card} key={key} type="button" onClick={() => show(key)}>
                <strong>{titles[key]}</strong>
              </button>
            ))}
          </div>
        ) : operation === 'labels' ? (
          <>
            <div className={css.operationToolbar}>
              <button type="button" disabled={busy || printLocked} onClick={() => edit('template')}>
                {LABEL_TEMPLATES[draft.template]}
              </button>
              <button type="button" disabled={busy || printLocked} onClick={() => edit('copies')}>
                {draft.copies}枚
              </button>
              <span>{devices?.printer ?? '機器未設定'}</span>
            </div>
            <div
              className={css.cards}
              data-count={shownFields.length}
              aria-label="ラベルの印刷内容"
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
                  <span className={css.label}>{PLACEHOLDER_SCHEMA[key].label}</span>
                  <strong>{draft.data[key] || '未入力'}</strong>
                </button>
              ))}
            </div>
            <div className={css.operationToolbar}>
              {fields.length > 6 && (
                <>
                  <button type="button" disabled={page === 0} onClick={() => setPage(page - 1)}>
                    前へ
                  </button>
                  <span>
                    {page + 1}/{Math.ceil(fields.length / 6)}
                  </span>
                  <button
                    type="button"
                    disabled={(page + 1) * 6 >= fields.length}
                    onClick={() => setPage(page + 1)}
                  >
                    次へ
                  </button>
                </>
              )}
              <button
                type="button"
                disabled={
                  !devices || busy || printLocked || !!devices.labelUnavailable || !!issues.length
                }
                onClick={() => requestConfirm('print')}
              >
                印刷内容を確認
              </button>
              <button type="button" disabled={busy} onClick={() => void native('work',{path:'/reports/labels'})}>業務のラベル画面を開く</button>
            </div>
            {!printLocked && (
              <p className={css.operationHint}>
                {devices?.labelUnavailable || issues.slice(0, 2).join('。')}
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
              {operation === 'scan' &&
                result?.scanValue &&
                /^[A-Za-z0-9-]{1,32}$/.test(result.scanValue) && (
                  <button
                    type="button"
                    disabled={busy || printLocked}
                    onClick={() => {
                      update({ template: 'carcass', data: { individualNumber: result.scanValue } });
                      show('labels');
                    }}
                  >
                    個体番号としてラベルへ
                  </button>
                )}
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
              <p>
                {confirm === 'print'
                  ? Object.entries(draft.data)
                      .filter(([key]) => fields.includes(key as (typeof fields)[number]))
                      .map(([key, value]) => `${fieldFor(key).label}：${value}`)
                      .join(' ／ ')
                  : confirm === 'test'
                    ? '画面の業務ラベルではなく、プリンタの接続試験です。'
                    : '計量器に載っている物を確認してください。'}
              </p>
              <p>{devices?.printer}</p>
              <button
                type="button"
                disabled={busy}
                className={css.primary}
                onClick={() => void run(confirm)}
              >
                この内容で実行する
              </button>
              <button type="button" onClick={() => setConfirm(null)}>
                戻る
              </button>
            </div>
          </div>
        )}
        {editing && (
          <div className={css.overlay}>
            <StationFieldEditor
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
                update(
                  editing === 'template'
                    ? { template: value }
                    : editing === 'copies'
                      ? { copies: Number(value) }
                      : { data: { [editing]: value } },
                );
                setEditing(null);
                setPage(0);
              }}
            />
          </div>
        )}
      </div>
    );
  },
);
