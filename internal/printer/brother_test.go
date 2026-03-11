package printer

import (
	"strings"
	"testing"
)

func TestParseAvailablePrinters(t *testing.T) {
	output := "printer Brother_QL-820NWB is idle. enabled since Thu 01 Jan 1970 00:00:00 AM JST\nprinter Brother_QL_820NWB_USB disabled since Thu 01 Jan 1970 00:00:00 AM JST\n"

	got := parseAvailablePrinters(output)
	if len(got) != 2 {
		t.Fatalf("expected 2 printers, got %d", len(got))
	}
	if got[0] != "Brother_QL-820NWB" {
		t.Fatalf("expected first printer to preserve lpstat order, got %q", got[0])
	}
	if got[1] != "Brother_QL_820NWB_USB" {
		t.Fatalf("expected second printer, got %q", got[1])
	}
}

func TestParseDefaultPrinter(t *testing.T) {
	output := "system default destination: Brother_QL-820NWB\n"
	if got := parseDefaultPrinter(output); got != "Brother_QL-820NWB" {
		t.Fatalf("expected default printer, got %q", got)
	}
}

func TestValidateStatus_ConfiguredMismatch(t *testing.T) {
	status := PrinterStatus{
		ConfiguredName: "Brother_QL-800",
		SelectedName:   "Brother_QL-800",
		DefaultName:    "Brother_QL-820NWB",
		Available:      []string{"Brother_QL-820NWB", "Brother_QL_820NWB_USB"},
		Source:         "configured",
	}

	err := validateStatus(status)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "PRINTER_NOT_CONFIGURED:") {
		t.Fatalf("expected PRINTER_NOT_CONFIGURED, got %q", msg)
	}
}
