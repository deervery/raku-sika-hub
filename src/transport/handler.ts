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

type ScaleRequest =
  | WeighRequest
  | TareRequest
  | ZeroRequest
  | HealthRequest
  | StatusRequest;

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
      const msg: ErrorResponse = {
        type: 'error',
        requestId,
        code: errorToCode(err),
        message: err instanceof Error ? err.message : String(err),
      };
      send(ws, msg);
    }
  }

  async function handleTare(ws: WebSocket, requestId: string): Promise<void> {
    try {
      await scale.tare();
      const msg: TareOkResponse = { type: 'tare_ok', requestId };
      send(ws, msg);
    } catch (err) {
      const msg: ErrorResponse = {
        type: 'error',
        requestId,
        code: 'PORT_ERROR',
        message: err instanceof Error ? err.message : String(err),
      };
      send(ws, msg);
    }
  }

  async function handleZero(ws: WebSocket, requestId: string): Promise<void> {
    try {
      await scale.zero();
      const msg: ZeroOkResponse = { type: 'zero_ok', requestId };
      send(ws, msg);
    } catch (err) {
      const msg: ErrorResponse = {
        type: 'error',
        requestId,
        code: 'PORT_ERROR',
        message: err instanceof Error ? err.message : String(err),
      };
      send(ws, msg);
    }
  }

  async function handleHealth(ws: WebSocket, requestId: string): Promise<void> {
    const state = deviceState.get();
    const msg: HealthOkResponse = {
      type: 'health_ok',
      requestId,
      connected: state.scaleConnected,
      port: state.scalePort || undefined,
    };
    send(ws, msg);
  }

  function handleMessage(ws: WebSocket, raw: string): void {
    let req: ScaleRequest;
    try {
      req = JSON.parse(raw);
    } catch {
      send(ws, {
        type: 'error',
        requestId: '',
        code: 'INVALID_REQUEST',
        message: 'Invalid JSON',
      });
      return;
    }

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
      default:
        send(ws, {
          type: 'error',
          requestId,
          code: 'UNKNOWN_TYPE',
          message: `Unknown request type: ${(req as BaseRequest).type}`,
        });
    }
  }

  return { handleMessage, sendConnectionStatus };
}

function errorToCode(err: unknown): string {
  const msg = err instanceof Error ? err.message : String(err);
  if (msg.includes('not connected')) return 'PORT_ERROR';
  if (msg.includes('unstable') || msg.includes('UNSTABLE')) return 'UNSTABLE';
  if (msg.includes('overload') || msg.includes('OVERLOAD')) return 'OVERLOAD';
  if (msg.includes('timeout') || msg.includes('TIMEOUT')) return 'TIMEOUT';
  return 'PORT_ERROR';
}
