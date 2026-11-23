import {
  buildColdStorageFields,
  DEFAULT_COLD_STORAGES,
  LEGACY_TEMP_FIELD_KEYS,
} from "../../lib/constants/cold-storage";
import { validDate } from "../../lib/voice-guidance/labelEditor";
import {
  forwardRef,
  useImperativeHandle,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import type { FormDefinition, FormField } from "../../types/forms";
import {
  FACILITY_REPORT_DAILY_FORM,
  FACILITY_REPORT_WEEKLY_FORM,
  FACILITY_REPORT_MONTHLY_FORM,
  FACILITY_REPORT_EMPLOYEE_CHECK_FORM,
} from "../../lib/constants/form-definitions";
import {
  reviewEntries,
  missingValue,
  validateManualPatch,
} from "../../lib/voice-guidance/stationUi";
import { draftValues } from "../../draftDefaults";
import { native } from "../../bridge";
import { DurableBusiness } from "../../durableBusiness";
import { CatalogPicker } from "./CatalogPicker";
import { StationFieldEditor } from "./StationFieldEditor";
import { swipeDirection, scanQuery } from "../../catalog";
import css from "./VoiceWorkspace.module.css";
const f = (
  label: string,
  type: FormField["type"] = "text",
  required = false,
  options?: string[],
): FormField => ({ label, type, required, display: true, order: 0, options });
export const inventoryForm = (kind: string): FormDefinition => ({
  formName:
    kind === "meat"
      ? "精肉を登録"
      : kind === "processed"
        ? "加工品を登録"
        : "ペットフードを登録",
  targetCollection: "",
  fields:
    kind === "meat"
      ? {
          productId: f("製品", "text", true),
          individualId: f("個体番号"),
          quantity: { ...f("重量", "number", true), suffix: "kg" },
          storageMethod: {
            ...f("保存方法", "select", true, [
              "frozen:冷凍",
              "refrigerated:冷蔵",
            ]),
            selectValueLabel: true,
          },
          processedAt: f("加工日", "date", true),
          deadlineDate: f("期限日", "date", true),
          metalDetectorChecked: f("金属探知機で確認した", "checkbox", true),
          recorder: f("記録者", "text", true),
          confirmer: f("確認者"),
          notes: f("備考"),
        }
      : {
          productId: f("製品", "text", true),
          quantity: f("数量", "number", true),
          manufacturedAt: f("製造日", "date", true),
          notes: f("備考"),
        },
});
const outboundForm: FormDefinition = {
  formName: "スキャンして出庫",
  targetCollection: "",
  fields: {
    lotId: f("在庫ラベル", "text", true),
    partnerName: f("出庫先", "text", true),
    shippedAt: f("出庫日", "date", true),
    recorder: f("記録者", "text", true),
    confirmer: f("確認者"),
  },
};
export type BusinessHandle = {
  patch: (raw: unknown) => object;
  context: () => {
    operation: string;
    fields: Record<string, { label: string; type: string }>;
  };
};
export const BusinessForm = forwardRef<
  BusinessHandle,
  {
    kind: string;
    scope: string;
    onClose: () => void;
    parent?: { id: string; date: string };
  }
>(function BusinessForm({ kind, scope, onClose, parent }, ref) {
  const [employee, setEmployee] = useState(false);
  const base =
    kind === "employee-checks"
      ? FACILITY_REPORT_EMPLOYEE_CHECK_FORM
      : kind === "daily"
        ? FACILITY_REPORT_DAILY_FORM
        : kind === "weekly"
          ? FACILITY_REPORT_WEEKLY_FORM
          : kind === "monthly"
            ? FACILITY_REPORT_MONTHLY_FORM
            : kind === "outbound"
              ? outboundForm
              : inventoryForm(kind);
  const key =
    "business-draft:v035:" +
    scope +
    ":" +
    kind +
    (parent ? ":" + parent.id : "");
  const [overrides, setOverrides] = useState<Record<string, unknown>>(() => {
    try {
      return JSON.parse(localStorage.getItem(key) || "{}");
    } catch {
      return {};
    }
  });
  const [config, setConfig] = useState<Record<string, any> | null>(null),
    [error, setError] = useState(""),
    [editing, setEditing] = useState(""),
    [page, setPage] = useState(0),
    [review, setReview] = useState(false),
    [busy, setBusy] = useState(false),
    [receiptVersion, tick] = useState(0);
  const inFlight = useRef(false),
    alive = useRef(true);
  useEffect(
    () => () => {
      alive.current = false;
    },
    [],
  );
  const [names, setNames] = useState<Record<string, string>>(() => {
    try {
      return JSON.parse(localStorage.getItem(key + ":names") || "{}");
    } catch {
      return {};
    }
  });
  const receipt = useMemo(
    () => new DurableBusiness(localStorage, key + ":receipt", native),
    [key],
  );
  const existing = useMemo(() => {
    try {
      return receipt.read();
    } catch (e) {
      return { error: String(e) };
    }
  }, [receipt, receiptVersion]);
  useEffect(() => {
    let current = true;
    native("catalog", { kind: "configuration" })
      .then((r) => {
        if (current) setConfig(r.item);
      })
      .catch((e) => {
        if (current) setError(e.message);
      });
    return () => {
      current = false;
    };
  }, []);
  const form = useMemo(() => {
    const fields = { ...base.fields };
    const employees = Array.isArray(config?.employees)
      ? config.employees
      : typeof config?.employees === "string"
        ? config.employees.split(/\r?\n/)
        : [];
    for (const [k, field] of Object.entries(fields))
      if (
        [
          "recorder",
          "confirmer",
          "responsiblePerson",
          "hygieneManager",
          "employeeName",
        ].includes(k)
      )
        fields[k] = {
          ...field,
          type: employees.length ? "select" : "text",
          options: employees,
        };
    if (kind === "daily") {
      for (const key of LEGACY_TEMP_FIELD_KEYS) delete fields[key];
      Object.assign(
        fields,
        buildColdStorageFields(
          Array.isArray(config?.coldStorages) && config.coldStorages.length
            ? config.coldStorages
            : DEFAULT_COLD_STORAGES,
          71,
          { omitDefaultValue: true },
        ),
      );
    }
    return { ...base, fields };
  }, [base, config, kind]);
  const values = draftValues(
      form,
      parent ? { ...overrides, date: parent.date } : overrides,
    ),
    entries = reviewEntries(form, values, "all");
  const [capacity,setCapacity]=useState(()=>window.innerHeight<760?4:6);
  useEffect(()=>{const resize=()=>{setCapacity(window.innerHeight<760?4:6);setPage(0);};window.addEventListener('resize',resize);return()=>window.removeEventListener('resize',resize);},[]);
  const cards = entries.slice(page * capacity, page * capacity + capacity);
  const pages = Math.max(1, Math.ceil(entries.length / capacity));
  function update(
    patch: Record<string, unknown>,
    labels?: Record<string, string>,
  ) {
    if (existing || inFlight.current) return;
    setReview(false);
    setOverrides((old) => {
      const next = { ...old, ...patch };
      localStorage.setItem(key, JSON.stringify(next));
      return next;
    });
    if (labels)
      setNames((old) => {
        const next = { ...old, ...labels };
        localStorage.setItem(key + ":names", JSON.stringify(next));
        return next;
      });
  }
  function issues() {
    const errors = entries
      .filter(([k, field]) => field.required && missingValue(values[k]))
      .map(([, field]) => field.label + "を入力してください。");
    if (
      ["meat", "processed", "pet"].includes(kind) &&
      (!Number.isFinite(Number(values.quantity)) ||
        Number(values.quantity) <= 0)
    )
      errors.push("正の数量を入力してください。");
    if(kind === "meat" && !/^(?:\d+|\d+\.\d{1,2}|0?\.\d{1,2})$/.test(String(values.quantity)))errors.push("精肉の重量はkg単位で小数第2位まで入力してください。");
    if (
      kind === "meat" &&
      values.metalDetectorChecked !== true &&
      values.metalDetectorChecked !== false
    )
      errors.push("金属探知機の確認結果を選択してください。");
    for (const [key, field] of entries) {
      const value = values[key];
      if (value && field.type === "date" && !validDate(String(value)))
        errors.push(field.label + "を確認してください。");
      if (
        value &&
        field.type === "month" &&
        !/^\d{4}-(0[1-9]|1[0-2])$/.test(String(value))
      )
        errors.push(field.label + "を確認してください。");
    }
    return errors;
  }
  async function send(recover = false) {
    if (inFlight.current) return;
    inFlight.current = true;
    setBusy(true);
    setError("");
    try {
      if (recover) await receipt.recover();
      else {
        const errors = issues();
        if (errors.length) throw Error(errors[0]);
        const data = Object.fromEntries(
          entries.map(([k]) => [k, values[k] ?? ""]),
        );
        if (parent) data.dailyReportId = parent.id;
        if (kind === "outbound") {
          delete data.lotId;
          data.status = "shipped";
        }
        await receipt.start(
          kind,
          data,
          kind === "outbound" ? String(values.lotId) : undefined,
        );
      }
      if (alive.current) tick((v) => v + 1);
    } catch (e) {
      if (alive.current) {
        setError(e instanceof Error ? e.message : "結果が不明です。");
        tick((v) => v + 1);
      }
    } finally {
      inFlight.current = false;
      if (alive.current) setBusy(false);
    }
  }
  async function weigh() {
    setBusy(true);
    setError("");
    try {
      const r = await native("weigh");
      if (r.status !== "ok" || !r.stable || !["kg", "g"].includes(r.unit))
        throw Error(r.message || "安定した計量値を取得できません。");
      const value = Number(r.value) / (r.unit === "g" ? 1000 : 1);
      if (!Number.isFinite(value) || value <= 0)
        throw Error("重量を確認してください。");
      update({ quantity: String(value) });
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
    } finally {
      setBusy(false);
    }
  }
  async function scan(camera: boolean) {
    setBusy(true);
    setError("");
    try {
      const r = scanQuery((await native(camera ? "cameraScan" : "scan")).value);
      if ("query" in r)
        throw Error("出庫する在庫ラベルのQRコードを読み取ってください。");
      const lot = (await native("catalog", { kind: "lot", id: r.id })).item;
      if (lot.status !== "in_stock") throw Error("この在庫は出庫できません。");
      update(
        { lotId: r.id },
        {
          lotId: [
            lot.productName,
            lot.currentWeightKg + " kg",
            lot.individualNumber,
          ]
            .filter(Boolean)
            .join(" / "),
        },
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "読み取れませんでした。");
    } finally {
      setBusy(false);
    }
  }
  useImperativeHandle(ref, () => ({
    context: () => ({
      operation: "business:" + kind,
      fields: Object.fromEntries(
        entries.map(([key, field]) => [
          key,
          { label: field.label, type: field.type },
        ]),
      ),
    }),
    patch: (raw: unknown) => {
      if (existing || busy || editing)
        throw Error("画面の操作を終えてから訂正してください。");
      if (
        raw &&
        typeof raw === "object" &&
        (raw as Record<string, unknown>).__review
      ) {
        const errors = issues();
        if (errors.length) throw Error(errors[0]);
        setReview(true);
        return { ok: true };
      }
      if (!raw || typeof raw !== "object" || Array.isArray(raw))
        throw Error("項目と値を教えてください。");
      const patch = { ...(raw as Record<string, unknown>) };
      if ("weight" in patch && kind === "meat") {
        patch.quantity = patch.weight;
        delete patch.weight;
      }
      if ("individualNumber" in patch) {
        setEditing("individualId");
        return { question: "個体を候補から選んでください。" };
      }
      if ("productId" in patch || "individualId" in patch || "lotId" in patch)
        throw Error("製品・個体・在庫は候補またはスキャンで選んでください。");
      update(validateManualPatch(patch, form, values));
      return { ok: true };
    },
  }));
  const swipe = useRef<{ x: number; y: number } | null>(null);
  if (employee && existing && "result" in existing && existing.result)
    return (
      <BusinessForm
        kind="employee-checks"
        scope={scope}
        parent={{
          id: String(existing.result.id),
          date: String(existing.data.date),
        }}
        onClose={() => setEmployee(false)}
      />
    );
  return (
    <div className={css.workflowPages}>
      <nav className={css.operationHeader}>
        <button type="button" disabled={busy} onClick={onClose}>
          戻る
        </button>
        <strong>{base.formName}</strong>
      </nav>
      {existing ? (
        <section className={css.workflowReview}>
          <h2>
            {"result" in existing && existing.result
              ? "登録済み"
              : "受付結果を確認"}
          </h2>
          {"data" in existing &&
            Object.entries(existing.data).map(([k, v]) => (
              <p key={k}>
                {form.fields[k]?.label || k}：{String(v)}
              </p>
            ))}
          {"error" in existing ? (
            <p role="alert">{existing.error}</p>
          ) : "result" in existing && existing.result ? (
            <>
              <p role="status">登録結果を確認しました。</p>
              {kind === "daily" && (
                <>
                  <p>続けて、この日報に従業員ごとの健康確認を記録できます。</p>
                  <button type="button" onClick={() => setEmployee(true)}>
                    従業員の健康確認を記録
                  </button>
                </>
              )}
              <button
                type="button"
                onClick={() => {
                  receipt.next();
                  localStorage.removeItem(key);
                  setOverrides({});
                  setNames({});
                  localStorage.removeItem(key + ":names");
                  tick((v) => v + 1);
                  setReview(false);
                }}
              >
                次の登録へ
              </button>
            </>
          ) : (
            <>
              <p>
                同じ受付番号・同じ内容でサーバーに結果を確認します。二重登録はしません。
              </p>
              <button
                type="button"
                disabled={busy}
                onClick={() => void send(true)}
              >
                受付結果を確認する
              </button>
            </>
          )}
        </section>
      ) : review ? (
        <section className={css.workflowReview}>
          <h2>この内容で登録</h2>
          {entries.map(([k, field]) => (
            <button
              className={css.card}
              type="button"
              key={k}
              onClick={() => {
                setReview(false);
                setEditing(k);
              }}
            >
              <span>{field.label}</span>
              <strong>{names[k] || String(values[k] ?? "未入力")+(values[k]&&field.suffix?" "+field.suffix:"")}</strong>
            </button>
          ))}
          <p>初期値を含め、点検結果と対象施設を確認してください。</p>
          <strong>{config?.name || "施設を確認中"}</strong>
          <button
            className={css.primary}
            type="button"
            disabled={busy || !config}
            onClick={() => void send()}
          >
            確認して登録する
          </button>
        </section>
      ) : (
        <>
          <div className={css.workflowActions}>
            {kind === "meat" && (
              <button
                type="button"
                disabled={busy}
                onClick={() => void weigh()}
              >
                計量器から重量を取得
              </button>
            )}
            {kind === "outbound" && (
              <>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void scan(true)}
                >
                  カメラで読み取る
                </button>
                <button
                  type="button"
                  disabled={busy}
                  onClick={() => void scan(false)}
                >
                  スキャナで読み取る
                </button>
              </>
            )}
          </div>
          <div
            className={css.cards}
            data-count={cards.length}
          data-workflow-cards="true"
            onTouchStart={(e) => {
              swipe.current = {
                x: e.touches[0].clientX,
                y: e.touches[0].clientY,
              };
            }}
            onTouchEnd={(e) => {
              if (swipe.current) {
                const d = swipeDirection(
                  e.changedTouches[0].clientX - swipe.current.x,
                  e.changedTouches[0].clientY - swipe.current.y,
                );
                setPage((p) => Math.max(0, Math.min(pages - 1, p + d)));
                swipe.current = null;
              }
            }}
          >
            {cards.map(([k, field]) => (
              <button
                className={css.card}
                type="button"
                key={k}
                disabled={busy || k === "lotId" || (!!parent && k === "date")}
                data-default={!Object.hasOwn(overrides, k)}
                data-missing={missingValue(values[k])}
                onClick={() => setEditing(k)}
              >
                <span className={css.label}>
                  {field.label}
                  {field.required ? "・必須" : ""}
                </span>
                <strong>
                  {names[k] ||
                    (typeof values[k] === "boolean"
                      ? values[k]
                        ? "はい"
                        : "いいえ"
                      : String(values[k] ?? "未入力")+(values[k]&&field.suffix?" "+field.suffix:"")) ||
                    "未入力"}
                </strong>
              </button>
            ))}
          </div>
          <nav className={css.workflowActions} data-workflow-footer="true">
            <button
              type="button"
              disabled={!page}
              onClick={() => setPage(page - 1)}
            >
              前へ
            </button>
            <span>
              {page + 1}/{pages}
            </span>
            <button
              type="button"
              disabled={page + 1 >= pages}
              onClick={() => setPage(page + 1)}
            >
              次へ
            </button>
            <button
              type="button"
              disabled={busy}
              onClick={() => {
                const errors = issues();
                if (errors.length) setError(errors[0]);
                else {
                  setReview(true);
                  setError("");
                }
              }}
            >
              登録内容を確認
            </button>
          </nav>
        </>
      )}
      {error && <p role="alert">{error}</p>}
      {busy && <p role="status">確認中…</p>}
      {editing && (
        <div className={css.overlay}>
          {["productId", "individualId"].includes(editing) ? (
            <CatalogPicker
              kind={editing === "productId" ? "products" : "individuals"}
              family={editing === "productId" ? kind : undefined}
              onClose={() => setEditing("")}
              onSelect={(row) => {
                if (
                  editing === "productId" &&
                  ((kind === "meat" && row.isTraceable !== true) ||
                    (kind !== "meat" && row.isTraceable === true))
                ) {
                  setError("この登録種別に対応する製品を選んでください。");
                  return;
                }
                update(
                  { [editing]: row.id },
                  { [editing]: String(row.name || row.individualNumber) },
                );
                setEditing("");
              }}
            />
          ) : (
            <StationFieldEditor
              key={editing}
              fieldKey={editing}
              field={form.fields[editing]}
              value={values[editing]}
              onClose={() => setEditing("")}
              onSave={(value) => {
                const patch = validateManualPatch(
                  { [editing]: value },
                  form,
                  values,
                );
                update(patch);
                if (
                  ["select", "radio", "checkbox"].includes(
                    form.fields[editing].type,
                  )
                )
                  setEditing("");
              }}
            />
          )}
        </div>
      )}
    </div>
  );
});
