import type { FormDefinition } from '@/types/forms';
import { matchesFieldCondition } from '@/lib/forms/formRuntime';

/** Business validation still runs in DynamicForm and the existing receiving API. */
export const RECEIVING_INPUT_KEYS = [
  'species',
  'sex',
  'weight',
  'individualNumber',
  'hunterName',
  'captureLocation',
  'captureType',
  'estimatedAge',
] as const;
export type ReceivingInputKey = (typeof RECEIVING_INPUT_KEYS)[number];
export type InputPatch = Partial<Record<ReceivingInputKey, string>>;
export type ChoiceOverrides = Record<string, Array<{ value: string; label: string }>>;

export function validateInputPatch(
  raw: unknown,
  form: FormDefinition,
  values: Record<string, unknown>,
  overrides: ChoiceOverrides = {},
): InputPatch {
  if (!raw || typeof raw !== 'object' || Array.isArray(raw))
    throw new Error('入力内容を確認できません。言い換えてください。');
  const patch: InputPatch = {};
  for (const [key, value] of Object.entries(raw)) {
    if (
      !RECEIVING_INPUT_KEYS.includes(key as ReceivingInputKey) ||
      typeof value !== 'string' ||
      value.length > 100
    )
      throw new Error('対応していない項目が含まれています。フォームで確認してください。');
    const field = form.fields[key];
    if (!field || !matchesFieldCondition(field, { ...values, ...raw }))
      throw new Error('今の作業では入力できない項目です。');
    if (value === '') throw new Error('消去は確認フォームで行ってください。');
    if (
      ['weight', 'estimatedAge'].includes(key) &&
      (!/^\d+(?:\.\d{1,3})?$/.test(value) || !Number.isFinite(Number(value)))
    )
      throw new Error('数値を確認してください。');
    if (key === 'individualNumber' && !/^\d{1,32}$/.test(value))
      throw new Error('個体番号は数字で入力してください。');
    if (['select', 'radio'].includes(field.type)) {
      const choices = overrides[key]?.map((o) => o.value) ?? field.options ?? [];
      if (!choices.includes(value)) throw new Error(`${field.label}を選択肢から選んでください。`);
    }
    patch[key as ReceivingInputKey] = value;
  }
  if (!Object.keys(patch).length)
    throw new Error('入力内容を特定できません。項目と値をもう少し教えてください。');
  return patch;
}

/** Offline path accepts only fully understood clauses; unsupported wording is never silently discarded. */
export function parseReceivingText(
  text: string,
  form: FormDefinition,
  values: Record<string, unknown>,
  overrides: ChoiceOverrides = {},
): InputPatch {
  const source = text
    .normalize('NFKC')
    .trim()
    .replace(/^訂正[、,\s]*/, '')
    .replace(/[。.!！]+$/, '');
  if (/じゃない|ではない|違う|ないです|取り消|削除|登録|印刷/.test(source))
    throw new Error('否定や操作の対象を確認したいので、フォームで修正してください。');
  const patch: InputPatch = {};
  const put = (key: ReceivingInputKey, value: string) => {
    if (key in patch && patch[key] !== value)
      throw new Error('同じ項目に複数の値があります。正しい値を教えてください。');
    patch[key] = value;
  };
  for (let clause of source.split(/[、,。\n]+/).filter(Boolean)) {
    clause = clause.trim().replace(/です$/, '').trim();
    if (/^(オス|雄|おす|メス|雌|めす)$/.test(clause)) {
      put('sex', /オス|雄|おす/.test(clause) ? 'オス' : 'メス');
      continue;
    }
    const weight =
      /^(?:体重|重量|推定体重)?\s*(?:は)?\s*(\d+(?:\.\d{1,3})?)\s*(キログラム|キロ|kg|グラム|g)$/i.exec(
        clause,
      );
    if (weight) {
      put('weight', String(Number(weight[1]) / (/^(g|グラム)$/i.test(weight[2]) ? 1000 : 1)));
      continue;
    }
    const id = /^(?:個体(?:番号|識別番号)?|番号)\s*(?:は)?\s*(\d{1,32})$/.exec(clause);
    if (id) {
      put('individualNumber', id[1]);
      continue;
    }
    const age = /^(?:年齢|推定年齢)?\s*(?:は)?\s*(\d+(?:\.\d+)?)歳$/.exec(clause);
    if (age) {
      put('estimatedAge', age[1]);
      continue;
    }
    let found = false;
    for (const key of ['species', 'captureType', 'hunterName', 'captureLocation'] as const) {
      const field = form.fields[key];
      if (!field) continue;
      const label =
        key === 'hunterName'
          ? '(?:ハンター|捕獲者|捕獲者の名前)'
          : key === 'captureLocation'
            ? '(?:場所|捕獲場所|エリア)'
            : field.label;
      const candidate = clause.replace(new RegExp(`^${label}\\s*(?:は)?\\s*`), '');
      const choices =
        overrides[key] ?? field.options?.map((value) => ({ value, label: value })) ?? [];
      const matches = choices.filter(
        (choice) => choice.label === candidate || choice.value === candidate,
      );
      if (matches.length === 1) {
        if (found) throw new Error('対象が曖昧です。項目名を添えてください。');
        put(key, matches[0].value);
        found = true;
      }
    }
    if (!found)
      throw new Error(
        `「${clause}」を特定できませんでした。項目ごとに区切るか、フォームで入力してください。`,
      );
  }
  return validateInputPatch(patch, form, values, overrides);
}
