import type { ReactNode } from "react";
import css from "./VoiceWorkspace.module.css";
export function ClearField({
  label,
  empty,
  onClear,
  children,
}: {
  label: string;
  empty: boolean;
  onClear: () => void;
  children: ReactNode;
}) {
  return (
    <div className={css.clearField}>
      {children}
      <button
        type="button"
        aria-label={`${label}をクリア`}
        disabled={empty}
        onMouseDown={(e) => e.preventDefault()}
        onClick={onClear}
      >
        ×
      </button>
    </div>
  );
}
