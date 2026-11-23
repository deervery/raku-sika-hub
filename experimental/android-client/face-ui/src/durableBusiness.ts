export type Receipt = {
  requestId: string;
  kind: string;
  id?: string;
  data: Record<string, unknown>;
  result?: Record<string, unknown>;
};
/** The confirmed snapshot and request ID survive process death. No automatic retry. */
export class DurableBusiness {
  constructor(
    private storage: Pick<Storage, "getItem" | "setItem" | "removeItem">,
    readonly key: string,
    private send: (op: string, args: Record<string, unknown>) => Promise<any>,
  ) {}
  read(): Receipt | null {
    const raw = this.storage.getItem(this.key);
    if (!raw) return null;
    const r = JSON.parse(raw);
    if (
      !r ||
      !/^[A-Za-z0-9:_-]{16,128}$/.test(r.requestId) ||
      !r.kind ||
      !r.data ||
      Array.isArray(r.data)
    )
      throw Error("受付記録が壊れています。新しく送信せず確認してください。");
    return r;
  }
  async start(kind: string, data: Record<string, unknown>, id?: string) {
    if (this.read()) throw Error("前回の受付結果を確認してください。");
    const receipt: Receipt = {
      requestId: crypto.randomUUID(),
      kind,
      id,
      data: JSON.parse(JSON.stringify(data)),
    };
    this.storage.setItem(this.key, JSON.stringify(receipt));
    return this.recover();
  }
  async recover() {
    const receipt = this.read();
    if (!receipt) throw Error("受付記録がありません。");
    if (receipt.result) return receipt.result;
    const result = await this.send("business", { ...receipt, confirmed: true });
    if (!result?.ok || !result.data || typeof result.data.id !== "string")
      throw Error("結果が不明です。同じ受付番号で確認してください。");
    receipt.result = result.data;
    this.storage.setItem(this.key, JSON.stringify(receipt));
    return result.data;
  }
  next() {
    const receipt = this.read();
    if (receipt && !receipt.result)
      throw Error("未確認の受付を先に確認してください。");
    this.storage.removeItem(this.key);
  }
}
