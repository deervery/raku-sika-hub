import { WebSocket } from 'ws';
import { ScaleDriver } from '../drivers/scale/types';
import { BrotherPrinterDriver } from '../drivers/printer/brother';
import { DeviceStateManager } from '../state/device';

/** WSA-compatible message types */
interface BaseRequest {
  type: string;
  requestId?: string;
}

interface WeighRequest extends BaseRequest {
  type: 'weigh';
}
interface TareRequest extends BaseRequest {
  type: 'tare';
}
interface ZeroRequest extends BaseRequest {
  type: 'zero';
}
interface HealthRequest extends BaseRequest {
  type: 'health';
}
interface StatusRequest extends BaseRequest {
  type: 'status';
}
interface PrintTestRequest extends BaseRequest {
  type: 'print_test';
}

type ScaleRequest =
  | WeighRequest
  | TareRequest
  | ZeroRequest
  | HealthRequest
  | StatusRequest
  | PrintTestRequest;

// Response types (WSA-compatible)
interface WeightResponse {
  type: 'weight';
  requestId: string;
  value: number;
  unit: string;
  stable: boolean;
}

interface WeighingResponse {
  type: 'weighing';
  requestId: string;
  retry: number;
  maxRetry: number;
}

interface TareOkResponse {
  type: 'tare_ok';
  requestId: string;
}

interface ZeroOkResponse {
  type: 'zero_ok';
  requestId: string;
}

interface HealthOkResponse {
  type: 'health_ok';
  requestId: string;
  connected: boolean;
  port?: string;
  printerConnected: boolean;
}

interface ErrorResponse {
  type: 'error';
  requestId: string;
  code: string;
  message: string;
}

interface ConnectionStatusResponse {
  type: 'connection_status';
  connected: boolean;
  port?: string;
}

function send(ws: WebSocket, data: unknown): void {
  if (ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(data));
  }
}

function sendError(ws: WebSocket, requestId: string, code: string, message: string): void {
  const msg: ErrorResponse = { type: 'error', requestId, code, message };
  send(ws, msg);
}

