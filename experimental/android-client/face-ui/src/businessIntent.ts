/** Exact field name + value. No inferred checks, IDs or approvals. */
export function businessIntent(
  text: string,
  fields: Record<string, { label: string; type: string }>,
) {
  const s = text
    .normalize("NFKC")
    .replace(/[。！!]$/, "")
    .trim();
  for (const [key, field] of Object.entries(fields).sort(
    (a, b) => b[1].label.length - a[1].label.length,
  )) {
    const prefix = field.label + "は";
    if (!s.startsWith(prefix)) continue;
    const value = s.slice(prefix.length).replace(/です$/, "").trim();
    if (!value) return { question: field.label + "を教えてください。" };
    if (["productId", "individualId", "lotId"].includes(key))
      return {
        question: field.label + "は画面の候補かスキャンで選んでください。",
      };
    if (field.type === "checkbox") {
      if (value === "はい") return { patch: { [key]: true } };
      if (value === "いいえ") return { patch: { [key]: false } };
      return { question: field.label + "は、はい・いいえで教えてください。" };
    }
    if (field.type === "number" && !/^-?\d+(?:\.\d+)?$/.test(value))
      return null;
    return { patch: { [key]: value } };
  }
  return null;
}
