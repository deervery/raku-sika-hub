import { ScaleDriver, WeighResult, ProgressCallback } from './types';

const MAX_RETRY = 10;
const RETRY_DELAY_MS = 500;

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export class MockScaleDriver implements ScaleDriver {
  private connected = false;
  private baseWeight = 12.34;

  async connect(): Promise<void> {
    this.connected = true;
    console.log('[MockScale] Connected');
  }

  async disconnect(): Promise<void> {
    this.connected = false;
    console.log('[MockScale] Disconnected');
  }

  isConnected(): boolean {
    return this.connected;
  }

  getPort(): string {
    return 'mock';
  }

  async weigh(onProgress: ProgressCallback): Promise<WeighResult> {
    if (!this.connected) throw new Error('not connected');

    // Simulate 1-3 unstable readings before stable
    const unstableCount = Math.floor(Math.random() * 3) + 1;

    for (let i = 1; i <= unstableCount; i++) {
      onProgress({ retry: i, maxRetry: MAX_RETRY });
      await sleep(RETRY_DELAY_MS);
    }

    // Return a slightly varying weight
    const jitter = (Math.random() - 0.5) * 0.1;
    const value = Math.round((this.baseWeight + jitter) * 100) / 100;

    return { value, unit: 'kg', stable: true };
  }

  async tare(): Promise<void> {
    if (!this.connected) throw new Error('not connected');
    this.baseWeight = 0;
    console.log('[MockScale] Tare done');
  }

  async zero(): Promise<void> {
    if (!this.connected) throw new Error('not connected');
    this.baseWeight = 0;
    console.log('[MockScale] Zero done');
  }

  /** For testing: set the base weight the mock will return */
  setWeight(value: number): void {
    this.baseWeight = value;
  }
}
