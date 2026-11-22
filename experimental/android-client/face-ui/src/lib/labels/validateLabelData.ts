/**
 * v0.8.2 Pure validation function for label data.
 *
 * Validates normalized label data against the placeholder schema:
 * - Required field presence
 * - Character limit enforcement
 * - Copy count range (1-30)
 *
 * This module is a PURE FUNCTION — no Firestore, no fetch, no DOM, no React.
 */

import type { PlaceholderName } from './placeholderSchema';
import { PLACEHOLDER_SCHEMA, getTemplatePlaceholders } from './placeholderSchema';
import type { TemplateKey } from './constants';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ValidationError = {
  field: PlaceholderName;
  type: 'missing' | 'too_long';
  limit?: number;
  actual?: number;
};

export type ValidationResult = {
  valid: boolean;
  errors: ValidationError[];
};

// ---------------------------------------------------------------------------
// Main function
// ---------------------------------------------------------------------------

export function validateLabelData(
  templateKey: TemplateKey,
  data: Record<string, string>,
  copies?: number,
  locale: 'ja' | 'en' = 'ja',
): ValidationResult {
  const errors: ValidationError[] = [];
  // locale-aware: EN traceable は EN 用の絞り込み placeholder リストを使う。
  // locale 省略時は JA 動作 (既存呼び出しの後方互換)。
  const placeholders = getTemplatePlaceholders(templateKey, locale);

  for (const ph of placeholders) {
    const def = PLACEHOLDER_SCHEMA[ph];
    if (!def) continue;

    const value = data[ph] ?? '';

    // 1. Check required fields are present and non-empty
    if (def.required && !value.trim()) {
      errors.push({ field: ph, type: 'missing' });
      continue; // no point checking length of a missing field
    }

    // 2. Check character limits (0 = no limit)
    if (def.charLimit > 0 && value.length > def.charLimit) {
      errors.push({
        field: ph,
        type: 'too_long',
        limit: def.charLimit,
        actual: value.length,
      });
    }
  }

  // 3. Check copies range: 1-30
  if (copies != null && (copies < 1 || copies > 30)) {
    // Use a special error entry — 'productName' placeholder is reused as a sentinel
    // since copies is not a PlaceholderName. We cast to keep the type consistent.
    errors.push({
      field: 'copies' as PlaceholderName,
      type: 'missing',
      limit: 30,
      actual: copies,
    });
  }

  return {
    valid: errors.length === 0,
    errors,
  };
}
