import { ScaleDriver, WeighResult, ProgressCallback } from './types';

/**
 * RealScaleDriver - A&D HV-60KCWP-K scale driver
 *
 * Uses serial port communication with the A&D protocol:
 *   Commands: Q\r\n (weigh), T\r\n (tare), R\r\n (zero)
 *   Responses: ST (stable), US (unstable), OL (overload), QT/TA (tare done), ZR (zero done)
 *
 * Serial config: 2400bps, 7 data bits, even parity, 1 stop bit
 *
 * TODO: Implement when A&D device arrives
 * Prerequisites:
 *   npm install serialport
 */

const MAX_RETRY = 10;
const RETRY_DELAY_MS = 500;

export interface RealScaleConfig {
  port: string; // e.g. /dev/ttyUSB0
  baudRate: number;
  dataBits: 7 | 8;
  parity: 'none' | 'even' | 'odd';
  stopBits: 1 | 2;
}

export class RealScaleDriver implements ScaleDriver {
  private connected = false;
  private portName: string;
  // private serialPort: any; // Will be SerialPort instance

  constructor(private config: RealScaleConfig) {
    this.portName = config.port;
  }

  async connect(): Promise<void> {
    // TODO: Open serial port
    // const { SerialPort } = require('serialport');
    // this.serialPort = new SerialPort({
    //   path: this.config.port,
    //   baudRate: this.config.baudRate,
    //   dataBits: this.config.dataBits,
    //   parity: this.config.parity,
    //   stopBits: this.config.stopBits,
    // });
    throw new Error(
      'RealScaleDriver not yet implemented. Install serialport and implement A&D protocol.',
    );
  }

  async disconnect(): Promise<void> {
    this.connected = false;
    // TODO: Close serial port
  }

  isConnected(): boolean {
    return this.connected;
  }

  getPort(): string {
    return this.portName;
  }

  async weigh(_onProgress: ProgressCallback): Promise<WeighResult> {
    // TODO: Send "Q\r\n", parse response
    // If header is "US", retry up to MAX_RETRY times with RETRY_DELAY_MS delay
    // If header is "ST", return weight
    // If header is "OL", throw overload error
    throw new Error('Not implemented');
  }

  async tare(): Promise<void> {
    // TODO: Send "T\r\n", wait for QT/TA response
    throw new Error('Not implemented');
  }

  async zero(): Promise<void> {
    // TODO: Send "R\r\n", wait for ZR response
    throw new Error('Not implemented');
  }

  /** List available serial ports (utility for debugging) */
  static async listPorts(): Promise<string[]> {
    try {
      const { SerialPort } = require('serialport');
      const ports = await SerialPort.list();
      return ports.map((p: any) => `${p.path} [${p.manufacturer || 'unknown'}]`);
    } catch {
      return ['serialport module not installed'];
    }
  }
}
