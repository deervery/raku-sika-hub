package scale

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deervery/raku-sika-hub/internal/config"
	"github.com/deervery/raku-sika-hub/internal/logging"
)

// mockPort simulates a serial port with preset responses.
type mockPort struct {
	reader    io.Reader
	writeErr  error
	readErr   error
	closed    bool
	writtenCh chan string
}

func newMockPort(responses ...string) *mockPort {
	data := strings.Join(responses, "")
	return &mockPort{
		reader:    bytes.NewBufferString(data),
		writtenCh: make(chan string, 10),
	}
}

func (m *mockPort) Read(p []byte) (int, error) {
	if m.readErr != nil {
		return 0, m.readErr
	}
	return m.reader.Read(p)
}

func (m *mockPort) Write(p []byte) (int, error) {
	if m.writeErr != nil {
		return 0, m.writeErr
	}
	m.writtenCh <- string(p)
	return len(p), nil
}

func (m *mockPort) Close() error {
	m.closed = true
	return nil
}

func testLogger(t *testing.T) *logging.Logger {
	t.Helper()
	dir := t.TempDir()
	l, err := logging.New(dir, logging.LevelInfo)
	if err != nil {
		t.Fatalf("create logger: %v", err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func newTestClient(t *testing.T, mock *mockPort) (*Client, chan bool) {
	t.Helper()
	statusCh := make(chan bool, 10)
	cfg := config.Default()
	cfg.Port = "/dev/ttyTEST"

	client := NewClient(cfg, testLogger(t), func(connected bool, port string) {
		statusCh <- connected
	})
	client.openPort = func(name string, cfg config.Config) (Port, error) {
		return mock, nil
	}

	// Manually connect
	client.mu.Lock()
	client.port = mock
	client.reader = bufio.NewReader(mock)
	client.mu.Unlock()
	client.portName.Store("/dev/ttyTEST")
	client.connected.Store(true)

	return client, statusCh
}

func TestWeigh_StableImmediate(t *testing.T) {
	mock := newMockPort("ST,+00012.34  kg\r\n")
	client, _ := newTestClient(t, mock)

	result, err := client.Weigh(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Header != HeaderST {
		t.Errorf("expected ST, got %s", result.Header)
	}
	if result.Value != 12.34 {
		t.Errorf("expected 12.34, got %f", result.Value)
	}
	if result.Unit != "kg" {
		t.Errorf("expected kg, got %s", result.Unit)
	}
	if !result.Stable {
		t.Error("expected stable=true")
	}
}

func TestWeigh_UnstableThenStable(t *testing.T) {
	mock := newMockPort(
		"US,+00012.30  kg\r\n",
		"US,+00012.32  kg\r\n",
		"ST,+00012.34  kg\r\n",
	)
	client, _ := newTestClient(t, mock)

	var retries int
	result, err := client.Weigh(context.Background(), func(retry, maxRetry int) {
		retries = retry
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if retries != 2 {
		t.Errorf("expected 2 retries, got %d", retries)
	}
	if result.Header != HeaderST {
		t.Errorf("expected ST, got %s", result.Header)
	}
	if result.Value != 12.34 {
		t.Errorf("expected 12.34, got %f", result.Value)
	}
}

func TestWeigh_UnstableTimeout(t *testing.T) {
	var responses []string
	for i := 0; i < maxWeighRetries; i++ {
		responses = append(responses, "US,+00012.30  kg\r\n")
	}
	mock := newMockPort(responses...)
	client, _ := newTestClient(t, mock)

	_, err := client.Weigh(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "UNSTABLE" {
		t.Errorf("expected UNSTABLE, got %v", err)
	}
}

func TestWeigh_Overload(t *testing.T) {
	mock := newMockPort("OL\r\n")
	client, _ := newTestClient(t, mock)

	_, err := client.Weigh(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if err.Error() != "OVERLOAD" {
		t.Errorf("expected OVERLOAD, got %v", err)
	}
}

func TestWeigh_IOError(t *testing.T) {
	mock := newMockPort()
	mock.readErr = errors.New("device removed")
	client, statusCh := newTestClient(t, mock)

	_, err := client.Weigh(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "PORT_ERROR") {
		t.Errorf("expected PORT_ERROR, got %v", err)
	}

	select {
	case connected := <-statusCh:
		if connected {
			t.Error("expected disconnected status")
		}
	default:
		t.Error("expected status callback")
	}

	if !mock.closed {
		t.Error("expected port to be closed")
	}
}

func TestTare_Success(t *testing.T) {
	mock := newMockPort("QT\r\n")
	client, _ := newTestClient(t, mock)

	err := client.Tare(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestZero_Success(t *testing.T) {
	mock := newMockPort("ZR\r\n")
	client, _ := newTestClient(t, mock)

	err := client.Zero(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
