import { loadConfig } from './config';
import { DeviceStateManager } from './state/device';
import { MockScaleDriver } from './drivers/scale/mock';
import { RealScaleDriver } from './drivers/scale/real';
import { BrotherPrinterDriver } from './drivers/printer/brother';
import { createWsServer } from './transport/server';
import { createHandler } from './transport/handler';
import { ScaleDriver } from './drivers/scale/types';

const RECONNECT_INTERVAL_MS = 3000;

async function main() {
  const config = loadConfig();
  console.log('[Hub] Starting raku-sika-hub...');
  console.log(`[Hub] Scale driver: ${config.scaleDriver}`);
  console.log(`[Hub] Port: ${config.port}`);

  // Initialize device state
  const deviceState = new DeviceStateManager();

  // Initialize scale driver
  let scale: ScaleDriver;
  if (config.scaleDriver === 'real') {
    scale = new RealScaleDriver({
      port: config.serialPort,
      baudRate: config.baudRate,
      dataBits: config.dataBits,
      parity: config.parity,
      stopBits: config.stopBits,
      ftdiVendorId: config.ftdiVendorId,
      ftdiProductId: config.ftdiProductId,
    });
  } else {
    scale = new MockScaleDriver();
  }

  // Initialize printer driver
  const printer = new BrotherPrinterDriver(config.printerName);

  // Connect scale
  try {
    await scale.connect();
    deviceState.update({
      scaleConnected: true,
      scalePort: scale.getPort(),
    });
    console.log('[Hub] Scale connected');
  } catch (err) {
    console.error('[Hub] Scale connection failed:', err instanceof Error ? err.message : err);
    deviceState.update({ scaleConnected: false });
  }

  // Check printer
  const printerAvailable = await printer.isAvailable();
  deviceState.update({ printerConnected: printerAvailable });
  console.log(`[Hub] Printer available: ${printerAvailable}`);

  // Create handler
  const handler = createHandler(scale, printer, deviceState);

  // Create WebSocket server
  const server = createWsServer((ws) => {
    // Send initial connection status on connect (WSA-compatible behavior)
    handler.sendConnectionStatus(ws);

    ws.on('message', (data) => {
      const raw = data.toString();
      console.log('[WS] Received:', raw);
      handler.handleMessage(ws, raw);
    });
  });

  // Broadcast device state changes to all clients
  deviceState.onUpdate((state) => {
    server.broadcast({
      type: 'connection_status',
      connected: state.scaleConnected,
      port: state.scalePort || undefined,
    });
  });

  // Start server
  await server.start(config.port);
  console.log('[Hub] Ready');

  // Reconnect loop for real scale driver
  let reconnectTimer: ReturnType<typeof setInterval> | null = null;
  if (config.scaleDriver === 'real') {
    reconnectTimer = setInterval(async () => {
      if (!scale.isConnected()) {
        try {
          await scale.connect();
          deviceState.update({
            scaleConnected: true,
            scalePort: scale.getPort(),
          });
          console.log('[Hub] Scale reconnected');
        } catch (err) {
          console.log('[Hub] Scale reconnect failed:', err instanceof Error ? err.message : err);
          deviceState.update({ scaleConnected: false });
        }
      }
    }, RECONNECT_INTERVAL_MS);
  }

  // Graceful shutdown
  const shutdown = async () => {
    console.log('\n[Hub] Shutting down...');
    if (reconnectTimer) clearInterval(reconnectTimer);
    await scale.disconnect();
    await server.stop();
    process.exit(0);
  };

  process.on('SIGINT', shutdown);
  process.on('SIGTERM', shutdown);
}

main().catch((err) => {
  console.error('[Hub] Fatal error:', err);
  process.exit(1);
});
