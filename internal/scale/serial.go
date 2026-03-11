package scale

import (
	"fmt"

	"github.com/deervery/raku-sika-hub/internal/config"
	"go.bug.st/serial"
)

// defaultPortOpener opens a real serial port using go.bug.st/serial.
func defaultPortOpener(name string, cfg config.Config) (Port, error) {
	parity, err := parseParity(cfg.Parity)
	if err != nil {
		return nil, err
	}
	stopBits, err := parseStopBits(cfg.StopBits)
	if err != nil {
		return nil, err
	}

	mode := &serial.Mode{
		BaudRate: cfg.BaudRate,
		DataBits: cfg.DataBits,
		Parity:   parity,
		StopBits: stopBits,
	}
	port, err := serial.Open(name, mode)
	if err != nil {
		return nil, fmt.Errorf("serial.Open(%s): %w", name, err)
	}
	port.SetReadTimeout(commandTimeout)
	return port, nil
}

func parseParity(s string) (serial.Parity, error) {
	switch s {
	case "none", "":
		return serial.NoParity, nil
	case "odd":
		return serial.OddParity, nil
	case "even":
		return serial.EvenParity, nil
	case "mark":
		return serial.MarkParity, nil
	case "space":
		return serial.SpaceParity, nil
	default:
		return serial.NoParity, fmt.Errorf("unknown parity: %q", s)
	}
}

func parseStopBits(n int) (serial.StopBits, error) {
	switch n {
	case 1:
		return serial.OneStopBit, nil
	case 2:
		return serial.TwoStopBits, nil
	default:
		return serial.OneStopBit, fmt.Errorf("unsupported stop bits: %d", n)
	}
}
