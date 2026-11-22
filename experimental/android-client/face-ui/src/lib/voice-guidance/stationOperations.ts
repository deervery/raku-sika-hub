import { type TemplateKey } from '@/lib/labels/constants';
import {
  PLACEHOLDER_SCHEMA,
  getTemplatePlaceholders,
  type PlaceholderName,
} from '@/lib/labels/placeholderSchema';
import {validDate,validQuantity} from './labelEditor';
import { validateLabelData } from '@/lib/labels/validateLabelData';

export const STATION_OPERATIONS = [
  'menu',
  'receiving',
  'labels',
  'weigh',
  'scan',
  'devices',
] as const;
export type StationOperation = (typeof STATION_OPERATIONS)[number];
export const LABEL_TEMPLATES: Record<TemplateKey, string> = {
  carcass: '個体ラベル',
  traceable: '精肉・個体番号あり',
  non_traceable: '精肉',
  processed: '加工品',
  pet: 'ペット用',
};
// Only printable operator fields. Facility identity, URLs, paths and device commands are never model inputs.
export const LABEL_INPUT_KEYS = [
  'productName',
  'productQuantity',
  'deadlineDate',
  'deadlineLabel',
  'storageTemperature',
  'individualNumber',
  'captureLocation',
  'species',
  'sex',
  'receivingDate',
  'productIngredient',
  'nutritionUnit',
  'caloriesQuantity',
  'proteinQuantity',
  'fatQuantity',
  'carbohydratesQuantity',
  'saltEquivalentQuantity',
  'attentionText',
  'isHeatedMeatProducts',
] as const;
export type LabelDraft = { template: TemplateKey; copies: number; data: Record<string, string> };
export const emptyLabelDraft = (): LabelDraft => ({ template: 'carcass', copies: 1, data: {} });
export type DeviceResult = {
  state: 'done' | 'pending' | 'unknown' | 'error';
  message: string;
  cards?: { label: string; value: string }[];
  weightKg?: number;
  scanValue?: string;
};
export type StationDevices = {
  identity: string;
  draftStorageKey?: string;
  printer: string;
  labelUnavailable?: string;
  printLabel: (draft: LabelDraft) => Promise<DeviceResult>;
  testPrint: () => Promise<DeviceResult>;
  weigh: () => Promise<DeviceResult>;
  scan: () => Promise<DeviceResult>;
  health: () => Promise<DeviceResult>;
  tare: () => Promise<DeviceResult>;
  zero: () => Promise<DeviceResult>;
};
export function readOperation(raw: unknown): StationOperation {
  if (
    !raw ||
    typeof raw !== 'object' ||
    Array.isArray(raw) ||
    Object.keys(raw).some((key) => key !== 'operation')
  )
    throw new Error('作業を選んでください。');
  const operation = (raw as { operation?: unknown }).operation;
  if (!STATION_OPERATIONS.includes(operation as StationOperation))
    throw new Error('対応していない作業です。');
  return operation as StationOperation;
}
export function mergeLabelDraft(previous: LabelDraft, raw: unknown): LabelDraft {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw))
    throw new Error('ラベル内容を確認してください。');
  const patch = raw as Record<string, unknown>;
  if (Object.keys(patch).some((key) => !['template', 'copies', 'data'].includes(key)))
    throw new Error('未対応のラベル項目です。');
  const template = patch.template ?? previous.template;
  if (typeof template !== 'string' || !Object.hasOwn(LABEL_TEMPLATES, template))
    throw new Error('ラベルの種類を選んでください。');
  const copies = patch.copies ?? previous.copies;
  if (typeof copies !== 'number' || !Number.isInteger(copies) || copies < 1 || copies > 30)
    throw new Error('枚数は1〜30枚で指定してください。');
  if (patch.data != null && (typeof patch.data !== 'object' || Array.isArray(patch.data)))
    throw new Error('ラベル内容を確認してください。');
  const data = { ...previous.data };
  for (const [key, value] of Object.entries(patch.data ?? {})) {
    if (
      !LABEL_INPUT_KEYS.includes(key as (typeof LABEL_INPUT_KEYS)[number]) ||
      typeof value !== 'string' ||
      value.length > 500
    )
      throw new Error('ラベルに使える項目を確認してください。');
    const clean=value.trim();
    const metadata=PLACEHOLDER_SCHEMA[key as PlaceholderName];
    if(metadata.charLimit>0&&clean.length>metadata.charLimit)throw new Error(`${metadata.label}は${metadata.charLimit}文字以内で入力してください。`);
    if(clean&&['receivingDate','deadlineDate'].includes(key)&&!validDate(clean))throw new Error(`${metadata.label}は実在する年月日を選んでください。`);
    if(clean&&key==='productQuantity'&&!validQuantity(clean))throw new Error('内容量は正の数値と単位を選んでください（個・包・袋は整数）。');
    if(clean&&key==='individualNumber'&&!/^[A-Za-z0-9-]{1,15}$/.test(clean))throw new Error('個体番号は15文字以内の英数字・ハイフンで入力してください。');
    data[key] = clean;
    if(key==='storageTemperature'){
      if(clean==='-18℃以下')data.deadlineLabel='賞味期限';
      else if(['4℃以下','10℃以下'].includes(clean))data.deadlineLabel='消費期限';
    }
  }
  return { template: template as TemplateKey, copies, data };
}
export function labelFields(draft: LabelDraft) {
  const fields = [...getTemplatePlaceholders(draft.template)];
  // Existing Lite requires receivingDate as traceable input even though it is not printed.
  if(draft.template==='traceable')fields.push('receivingDate');
  return fields.filter((key) =>
    LABEL_INPUT_KEYS.includes(key as (typeof LABEL_INPUT_KEYS)[number]),
  );
}
export function labelIssues(draft: LabelDraft) {
  const errors = validateLabelData(draft.template, draft.data, draft.copies)
    .errors.filter((error) =>
      LABEL_INPUT_KEYS.includes(error.field as (typeof LABEL_INPUT_KEYS)[number]),
    )
    .map(
      (error) =>
        `${PLACEHOLDER_SCHEMA[error.field as PlaceholderName]?.label ?? error.field}${error.type === 'too_long' ? 'が長すぎます' : 'を入力してください'}`,
    );
  if (!Number.isInteger(draft.copies) || draft.copies < 1 || draft.copies > 30)
    errors.push('枚数は1〜30枚で指定してください');
  for(const key of labelFields(draft)){
    const value=draft.data[key]||'';
    if(['receivingDate','deadlineDate'].includes(key)&&value&&!validDate(value))errors.push(`${PLACEHOLDER_SCHEMA[key].label}は実在する年月日を選んでください`);
    if(key==='productQuantity'&&value&&!validQuantity(value))errors.push('内容量は正の数値と単位を選んでください（個・包・袋は整数）');
    if(key==='individualNumber'&&value&&!/^[A-Za-z0-9-]{1,15}$/.test(value))errors.push('個体番号は15文字以内の英数字・ハイフンで入力してください');
    if(key==='species'&&value&&!['シカ','クマ','イノシシ','アライグマ'].includes(value))errors.push('獣種を選び直してください');
    if(key==='sex'&&value&&!['オス','メス'].includes(value))errors.push('性別を選び直してください');
    if(key==='deadlineLabel'&&value&&!['賞味期限','消費期限'].includes(value))errors.push('期限区分を選び直してください');
  }
  if(draft.template==='traceable'&&!draft.data.receivingDate)errors.push('搬入日を入力してください');
  return errors;
}
/** An HTTP acknowledgement does not prove that a physical label came out. */
export function printResult(body: {
  printState?: string;
  jobId?: string;
  message?: string;
  diagnosis?: { blocking?: boolean };
}): DeviceResult {
  const state = body.diagnosis?.blocking
    ? 'unknown'
    : body.printState === 'done'
      ? 'done'
      : 'pending';
  return {
    state,
    message:
      state === 'done'
        ? '機器から印刷完了が返りました'
        : state === 'pending'
          ? '送信済み・出力確認待ち'
          : '出力を確認できません。プリンタを確認してください',
    cards: body.jobId ? [{ label: '印刷ジョブ', value: String(body.jobId) }] : [],
  };
}

