export type FormField = {
  label: string;
  type:
    | 'text'
    | 'number'
    | 'date'
    | 'datetime-local'
    | 'time'
    | 'month'
    | 'radio'
    | 'checkbox'
    | 'select'
    | 'textarea'
    | 'file'
    | 'url'
    | 'color'
    | 'tel'
    | 'week'
    // セクション見出し用（入力は伴わない）
    | 'h2title'
    | 'h3title'
    // 説明テキスト（入力は伴わない）
    | 'description';
  required: boolean;
  order: number;
  display?: boolean;
  placeholder?: string;
  options?: string[];
  /** select type: オプションが value:label 形式の場合 true */
  selectValueLabel?: boolean;
  defaultValue?: string | boolean | number | string[];
  h2title?: string;
  h3title?: string;
  suffix?: string;
  prefix?: string;
  stepUp?: number;
  stepDown?: number;
  /** この値の場合に送信時に警告confirmを表示する（例: '未実施'） */
  warnValues?: string[];
  /** 値が空の場合に送信時に警告confirmを表示する */
  warnIfEmpty?: boolean;
  /** warnValues/warnIfEmpty に該当した場合のメッセージ（省略時はデフォルトメッセージ） */
  warnMessage?: string;
  /**
   * text 型を「半角数字 N 桁」に限定する。
   * 入力時に数字以外を除去して N 桁で打ち切り、送信時に桁数を検証する。
   */
  digits?: number;
  /**
   * condition により非表示になっている間、送信値を空にする。
   * 表示条件を満たさない値が保存に残るのを防ぐ（未指定時は従来どおり値を保持）。
   */
  clearWhenHidden?: boolean;
  condition?:
    | {
        field: string;
        value: string | number | boolean | (string | number | boolean)[];
      }
    | Array<{
        field: string;
        value: string | number | boolean | (string | number | boolean)[];
      }>;
};

export type FormDefinition = {
  formName: string;
  targetCollection: string;
  description?: string;
  version?: number;
  default?: boolean;
  onboarding?: boolean;
  fields: { [key: string]: FormField };
};
