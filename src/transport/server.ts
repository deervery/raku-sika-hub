import { WebSocketServer, WebSocket } from 'ws';
import http from 'http';

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
): WsServer {
  const httpServer = http.createServer((_req, res) => {
    // Simple HTTP health check endpoint
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ status: 'ok', service: 'raku-sika-hub' }));
  });

  const wss = new WebSocketServer({
    server: httpServer,
    verifyClient: (info: { origin: string }) => isOriginAllowed(info.origin),
  });

  wss.on('connection', (ws) => {
    console.log(`[WS] Client connected (total: ${wss.clients.size})`);
    onConnection(ws);

    ws.on('close', () => {
      console.log(`[WS] Client disconnected (total: ${wss.clients.size})`);
    });

    ws.on('error', (err) => {
      console.error('[WS] Client error:', err.message);
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
        console.log(`[WS] Server listening on 0.0.0.0:${port}`);
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