export async function hubRequest(base: string, path: string, method = 'GET') {
  const response = await fetch(`${base.replace(/\/$/, '')}${path}`, {
    method,
    signal: AbortSignal.timeout(8000),
  });
  if (!response.ok) throw new Error('機器との通信に失敗しました。接続を確認してください。');
  return response.json();
}
export function createHubDevices(
  base: string,
): Pick<StationDevices, 'testPrint' | 'weigh' | 'scan' | 'health' | 'tare' | 'zero'> {
  const send = async (path: string): Promise<DeviceResult> => {
    try {
      const body = await hubRequest(base, path, 'POST');
      if (body.status === 'error')
        return { state: 'error', message: '機器が操作を受け付けませんでした' };
      return path === '/printer/test'
        ? printResult(body)
        : { state: 'done', message: '計量器が操作を受け付けました' };
    } catch {
      return {
        state: 'unknown',
        message: '送信結果を確認できません。機器を確認してから次の操作に進んでください',
      };
    }
  };
  return {
    testPrint: () => send('/printer/test'),
    tare: () => send('/scale/tare'),
    zero: () => send('/scale/zero'),
    weigh: async () => {
      const body = await hubRequest(base, '/scale/weigh', 'POST');
      if (
        body.status !== 'ok' ||
        body.stable !== true ||
        typeof body.value !== 'number' ||
        !Number.isFinite(body.value) ||
        !['kg', 'g'].includes(body.unit)
      )
        return {
          state: 'pending',
          message: '安定した重量を取得できません。載せた物を確認して再計量してください',
        };
      const weightKg = body.unit === 'g' ? body.value / 1000 : body.value;
      return {
        state: 'done',
        message: '安定した重量を取得しました',
        weightKg,
        cards: [{ label: '重量', value: `${weightKg} kg` }],
      };
    },
    scan: async () => {
      // First read establishes a baseline; never reuse the last item as a new scan.
      const baseline = await hubRequest(base, '/scanner/scan');
      const started = Date.now();
      while (Date.now() - started < 20000) {
        await new Promise((resolve) => setTimeout(resolve, 500));
        const body = await hubRequest(base, '/scanner/scan');
        if (
          body.status === 'ok' &&
          typeof body.value === 'string' &&
          body.value &&
          body.scannedAt &&
          body.scannedAt !== baseline.scannedAt
        )
          return {
            state: 'done',
            message: '読み取りました',
            scanValue: body.value.slice(0, 1000),
            cards: [
              {
                label: '読み取り内容',
                value: body.value.length > 140 ? `${body.value.slice(0, 140)}…` : body.value,
              },
            ],
          };
      }
      return {
        state: 'pending',
        message: '新しい読み取りがありません。スキャン後、もう一度試してください',
      };
    },
    health: async () => {
      const body = await hubRequest(base, '/health');
      const status = (value: unknown) =>
        value === true ? '接続中' : value === false ? '未接続' : '未確認';
      return {
        state: 'done',
        message: '現在の機器状態',
        cards: [
          {
            label: 'プリンタ',
            value:
              body.printer?.backendReady === false
                ? '出力先に接続できません'
                : status(body.printer?.connected),
          },
          { label: '計量器', value: status(body.scale?.connected) },
          { label: 'スキャナ', value: status(body.scanner?.connected) },
        ],
      };
    },
  };
}
