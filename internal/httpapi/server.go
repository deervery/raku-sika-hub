package httpapi

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

// adminAssets is the maintenance GUI, compiled into the binary so that
// deploying it is exactly the existing hub deploy (ops/scripts/deploy-hub.sh)
// with no extra service, port or asset directory to keep in sync.
//
//go:embed webadmin
var adminAssets embed.FS

// Server is the HTTP REST API server.
type Server struct {
	handler    *Handler
	wsRoutes   RouteRegistrar
	httpSrv    *http.Server
	logger     *logging.Logger
	listenAddr string
}

// RouteRegistrar mounts extra routes onto the HTTP mux.
type RouteRegistrar interface {
	RegisterRoutes(mux *http.ServeMux)
}

// NewServer creates a new HTTP API server.
func NewServer(handler *Handler, wsRoutes RouteRegistrar, logger *logging.Logger, listenAddr string) *Server {
	return &Server{
		handler:    handler,
		wsRoutes:   wsRoutes,
		logger:     logger,
		listenAddr: listenAddr,
	}
}

// mountAdmin serves the embedded maintenance GUI at /admin/.
func (s *Server) mountAdmin(mux *http.ServeMux) error {
	sub, err := fs.Sub(adminAssets, "webadmin")
	if err != nil {
		return err
	}
	mux.Handle("GET /admin/", http.StripPrefix("/admin/", http.FileServerFS(sub)))
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admin/", http.StatusMovedPermanently)
	})
	return nil
}

// Start begins listening. It blocks until the server is shut down.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", s.handler.HandleHealth)
	mux.HandleFunc("/ws/status", s.handler.HandleWSStatus)
	mux.HandleFunc("/version", s.handler.HandleVersion)
	mux.HandleFunc("/scale/weigh", s.handler.HandleScaleWeigh)
	mux.HandleFunc("/scale/tare", s.handler.HandleScaleTare)
	mux.HandleFunc("/scale/zero", s.handler.HandleScaleZero)
	mux.HandleFunc("/printer/print", s.handler.HandlePrinterPrint)
	mux.HandleFunc("/printer/preview", s.handler.HandlePrinterPreview)
	mux.HandleFunc("/printer/test", s.handler.HandlePrinterTest)
	mux.HandleFunc("/printer/queue", s.handler.HandlePrinterQueue)
	mux.HandleFunc("/printer/jobs/{id}", s.handler.HandlePrinterJob)
	mux.HandleFunc("/scanner/scan", s.handler.HandleScannerScan)
	mux.HandleFunc("/system/network", s.handler.HandleNetwork)
	mux.HandleFunc("/system/network/connect", s.handler.HandleNetworkConnect)
	if err := s.mountAdmin(mux); err != nil {
		// The GUI is a maintenance aid; losing it must not stop the hub from
		// printing, so we log and carry on.
		s.logger.Warn("admin GUI unavailable: %v", err)
	}
	if s.wsRoutes != nil {
		s.wsRoutes.RegisterRoutes(mux)
	}

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
