import { PrinterDriver, PrintRequest } from './types';
import { exec } from 'child_process';
import { promisify } from 'util';

const execAsync = promisify(exec);

/**
 * BrotherPrinterDriver
 *
 * For v1, this uses brother-ql or lp command to print to a Brother label printer.
 * On Raspberry Pi, the printer is typically accessible via USB at /dev/usb/lp0.
 *
 * Setup on Pi:
 *   sudo apt-get install printer-driver-ptouch
 *   # Or use brother-ql Python tool:
 *   pip3 install brother_ql
 */
export class BrotherPrinterDriver implements PrinterDriver {
  constructor(private printerName: string) {}

  async isAvailable(): Promise<boolean> {
    try {
      const { stdout } = await execAsync('lpstat -p 2>/dev/null || echo "no cups"');
      return stdout.includes(this.printerName) || stdout.includes('QL');
    } catch {
      return false;
    }
  }

  async testPrint(): Promise<void> {
    console.log(`[BrotherPrinter] Test print requested on ${this.printerName}`);
    // Try lp command first
    try {
      await execAsync(
        `echo "RakuSika Hub Test Print\\n$(date)" | lp -d "${this.printerName}" -`,
      );
      console.log('[BrotherPrinter] Test print sent via lp');
      return;
    } catch (e) {
      console.log('[BrotherPrinter] lp failed, printer may not be configured yet');
      console.log(
        '[BrotherPrinter] To set up: sudo apt-get install printer-driver-ptouch',
      );
      throw new Error(`Printer "${this.printerName}" not available. ${e}`);
    }
  }

  async print(request: PrintRequest): Promise<void> {
    console.log(
      `[BrotherPrinter] Print: template=${request.templateKey}, copies=${request.copies}`,
    );
    console.log('[BrotherPrinter] Data:', JSON.stringify(request.data));
    // TODO: Implement actual label rendering and printing
    // For v1, this would use brother_ql or CUPS with a template system
    throw new Error('Label printing not yet implemented. Use testPrint() for now.');
  }
}
