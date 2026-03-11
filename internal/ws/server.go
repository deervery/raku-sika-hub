package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
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
	hub        *Hub
	handler    *Handler
	httpSrv    *http.Server
	logger     *logging.Logger
	listenAddr string
	maxClients int
}

// NewServer creates a new WebSocket Server.
func NewServer(hub *Hub, handler *Handler, logger *logging.Logger, listenAddr string, maxClients int) *Server {
	return &Server{
		hub:        hub,
		handler:    handler,
		logger:     logger,
		listenAddr: listenAddr,
		maxClients: maxClients,
	}
}

// Start begins listening on the configured address.
func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/", s.handleWS)

	s.httpSrv = &http.Server{
		Addr:    s.listenAddr,
		Handler: mux,
		BaseContext: func(l net.Listener) context.Context {
			return ctx
		},
	}

	s.logger.Info("WebSocket server starting on %s (max clients: %d)", s.listenAddr, s.maxClients)
	err := s.httpSrv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("http listen: %w", err)
	}
	return nil
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
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

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{
			"localhost:*",
			"127.0.0.1:*",
			"preview.rakusika.com",
			"rakusika.com",
			"*.rakusika.com",
		},
	})
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
