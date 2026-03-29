package app

import (
	"context"
	"fmt"

	"github.com/deervery/raku-sika-hub/internal/config"
	"github.com/deervery/raku-sika-hub/internal/httpapi"
	"github.com/deervery/raku-sika-hub/internal/logging"
	"github.com/deervery/raku-sika-hub/internal/printer"
	"github.com/deervery/raku-sika-hub/internal/scale"
	"github.com/deervery/raku-sika-hub/internal/scanner"
	"github.com/deervery/raku-sika-hub/internal/ws"
)

// App is the top-level application container.
type App struct {
	cfg           config.Config
	logger        *logging.Logger
	hub           *ws.Hub
	scaleClient   *scale.Client
	printer       *printer.Brother
	scannerClient *scanner.Client
	wsHandler     *ws.Handler
	wsServer      *ws.Server
	httpServer    *httpapi.Server
}

// New creates a new App from the given configuration.
func New(cfg config.Config, version, commit, buildDate string) (*App, error) {
	level := logging.ParseLevel(cfg.LogLevel)
	logger, err := logging.New(logging.LogDir(), level)
	if err != nil {
		return nil, fmt.Errorf("create logger: %w", err)
	}

	hub := ws.NewHub()

	scaleClient := scale.NewClient(cfg, logger, func(connected bool, port string) {
		hub.Broadcast(ws.ConnectionStatus{
			Type:      "connection_status",
			Connected: connected,
			Port:      port,
		})
	})

	prn := printer.NewBrother(cfg.PrinterName, cfg.FontPath, cfg.AssetsDir, logger)

	// Scanner (optional: only created if any scanner config is set)
	var sc *scanner.Client
	if cfg.ScannerDeviceName != "" || cfg.ScannerVid != "" {
		sc = scanner.NewClient(cfg, logger)
		logger.Info("barcode scanner enabled (name=%q vid=%q pid=%q)", cfg.ScannerDeviceName, cfg.ScannerVid, cfg.ScannerPid)
	}

	a := &App{
		cfg:           cfg,
		logger:        logger,
		hub:           hub,
		scaleClient:   scaleClient,
		printer:       prn,
		scannerClient: sc,
	}

	// WebSocket server (optional, disabled by default)
	if cfg.EnableWebSocket {
		wsHandler := ws.NewHandler(scaleClient, prn, hub, logger, cfg.AssetsDir)
		wsServer := ws.NewServer(hub, wsHandler, logger, cfg.ListenAddr, 5)
		a.wsHandler = wsHandler
		a.wsServer = wsServer
	}

	// HTTP REST API server (always enabled)
	var scannerIface httpapi.ScannerClient
	if sc != nil {
		scannerIface = sc
	}
	httpHandler := httpapi.NewHandler(scaleClient, prn, scannerIface, logger, version, commit, buildDate, cfg.AssetsDir, cfg.ProcessorName, cfg.ProcessorLocation, cfg.CaptureLocation)
	a.httpServer = httpapi.NewServer(httpHandler, logger, cfg.ListenAddr)

	return a, nil
}

// Run starts the application and blocks until the context is cancelled.
func (a *App) Run(ctx context.Context) error {
	a.logger.Info("starting raku-sika-hub v0.2 (listen=%s, ws=%v)", a.cfg.ListenAddr, a.cfg.EnableWebSocket)

	a.scaleClient.Start(ctx)

	if a.scannerClient != nil {
		a.scannerClient.Start(ctx)
	}

	if a.cfg.EnableWebSocket && a.wsServer != nil {
		return a.wsServer.Start(ctx)
	}

	return a.httpServer.Start(ctx)
}

// Stop gracefully shuts down all components.
func (a *App) Stop() {
	a.logger.Info("stopping raku-sika-hub")

	ctx := context.Background()
	if a.wsServer != nil {
		a.wsServer.Stop(ctx)
	}
	if a.httpServer != nil {
		a.httpServer.Stop(ctx)
	}
	a.scaleClient.Stop()
	if a.scannerClient != nil {
		a.scannerClient.Stop()
	}
	a.logger.Close()
}
