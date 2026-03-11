import { SerialPort } from 'serialport';
import { ReadlineParser } from '@serialport/parser-readline';
import { ScaleDriver, WeighResult, ProgressCallback } from './types';

const MAX_RETRY = 10;
const RETRY_DELAY_MS = 500;
const COMMAND_TIMEOUT_MS = 3000;

export interface RealScaleConfig {
  port: string; // e.g. /dev/ttyUSB0, or '' for auto-detect
  baudRate: number;
  dataBits: 7 | 8;
  parity: 'none' | 'even' | 'odd';
  stopBits: 1 | 2;
  ftdiVendorId: string;
  ftdiProductId: string;
}

// A&D response header codes
type ResponseHeader = 'ST' | 'US' | 'OL' | 'QT' | 'TA' | 'ZR';

interface ParsedResponse {
  header: ResponseHeader;
  value: number;
  unit: string;
}

export class RealScaleDriver implements ScaleDriver {
  private connected = false;
  private portName = '';
  private serialPort: SerialPort | null = null;
  private parser: ReadlineParser | null = null;
  private commandMutex = false;

  constructor(private config: RealScaleConfig) {}

  async connect(): Promise<void> {
    // Auto-detect FTDI port if not specified
    const portPath = this.config.port || await this.detectFtdiPort();

    this.serialPort = new SerialPort({
      path: portPath,
      baudRate: this.config.baudRate,
      dataBits: this.config.dataBits,
      parity: this.config.parity,
      stopBits: this.config.stopBits,
      autoOpen: false,
    });

    this.parser = this.serialPort.pipe(new ReadlineParser({ delimiter: '\r\n' }));

    await new Promise<void>((resolve, reject) => {
      this.serialPort!.open((err) => {
        if (err) reject(new Error(`Failed to open ${portPath}: ${err.message}`));
        else resolve();
      });
    });

    this.serialPort.on('close', () => {
      console.log('[RealScale] Port closed');
      this.connected = false;
    });

    this.serialPort.on('error', (err) => {
      console.error('[RealScale] Port error:', err.message);
      this.connected = false;
    });

    // Verify communication with a test query
    try {
      await this.sendCommand('Q\r\n');
      this.portName = portPath;
      this.connected = true;
      console.log(`[RealScale] Connected on ${portPath}`);
    } catch (err) {
      this.serialPort.close();
      this.serialPort = null;
      this.parser = null;
      throw new Error(`Scale not responding on ${portPath}: ${err instanceof Error ? err.message : err}`);
    }
  }

  async disconnect(): Promise<void> {
    this.connected = false;
    if (this.serialPort?.isOpen) {
      await new Promise<void>((resolve) => {
        this.serialPort!.close(() => resolve());
      });
    }
    this.serialPort = null;
    this.parser = null;
  }

  isConnected(): boolean {
    return this.connected;
  }

  getPort(): string {
    return this.portName;
  }

  async weigh(onProgress: ProgressCallback): Promise<WeighResult> {
    this.ensureConnected();

    for (let retry = 0; retry < MAX_RETRY; retry++) {
      onProgress({ retry: retry + 1, maxRetry: MAX_RETRY });

      const line = await this.sendCommand('Q\r\n');
      const parsed = this.parseWeighResponse(line);

      if (parsed.header === 'ST') {
        return { value: parsed.value, unit: parsed.unit, stable: true };
      }

      if (parsed.header === 'OL') {
        throw new Error('OVERLOAD: Scale overloaded');
      }

      // US (unstable) — wait and retry
      if (retry < MAX_RETRY - 1) {
        await this.delay(RETRY_DELAY_MS);
      }
    }

    throw new Error('UNSTABLE: Weight not stable after max retries');
  }

  async tare(): Promise<void> {
    this.ensureConnected();
    const line = await this.sendCommand('T\r\n');
    const header = line.substring(0, 2).trim();
    if (header !== 'QT' && header !== 'TA') {
      throw new Error(`Tare failed: unexpected response "${line}"`);
    }
  }

  async zero(): Promise<void> {
    this.ensureConnected();
    const line = await this.sendCommand('Z\r\n');
    const header = line.substring(0, 2).trim();
    if (header !== 'ZR') {
      throw new Error(`Zero failed: unexpected response "${line}"`);
    }
  }

  /** List available serial ports (utility for debugging) */
  static async listPorts(): Promise<string[]> {
    const ports = await SerialPort.list();
    return ports.map((p) => `${p.path} [VID:${p.vendorId || '?'} PID:${p.productId || '?'} ${p.manufacturer || ''}]`);
  }

  // --- Private methods ---

  private async detectFtdiPort(): Promise<string> {
    const ports = await SerialPort.list();
    const vid = this.config.ftdiVendorId.toLowerCase();
    const pid = this.config.ftdiProductId.toLowerCase();

    const match = ports.find(
      (p) =>
        p.vendorId?.toLowerCase() === vid &&
        p.productId?.toLowerCase() === pid,
    );

    if (!match) {
      throw new Error(
        `FTDI device not found (VID:${vid} PID:${pid}). Available ports: ${ports.map((p) => p.path).join(', ') || 'none'}`,
      );
    }

    console.log(`[RealScale] Auto-detected FTDI device at ${match.path}`);
    return match.path;
  }

  private sendCommand(cmd: string): Promise<string> {
    return new Promise(async (resolve, reject) => {
      if (this.commandMutex) {
        reject(new Error('Command already in progress'));
        return;
      }

      this.commandMutex = true;

      const timeout = setTimeout(() => {
        this.commandMutex = false;
        cleanup();
        reject(new Error('TIMEOUT: No response from scale'));
      }, COMMAND_TIMEOUT_MS);

      const onLine = (line: string) => {
        clearTimeout(timeout);
        this.commandMutex = false;
        cleanup();
        resolve(line);
      };

      const cleanup = () => {
        this.parser?.removeListener('data', onLine);
      };

      this.parser!.on('data', onLine);

      this.serialPort!.write(cmd, (err) => {
        if (err) {
          clearTimeout(timeout);
          this.commandMutex = false;
          cleanup();
          reject(new Error(`Write failed: ${err.message}`));
        }
      });
    });
  }

  /**
   * Parse A&D weigh response.
   * Format: "HD,+NNNNN.NN  UU" or "HD,+NNNNN.NN UU"
   * where HD = header (ST/US/OL), value is signed decimal, UU is unit (g/kg)
   */
  private parseWeighResponse(line: string): ParsedResponse {
    // e.g. "ST,+00123.45  g" or "US,-00001.20 kg"
    const match = line.match(/^(ST|US|OL|QT|TA|ZR),\s*([+-]?\d+\.?\d*)\s+(\S+)/);
    if (match) {
      return {
        header: match[1] as ResponseHeader,
        value: parseFloat(match[2]),
        unit: match[3],
      };
    }

    // Header-only responses (e.g. "QT" or "ZR")
    const headerOnly = line.substring(0, 2).trim();
    if (['ST', 'US', 'OL', 'QT', 'TA', 'ZR'].includes(headerOnly)) {
      return { header: headerOnly as ResponseHeader, value: 0, unit: '' };
    }

    throw new Error(`Unparseable scale response: "${line}"`);
  }

  private ensureConnected(): void {
    if (!this.connected || !this.serialPort?.isOpen) {
      throw new Error('Scale not connected');
    }
  }

  private delay(ms: number): Promise<void> {
    return new Promise((resolve) => setTimeout(resolve, ms));
  }
}
