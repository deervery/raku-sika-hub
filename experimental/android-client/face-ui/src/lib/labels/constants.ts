import type { LabelFieldMapping } from './types';

export type TemplateKey = 'traceable' | 'non_traceable' | 'processed' | 'pet' | 'carcass';
export type HubPrintTemplate =
  | 'traceable_deer'
  | 'traceable_bear'
  | 'non_traceable_deer'
  | 'processed'
  | 'pet'
  | 'carcass_deer'
  | 'carcass_bear';

export type LabelData = Partial<{
  productName: string;
  productQuantity: string; // e.g. "1.25 kg" or "10 個"
  deadlineDate: string; // e.g. "2025年4月25日"
  storageTemperature: string;
  storageMethod: string; // '冷凍' | '冷蔵' | 'frozen' | 'refrigerated' — used to pick defaults
  // traceable
  individualId: string; // Firestore doc.id (transactionIndividuals) - for QR code URL
  individualNumber: string; // 個体識別番号（表示用、e.g. "202508-1203"）
  productId: string; // Firestore doc.id (masterProducts)
  lotId: string; // Firestore doc.id (transactionMeatLots) - for QR code URL
  captureLocation: string;
  qrCode: string; // QR code URL (auto-generated from lotId or individualId)
  // processed
  productIngredient: string;
  nutritionUnit: string;
  caloriesQuantity: string;
  proteinQuantity: string;
  fatQuantity: string;
  carbohydratesQuantity: string;
  saltEquivalentQuantity: string;
  // carcass
  species: string;
  sex: string;
  receivingDate: string;
  facilityName: string;
  // misc
  attentionText: string;
  isHeatedMeatProducts: string | boolean;
  // EN bilingual traceable (issue #271) — Hub 経由のみ消費 (bPAC EN テンプレでは未使用)
  productNameJa: string;
  deadlineDateJa: string;
  storageTemperatureJa: string;
  speciesOfOrigin: string;
  speciesOfOriginJa: string;
  countryOfOrigin: string;
  countryOfOriginJa: string;
  processingPlantName: string;
  address: string;
  addressJa: string;
  /** エゾシカ認証マーク表示制御 (issue #271): "true" / "false"。
   *  Hub の traceable renderer (JA / EN bilingual 両方) が ninsyo_logo.jpg の
   *  表示有無を切り替える。施設マスタの ezoshikaCertified boolean から伝播。 */
  ezoshikaCertified: string;
  /** プラマーク＋「外装」表示制御: "true" / "false"。施設マスタの plaMark boolean から伝播。
   *  Hub が精肉（トレサ・非トレサ）・加工品・ペットのラベルに描く。 */
  plaMark: string;
}>;

export const REQUIRED_PLACEHOLDERS: Record<TemplateKey, string[]> = {
  traceable: [
    'productName',
    'productQuantity',
    'deadlineDate',
    'storageTemperature',
    'individualNumber',
  ],
  non_traceable: ['productName', 'productQuantity', 'deadlineDate', 'storageTemperature'],
  processed: ['productName', 'productQuantity', 'deadlineDate', 'storageTemperature'],
  pet: ['productName', 'productQuantity', 'deadlineDate', 'storageTemperature'],
  carcass: ['individualNumber'],
};

export type LabelLocaleKey = 'ja' | 'en';

/**
 * EN モード専用の必須プレースホルダ。EN .lbx テンプレートはシンプル化されており、
 * 4フィールドだけを使う。EN 未対応のテンプレートキーは含まない（その場合は JA フォールバック）。
 */
export const REQUIRED_PLACEHOLDERS_EN: Partial<Record<TemplateKey, string[]>> = {
  traceable: ['productName', 'productQuantity', 'deadlineDate', 'individualNumber'],
};

/**
 * テンプレート種別 + locale から必須プレースホルダ配列を返す。
 * locale 省略時は JA 動作（既存呼び出しの後方互換）。
 */