export function createHandler(
  scale: ScaleDriver,
  printer: BrotherPrinterDriver,
  deviceState: DeviceStateManager,
) {
  function sendConnectionStatus(ws: WebSocket): void {
    const state = deviceState.get();
    const msg: ConnectionStatusResponse = {
      type: 'connection_status',
      connected: state.scaleConnected,
      port: state.scalePort || undefined,
    };
    send(ws, msg);
  }

  async function handleWeigh(ws: WebSocket, requestId: string): Promise<void> {
    // Pre-check: scale connected?
    if (!scale.isConnected()) {
      sendError(ws, requestId, 'SCALE_NOT_CONNECTED',
        'スケールが接続されていません。USBケーブルを確認してください。再接続ループが自動で試行中です。');
      return;
    }

    try {
      const result = await scale.weigh((progress) => {
        const msg: WeighingResponse = {
          type: 'weighing',
          requestId,
          retry: progress.retry,
          maxRetry: progress.maxRetry,
        };
        send(ws, msg);
      });

      deviceState.update({ currentWeight: result.value, currentUnit: result.unit });

      const msg: WeightResponse = {
        type: 'weight',
        requestId,
        value: result.value,
        unit: result.unit,
        stable: result.stable,
      };
      send(ws, msg);
    } catch (err) {
      const { code, message } = classifyScaleError(err);
      sendError(ws, requestId, code, message);
    }
  }

  async function handleTare(ws: WebSocket, requestId: string): Promise<void> {
    if (!scale.isConnected()) {
      sendError(ws, requestId, 'SCALE_NOT_CONNECTED',
        'スケールが接続されていません。USBケーブルを確認してください。');
      return;
    }

    try {
      await scale.tare();
      const msg: TareOkResponse = { type: 'tare_ok', requestId };
      send(ws, msg);
    } catch (err) {
      const { code, message } = classifyScaleError(err);
      sendError(ws, requestId, code, message);
    }
  }

  async function handleZero(ws: WebSocket, requestId: string): Promise<void> {
    if (!scale.isConnected()) {
      sendError(ws, requestId, 'SCALE_NOT_CONNECTED',
        'スケールが接続されていません。USBケーブルを確認してください。');
      return;
    }

    try {
      await scale.zero();
      const msg: ZeroOkResponse = { type: 'zero_ok', requestId };
      send(ws, msg);
    } catch (err) {
      const { code, message } = classifyScaleError(err);
      sendError(ws, requestId, code, message);
    }
  }

  async function handlePrintTest(ws: WebSocket, requestId: string): Promise<void> {
    // Pre-check: printer available?
    const available = await printer.isAvailable();
    if (!available) {
      sendError(ws, requestId, 'PRINTER_NOT_CONFIGURED',
        `プリンタ "${(printer as any).printerName}" がCUPSに登録されていません。` +
        ' Raspberry Pi上で sudo apt-get install printer-driver-ptouch && CUPS設定 を行ってください。');
      return;
    }

    try {
      await printer.testPrint();
      send(ws, { type: 'print_test_ok', requestId, message: 'テスト印刷を送信しました' });
    } catch (err) {
      const errMsg = err instanceof Error ? err.message : String(err);
      // Classify printer errors
      if (errMsg.includes('Permission denied') || errMsg.includes('EACCES')) {
        sendError(ws, requestId, 'PRINTER_PERMISSION_DENIED',
          'プリンタへのアクセス権限がありません。sudo usermod -aG lpadmin pi を実行してください。');
      } else if (errMsg.includes('not accepting') || errMsg.includes('disabled')) {
        sendError(ws, requestId, 'PRINTER_DISABLED',
          'プリンタが無効化されています。CUPSの管理画面でプリンタを有効にしてください。');
      } else if (errMsg.includes('paper') || errMsg.includes('media')) {
        sendError(ws, requestId, 'PRINTER_PAPER_ERROR',
          'ラベル用紙を確認してください。用紙切れまたは用紙ジャムの可能性があります。');
      } else {
        sendError(ws, requestId, 'PRINTER_ERROR',
          `印刷エラー: ${errMsg}`);
      }
    }
  }

  async function handleHealth(ws: WebSocket, requestId: string): Promise<void> {
    const state = deviceState.get();
    const printerConnected = await printer.isAvailable();
    const msg: HealthOkResponse = {
      type: 'health_ok',
      requestId,
      connected: state.scaleConnected,
      port: state.scalePort || undefined,
      printerConnected,
    };
    send(ws, msg);
  }

  function handleMessage(ws: WebSocket, raw: string): void {
    // Empty message check
    if (!raw || raw.trim().length === 0) {
      sendError(ws, '', 'INVALID_REQUEST', '空のメッセージです。JSON形式で送信してください。');
      return;
    }

    // JSON parse
    let req: BaseRequest;
    try {
      req = JSON.parse(raw);
    } catch {
      sendError(ws, '', 'INVALID_REQUEST',
        `JSONパースエラー。正しいJSON形式で送信してください。受信内容: "${raw.substring(0, 100)}"`);
      return;
    }

    // type field validation
    if (!req.type || typeof req.type !== 'string') {
      sendError(ws, req.requestId || '', 'INVALID_REQUEST',
        'type フィールドが必要です。例: { "type": "weigh", "requestId": "1" }');
      return;
    }

    // requestId validation (warn if missing, but don't reject)
    const requestId = req.requestId || '';

    switch (req.type) {
      case 'weigh':
        handleWeigh(ws, requestId);
        break;
      case 'tare':
        handleTare(ws, requestId);
        break;
      case 'zero':
        handleZero(ws, requestId);
        break;
      case 'health':
        handleHealth(ws, requestId);
        break;
      case 'status':
        sendConnectionStatus(ws);
        break;
      case 'print_test':
        handlePrintTest(ws, requestId);
        break;
      default:
        sendError(ws, requestId, 'UNKNOWN_TYPE',
          `不明なリクエストタイプ: "${req.type}"。` +
          '使用可能: weigh, tare, zero, health, status, print_test');
    }
  }

  return { handleMessage, sendConnectionStatus };
}

/**
 * Classify scale errors into specific error codes with Japanese messages for on-site debugging.
 */
