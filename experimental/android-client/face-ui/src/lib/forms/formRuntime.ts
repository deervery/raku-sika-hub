import type { FormDefinition, FormField } from '@/types/forms';
import { canClearHiddenField, digitsFieldError } from './fieldValidation';
import { getDefaultInitialValue, getEmptyInitialValue, inferEditMode } from './inputMapping';

/** Existing DynamicForm semantics; `display` is intentionally not a condition. */
export function matchesFieldCondition(field: FormField, values: Record<string, unknown>): boolean {
  if (!field.condition) return true;
  const match = (condition: {
    field: string;
    value: string | number | boolean | (string | number | boolean)[];
  }) => {
    const current = values[condition.field];
    if (Array.isArray(condition.value)) {
      if (Array.isArray(current)) return condition.value.some((value) => current.includes(value));
      return condition.value.includes(current as string | number | boolean);
    }
    if (Array.isArray(current)) return current.includes(condition.value);
    return current === condition.value;
  };
  return Array.isArray(field.condition) ? field.condition.every(match) : match(field.condition);
}

export function isFormInput(field: FormField): boolean {
  return field.type !== 'h2title' && field.type !== 'h3title' && field.type !== 'description';
}

export function initializeFormValues(
  form: FormDefinition,
  initialValues?: Record<string, unknown>,
  explicitEditMode?: boolean,
): Record<string, unknown> {
  const editMode = inferEditMode(initialValues, explicitEditMode);
  const values: Record<string, unknown> = {};
  for (const [key, field] of Object.entries(form.fields)) {
    values[key] =
      initialValues && key in initialValues
        ? initialValues[key]
        : editMode
          ? getEmptyInitialValue(field)
          : getDefaultInitialValue(field);
  }
  return values;
}

export type FormValidationIssue = {
  key: string;
  kind: 'required' | 'digits';
  message: string;
};

/** Preserve field-definition order and the existing first-error behavior. */
export function findFormValidationIssue(
  form: FormDefinition,
  values: Record<string, unknown>,
): FormValidationIssue | null {
  for (const [key, field] of Object.entries(form.fields)) {
    if (!isFormInput(field) || !matchesFieldCondition(field, values)) continue;
    const value = values[key];
    if (
      field.required &&
      (value == null ||
        value === '' ||
        (Array.isArray(value) && value.length === 0) ||
        (typeof value === 'number' && Number.isNaN(value)))
    )
      return { key, kind: 'required', message: `${field.label} は必須です。` };
    const message = digitsFieldError(field, value);
    if (message) return { key, kind: 'digits', message };
  }
  return null;
}

export function getFormWarnings(form: FormDefinition, values: Record<string, unknown>): string[] {
  const warnings: string[] = [];
  for (const [key, field] of Object.entries(form.fields)) {
    if (!isFormInput(field) || !matchesFieldCondition(field, values)) continue;
    const value = values[key];
    if (field.warnValues?.length && typeof value === 'string' && field.warnValues.includes(value)) {
      warnings.push(field.warnMessage || `「${field.label}」が「${value}」です。`);
    }
    if (field.warnIfEmpty && (value == null || value === '')) {
      warnings.push(field.warnMessage || `「${field.label}」が未入力です。`);
    }
  }
  return warnings;
}

/**
 * Pure submission conversion. File upload stays with the existing I/O adapter.
 * Untouched legacy hidden values must survive unrelated edits.
 */
export function getFormSubmissionValue(
  key: string,
  field: FormField,
  values: Record<string, unknown>,
  touched: ReadonlySet<string>,
): unknown {
  if (
    field.clearWhenHidden &&
    !matchesFieldCondition(field, values) &&
    canClearHiddenField(key, field, touched)
  ) {
    return '';
  }
  const raw = values[key];
  if (raw == null || raw === '') return raw;
  if (field.type === 'number') {
    const value = typeof raw === 'number' ? raw : Number(raw);
    return Number.isFinite(value) ? value : raw;
  }
  return raw;
}
