package scale

import "io"

// Port abstracts a serial port for testability.
// In production this wraps go.bug.st/serial.Port;
// in tests a mock implementation is injected.
type Port interface {
	io.ReadWriteCloser
}
