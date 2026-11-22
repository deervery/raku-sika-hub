'use client';

import type { FormDefinition, FormField } from '@/types/forms';
import { isFormInput, matchesFieldCondition } from '@/lib/forms/formRuntime';
import type { ChoiceOverrides } from '@/lib/voice-guidance/receivingInput';
import styles from './Station.module.css';

type Entry = { key: string; field: FormField; section: string };
export function receivingReviewPages(form: FormDefinition, values: Record<string, unknown>) {
  let section = 'その他';
  const entries: Entry[] = [];
  for (const [key, field] of Object.entries(form.fields).sort((a, b) => a[1].order - b[1].order)) {
    if (!matchesFieldCondition(field, values)) continue;
    if (field.type === 'h2title') section = field.label;
    if (isFormInput(field)) entries.push({ key, field, section });
  }
  const overviewKeys = [
    'species',
    'sex',
    'weight',
    'hunterName',
    'captureLocation',
    'individualNumber',
  ];
  const overview = overviewKeys.flatMap((key) => entries.filter((entry) => entry.key === key));
  const pages: { title: string; entries: Entry[] }[] = overview.length
    ? [{ title: '搬入の概要', entries: overview }]
    : [];
  const included = new Set(overview.map((entry) => entry.key));
  for (const entry of entries.filter((item) => !included.has(item.key))) {
    const last = pages[pages.length - 1];
    if (last && last.title === entry.section) last.entries.push(entry);
    else pages.push({ title: entry.section, entries: [entry] });
  }
  return pages.length ? pages : [{ title: '搬入の内容', entries: [] }];
}

export function receivingReviewValue(
  field: FormField,
  value: unknown,
  choices?: ChoiceOverrides[string],
) {
  if (value == null || value === '' || (Array.isArray(value) && !value.length)) return '未入力';
  if (field.type === 'file') return Array.isArray(value) ? `添付 ${value.length} 件` : '添付あり';
  if (typeof value === 'boolean') return value ? 'はい' : 'いいえ';
  const label = (item: unknown) => {
    const raw = String(item);
    const option = choices?.find((choice) => choice.value === raw);
    if (option) return option.label;
    if (field.selectValueLabel) {
      const pair = field.options?.find((choice) => choice.slice(0, choice.indexOf(':')) === raw);
      if (pair) return pair.slice(pair.indexOf(':') + 1);
    }
    return raw;
  };
  return Array.isArray(value) ? value.map(label).join('・') : label(value);
}

export function ReceivingReview({
  form,
  values,
  options,
  onBack,
  onEdit,
}: {
  form: FormDefinition;
  values: Record<string, unknown>;
  options: ChoiceOverrides;
  onBack: () => void;
  onEdit: () => void;
}) {
  const pages = receivingReviewPages(form, values);
  const entries = pages.flatMap((page) => page.entries);
  return (
    <div className={styles.reviewScreen} aria-label="フォーム内容の確認">
      <header className={styles.reviewHeader}>
        <span className={styles.reviewCaption}>フォーム内容の確認 · 登録前</span>
        <h2>搬入の内容</h2>
        <p className={styles.screenReader} role="status">
          {entries.length} 項目を確認
        </p>
      </header>
      <div className={styles.fullReviewCards} aria-label="下書きの内容">
        {entries.map(({ key, field }, position) => {
          const value = receivingReviewValue(field, values[key], options[key]);
          const warning = field.warnValues?.includes(String(values[key]));
          return (
            <article
              key={key}
              className={styles.reviewCard}
              style={{ animationDelay: `${Math.min(position * 55, 350)}ms` }}
              data-missing={value === '未入力'}
              data-warning={!!warning}
            >
              <p className={styles.reviewLabel}>
                {field.label}
                {field.required && <span className={styles.required}>必須</span>}
              </p>
              <p className={styles.reviewValue}>
                {value !== '未入力' && field.prefix && (
                  <span className={styles.unit}>{field.prefix}</span>
                )}
                {value}
                {value !== '未入力' && field.suffix && (
                  <span className={styles.unit}>{field.suffix}</span>
                )}
              </p>
              {warning && <span className={styles.warning}>要確認</span>}
            </article>
          );
        })}
        {!entries.length && <p>確認する項目はありません。</p>}
      </div>
      <footer className={styles.reviewFooter}>
        <div className={styles.confirmActions}>
          <button type="button" className={styles.secondaryButton} onClick={onBack}>
            会話に戻る
          </button>
          <button type="button" className={styles.primaryButton} onClick={onEdit}>
            フォームで確認・修正する →
          </button>
        </div>
      </footer>
    </div>
  );
}
