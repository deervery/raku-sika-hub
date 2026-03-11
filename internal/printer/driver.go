package printer

import (
	"fmt"
	"strings"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

const (
	DriverPtouchTemplate = "ptouch_template"
	DriverCUPSPNG        = "cups_png"
)

// Driver defines the printer operations used by the WebSocket handlers.
type Driver interface {
	IsAvailable() bool
	Status() (PrinterStatus, error)
	LogStatus(context string)
	TestPrint() error
	PrintLabel(data LabelData) error
	PreviewLabel(data LabelData) ([]byte, error)
	CanPrintLabels() bool
	Queue() (QueueStatus, error)
	ClearQueue() (QueueStatus, error)
}

type DriverConfig struct {
	DriverName      string
	PrinterName     string
	FontPath        string
	PrinterAddress  string
	TemplateMapPath string
}

// NewDriver creates the configured printer driver.
func NewDriver(cfg DriverConfig, logger *logging.Logger) (Driver, error) {
	name := strings.TrimSpace(cfg.DriverName)
	if name == "" {
		name = DriverPtouchTemplate
	}

	switch name {
	case DriverPtouchTemplate:
		return NewPtouchTemplate(cfg, logger)
	case DriverCUPSPNG:
		return NewBrother(cfg.PrinterName, cfg.FontPath, logger), nil
	default:
		return nil, fmt.Errorf("unknown printer driver: %s", name)
	}
}