export function getRequiredPlaceholders(
  templateKey: TemplateKey,
  locale: LabelLocaleKey = 'ja',
): string[] {
  if (locale === 'en') {
    const en = REQUIRED_PLACEHOLDERS_EN[templateKey];
    if (en) return en;
  }
  return REQUIRED_PLACEHOLDERS[templateKey] || [];
}

const DEFAULT_BPAC_PLACEHOLDERS: Record<TemplateKey, string[]> = {
  traceable: [
    'productName',
    'productQuantity',
    'deadlineDate',
    'storageTemperature',
    'individualId',
    'captureLocation',
    'qrCode',
    'attentionText',
  ],
  non_traceable: [
    'productName',
    'productQuantity',
    'deadlineDate',
    'storageTemperature',
    'attentionText',
  ],
  processed: [
    'productName',
    'productQuantity',
    'deadlineDate',
    'storageTemperature',
    'productIngredient',
    'nutritionUnit',
    'caloriesQuantity',
    'proteinQuantity',
    'fatQuantity',
    'carbohydratesQuantity',
    'saltEquivalentQuantity',
    'isHeatedMeatProducts',
    'attentionText',
  ],
  pet: [
    'productName',
    'productQuantity',
    'deadlineDate',
    'storageTemperature',
    'productIngredient',
    'nutritionUnit',
    'caloriesQuantity',
    'proteinQuantity',
    'fatQuantity',
    'carbohydratesQuantity',
    'saltEquivalentQuantity',
    'companyBlock',
    'facilityBlock',
    'attentionText',
  ],
  carcass: [
    'individualId',
    'qrCode',
    'facilityName',
    'sex',
    'receivingDate',
    'species',
    'attentionText',
  ],
};

const FALLBACK_BPAC_PLACEHOLDERS = [
  'productName',
  'productQuantity',
  'deadlineDate',
  'storageTemperature',
  'individualId',
  'individualNumber',
  'captureLocation',
  'qrCode',
  'attentionText',
  'facilityName',
  'sex',
  'receivingDate',
  'species',
  'productIngredient',
  'nutritionUnit',
  'caloriesQuantity',
  'proteinQuantity',
  'fatQuantity',
  'carbohydratesQuantity',
  'saltEquivalentQuantity',
  'isHeatedMeatProducts',
] as const;

export function getBpacPlaceholdersForTemplate(templateKey: string): string[] {
  const resolved = DEFAULT_BPAC_PLACEHOLDERS[templateKey as TemplateKey];
  return resolved ? [...resolved] : [...FALLBACK_BPAC_PLACEHOLDERS];
}

export function formatDeadlineToJapanese(dateInput: unknown): string | undefined {
  try {
    if (!dateInput) return undefined;
    let d: Date | null = null;
    if (typeof dateInput === 'string') {
      // accept YYYY-MM-DD or ISO
      const tmp = new Date(dateInput);
      if (!isNaN(tmp.getTime())) d = tmp;
    } else if (dateInput instanceof Date) {
      d = dateInput;
    }
    if (!d) return undefined;
    // JST 表示
    const jst = new Date(d.getTime() + 9 * 60 * 60 * 1000);
    const y = jst.getUTCFullYear();
    const m = jst.getUTCMonth() + 1;
    const day = jst.getUTCDate();
    return `${y}年${m}月${day}日`;
  } catch {
    return undefined;
  }
}

/**
 * 個体識別番号をハイフン区切り形式にフォーマットする
 * 10桁の数字 → xxxx-xx-xx-xx（4-2-2-2）
 * 11桁の数字 → xxxx-xx-xx-xxx（4-2-2-3）
 * 既にハイフン付きの場合はそのまま返す
 */
export function formatIndividualNumber(raw: string | undefined | null): string {
  if (!raw) return '';
  // 既に整形済み（10桁=4-2-2-2 / 11桁=4-2-2-3）ならそのまま
  if (/^\d{4}-\d{2}-\d{2}-\d{2,3}$/.test(raw)) return raw;
  const digits = raw.replace(/\D/g, '');
  if (digits.length === 10) {
    return `${digits.slice(0, 4)}-${digits.slice(4, 6)}-${digits.slice(6, 8)}-${digits.slice(8, 10)}`;
  }
  if (digits.length === 11) {
    return `${digits.slice(0, 4)}-${digits.slice(4, 6)}-${digits.slice(6, 8)}-${digits.slice(8, 11)}`;
  }
  // 10桁・11桁でなければそのまま返す
  return raw;
}

