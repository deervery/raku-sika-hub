package app

import (
	"context"
	"fmt"

	"github.com/deervery/raku-sika-hub/internal/config"
	"github.com/deervery/raku-sika-hub/internal/logging"
	"github.com/deervery/raku-sika-hub/internal/printer"
	"github.com/deervery/raku-sika-hub/internal/scale"
	"github.com/deervery/raku-sika-hub/internal/ws"
)

// App is the top-level application container.
type App struct {
	cfg         config.Config
	logger      *logging.Logger
	hub         *ws.Hub
	scaleClient *scale.Client
	printer     printer.Driver
	handler     *ws.Handler
	wsServer    *ws.Server
}

// New creates a new App from the given configuration.
func New(cfg config.Config) (*App, error) {
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

	if cfg.ScaleDriver == "mock" {
		logger.Info("SCALE_DRIVER=mock: モックスケールを使用します")
		scaleClient.SetMockMode()
	}

	prn, err := printer.NewDriver(printer.DriverConfig{
		DriverName:      cfg.PrinterDriver,
		PrinterName:     cfg.PrinterName,
		FontPath:        cfg.FontPath,
		PrinterAddress:  cfg.PrinterAddress,
		TemplateMapPath: cfg.TemplateMapPath,
	}, logger)
	if err != nil {
		return nil, fmt.Errorf("create printer driver: %w", err)
	}
	handler := ws.NewHandler(scaleClient, prn, hub, logger)
	wsServer := ws.NewServer(hub, handler, logger, cfg.ListenAddr, cfg.MaxClients, cfg.AllowedOrigins, cfg.AllowAllOrigins)
	if cfg.TLSCertPath != "" && cfg.TLSKeyPath != "" {
		wsServer.SetTLS(cfg.TLSCertPath, cfg.TLSKeyPath)
	}

	return &App{
		cfg:         cfg,
		logger:      logger,
		hub:         hub,
		scaleClient: scaleClient,
		printer:     prn,
		handler:     handler,
		wsServer:    wsServer,
	}, nil
}

// Run starts the application and blocks until the context is cancelled.
func (a *App) Run(ctx context.Context) error {
	printerName := a.cfg.PrinterName
	if printerName == "" {
		printerName = "(cups fallback)"
	}
	scaleDriver := a.cfg.ScaleDriver
	if scaleDriver == "" {
		scaleDriver = "auto"
	}
	a.logger.Info(
		"starting raku-sika-hub (listen=%s, printer=%s, maxClients=%d, scaleDriver=%s, printerDriver=%s, printerAddress=%s, templateMap=%s)",
		a.cfg.ListenAddr,
		printerName,
		a.cfg.MaxClients,
		scaleDriver,
		a.cfg.PrinterDriver,
		a.cfg.PrinterAddress,
		a.cfg.TemplateMapPath,
	)

	a.scaleClient.Start(ctx)

	// wsServer.Start blocks until shutdown
	err := a.wsServer.Start(ctx)

	a.logger.Info("raku-sika-hub stopped")
	return err
}

// Stop gracefully shuts down all components.
func (a *App) Stop() {
	a.logger.Info("stopping raku-sika-hub")

	ctx := context.Background()
	a.wsServer.Stop(ctx)
	a.scaleClient.Stop()
	a.logger.Close()
}
