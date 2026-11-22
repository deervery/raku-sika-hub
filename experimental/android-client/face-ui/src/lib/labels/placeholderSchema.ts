/**
 * v0.8.2 Canonical placeholder schema for label templates.
 *
 * This module defines the single source of truth for all placeholder keys,
 * their metadata, and which template types use them.
 */

import type { FieldSource } from './types';
import type { TemplateKey } from './constants';

// ---------------------------------------------------------------------------
// PlaceholderName — union of every canonical key
// ---------------------------------------------------------------------------

export type PlaceholderName =
  // Common
  | 'productName'
  | 'productQuantity'
  | 'deadlineDate'
  | 'deadlineLabel'
  | 'storageTemperature'
  | 'attentionText'
  // Company info (masterUsers)
  | 'companyName'
  | 'companyPostalCode'
  | 'companyAddress'
  | 'companyPhone'
  | 'companyRepresentative'
  | 'companyBlock'
  // Facility info (masterFacilities)
  | 'facilityName'
  | 'facilityPostalCode'
  | 'facilityAddress'
  | 'facilityPhone'
  | 'facilityBlock'
  // Traceable-specific
  | 'individualNumber'
  | 'captureLocation'
  | 'qrCode'
  // EN bilingual traceable (issue #271) — Hub のみ消費。bPAC EN テンプレ (.lbx) は無視
  | 'productNameJa'
  | 'deadlineDateJa'
  | 'storageTemperatureJa'
  | 'speciesOfOrigin'
  | 'speciesOfOriginJa'
  | 'countryOfOrigin'
  | 'countryOfOriginJa'
  | 'processingPlantName'
  | 'address'
  | 'addressJa'
  | 'ezoshikaCertified'
  | 'plaMark'
  // Processed-specific
  | 'productIngredient'
  | 'additives'
  | 'allergens'
  | 'nutritionUnit'
  | 'caloriesQuantity'
  | 'proteinQuantity'
  | 'fatQuantity'
  | 'carbohydratesQuantity'
  | 'saltEquivalentQuantity'
  | 'isHeatedMeatProducts'
  // Carcass-specific
  | 'species'
  | 'sex'
  | 'receivingDate';

// ---------------------------------------------------------------------------
// PlaceholderDef
// ---------------------------------------------------------------------------

export type PlaceholderDef = {
  /** Japanese display name */
  label: string;
  /** Where the value comes from */
  source: FieldSource;
  /** Maximum character count (0 = no limit) */
  charLimit: number;
  /** Whether the placeholder is required for its template type */
  required: boolean;
};

// ---------------------------------------------------------------------------
// PLACEHOLDER_SCHEMA — metadata for every canonical key
// ---------------------------------------------------------------------------

export const PLACEHOLDER_SCHEMA: Record<PlaceholderName, PlaceholderDef> = {
  // --- Common ---
  productName: { label: '製品名', source: 'product', charLimit: 20, required: true },
  productQuantity: { label: '内容量', source: 'product', charLimit: 10, required: true },
  deadlineDate: { label: '賞味期限', source: 'computed', charLimit: 15, required: true },
  // 期限呼称 (#383): 冷蔵=消費期限 / 冷凍=賞味期限。.lbx のキャプションを動的差し替えするための値。
  // 物理テンプレに対応オブジェクトが無ければ bPAC は自動 skip する（後方互換）。
  deadlineLabel: { label: '期限区分', source: 'computed', charLimit: 15, required: false },
  storageTemperature: { label: '保存方法', source: 'facility', charLimit: 30, required: true },
  attentionText: { label: '注意書き', source: 'product', charLimit: 80, required: false },

  // --- Company info (masterUsers) ---
  companyName: { label: '会社名', source: 'facility', charLimit: 30, required: true },
  companyPostalCode: { label: '郵便番号', source: 'facility', charLimit: 8, required: false },
  companyAddress: { label: '会社住所', source: 'facility', charLimit: 60, required: false },
  companyPhone: { label: '会社電話番号', source: 'facility', charLimit: 15, required: false },
  companyRepresentative: { label: '代表者名', source: 'facility', charLimit: 20, required: false },
  companyBlock: { label: '会社情報', source: 'computed', charLimit: 0, required: false },

  // --- Facility info (masterFacilities) ---
  facilityName: { label: '施設名', source: 'facility', charLimit: 30, required: true },
  facilityPostalCode: { label: '施設郵便番号', source: 'facility', charLimit: 8, required: false },
  facilityAddress: { label: '施設所在地', source: 'facility', charLimit: 60, required: true },
  facilityPhone: { label: '施設電話番号', source: 'facility', charLimit: 15, required: false },
  facilityBlock: { label: '施設情報', source: 'computed', charLimit: 0, required: false },

  // --- Traceable-specific ---
  individualNumber: { label: '個体識別番号', source: 'individual', charLimit: 15, required: true },
  captureLocation: { label: '捕獲場所', source: 'individual', charLimit: 30, required: true },
  qrCode: { label: 'QRコード URL', source: 'computed', charLimit: 0, required: false },

  // --- Processed-specific ---
  productIngredient: { label: '原材料名', source: 'product', charLimit: 120, required: true },
  additives: { label: '添加物', source: 'product', charLimit: 60, required: false },
  allergens: { label: 'アレルゲン', source: 'product', charLimit: 60, required: false },
  nutritionUnit: { label: '栄養表示単位', source: 'product', charLimit: 30, required: false },
  caloriesQuantity: { label: '熱量', source: 'product', charLimit: 10, required: false },
  proteinQuantity: { label: 'たんぱく質', source: 'product', charLimit: 10, required: false },
  fatQuantity: { label: '脂質', source: 'product', charLimit: 10, required: false },
  carbohydratesQuantity: { label: '炭水化物', source: 'product', charLimit: 10, required: false },
  saltEquivalentQuantity: {
    label: '食塩相当量',
    source: 'product',
    charLimit: 10,
    required: false,
  },
  isHeatedMeatProducts: {
    label: '加熱食肉製品区分',
    source: 'product',
    charLimit: 0,
    required: false,
  },

  // --- Carcass-specific ---
  species: { label: '獣種', source: 'individual', charLimit: 10, required: true },
  sex: { label: '性別', source: 'individual', charLimit: 5, required: true },
  receivingDate: { label: '搬入日', source: 'lot', charLimit: 15, required: true },

  // --- EN bilingual traceable (issue #271) — Hub のみ消費 ---
  productNameJa: { label: '製品名(JA)', source: 'product', charLimit: 20, required: false },
  deadlineDateJa: { label: '賞味期限(JA)', source: 'computed', charLimit: 15, required: false },
  storageTemperatureJa: { label: '保存方法(JA)', source: 'facility', charLimit: 30, required: false },
  speciesOfOrigin: { label: '品種(EN)', source: 'computed', charLimit: 30, required: false },
  speciesOfOriginJa: { label: '品種(JA)', source: 'computed', charLimit: 30, required: false },
  countryOfOrigin: { label: '原産地(EN)', source: 'computed', charLimit: 30, required: false },
  countryOfOriginJa: { label: '原産地(JA)', source: 'computed', charLimit: 30, required: false },
  processingPlantName: { label: '製造所名(EN)', source: 'facility', charLimit: 30, required: false },
  address: { label: '住所(EN)', source: 'facility', charLimit: 80, required: false },
  addressJa: { label: '住所(JA)', source: 'facility', charLimit: 80, required: false },
  ezoshikaCertified: { label: 'エゾシカ認証取得', source: 'facility', charLimit: 0, required: false },
  plaMark: { label: 'プラマーク（外装）', source: 'facility', charLimit: 0, required: false },
};