export function normalizeLabelData(templateKey: TemplateKey, data: LabelData): LabelData {
  const normalized: LabelData = { ...data };

  // Trim common strings
  const trim = (v: unknown) => (typeof v === 'string' ? v.trim() : v);
  normalized.productName = trim(normalized.productName) as string | undefined;
  normalized.productQuantity = trim(normalized.productQuantity) as string | undefined;
  normalized.storageTemperature = trim(normalized.storageTemperature) as string | undefined;

  // Deadline formatting: allow YYYY-MM-DD / ISO / already formatted
  if (normalized.deadlineDate) {
    const reJa = /\d{4}年\d{1,2}月\d{1,2}日/;
    if (!reJa.test(String(normalized.deadlineDate))) {
      const formatted = formatDeadlineToJapanese(normalized.deadlineDate);
      if (formatted) normalized.deadlineDate = formatted;
    }
  }

  // Traceable specific
  if (templateKey === 'traceable') {
    // individualIdはFirestore doc.id（QRコードURL用）
    normalized.individualId = trim(normalized.individualId) as string | undefined;
    // individualNumberは人間向け表示用（ラベル印字用）
    normalized.individualNumber = trim(normalized.individualNumber) as string | undefined;
    normalized.productId = trim(normalized.productId) as string | undefined;
    normalized.captureLocation = trim(normalized.captureLocation) as string | undefined;

    // QRコードURLを自動生成（lotId優先、フォールバックでindividualId）
    normalized.lotId = trim(normalized.lotId) as string | undefined;
    if (!normalized.qrCode) {
      if (normalized.lotId) {
        normalized.qrCode = `https://rakusika.com/t/${normalized.lotId}`;
      } else if (normalized.individualId) {
        normalized.qrCode = `https://rakusika.com/t/${normalized.individualId}`;
      }
    }
    normalized.qrCode = trim(normalized.qrCode) as string | undefined;
  }

  // Carcass specific
  if (templateKey === 'carcass') {
    normalized.individualId = trim(normalized.individualId) as string | undefined;
    normalized.individualNumber = trim(normalized.individualNumber) as string | undefined;
    normalized.captureLocation = trim(normalized.captureLocation) as string | undefined;
    normalized.species = trim(normalized.species) as string | undefined;
    normalized.sex = trim(normalized.sex) as string | undefined;
    normalized.receivingDate = trim(normalized.receivingDate) as string | undefined;
    normalized.facilityName = trim(normalized.facilityName) as string | undefined;

    // QRコードURLを自動生成（individualId ベース）
    if (!normalized.qrCode && normalized.individualId) {
      normalized.qrCode = `https://rakusika.com/t/${normalized.individualId}`;
    }
    normalized.qrCode = trim(normalized.qrCode) as string | undefined;
  }

  // Processed / Pet: just trim
  if (templateKey === 'processed' || templateKey === 'pet') {
    normalized.productIngredient = trim(normalized.productIngredient) as string | undefined;
    normalized.nutritionUnit = trim(normalized.nutritionUnit) as string | undefined;
    normalized.caloriesQuantity = trim(normalized.caloriesQuantity) as string | undefined;
    normalized.proteinQuantity = trim(normalized.proteinQuantity) as string | undefined;
    normalized.fatQuantity = trim(normalized.fatQuantity) as string | undefined;
    normalized.carbohydratesQuantity = trim(normalized.carbohydratesQuantity) as string | undefined;
    normalized.saltEquivalentQuantity = trim(normalized.saltEquivalentQuantity) as
      | string
      | undefined;
  }

  return normalized;
}

