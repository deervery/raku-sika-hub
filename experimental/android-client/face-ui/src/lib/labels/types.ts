/**
 * ラベルフィールドマッピングの型定義
 */

/** フィールドのデータソース */
export type FieldSource = 'product' | 'individual' | 'lot' | 'facility' | 'computed';

/** computed フィールドの計算タイプ */
export type ComputeType = 'deadline' | 'qrCode';

/** 保存温度プリセット */
export type StorageTemperaturePreset = 'frozen' | 'refrigerated' | 'ambient';

/** ラベルテンプレートのフィールドマッピング定義 */
export type LabelFieldMapping = {
  /** b-PACテンプレート上のプレースホルダー名 */
  placeholder: string;
  /** UIに表示する日本語ラベル */
  label: string;
  /** データの取得元 */
  source: FieldSource;
  /** ソースオブジェクト内のフィールド名（source が 'computed' 以外の場合） */
  sourceField?: string;
  /** 必須フィールドかどうか */
  required: boolean;
  /** computed フィールドの計算方式 */
  computeType?: ComputeType;
  /** 出力フォーマット（例: "{value}℃"） */
  format?: string;
};

/** Firestore masterLabelTemplates ドキュメントの型 */
export type MasterLabelTemplate = {
  id?: string; // Firestoreドキュメント ID
  facilityId: string;
  name: string;
  templateKey: string;
  isTraceable: boolean;
  storageTemperaturePreset?: StorageTemperaturePreset;
  /** PC上のラベルファイル絶対パス (例: C:\Labels\traceable_label.lbx) */
  labelPathLocal: string;
  fieldMappings: LabelFieldMapping[];
  order: number;
  createdBy: string;
  updatedBy: string;
  createdAt: unknown;
  updatedAt: unknown;
};
