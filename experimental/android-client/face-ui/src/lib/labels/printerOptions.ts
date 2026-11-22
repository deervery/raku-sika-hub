/**
 * 印刷機（プリンタ）の選択肢。
 *
 * 施設設定（設定 → 施設 → ラベル印刷設定）と印刷ダイアログの詳細設定で
 * 同じ一覧を使うための単一の出どころ。ここを直せば両方に反映される。
 *
 * この値は **PC直結（bPAC）モード専用**。Windows のプリンタ名をそのまま
 * b-PAC に渡すため、Windows 側でドライバをインストールしたときの名前と
 * 一致している必要がある。Hub 経由の印刷では使われない（Hub 側が CUPS の
 * キューを自動で解決するため）。
 */
export type PrinterOption = {
  /** b-PAC に渡す Windows プリンタ名 */
  value: string;
  /** 画面に出す表示名 */
  label: string;
};

export const PRINTER_OPTIONS: PrinterOption[] = [
  { value: 'Brother QL-800', label: 'Brother QL-800（デフォルト）' },
  { value: 'Brother QL-820NWB', label: 'Brother QL-820NWB' },
];

/** form-definitions の select が使う `値:表示名` 形式。 */
export const PRINTER_OPTION_STRINGS: string[] = PRINTER_OPTIONS.map((o) => `${o.value}:${o.label}`);

/**
 * QL-820NWB を使っているかを判定する。
 *
 * 同じプリンタでも経路によって表記が違う:
 * - Hub 経由: `/health` の `printer.model` → `"QL-820NWB"`
 * - Hub の CUPS キュー名 → `"Brother_QL_820NWB_USB"`
 * - PC直結(bPAC): 施設設定の Windows プリンタ名 → `"Brother QL-820NWB"`
 *
 * 判定を1箇所に閉じるため、英数字以外を落として `QL820` を含むかで見る。
 * 候補は複数渡せる（どれか1つでも一致すれば true）。
 */
export function isQL820Printer(...candidates: (string | null | undefined)[]): boolean {
  return candidates.some((candidate) =>
    (candidate ?? '')
      .toUpperCase()
      .replace(/[^A-Z0-9]/g, '')
      .includes('QL820'),
  );
}
