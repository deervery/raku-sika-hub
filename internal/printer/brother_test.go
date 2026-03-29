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

func TestParseMediaOptions(t *testing.T) {
	output := strings.Join([]string{
		"PageSize/Media Size: *w62h100/62 mm x 100 mm w62h29/62 mm x 29 mm roll-62/62 mm Continuous",
		"media/Media Tracking: *continuous die-cut",
		"",
	}, "\n")

	got := parseMediaOptions(output)
	want := []string{"roll-62", "w62h100", "w62h29"}
	if len(got) != len(want) {
		t.Fatalf("expected %d media options, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected media[%d]=%q, got %q", i, want[i], got[i])
		}
	}
}

func TestSelectPreferredMediaOption_PrefersContinuous62mm(t *testing.T) {
	options := []string{"w62h100", "roll-62", "w62h29"}
	if got := selectPreferredMediaOption(options); got != "roll-62" {
		t.Fatalf("expected roll-62, got %q", got)
	}
}

func TestSelectPreferredMediaOption_FallsBackTo62mmOption(t *testing.T) {
	options := []string{"w62h100", "w62h29"}
	if got := selectPreferredMediaOption(options); got != "w62h100" {
		t.Fatalf("expected w62h100, got %q", got)
	}
}

func TestSelectPreferredMediaOption_ReturnsEmptyWhen62mmUnavailable(t *testing.T) {
	options := []string{"A4", "Letter", "w29h90"}
	if got := selectPreferredMediaOption(options); got != "" {
		t.Fatalf("expected empty selection, got %q", got)
	}
}