export function validateRequiredPlaceholders(
  templateKey: TemplateKey,
  data: LabelData,
  locale: LabelLocaleKey = 'ja',
): { missingPlaceholders: string[] } {
  const required = getRequiredPlaceholders(templateKey, locale);
  const missing = required.filter((key) => {
    const v = (data as Record<string, unknown>)[key];
    return v == null || (typeof v === 'string' && v.trim() === '');
  });
  return { missingPlaceholders: missing };
}

function isBearValue(value: unknown): boolean {
  if (typeof value !== 'string') return false;
  return /(クマ|熊|bear)/i.test(value.trim());
}

/**
 * 任意の獣種表記文字列 (例: '蝦夷鹿' / 'エゾシカ' / 'ニホンジカ' / 'ヒグマ' /
 * 'ツキノワグマ' / '羆' / '猪' / 'wild boar' 等の legacy / 別名表記) を、内部
 * enum キー ('シカ' / 'クマ' / 'イノシシ' / 'アライグマ') に正規化する。
 *
 * 該当しないときは null を返す。speciesLabelMap[key] 検索用。
 *
 * 命名パターン補足:
 *   - 'ヒグマ' / 'ツキノワグマ' は連声で 'クマ' が 'グマ' になるため 'グマ' も拾う
 *   - 'ニホンジカ' は連濁で 'シカ' が 'ジカ' になるため 'ジカ' も拾う
 *   - '羆' (=ヒグマ) は 'クマ' / '熊' を含まない独立漢字なので明示
 */
export type SpeciesEnumKey = 'シカ' | 'クマ' | 'イノシシ' | 'アライグマ';
export function resolveSpeciesKey(value: unknown): SpeciesEnumKey | null {
  if (typeof value !== 'string') return null;
  const v = value.trim();
  if (!v) return null;
  // 順序注意: 'アライグマ' は 'グマ' を含むため先に判定する必要がある。
  if (/(アライグマ|raccoon)/i.test(v)) return 'アライグマ';
  if (/(イノシシ|猪|boar)/i.test(v)) return 'イノシシ';
  if (/(クマ|グマ|熊|羆|bear)/i.test(v)) return 'クマ';
  if (/(シカ|ジカ|鹿|deer)/i.test(v)) return 'シカ';
  return null;
}

/**
 * 精肉ラベル品名末尾に「（{施設設定の獣種表記}）」を付与する (#270)。
 *
 * - locale='ja' のみ動作。EN ラベルでは productName が "Top Round/\nウチモモ" 形式の
 *   バイリンガル文字列なので、JA 接尾辞を入れると構造が壊れるため何もしない。
 * - species 値は resolveSpeciesKey で enum キーに正規化してから lookup する
 *   (legacy データの '蝦夷鹿' / 'ヒグマ' 等にも対応)
 * - speciesLabelMap[key] が未設定 / 空文字 のときは付与しない (「マッピング空＝表示しない」仕様)
 * - 既に同じ表記で終わっている場合は再付与しない (再印刷 / hydration→print の冪等性)
 *
 * @returns 必要なら suffix 付き productName、そうでなければ元の productName をそのまま返す
 */
export function applySpeciesSuffix(
  productName: string,
  species: string,
  speciesLabelMap: Record<string, string> | undefined,
  locale: 'ja' | 'en' = 'ja',
): string {
  if (locale !== 'ja') return productName;
  if (!productName || !species) return productName;
  const speciesKey = resolveSpeciesKey(species);
  const label = speciesKey ? speciesLabelMap?.[speciesKey]?.trim() : undefined;
  if (!label) return productName;
  const suffixJa = `（${label}）`;
  const suffixEn = `(${label})`;
  if (productName.endsWith(suffixJa) || productName.endsWith(suffixEn)) {
    return productName;
  }
  return `${productName}${suffixJa}`;
}

export function resolveHubPrintTemplate(
  templateKey: TemplateKey,
  data?: { species?: unknown; productName?: unknown },
): HubPrintTemplate {
  switch (templateKey) {
    case 'traceable':
      return isBearValue(data?.species) || isBearValue(data?.productName)
        ? 'traceable_bear'
        : 'traceable_deer';
    case 'non_traceable':
      return 'non_traceable_deer';
    case 'processed':
      return 'processed';
    case 'pet':
      return 'pet';
    case 'carcass':
      return isBearValue(data?.species) ? 'carcass_bear' : 'carcass_deer';
  }
}

