package httpapi

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

// Server is the HTTP REST API server.
type Server struct {
	handler    *Handler
	httpSrv    *http.Server
	logger     *logging.Logger
	listenAddr string
}

// NewServer creates a new HTTP API server.
func NewServer(handler *Handler, logger *logging.Logger, listenAddr string) *Server {
	return &Server{
		handler:    handler,
		logger:     logger,
		listenAddr: listenAddr,
	}
}

// Start begins listening. It blocks until the server is shut down.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handler.HandleHealth)
	mux.HandleFunc("/version", s.handler.HandleVersion)
	mux.HandleFunc("/scale/weigh", s.handler.HandleScaleWeigh)
	mux.HandleFunc("/scale/tare", s.handler.HandleScaleTare)
	mux.HandleFunc("/scale/zero", s.handler.HandleScaleZero)
	mux.HandleFunc("/printer/print", s.handler.HandlePrinterPrint)
	mux.HandleFunc("/printer/preview", s.handler.HandlePrinterPreview)
	mux.HandleFunc("/printer/test", s.handler.HandlePrinterTest)
	mux.HandleFunc("/printer/queue", s.handler.HandlePrinterQueue)
	mux.HandleFunc("/scanner/scan", s.handler.HandleScannerScan)

	// Apply middleware: CORS → LAN restriction → routes
	var handler http.Handler = mux
	handler = LANOnly(handler)
	handler = CORS(handler)

	s.httpSrv = &http.Server{
		Addr:    s.listenAddr,
		Handler: handler,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	s.logger.Info("HTTP API server starting on %s", s.listenAddr)
	err := s.httpSrv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http listen: %w", err)
	}
	return nil
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}
