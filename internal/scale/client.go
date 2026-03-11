package scale

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/deervery/raku-sika-hub/internal/config"
	"github.com/deervery/raku-sika-hub/internal/logging"
)

const (
	maxWeighRetries = 10
	weighRetryDelay = 500 * time.Millisecond
	reconnectDelay  = 3 * time.Second
	commandTimeout  = 3 * time.Second
)

// StatusFunc is called when connection status changes.
type StatusFunc func(connected bool, port string)

// ProgressFunc is called during weigh retries to report progress.
type ProgressFunc func(retry, maxRetry int)

// PortOpener opens a serial port. Replaced in tests with a mock.
type PortOpener func(name string, cfg config.Config) (Port, error)

// Client manages serial communication with an A&D scale.
type Client struct {
	cfg       config.Config
	mu        sync.Mutex
	port      Port
	reader    *bufio.Reader
	portName  atomic.Value // string
	connected atomic.Bool
	onStatus  StatusFunc
	openPort  PortOpener
	logger    *logging.Logger
	cancel    context.CancelFunc
	done      chan struct{}
}

// NewClient creates a new scale Client.
func NewClient(cfg config.Config, logger *logging.Logger, onStatus StatusFunc) *Client {
	return &Client{
		cfg:      cfg,
		onStatus: onStatus,
		openPort: nil,
		logger:   logger,
	}
}

// Start begins the reconnect loop in a goroutine.
func (c *Client) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	c.done = make(chan struct{})
	if c.openPort == nil {
		c.openPort = defaultPortOpener
	}
	go c.reconnectLoop(ctx)
}

// Stop cancels the reconnect loop and closes the port.
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
		<-c.done
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closePortLocked()
}

// Connected returns true if the scale port is currently open.
func (c *Client) Connected() bool {
	return c.connected.Load()
}

// PortName returns the current port name.
func (c *Client) PortName() string {
	v := c.portName.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

// Weigh sends a weigh command and waits for a stable reading.
func (c *Client) Weigh(ctx context.Context, progress ProgressFunc) (*WeighResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected.Load() {
		return nil, errors.New("not connected")
	}

	for i := 0; i < maxWeighRetries; i++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		result, err := c.sendWeighLocked()
		if err != nil {
			c.closePortLocked()
			c.setStatusLocked(false, "")
			return nil, fmt.Errorf("PORT_ERROR: %w", err)
		}

		switch result.Header {
		case HeaderST:
			return result, nil
		case HeaderOL:
			return nil, errors.New("OVERLOAD")
		case HeaderUS:
			if progress != nil {
				progress(i+1, maxWeighRetries)
			}
			if i < maxWeighRetries-1 {
				c.mu.Unlock()
				select {
				case <-time.After(weighRetryDelay):
				case <-ctx.Done():
					c.mu.Lock()
					return nil, ctx.Err()
				}
				c.mu.Lock()
				if !c.connected.Load() {
					return nil, errors.New("not connected")
				}
			}
		default:
			return nil, fmt.Errorf("unexpected header: %s", result.Header)
		}
	}

	return nil, errors.New("UNSTABLE")
}

// Tare sends a tare command to the scale.
func (c *Client) Tare(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected.Load() {
		return errors.New("not connected")
	}

	resp, err := c.sendCommandLocked(CmdTare)
	if err != nil {
		c.closePortLocked()
		c.setStatusLocked(false, "")
		return fmt.Errorf("PORT_ERROR: %w", err)
	}

	if _, ok := ParseAckResponse(resp); !ok {
		return fmt.Errorf("unexpected tare response: %q", resp)
	}
	return nil
}

// Zero sends a zero command to the scale.
func (c *Client) Zero(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected.Load() {
		return errors.New("not connected")
	}

	resp, err := c.sendCommandLocked(CmdZero)
	if err != nil {
		c.closePortLocked()
		c.setStatusLocked(false, "")
		return fmt.Errorf("PORT_ERROR: %w", err)
	}

	if _, ok := ParseAckResponse(resp); !ok {
		return fmt.Errorf("unexpected zero response: %q", resp)
	}
	return nil
}

// HealthCheck verifies the scale is still responding.
func (c *Client) HealthCheck(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected.Load() {
		return errors.New("not connected")
	}

	_, err := c.sendCommandLocked(CmdWeigh)
	if err != nil {
		c.logger.Warn("health check failed: %v", err)
		c.closePortLocked()
		c.setStatusLocked(false, "")
		return fmt.Errorf("PORT_ERROR: %w", err)
	}
	return nil
}

func (c *Client) sendWeighLocked() (*WeighResult, error) {
	resp, err := c.sendCommandLocked(CmdWeigh)
	if err != nil {
		return nil, err
	}
	result, err := ParseWeighResponse(resp)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *Client) sendCommandLocked(cmd string) (string, error) {
	if _, err := c.port.Write([]byte(cmd)); err != nil {
		return "", fmt.Errorf("write: %w", err)
	}
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read: %w", err)
	}
	return line, nil
}

func (c *Client) reconnectLoop(ctx context.Context) {
	defer close(c.done)

	reconnect := time.NewTicker(reconnectDelay)
	defer reconnect.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-reconnect.C:
			if !c.connected.Load() {
				c.tryConnect()
			}
		}
	}
}

func (c *Client) tryConnect() {
	portName := c.cfg.Port
	if portName == "" {
		var err error
		portName, err = DetectPort(c.cfg.VID, c.cfg.PID)
		if err != nil {
			c.logger.Info("port detect: %v", err)
			return
		}
	}

	port, err := c.openPort(portName, c.cfg)
	if err != nil {
		c.logger.Warn("open port %s: %v", portName, err)
		return
	}

	c.mu.Lock()
	c.port = port
	c.reader = bufio.NewReader(port)

	// Verify the scale actually responds before marking as connected.
	_, err = c.sendCommandLocked(CmdWeigh)
	if err != nil {
		c.logger.Info("scale not responding on %s: %v", portName, err)
		c.port.Close()
		c.port = nil
		c.reader = nil
		c.mu.Unlock()
		return
	}
	c.mu.Unlock()

	c.portName.Store(portName)
	c.connected.Store(true)

	c.logger.Info("connected to %s", portName)
	if c.onStatus != nil {
		c.onStatus(true, portName)
	}
}

func (c *Client) closePortLocked() {
	if c.port != nil {
		c.port.Close()
		c.port = nil
		c.reader = nil
	}
	c.connected.Store(false)
	c.portName.Store("")
}

func (c *Client) setStatusLocked(connected bool, port string) {
	if c.onStatus != nil {
		c.onStatus(connected, port)
	}
}
