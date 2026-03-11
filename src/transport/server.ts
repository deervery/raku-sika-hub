import { WebSocketServer, WebSocket } from 'ws';
import http from 'http';

const MAX_MESSAGE_SIZE = 4096; // 4KB — reject oversized messages

export interface WsServer {
  httpServer: http.Server;
  wss: WebSocketServer;
  broadcast(data: unknown): void;
  start(port: number): Promise<void>;
  stop(): Promise<void>;
}

const ALLOWED_ORIGINS = [
  /^https?:\/\/localhost(:\d+)?$/,
  /^https?:\/\/127\.0\.0\.1(:\d+)?$/,
  /^https?:\/\/(.*\.)?rakusika\.com$/,
  /^https?:\/\/preview\.rakusika\.com$/,
];

function isOriginAllowed(origin: string | undefined): boolean {
  if (!origin) return true; // Allow no-origin (non-browser clients)
  return ALLOWED_ORIGINS.some((pattern) => pattern.test(origin));
}

export function createWsServer(
  onConnection: (ws: WebSocket) => void,
  maxClients: number = 1,
): WsServer {
  const httpServer = http.createServer((_req, res) => {
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok', service: 'raku-sika-hub' }));
  });

  const wss = new WebSocketServer({
    server: httpServer,
    maxPayload: MAX_MESSAGE_SIZE,
    verifyClient: (info: { origin: string; req: http.IncomingMessage }, callback) => {
      // CORS check
      if (!isOriginAllowed(info.origin)) {
        console.warn(`[WS] Connection rejected: origin not allowed (${info.origin})`);
        callback(false, 403, 'Origin not allowed');
        return;
      }

      // Client limit check — count existing OPEN/CONNECTING clients
      const currentClients = [...wss.clients].filter(
        (c) => c.readyState === WebSocket.OPEN || c.readyState === WebSocket.CONNECTING,
      ).length;

      if (currentClients >= maxClients) {
        console.warn(
          `[WS] Connection rejected: client limit reached (${currentClients}/${maxClients}). ` +
          `Another device is already connected. Disconnect it first.`,
        );
        callback(false, 429, 'Too Many Connections: Another client is already connected to raku-sika-hub. Disconnect the existing client first.');
        return;
      }

      callback(true);
    },
  });

  wss.on('connection', (ws, req) => {
    const remoteAddr = req.socket.remoteAddress || 'unknown';
    const origin = req.headers.origin || 'no-origin';
    console.log(`[WS] Client connected from ${remoteAddr} (origin: ${origin}, total: ${wss.clients.size})`);
    onConnection(ws);

    ws.on('close', (code, reason) => {
      console.log(
        `[WS] Client disconnected from ${remoteAddr} (code: ${code}, reason: ${reason.toString() || 'none'}, total: ${wss.clients.size})`,
      );
    });

    ws.on('error', (err) => {
      console.error(`[WS] Client error from ${remoteAddr}:`, err.message);
    });
  });

  function broadcast(data: unknown): void {
    const msg = JSON.stringify(data);
    for (const client of wss.clients) {
      if (client.readyState === WebSocket.OPEN) {
        client.send(msg);
      }
    }
  }

  async function start(port: number): Promise<void> {
    return new Promise((resolve) => {
      httpServer.listen(port, '0.0.0.0', () => {
        console.log(`[WS] Server listening on 0.0.0.0:${port} (max clients: ${maxClients})`);
        resolve();
      });
    });
  }

  async function stop(): Promise<void> {
    return new Promise((resolve, reject) => {
      wss.close(() => {
        httpServer.close((err) => {
          if (err) reject(err);
          else resolve();
        });
      });
    });
  }

  return { httpServer, wss, broadcast, start, stop };
}
