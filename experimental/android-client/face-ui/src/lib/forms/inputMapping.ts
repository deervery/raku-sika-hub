import type { FormField } from '@/types/forms';

export type FieldControlKind =
  | 'heading'
  | 'textarea'
  | 'select'
  | 'radio'
  | 'checkbox-single'
  | 'checkbox-multi'
  | 'file'
  | 'input'
  | 'week-picker';

export type FieldControlMeta = {
  kind: FieldControlKind;
  htmlInputType?: React.InputHTMLAttributes<HTMLInputElement>['type']; // when kind === "input"
};

export function getFieldControl(field: Pick<FormField, 'type' | 'options'>): FieldControlMeta {
  switch (field.type) {
    case 'h2title':
    case 'h3title':
      return { kind: 'heading' };
    case 'textarea':
      return { kind: 'textarea' };
    case 'select':
      return { kind: 'select' };
    case 'radio':
      return { kind: 'radio' };
    case 'checkbox': {
      const hasOptions = Array.isArray(field.options) && field.options.length > 0;
      return { kind: hasOptions ? 'checkbox-multi' : 'checkbox-single' };
    }
    case 'file':
      return { kind: 'file' };
    case 'week':
      return { kind: 'week-picker' };
    case 'text':
    case 'number':
    case 'date':
    case 'time':
    case 'datetime-local':
    case 'month':
    case 'url':
    case 'color':
    case 'tel':
      return { kind: 'input', htmlInputType: field.type };
    default:
      return { kind: 'input', htmlInputType: 'text' };
  }
}

export function getDefaultInitialValue(field: FormField): unknown {
  const meta = getFieldControl(field);
  if (meta.kind === 'heading') return undefined;
  if (field.defaultValue !== undefined) {
    return coerceValueForKind(meta.kind, field.defaultValue, field.options);
  }
  return getEmptyInitialValue(field);
}

/**
 * 編集モードで「保存されていないキー」に使う空値。
 * defaultValue を一切参照せず、フィールド種別から空相当の値を返す。
 *
 * 用途: 既存レコードを編集中、フォーム定義に新規追加されたフィールドが値を持たないとき、
 * default 値で勝手に埋めず空のままにする（ユーザーに「未入力」を明示する）。
 */
export function getEmptyInitialValue(field: FormField): unknown {
  const meta = getFieldControl(field);
  if (meta.kind === 'heading') return undefined;
  switch (meta.kind) {
    case 'checkbox-single':
      return false;
    case 'checkbox-multi':
      return [] as string[];
    case 'radio':
    case 'select':
      return '';
    case 'file':
      return undefined;
    case 'week-picker':
    case 'input':
    case 'textarea':
    default:
      return '';
  }
}

export function coerceValueForKind(
  kind: FieldControlKind,
  value: unknown,
  options?: string[],
): string | number | boolean | string[] | undefined {
  switch (kind) {
    case 'checkbox-single':
      return Boolean(value);
    case 'checkbox-multi': {
      if (Array.isArray(value)) return value.map((v) => String(v));
      if (typeof value === 'string') {
        const set = new Set((options || []).map((o) => String(o)));
        return value
          .split(',')
          .map((s) => s.trim())
          .filter((s) => s.length > 0 && (set.size === 0 || set.has(s)));
      }
      return [];
    }
    case 'radio':
    case 'select':
    case 'week-picker':
    case 'input':
    case 'textarea':
      return String(value ?? '');
    case 'file':
      return undefined;
    case 'heading':
      return undefined;
  }
}

export function coerceDefaultValueForField(
  field: Pick<FormField, 'type' | 'options'>,
  value: unknown,
): string | number | boolean | string[] | undefined {
  const meta = getFieldControl(field);
  if (field.type === 'number') {
    const n = Number(value);
    return Number.isFinite(n) ? n : undefined;
  }
  return coerceValueForKind(meta.kind, value, field.options);
}

/**
 * 編集モードかどうかを判定する。
 *
 * 判定ロジック:
 *  1. props.editMode が boolean なら最優先
 *  2. それ以外で initialValues に保存済みレコードのマーカー (id / _id / createdAt / updatedAt) があれば編集モード
 *  3. それ以外（undefined / 空オブジェクト / preset のみ）は新規モード
 *
 * @param initialValues コンポーネントに渡される初期値オブジェクト
 * @param explicitEditMode props で明示された editMode（任意）
 * @returns 編集モードなら true、新規モードなら false
 */
export function inferEditMode(
  initialValues: Record<string, unknown> | undefined | null,
  explicitEditMode?: boolean,
): boolean {
  if (typeof explicitEditMode === 'boolean') return explicitEditMode;
  if (!initialValues) return false;
  return (
    typeof (initialValues as Record<string, unknown>).id === 'string' ||
    typeof (initialValues as Record<string, unknown>)._id === 'string' ||
    'createdAt' in (initialValues as Record<string, unknown>) ||
    'updatedAt' in (initialValues as Record<string, unknown>)
  );
}