// ---------------------------------------------------------------------------
// TEMPLATE_PLACEHOLDERS — which placeholders each template type uses
// ---------------------------------------------------------------------------

/**
 * EN ロケール時に使うテンプレート別プレースホルダ一覧。
 * EN .lbx は運用ルールでシンプル化されており、bPAC が試行する placeholder を
 * 物理テンプレートに合わせて絞り込むことで withAutoRetry のスキップ往復を削減する。
 * EN 未対応のテンプレートは含まない（その場合は JA フォールバック）。
 */
export const TEMPLATE_PLACEHOLDERS_EN: Partial<Record<TemplateKey, PlaceholderName[]>> = {
  traceable: ['productName', 'productQuantity', 'deadlineDate', 'individualNumber', 'qrCode'],
};

/**
 * テンプレート種別 + locale から bPAC が populate するプレースホルダ一覧を返す。
 * locale 省略時は JA 動作（既存呼び出しの後方互換）。
 */
export function getTemplatePlaceholders(
  templateKey: TemplateKey,
  locale: 'ja' | 'en' = 'ja',
): PlaceholderName[] {
  if (locale === 'en') {
    const en = TEMPLATE_PLACEHOLDERS_EN[templateKey];
    if (en) return en;
  }
  return TEMPLATE_PLACEHOLDERS[templateKey] ?? [];
}

export const TEMPLATE_PLACEHOLDERS: Record<TemplateKey, PlaceholderName[]> = {
  traceable: [
    'productName',
    'productQuantity',
    'deadlineDate',
    'deadlineLabel',
    'storageTemperature',
    'individualNumber',
    'captureLocation',
    'qrCode',
    'companyBlock',
    'facilityBlock',
    // species は印字に使わないが、resolveFacilityLabelPath でテンプレート (_deer/_bear) を
    // 選ぶために必要。欠落時の silent fallback (deer 固定) を避けるため required に含める。
    // bPAC は .lbx に species 欄が無ければ withAutoRetry が自動で skip する。Hub は送信前に除外。
    'species',
  ],
  non_traceable: [
    'productName',
    'productQuantity',
    'deadlineDate',
    'deadlineLabel',
    'storageTemperature',
    'attentionText',
  ],
  processed: [
    'productName',
    'productQuantity',
    'deadlineDate',
    'deadlineLabel',
    'storageTemperature',
    'productIngredient',
    'attentionText',
    'nutritionUnit',
    'caloriesQuantity',
    'proteinQuantity',
    'fatQuantity',
    'carbohydratesQuantity',
    'saltEquivalentQuantity',
    'isHeatedMeatProducts',
  ],
  pet: [
    'productName',
    'productQuantity',
    'deadlineDate',
    'deadlineLabel',
    'storageTemperature',
    'companyBlock',
    'facilityBlock',
  ],
  carcass: ['individualNumber', 'species', 'sex', 'receivingDate', 'qrCode', 'facilityName'],
};
