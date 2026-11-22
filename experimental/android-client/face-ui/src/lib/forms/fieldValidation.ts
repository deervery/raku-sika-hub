import type { FormField } from '@/types/forms';

/**
 * `digits` 指定フィールド（半角数字 N 桁）の入力値ユーティリティ。
 *
 * 入力中は数字以外を除去して N 桁で打ち切り、送信時に桁数が揃っているかを検証する。
 * 未入力は任意項目として valid 扱い（必須判定は `required` が担う）。
 */

/** 入力途中の値を N 桁の半角数字に整える。 */
export function sanitizeDigitsInput(value: string, digits: number): string {
  return value.replace(/[^0-9]/g, '').slice(0, digits);
}

/**
 * 送信時の桁数検証。問題なければ null、不正ならエラーメッセージを返す。
 * `digits` 未指定のフィールド・空値は常に null。
 */
export function digitsFieldError(field: FormField, value: unknown): string | null {
  const digits = field.digits;
  if (!digits || digits <= 0) return null;
  if (value == null || value === '') return null;
  const str = String(value);
  if (new RegExp(`^\\d{${digits}}$`).test(str)) return null;
  return `${field.label} は半角数字${digits}桁で入力してください。`;
}

/**
 * `clearWhenHidden` のフィールドを、非表示だからといって実際に空へ倒してよいか。
 *
 * クリアしてよいのは、この編集セッションで当該フィールド自身か、その表示条件に
 * 使われているフィールドを実際に触った場合だけ。無条件にクリアすると、条件
 * フィールドが未設定のまま値だけを持つ既存レコード（www からの移行データなど）を、
 * 無関係な項目を直して保存しただけで消してしまう。
 *
 * @param key    対象フィールドのキー
 * @param field  対象フィールドの定義
 * @param touched このセッションで触られたフィールドキーの集合
 */
export function canClearHiddenField(
  key: string,
  field: FormField,
  touched: ReadonlySet<string>,
): boolean {
  if (touched.has(key)) return true;
  const condition = field.condition;
  if (!condition) return false;
  const conditions = Array.isArray(condition) ? condition : [condition];
  return conditions.some((c) => touched.has(c.field));
}
