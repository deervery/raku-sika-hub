export interface PrintRequest {
  /** Template identifier */
  templateKey: string;
  /** Data to fill into the template */
  data: Record<string, unknown>;
  /** Number of copies */
  copies: number;
}

export interface PrinterDriver {
  /** Check if the printer is available */
  isAvailable(): Promise<boolean>;
  /** Print a test page */
  testPrint(): Promise<void>;
  /** Print a label with data */
  print(request: PrintRequest): Promise<void>;
}
