export interface WeighResult {
  value: number;
  unit: string;
  stable: boolean;
}

export interface WeighProgress {
  retry: number;
  maxRetry: number;
}

export type ProgressCallback = (progress: WeighProgress) => void;

export interface ScaleDriver {
  /** Connect to the scale device */
  connect(): Promise<void>;
  /** Disconnect from the scale device */
  disconnect(): Promise<void>;
  /** Whether the scale is currently connected */
  isConnected(): boolean;
  /** Get current port name */
  getPort(): string;
  /** Request a weight reading. Retries until stable or maxRetry reached. */
  weigh(onProgress: ProgressCallback): Promise<WeighResult>;
  /** Tare the scale */
  tare(): Promise<void>;
  /** Zero the scale */
  zero(): Promise<void>;
}