export const DEFAULT_PRINTER_NAME = 'Brother QL-800';

export type FacilityLabelTemplateConfig = {
  /** 施設名 */
  name?: string;
  /** ラベルテンプレートのベースディレクトリ（末尾 \ 無し、例: C:\RakuSika） */
  labelBaseDir?: string;
  /** @deprecated レガシーフィールド群（後方互換） */
  labelPathLocal?: string;
  traceableLabelName?: string;
  nonTraceableLabelName?: string;
  processedProductLabel?: string;
  petProductLabelName?: string;
  traceableDeerLabelPath?: string;
  traceableBearLabelPath?: string;
  traceableBoarLabelPath?: string;
  traceableRaccoonLabelPath?: string;
  nonTraceableLabelPath?: string;
  processedLabelPath?: string;
  petLabelPath?: string;
  traceableLabelPath?: string;
  defaultExpirationDays?: number;
  defaultStorageTemperature?: string; // レガシー: 後方互換
  defaultStorageTemperatureFrozen?: string;
  defaultStorageTemperatureChilled?: string;
  labelQuickMode?: boolean;
  printerName?: string;
};

/**
 * bpac / Hub 共通のテンプレートファイル名（拡張子なし）。
 * Hub テンプレート名と一致させ、bpac 側は .lbx を付与する。
 */
export const LABEL_TEMPLATE_FILES: Record<string, string> = {
  traceable_deer: 'traceable_deer',
  traceable_bear: 'traceable_bear',
  traceable_boar: 'traceable_boar',
  traceable_raccoon: 'traceable_raccoon',
  non_traceable: 'non_traceable',
  processed: 'processed',
  pet: 'pet',
  carcass_deer: 'carcass_deer',
  carcass_bear: 'carcass_bear',
};

/** 全テンプレートファイル一覧（UI表示用） */
export const LABEL_TEMPLATE_FILE_LIST = [
  { key: 'traceable_deer', label: '精肉（シカ）', file: 'traceable_deer.lbx' },
  { key: 'traceable_bear', label: '精肉（クマ）', file: 'traceable_bear.lbx' },
  { key: 'traceable_boar', label: '精肉（イノシシ）', file: 'traceable_boar.lbx' },
  { key: 'traceable_raccoon', label: '精肉（アライグマ）', file: 'traceable_raccoon.lbx' },
  { key: 'non_traceable', label: '非トレサ精肉', file: 'non_traceable.lbx' },
  { key: 'processed', label: '加工品', file: 'processed.lbx' },
  { key: 'pet', label: 'ペットフード', file: 'pet.lbx' },
  { key: 'carcass_deer', label: '枝肉（シカ）', file: 'carcass_deer.lbx' },
  { key: 'carcass_bear', label: '枝肉（クマ）', file: 'carcass_bear.lbx' },
];

/** labelBaseDir を正規化（末尾の \ / を除去） */
export function normalizeLabelBaseDir(dir: string): string {
  return dir.trim().replace(/[\\/]+$/, '');
}

/**
 * テンプレート種別 + 獣種から .lbx フルパスを組み立てる。
 * labelBaseDir + \ + {template_name}.lbx
 */
export function resolveFacilityLabelPath(
  facility: FacilityLabelTemplateConfig,
  templateKey: TemplateKey,
  species?: string,
  locale?: string,
): string {
  const baseDir = facility.labelBaseDir ? normalizeLabelBaseDir(facility.labelBaseDir) : '';
  if (!baseDir) return '';

  let fileName: string;
  if (templateKey === 'traceable') {
    fileName = resolveSpeciesFileName(species);
  } else if (templateKey === 'carcass') {
    fileName = resolveCarcassSpeciesFileName(species);
  } else {
    fileName = LABEL_TEMPLATE_FILES[templateKey] || templateKey;
  }

  // English templates use _en suffix (traceable only)
  if (locale === 'en' && templateKey === 'traceable') {
    fileName = `${fileName}_en`;
  }

  return `${baseDir}\\${fileName}.lbx`;
}

