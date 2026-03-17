package scanner

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/deervery/raku-sika-hub/internal/config"
	"github.com/deervery/raku-sika-hub/internal/logging"
)

const (
	reconnectDelay = 3 * time.Second
)

// inputEvent matches the Linux input_event struct.
// On 64-bit: struct timeval (16 bytes) + __u16 type + __u16 code + __s32 value = 24 bytes
type inputEvent struct {
	TimeSec  int64
	TimeUsec int64
	Type     uint16
	Code     uint16
	Value    int32
}

var inputEventSize = int(unsafe.Sizeof(inputEvent{}))

// scanResult holds a consumed scan value.
type scanResult struct {
	value     string
	scannedAt time.Time
}

// Client reads barcode scans from a Linux evdev device.
type Client struct {
	cfg        config.Config
	logger     *logging.Logger
	connected  atomic.Bool
	devicePath atomic.Value // string

	mu     sync.Mutex
	latest *scanResult // latest unconsumed scan

	cancel context.CancelFunc
	done   chan struct{}
}

// NewClient creates a new scanner Client.
func NewClient(cfg config.Config, logger *logging.Logger) *Client {
	c := &Client{
		cfg:    cfg,
		logger: logger,
	}
	c.devicePath.Store("")
	return c
}

// Start begins the device detection and read loop.
func (c *Client) Start(ctx context.Context) {
	ctx, c.cancel = context.WithCancel(ctx)
	c.done = make(chan struct{})
	go c.loop(ctx)
}

// Stop stops the scanner client.
func (c *Client) Stop() {
	if c.cancel != nil {
		c.cancel()
		<-c.done
	}
}

// Connected returns true if the scanner device is open.
func (c *Client) Connected() bool {
	return c.connected.Load()
}

// DevicePath returns the current device path.
func (c *Client) DevicePath() string {
	v := c.devicePath.Load()
	if v == nil {
		return ""
	}
	return v.(string)
}

// Consume returns the latest scan value and clears it.
// Returns ok=false if no new scan is available.
func (c *Client) Consume() (value string, scannedAt string, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.latest == nil {
		return "", "", false
	}

	result := c.latest
	c.latest = nil
	return result.value, result.scannedAt.UTC().Format(time.RFC3339Nano), true
}

func (c *Client) loop(ctx context.Context) {
	defer close(c.done)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		devPath, err := DetectDevice(c.cfg.ScannerDeviceName, c.cfg.ScannerVid, c.cfg.ScannerPid)
		if err != nil {
			c.connected.Store(false)
			c.devicePath.Store("")
			select {
			case <-time.After(reconnectDelay):
				continue
			case <-ctx.Done():
				return
			}
		}

		c.logger.Info("scanner: opening %s", devPath)
		c.readDevice(ctx, devPath)
		c.connected.Store(false)
		c.devicePath.Store("")
		c.logger.Info("scanner: device closed, will retry")

		select {
		case <-time.After(reconnectDelay):
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) readDevice(ctx context.Context, devPath string) {
	f, err := os.Open(devPath)
	if err != nil {
		c.logger.Warn("scanner: open %s: %v", devPath, err)
		return
	}
	defer f.Close()

	// Try to grab exclusive access (EVIOCGRAB).
	if err := grabDevice(f); err != nil {
		c.logger.Warn("scanner: EVIOCGRAB %s: %v (continuing without exclusive access)", devPath, err)
	}

	c.connected.Store(true)
	c.devicePath.Store(devPath)

	buf := make([]byte, inputEventSize*64)
	var current []byte
	shifted := false

	// Read in a goroutine so we can cancel via context.
	readCh := make(chan []byte)
	errCh := make(chan error, 1)
	go func() {
		for {
			n, err := f.Read(buf)
			if err != nil {
				errCh <- err
				return
			}
			data := make([]byte, n)
			copy(data, buf[:n])
			readCh <- data
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			c.logger.Warn("scanner: read error: %v", err)
			return
		case data := <-readCh:
			for len(data) >= inputEventSize {
				var ev inputEvent
				ev.TimeSec = int64(binary.LittleEndian.Uint64(data[0:8]))
				ev.TimeUsec = int64(binary.LittleEndian.Uint64(data[8:16]))
				ev.Type = binary.LittleEndian.Uint16(data[16:18])
				ev.Code = binary.LittleEndian.Uint16(data[18:20])
				ev.Value = int32(binary.LittleEndian.Uint32(data[20:24]))
				data = data[inputEventSize:]

				if ev.Type != evKey {
					continue
				}

				// Track shift state.
				if ev.Code == keyLShift || ev.Code == keyRShift {
					shifted = ev.Value == keyPressed
					continue
				}

				// Only process key-down events.
				if ev.Value != keyPressed {
					continue
				}

				if ev.Code == keyEnter {
					// End of scan.
					if len(current) > 0 {
						value := string(current)
						c.mu.Lock()
						c.latest = &scanResult{
							value:     value,
							scannedAt: time.Now(),
						}
						c.mu.Unlock()
						c.logger.Info("scanner: scanned %q", value)
						current = current[:0]
					}
					continue
				}

				// Map keycode to character.
				var ch byte
				if shifted {
					ch = shiftKeyMap[ev.Code]
				} else {
					ch = keyMap[ev.Code]
				}
				if ch != 0 {
					current = append(current, ch)
				}
			}
		}
	}
}

// grabDevice attempts to grab exclusive access to the evdev device via EVIOCGRAB ioctl.
func grabDevice(f *os.File) error {
	// EVIOCGRAB = _IOW('E', 0x90, int) = 0x40044590
	const EVIOCGRAB = 0x40044590
	_, _, errno := ioctlGrab(f.Fd(), EVIOCGRAB, 1)
	if errno != 0 {
		return fmt.Errorf("ioctl EVIOCGRAB: %w", errno)
	}
	return nil
}
