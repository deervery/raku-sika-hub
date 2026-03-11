package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/deervery/raku-sika-hub/internal/logging"
)

// Hub manages connected WebSocket clients and broadcasts messages.
type Hub struct {
	mu      sync.RWMutex
	clients map[*WSClient]struct{}
}

// NewHub creates a new Hub.
func NewHub() *Hub {
	return &Hub{
		clients: make(map[*WSClient]struct{}),
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(c *WSClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(c *WSClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

// ClientCount returns the number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.sendCh <- data:
		default:
			// Drop if send channel is full
		}
	}
}

// WSClient represents a connected WebSocket client.
type WSClient struct {
	conn   *websocket.Conn
	sendCh chan []byte
	hub    *Hub
}

// Send queues a message for sending to this client.
func (c *WSClient) Send(msg any) {
	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case c.sendCh <- data:
	default:
	}
}

// Server is the WebSocket HTTP server.
type Server struct {
	hub             *Hub
	handler         *Handler
	httpSrv         *http.Server
	logger          *logging.Logger
	listenAddr      string
	maxClients      int
	originPatterns  []string
	allowAllOrigins bool
}

// NewServer creates a new WebSocket Server.
func NewServer(hub *Hub, handler *Handler, logger *logging.Logger, listenAddr string, maxClients int, originPatterns []string, allowAllOrigins bool) *Server {
	return &Server{
		hub:             hub,
		handler:         handler,
		logger:          logger,
		listenAddr:      listenAddr,
		maxClients:      maxClients,
		originPatterns:  originPatterns,
		allowAllOrigins: allowAllOrigins,
	}
}

// Start begins listening on the configured address.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/printer/queue", s.handlePrinterQueue)
	mux.HandleFunc("/", s.handleWS)

	s.httpSrv = &http.Server{
		Addr:    s.listenAddr,
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	s.logger.Info(
		"WebSocket server starting on %s (max clients: %d, allowAllOrigins=%t, allowedOrigins=%v)",
		s.listenAddr,
		s.maxClients,
		s.allowAllOrigins,
		s.originPatterns,
	)
	err := s.httpSrv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http listen: %w", err)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if s.handleCORS(w, r, http.MethodGet) {
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	snapshot := s.handler.SnapshotHealth()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if err := json.NewEncoder(w).Encode(snapshot); err != nil {
		s.logger.Warn("health encode: %v", err)
	}
}

func (s *Server) handlePrinterQueue(w http.ResponseWriter, r *http.Request) {
	if s.handleCORS(w, r, http.MethodGet, http.MethodDelete) {
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	switch r.Method {
	case http.MethodGet:
		status, err := s.handler.PrinterQueue()
		if err != nil {
			s.writePrinterQueueError(w, err)
			return
		}
		if err := json.NewEncoder(w).Encode(status); err != nil {
			s.logger.Warn("printer queue encode: %v", err)
		}
	case http.MethodDelete:
		status, err := s.handler.ClearPrinterQueue()
		if err != nil {
			s.writePrinterQueueError(w, err)
			return
		}
		if err := json.NewEncoder(w).Encode(status); err != nil {
			s.logger.Warn("printer queue clear encode: %v", err)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) writePrinterQueueError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	if msg := err.Error(); msg != "" {
		switch {
		case containsAny(msg, "PRINTER_NOT_CONFIGURED:", "unknown destination"):
			status = http.StatusBadRequest
		case containsAny(msg, "PRINTER_PERMISSION_DENIED:", "Permission denied"):
			status = http.StatusForbidden
		}
	}
	w.WriteHeader(status)
	if encodeErr := json.NewEncoder(w).Encode(map[string]string{"message": err.Error()}); encodeErr != nil {
		s.logger.Warn("printer queue error encode: %v", encodeErr)
	}
}

func containsAny(text string, patterns ...string) bool {
	for _, pattern := range patterns {
		if pattern != "" && strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

func (s *Server) handleCORS(w http.ResponseWriter, r *http.Request, methods ...string) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}

	if !s.allowAllOrigins && !originAllowed(origin, s.originPatterns) {
		http.Error(w, "forbidden origin", http.StatusForbidden)
		return true
	}

	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Vary", "Origin")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(methods, ", "))

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return true
	}
	return false
}

func originAllowed(origin string, patterns []string) bool {
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	host := u.Host
	if host == "" {
		return false
	}
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		if ok, _ := path.Match(pattern, host); ok {
			return true
		}
		if hostOnly := strings.Split(host, ":")[0]; hostOnly != host {
			if ok, _ := path.Match(pattern, hostOnly); ok {
				return true
			}
		}
	}
	return false
}

// Stop gracefully shuts down the HTTP server.
func (s *Server) Stop(ctx context.Context) error {
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	// Check client limit before accepting
	if s.hub.ClientCount() >= s.maxClients {
		s.logger.Warn("connection rejected: client limit reached (%d/%d). remote=%s",
			s.hub.ClientCount(), s.maxClients, r.RemoteAddr)
		http.Error(w,
			"Too Many Connections: 既に別のクライアントが接続中です。既存の接続を切断してから再試行してください。",
			http.StatusTooManyRequests)
		return
	}

	acceptOptions := &websocket.AcceptOptions{}
	if !s.allowAllOrigins {
		acceptOptions.OriginPatterns = s.originPatterns
	}

	conn, err := websocket.Accept(w, r, acceptOptions)
	if err != nil {
		s.logger.Warn("ws accept: %v (remote=%s)", err, r.RemoteAddr)
		return
	}

	client := &WSClient{
		conn:   conn,
		sendCh: make(chan []byte, 32),
		hub:    s.hub,
	}
	s.hub.Register(client)
	s.logger.Info("client connected (remote=%s, total=%d)", r.RemoteAddr, s.hub.ClientCount())

	// Send current connection status on connect
	s.handler.SendCurrentStatus(client)

	ctx := r.Context()

	// Write pump
	go func() {
		defer func() {
			s.hub.Unregister(client)
			conn.CloseNow()
			s.logger.Info("client disconnected (remote=%s, total=%d)", r.RemoteAddr, s.hub.ClientCount())
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-client.sendCh:
				if !ok {
					return
				}
				writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Write(writeCtx, websocket.MessageText, msg)
				cancel()
				if err != nil {
					return
				}
			}
		}
	}()

	// Read pump (blocks)
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return
		}
		s.handler.HandleMessage(ctx, client, data)
	}
}
