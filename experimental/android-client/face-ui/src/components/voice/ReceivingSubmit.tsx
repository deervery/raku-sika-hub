import {findFormValidationIssue} from '../../lib/forms/formRuntime';
import { useMemo, useRef, useState } from "react";
import { RECEIVING_FORM } from "../../lib/constants/form-definitions";
import {
  reviewEntries,
  missingValue,
} from "../../lib/voice-guidance/stationUi";
import { DurableBusiness } from "../../durableBusiness";
import { native } from "../../bridge";
import css from "./VoiceWorkspace.module.css";
export function ReceivingSubmit({
  scope,
  values,
  onClose,
  onRegistered,
}: {
  scope: string;
  values: Record<string, unknown>;
  onClose: () => void;
  onRegistered: () => void;
}) {
  const receipt = useMemo(
    () =>
      new DurableBusiness(
        localStorage,
        "receiving-receipt:v035:" + scope,
        native,
      ),
    [scope],
  );
  const [version, refresh] = useState(0),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const inFlight = useRef(false);
  const existing = useMemo(() => {
    try {
      return receipt.read();
    } catch (e) {
      return { error: String(e) };
    }
  }, [receipt, version]);
  const entries = reviewEntries(RECEIVING_FORM, values, "all");
  const missing = entries.filter(
    ([key, field]) => field.required && missingValue(values[key]),
  );
  async function submit() {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError("");
    try {
      if (existing) await receipt.recover();
      else {
        const invalid=findFormValidationIssue(RECEIVING_FORM,values);if(invalid)throw Error(invalid.message);
        if (missing.length) throw Error("未入力の項目があります。");
        await receipt.start(
          "receiving",
          Object.fromEntries(entries.map(([key]) => [key, values[key] ?? ""])),
        );
      }
      refresh((v) => v + 1);
    } catch (e) {
      setError(e instanceof Error ? e.message : "受付結果が不明です。");
      refresh((v) => v + 1);
    } finally {
      setBusy(false);
      inFlight.current = false;
    }
  }
  return (
    <div className={css.overlay}>
      <section
        className={css.editor}
        role="dialog"
        aria-modal="true"
        aria-label="搬入登録の確認"
      >
        <header>
          <h2>搬入登録</h2>
          <button type="button" disabled={busy} onClick={onClose}>
            閉じる
          </button>
        </header>
        {existing && "error" in existing ? (
          <p role="alert">{existing.error}</p>
        ) : existing?.result ? (
          <>
            <p role="status">搬入を登録しました。</p>
            <button
              type="button"
              onClick={() => {
                receipt.next();
                onRegistered();
              }}
            >
              登録結果を確認した
            </button>
          </>
        ) : (
          <>
            <div className={css.confirmFields}>
              {(existing
                ? Object.entries(existing.data)
                : entries.map(([k, field]) => [k, values[k]])
              ).map(([k, value]) => (
                <p key={String(k)}>
                  <span>
                    {RECEIVING_FORM.fields[String(k)]?.label || String(k)}
                  </span>
                  <strong>
                    {Array.isArray(value)
                      ? value.join("、")
                      : String(value ?? "未入力")}
                  </strong>
                </p>
              ))}
            </div>
            {!!missing.length && !existing && (
              <p role="alert">
                未入力：{missing.map(([, field]) => field.label).join("、")}
              </p>
            )}
            <p>
              {existing
                ? "同じ受付番号で登録結果を確認します。"
                : "初期値を含め、点検結果を確認してから登録してください。"}
            </p>
            <button
              className={css.primary}
              type="button"
              disabled={busy || (!existing && !!missing.length)}
              onClick={() => void submit()}
            >
              {existing ? "受付結果を確認する" : "確認して搬入を登録する"}
            </button>
          </>
        )}
        {error && <p role="alert">{error}</p>}
      </section>
    </div>
  );
}