/** 獣種からテンプレートファイル名を解決 */
function resolveSpeciesFileName(species?: string): string {
  if (!species) return 'traceable_deer';
  const s = species.trim();
  if (/(クマ|熊|bear)/i.test(s)) return 'traceable_bear';
  if (/(イノシシ|猪|boar)/i.test(s)) return 'traceable_boar';
  if (/(アライグマ|洗熊|raccoon)/i.test(s)) return 'traceable_raccoon';
  return 'traceable_deer';
}

/** 枝肉用: 獣種からテンプレートファイル名を解決 */
function resolveCarcassSpeciesFileName(species?: string): string {
  if (!species) return 'carcass_deer';
  const s = species.trim();
  if (/(クマ|熊|bear)/i.test(s)) return 'carcass_bear';
  return 'carcass_deer';
}

/** @deprecated レガシー名前フィールドのマッピング */
export function mapTemplateKeyToFacilityField(
  templateKey: TemplateKey,
): keyof FacilityLabelTemplateConfig {
  switch (templateKey) {
    case 'traceable':
      return 'traceableLabelName';
    case 'non_traceable':
      return 'nonTraceableLabelName';
    case 'processed':
      return 'processedProductLabel';
    case 'pet':
      return 'petProductLabelName';
    case 'carcass':
      return 'traceableLabelName'; // carcass は専用フィールドなし、フォールバック
  }
}

/**
 * 動的テンプレートからテンプレート解決情報を取得する
 * labelTemplateId → masterLabelTemplates.labelPathLocal をフルパスとして使用
 */
export type DynamicTemplateResolved = {
  /** ラベルファイルの絶対パス (例: C:\RakuSika\mnl\traceable_label.lbx) */
  templatePath: string;
  templateKey: string;
  fieldMappings?: LabelFieldMapping[];
  storageTemperaturePreset?: string;
};

export async function resolveDynamicTemplate(
  adminDb: FirebaseFirestore.Firestore,
  facilityId: string,
  opts: { labelTemplateId?: string; templateKey?: TemplateKey; species?: string },
): Promise<{ resolved: DynamicTemplateResolved; facility: FacilityLabelTemplateConfig }> {
  const facilitySnap = await adminDb.collection('masterFacilities').doc(facilityId).get();
  const facility = (facilitySnap.data() || {}) as FacilityLabelTemplateConfig;

  if (!opts.labelTemplateId) {
    // labelTemplateId が未指定 → 施設の獣種別パスにフォールバック
    const tk = opts.templateKey || 'non_traceable';
    const facilityPath = resolveFacilityLabelPath(facility, tk, opts.species);
    return {
      resolved: {
        templatePath: facilityPath,
        templateKey: tk,
      },
      facility,
    };
  }

  const templateSnap = await adminDb
    .collection('masterLabelTemplates')
    .doc(opts.labelTemplateId)
    .get();
  if (!templateSnap.exists) {
    console.warn(`[ラベル] masterLabelTemplates/${opts.labelTemplateId} が見つかりません。`);
    return {
      resolved: {
        templatePath: '',
        templateKey: opts.templateKey || 'non_traceable',
      },
      facility,
    };
  }

  const tpl = templateSnap.data()!;
  const templatePath = (tpl.labelPathLocal as string) || '';

  if (!templatePath) {
    console.warn(
      `[ラベル] masterLabelTemplates/${opts.labelTemplateId} に labelPathLocal が未設定です。`,
    );
  }

  return {
    resolved: {
      templatePath,
      templateKey: tpl.templateKey || 'custom',
      fieldMappings: tpl.fieldMappings || [],
      storageTemperaturePreset:
        typeof tpl.storageTemperaturePreset === 'string' ? tpl.storageTemperaturePreset : undefined,
    },
    facility,
  };
}
