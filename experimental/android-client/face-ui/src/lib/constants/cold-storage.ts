/**
 * Cold storage (refrigerator/freezer) configuration types and helpers.
 * Used by facility settings and daily report temperature management.
 */
import type { FormField } from '@/types/forms';

export type ColdStorageUnit = {
  id: string;
  name: string;
  type: 'refrigerator' | 'freezer';
  defaultTemp: number;
};

/** Default cold storage configuration (backward compatible with legacy hardcoded fields) */
export const DEFAULT_COLD_STORAGES: ColdStorageUnit[] = [
  { id: 'ref1', name: '冷蔵庫', type: 'refrigerator', defaultTemp: 2 },
  { id: 'frz1', name: '冷凍庫', type: 'freezer', defaultTemp: -20 },
];

/**
 * Generate dynamic form fields for cold storage temperature management.
 * These replace the hardcoded refrigeratorTemperatureBefore/After and freezerTemperatureBefore/After fields.
 */
export function buildColdStorageFields(
  coldStorages: ColdStorageUnit[],
  baseOrder: number = 71,
  options?: { omitDefaultValue?: boolean },
): Record<string, FormField> {
  const fields: Record<string, FormField> = {};
  let order = baseOrder;

  for (const unit of coldStorages) {
    // Section header per unit
    fields[`h3_coldStorage_${unit.id}`] = {
      h3title: unit.name,
      label: unit.name,
      type: 'h3title',
      required: false,
      order: order++,
    };
    fields[`coldStorage_${unit.id}_before`] = {
      label: `${unit.name} 始業前温度`,
      type: 'number',
      required: true,
      display: true,
      order: order++,
      ...(options?.omitDefaultValue ? {} : { defaultValue: unit.defaultTemp }),
      suffix: '\u2103',
    };
    fields[`coldStorage_${unit.id}_after`] = {
      label: `${unit.name} 作業終了後温度`,
      type: 'number',
      required: true,
      display: true,
      order: order++,
      ...(options?.omitDefaultValue ? {} : { defaultValue: unit.defaultTemp }),
      suffix: '\u2103',
    };
  }

  return fields;
}

/**
 * Legacy field keys used before dynamic cold storage.
 * Needed for backward compatibility when displaying old reports.
 */
export const LEGACY_TEMP_FIELD_KEYS = [
  'h3_refrigerator',
  'refrigeratorTemperatureBefore',
  'refrigeratorTemperatureAfter',
  'h3_freezer',
  'freezerTemperatureBefore',
  'freezerTemperatureAfter',
];

/**
 * Get all dynamic cold storage field keys (for hidden field lists).
 */
export function getColdStorageFieldKeys(coldStorages: ColdStorageUnit[]): string[] {
  const keys: string[] = [];
  for (const unit of coldStorages) {
    keys.push(`h3_coldStorage_${unit.id}`);
    keys.push(`coldStorage_${unit.id}_before`);
    keys.push(`coldStorage_${unit.id}_after`);
  }
  return keys;
}

/**
 * Check if a record has legacy temperature fields (pre-dynamic cold storage).
 */
export function hasLegacyTempFields(record: Record<string, unknown>): boolean {
  return (
    'refrigeratorTemperatureBefore' in record ||
    'refrigeratorTemperatureAfter' in record ||
    'freezerTemperatureBefore' in record ||
    'freezerTemperatureAfter' in record
  );
}

/**
 * Check if a record has dynamic cold storage fields.
 */
export function hasDynamicColdStorageFields(record: Record<string, unknown>): boolean {
  return Object.keys(record).some((k) => k.startsWith('coldStorage_'));
}

/**
 * Migrate legacy temperature values to dynamic cold storage field keys
 * (for display purposes only -- does not modify stored data).
 */
export function migrateLegacyTempValues(record: Record<string, unknown>): Record<string, unknown> {
  const migrated: Record<string, unknown> = { ...record };
  // Map legacy field names to default cold storage IDs
  if ('refrigeratorTemperatureBefore' in record) {
    migrated['coldStorage_ref1_before'] = record['refrigeratorTemperatureBefore'];
  }
  if ('refrigeratorTemperatureAfter' in record) {
    migrated['coldStorage_ref1_after'] = record['refrigeratorTemperatureAfter'];
  }
  if ('freezerTemperatureBefore' in record) {
    migrated['coldStorage_frz1_before'] = record['freezerTemperatureBefore'];
  }
  if ('freezerTemperatureAfter' in record) {
    migrated['coldStorage_frz1_after'] = record['freezerTemperatureAfter'];
  }
  return migrated;
}
