package scale

import (
	"bytes"
	"sync"

	"github.com/deervery/raku-sika-hub/internal/config"
)

// mockScalePort simulates an A&D HV-C series scale for SCALE_DRIVER=mock mode.
// It responds to Q/T/R commands with appropriate stable/ack responses.
type mockScalePort struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (m *mockScalePort) Write(p []byte) (int, error) {
	cmd := string(p)
	m.mu.Lock()
	defer m.mu.Unlock()

	switch cmd {
	case CmdWeigh:
		m.buf.WriteString("ST,+00000.00  kg\r\n")
	case CmdTare:
		m.buf.WriteString("QT\r\n")
	case CmdZero:
		m.buf.WriteString("ZR\r\n")
	default:
		m.buf.WriteString("ST,+00000.00  kg\r\n")
	}
	return len(p), nil
}

func (m *mockScalePort) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buf.Read(p)
}

func (m *mockScalePort) Close() error {
	return nil
}

// MockPortOpener returns a PortOpener that creates mock scale ports.
func MockPortOpener() PortOpener {
	return func(name string, cfg config.Config) (Port, error) {
		return &mockScalePort{}, nil
	}
}
