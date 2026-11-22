import type { FormDefinition, FormField } from '@/types/forms';
import { isFormInput, matchesFieldCondition } from '@/lib/forms/formRuntime';
import { RECEIVING_INPUT_KEYS, type ChoiceOverrides } from './receivingInput';

export type ReviewMode = 'filled' | 'missing' | 'all';
export const missingValue = (value: unknown) =>
  value == null || value === '' || (Array.isArray(value) && value.length === 0);
export function fieldChoices(field: FormField, overrides?: ChoiceOverrides[string]) {
  return (
    overrides ??
    (field.options ?? []).map((value) => {
      const split = field.selectValueLabel ? value.indexOf(':') : -1;
      return split < 0
        ? { value, label: value }
        : { value: value.slice(0, split), label: value.slice(split + 1) };
    })
  );
}
export function reviewEntries(
  form: FormDefinition,
  values: Record<string, unknown>,
  mode: ReviewMode,
  keys?: string[],
) {
  return Object.entries(form.fields)
    .filter(
      ([key, field]) =>
        isFormInput(field) &&
        field.display !== false &&
        matchesFieldCondition(field, values) &&
        (!keys?.length || keys.includes(key)),
    )
    .filter(
      ([key]) =>
        mode === 'all' ||
        (mode === 'missing' ? missingValue(values[key]) : !missingValue(values[key])),
    )
    .sort(([a, fa], [b, fb]) => {
      const primary = (key: string) =>
        RECEIVING_INPUT_KEYS.includes(key as (typeof RECEIVING_INPUT_KEYS)[number]) ? 0 : 1;
      return (
        primary(a) - primary(b) || Number(fb.required) - Number(fa.required) || fa.order - fb.order
      );
    });
}
const normalize = (value: string) =>
  value
    .normalize('NFKC')
    .toLocaleLowerCase('ja')
    .replace(/[\s　・]/g, '')
    .replace(/[ァ-ヶ]/g, (char) => String.fromCharCode(char.charCodeAt(0) - 0x60));
function distance(a: string, b: string) {
  let row = Array.from({ length: b.length + 1 }, (_, i) => i);
  for (let i = 0; i < a.length; i++) {
    const next = [i + 1];
    for (let j = 0; j < b.length; j++)
      next.push(Math.min(next[j] + 1, row[j + 1] + 1, row[j] + Number(a[i] !== b[j])));
    row = next;
  }
  return row[b.length];
}
export function rankChoices(query: string, choices: ChoiceOverrides[string]) {
  const q = normalize(query).slice(0, 100);
  // TODO: 将来JEVなどによる候補検索・読み仮名照合をここへ接続できるよう、順位付けをUIから分離する。
  // 類似度は表示順だけに使う。人物の自動同定・自動選択はしない。
  return choices
    .map((choice, index) => {
      const name = normalize(choice.label).slice(0, 100);
      const score = !q
        ? 0
        : name === q
          ? 1000
          : name.includes(q) || q.includes(name)
            ? 500 - Math.abs(name.length - q.length)
            : 100 - distance(q, name);
      return { ...choice, score, index };
    })
    .sort((a, b) => b.score - a.score || a.index - b.index);
}
/** Manual taps may edit active form fields, but cannot bypass the final form validation. */
export function validateManualPatch(
  raw: Record<string, unknown>,
  form: FormDefinition,
  values: Record<string, unknown>,
  options: ChoiceOverrides = {},
) {
  const result: Record<string, unknown> = {};
  for (const [key, value] of Object.entries(raw)) {
    const field = form.fields[key];
    if (
      !field ||
      !isFormInput(field) ||
      field.display === false ||
      field.type === 'file' ||
      !matchesFieldCondition(field, { ...values, ...raw })
    )
      throw new Error('この項目は今は変更できません。');
    const choices = fieldChoices(field, options[key]);
    if (field.type === 'checkbox') {
      if (
        choices.length
          ? !Array.isArray(value) ||
            value.some((item) => !choices.some((option) => option.value === item))
          : typeof value !== 'boolean'
      )
        throw new Error('選択内容を確認してください。');
    } else {
      if (typeof value !== 'string' && typeof value !== 'number')
        throw new Error('入力内容を確認してください。');
      if (String(value).length > 1000) throw new Error('入力が長すぎます。');
      if (
        value !== '' &&
        ['select', 'radio'].includes(field.type) &&
        !choices.some((option) => option.value === value)
      )
        throw new Error('選択肢から選んでください。');
      if (value !== '' && field.type === 'number' && !Number.isFinite(Number(value)))
        throw new Error('数値を確認してください。');
      if (value !== '' && field.digits && !new RegExp(`^\\d{${field.digits}}$`).test(String(value)))
        throw new Error(`${field.digits}桁で入力してください。`);
    }
    result[key] = value;
  }
  return result;
}