function classifyScaleError(err: unknown): { code: string; message: string } {
  const raw = err instanceof Error ? err.message : String(err);

  // Scale not connected / disconnected mid-operation
  if (raw.includes('not connected') || raw.includes('Scale not connected')) {
    return {
      code: 'SCALE_NOT_CONNECTED',
      message: 'スケールが切断されました。USBケーブルを確認してください。自動再接続を試行中です。',
    };
  }

  // Command mutex busy — another operation in progress
  if (raw.includes('Command already in progress') || raw.includes('already in progress')) {
    return {
      code: 'SCALE_BUSY',
      message: '別のコマンドを処理中です。少し待ってから再試行してください。',
    };
  }

  // Timeout — scale not responding
  if (raw.includes('TIMEOUT') || raw.includes('timeout') || raw.includes('No response')) {
    return {
      code: 'TIMEOUT',
      message: 'スケールから応答がありません（3秒タイムアウト）。' +
        'スケールの電源が入っているか確認してください。USBケーブルの抜き差しも試してください。',
    };
  }

  // Weight unstable
  if (raw.includes('UNSTABLE') || raw.includes('unstable') || raw.includes('not stable')) {
    return {
      code: 'UNSTABLE',
      message: '計量値が安定しません（10回リトライ超過）。' +
        '計量台の上の物が動いていないか、風や振動がないか確認してください。',
    };
  }

  // Overload
  if (raw.includes('OVERLOAD') || raw.includes('overload') || raw.includes('OL')) {
    return {
      code: 'OVERLOAD',
      message: 'スケールが過負荷状態です。最大計量（60kg）を超える荷物が乗っています。',
    };
  }

  // Serial write failed
  if (raw.includes('Write failed')) {
    return {
      code: 'SERIAL_WRITE_ERROR',
      message: 'シリアルポートへの書き込みに失敗しました。USBケーブルが抜けた可能性があります。',
    };
  }

  // Serial port permission denied
  if (raw.includes('Permission denied') || raw.includes('EACCES')) {
    return {
      code: 'PERMISSION_DENIED',
      message: 'シリアルポートのアクセス権限がありません。sudo usermod -aG dialout $USER を実行して再ログインしてください。',
    };
  }

  // Serial port busy / in use by another process
  if (raw.includes('EBUSY') || raw.includes('Resource busy') || raw.includes('locked')) {
    return {
      code: 'PORT_IN_USE',
      message: 'シリアルポートが別のプロセスに使用されています。他にスケールを使用しているプログラムがないか確認してください。',
    };
  }

  // Port open failed
  if (raw.includes('Failed to open') || raw.includes('ENOENT') || raw.includes('No such file')) {
    return {
      code: 'PORT_NOT_FOUND',
      message: `シリアルポートが見つかりません。USBケーブルが接続されているか確認してください。詳細: ${raw}`,
    };
  }

  // FTDI device not found
  if (raw.includes('FTDI device not found')) {
    return {
      code: 'FTDI_NOT_FOUND',
      message: raw + ' USBケーブルを確認し、スケールの電源を入れてください。',
    };
  }

  // Scale not responding to initial query
  if (raw.includes('Scale not responding')) {
    return {
      code: 'SCALE_NO_RESPONSE',
      message: 'ポートは開けましたが、スケールが応答しません。' +
        'ボーレート設定（2400bps）がスケール側と一致しているか確認してください。',
    };
  }

  // Unparseable response
  if (raw.includes('Unparseable') || raw.includes('unexpected response')) {
    return {
      code: 'UNEXPECTED_RESPONSE',
      message: `スケールから予期しない応答がありました。詳細: ${raw}`,
    };
  }

  // Tare/Zero specific failures
  if (raw.includes('Tare failed')) {
    return {
      code: 'TARE_FAILED',
      message: `風袋引きに失敗しました。スケールの状態を確認してください。詳細: ${raw}`,
    };
  }
  if (raw.includes('Zero failed')) {
    return {
      code: 'ZERO_FAILED',
      message: `ゼロリセットに失敗しました。計量台に物が乗っていないか確認してください。詳細: ${raw}`,
    };
  }

  // Fallback — unknown error
  return {
    code: 'UNKNOWN_ERROR',
    message: `予期しないエラーが発生しました: ${raw}`,
  };
}
