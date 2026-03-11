export interface HubConfig {
  /** WebSocket server port */
  port: number;
  /** Scale driver: 'mock' | 'real' */
  scaleDriver: 'mock' | 'real';
  /** Serial port for real scale (e.g. /dev/ttyUSB0) */
  serialPort: string;
  /** Serial baud rate */
  baudRate: number;
  /** Serial data bits */
  dataBits: 7 | 8;
  /** Serial parity */
  parity: 'none' | 'even' | 'odd';
  /** Serial stop bits */
  stopBits: 1 | 2;
  /** FTDI vendor ID for auto-detection */
  ftdiVendorId: string;
  /** FTDI product ID for auto-detection */
  ftdiProductId: string;
  /** Printer name */
  printerName: string;
  /** Max concurrent WebSocket clients (v1: single client) */
  maxClients: number;
  /** Log level */
  logLevel: 'ERROR' | 'WARN' | 'INFO' | 'DEBUG';
}

const defaults: HubConfig = {
  port: 19800,
  scaleDriver: 'mock',
  serialPort: '',
  baudRate: 2400,
  dataBits: 7,
  parity: 'even',
  stopBits: 1,
  ftdiVendorId: '0403',
  ftdiProductId: '6015',
  printerName: 'Brother QL-800',
  maxClients: 1,
  logLevel: 'INFO',
};

export function loadConfig(): HubConfig {
  const env = process.env;
  return {
    port: parseInt(env.HUB_PORT || '', 10) || defaults.port,
    scaleDriver: (env.SCALE_DRIVER as 'mock' | 'real') || defaults.scaleDriver,
    serialPort: env.SERIAL_PORT || defaults.serialPort,
    baudRate: parseInt(env.BAUD_RATE || '', 10) || defaults.baudRate,
    dataBits: (parseInt(env.DATA_BITS || '', 10) as 7 | 8) || defaults.dataBits,
    parity: (env.PARITY as 'none' | 'even' | 'odd') || defaults.parity,
    stopBits: (parseInt(env.STOP_BITS || '', 10) as 1 | 2) || defaults.stopBits,
    ftdiVendorId: env.FTDI_VID || defaults.ftdiVendorId,
    ftdiProductId: env.FTDI_PID || defaults.ftdiProductId,
    printerName: env.PRINTER_NAME || defaults.printerName,
    maxClients: parseInt(env.MAX_CLIENTS || '', 10) || defaults.maxClients,
    logLevel: (env.LOG_LEVEL as HubConfig['logLevel']) || defaults.logLevel,
  };
}
