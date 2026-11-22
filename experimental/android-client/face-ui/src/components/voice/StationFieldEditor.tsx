'use client';
import { useState } from 'react';
import type { FormField } from '@/types/forms';
import { fieldChoices, rankChoices } from '@/lib/voice-guidance/stationUi';
import type { ChoiceOverrides } from '@/lib/voice-guidance/receivingInput';
import css from './VoiceWorkspace.module.css';

export function StationFieldEditor({
  fieldKey,
  field,
  value,
  options,
  query = '',
  onSave,
  onClose,
  onAdd,
}: {
  fieldKey: string;
  field: FormField;
  value: unknown;
  options?: ChoiceOverrides[string];
  query?: string;
  onSave: (value: unknown) => void;
  onClose: () => void;
  onAdd?: (name: string) => Promise<void>;
}) {
  const [draft, setDraft] = useState(Array.isArray(value) ? value : String(value ?? ''));
  const [name, setName] = useState(query);
  const [page, setPage] = useState(0);
  const [adding, setAdding] = useState(false);
  const [error, setError] = useState('');
  const [textMode,setTextMode]=useState(fieldKey==='individualNumber'&&/[A-Za-z-]/.test(String(value??'')));
  const choices = rankChoices(name, fieldChoices(field, options));
  const numeric = !textMode&&(field.type === 'number' || fieldKey === 'individualNumber' || !!field.digits);
  const selected = Array.isArray(draft) ? draft : [];
  const save = (next: unknown) => {
    try {
      onSave(next);
    } catch (e) {
      setError(e instanceof Error ? e.message : '入力を確認してください');
    }
  };
  return (
    <div className={css.editor} role="dialog" aria-modal="true" aria-label={`${field.label}を編集`}>
      <header>
        <h2>{field.label}</h2>
        <button type="button" onClick={onClose} aria-label="編集を閉じる">
          ×
        </button>
      </header>
      {fieldKey==='individualNumber'&&<button type="button" onClick={()=>setTextMode(!textMode)}>{textMode?'数字ボタンで入力':'英字・ハイフンを入力'}</button>}
      {['nutritionUnit','storageTemperature'].includes(fieldKey)&&<button type="button" onClick={()=>setTextMode(!textMode)}>{textMode?'選択肢から選ぶ':'その他の表記を入力'}</button>}
      {fieldKey === 'hunterName' && (
        <label className={css.nameQuery}>
          名前
          <input
            aria-label="捕獲者の名前"
            value={name}
            maxLength={100}
            onChange={(e) => {
              setName(e.target.value);
              setPage(0);
            }}
          />
        </label>
      )}
      {choices.length > 0 && !textMode ? (
        <>
          {name && <p>近い候補</p>}
          <div className={css.choices}>
            {choices.slice(page * 6, page * 6 + 6).map((option) => (
              <button
                type="button"
                key={option.value}
                aria-pressed={
                  field.type === 'checkbox'
                    ? selected.includes(option.value)
                    : String(value) === option.value
                }
                onClick={() =>
                  field.type === 'checkbox'
                    ? setDraft(
                        selected.includes(option.value)
                          ? selected.filter((v) => v !== option.value)
                          : [...selected, option.value],
                      )
                    : save(option.value)
                }
              >
                {option.label}
              </button>
            ))}
          </div>
          {choices.length > 6 && (
            <nav>
              <button type="button" disabled={page === 0} onClick={() => setPage(page - 1)}>
                前へ
              </button>
              <span>
                {page + 1} / {Math.ceil(choices.length / 6)}
              </span>
              <button
                type="button"
                disabled={(page + 1) * 6 >= choices.length}
                onClick={() => setPage(page + 1)}
              >
                次へ
              </button>
            </nav>
          )}
          {field.type === 'checkbox' && (
            <button type="button" className={css.primary} onClick={() => save(selected)}>
              この内容にする
            </button>
          )}
        </>
      ) : field.type === 'checkbox' ? (
        <div className={css.choices}>
          <button type="button" onClick={() => save(true)}>
            はい
          </button>
          <button type="button" onClick={() => save(false)}>
            いいえ
          </button>
        </div>
      ) : numeric ? (
        <>
          <output className={css.number}>
            {String(draft) || '—'}
            <small>{field.suffix}</small>
          </output>
          <div className={css.keypad}>
            {[
              '1',
              '2',
              '3',
              '4',
              '5',
              '6',
              '7',
              '8',
              '9',
              field.type === 'number' ? '.' : '00',
              '0',
              '⌫',
            ].map((key) => (
              <button
                type="button"
                key={key}
                onClick={() =>
                  setDraft((previous) =>
                    key === '⌫'
                      ? String(previous).slice(0, -1)
                      : key === '.' && String(previous).includes('.')
                        ? previous
                        : (String(previous) + key).slice(0, field.digits || 32),
                  )
                }
              >
                {key}
              </button>
            ))}
          </div>
          <button type="button" className={css.primary} onClick={() => save(String(draft))}>
            この値にする
          </button>
        </>
      ) : (
        (textMode || !['select', 'radio'].includes(field.type)) && (
          <>
            <input
              aria-label={field.label}
              type={
                ['date', 'datetime-local', 'time', 'month', 'week'].includes(field.type)
                  ? field.type
                  : 'text'
              }
              value={String(draft)}
              maxLength={1000}
              onChange={(e) => setDraft(e.target.value)}
            />
            <button type="button" className={css.primary} onClick={() => save(String(draft))}>
              この内容にする
            </button>
          </>
        )
      )}
      {!choices.length && ['select', 'radio'].includes(field.type) && (
        <p>一致する候補がありません。</p>
      )}
      {onAdd && name.trim() && !choices.some((choice) => choice.label === name.trim()) && (
        <div className={css.addPerson}>
          <p>「{name.trim()}」さんを新しく追加しますか？</p>
          <button
            type="button"
            disabled={adding}
            onClick={async () => {
              setAdding(true);
              try {
                await onAdd(name.trim());
              } catch (e) {
                setError(e instanceof Error ? e.message : '追加できませんでした');
              } finally {
                setAdding(false);
              }
            }}
          >
            {adding ? '追加中…' : '今回の入力に追加する'}
          </button>
        </div>
      )}
      {error && <p role="alert">{error}</p>}
    </div>
  );
}
