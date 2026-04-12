package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleVersion(t *testing.T) {
	h := &Handler{
		version:   "0.2.0",
		commit:    "abc1234",
		buildDate: "2026-03-17T00:00:00Z",
	}

	req := httptest.NewRequest(http.MethodGet, "/version", nil)
	rec := httptest.NewRecorder()
	h.HandleVersion(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp VersionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Version != "0.2.0" {
		t.Errorf("version = %q, want %q", resp.Version, "0.2.0")
	}
	if resp.Commit != "abc1234" {
		t.Errorf("commit = %q, want %q", resp.Commit, "abc1234")
	}
}

func TestHandleVersion_MethodNotAllowed(t *testing.T) {
	h := &Handler{}
	req := httptest.NewRequest(http.MethodPost, "/version", nil)
	rec := httptest.NewRecorder()
	h.HandleVersion(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleScannerScan_NotConnected(t *testing.T) {
	h := &Handler{scanner: nil}
	req := httptest.NewRequest(http.MethodGet, "/scanner/scan", nil)
	rec := httptest.NewRecorder()
	h.HandleScannerScan(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
}

func TestParseLpstatOutput(t *testing.T) {
	jobs := parseLpstatOutput("Brother_QL_820NWB_USB-8 rakusika 104448 Sun Apr 12 22:37:10 2026\n")
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != "Brother_QL_820NWB_USB-8" {
		t.Fatalf("unexpected job id %q", jobs[0].ID)
	}
	if jobs[0].State != "queued" {
		t.Fatalf("unexpected state %q", jobs[0].State)
	}
}

func TestParsePrinterStateFromLpstat(t *testing.T) {
	if got := parsePrinterStateFromLpstat("printer Brother_QL_820NWB_USB now printing Brother_QL_820NWB_USB-8.  enabled since ..."); got != "printing" {
		t.Fatalf("expected printing, got %q", got)
	}
	if got := parsePrinterStateFromLpstat("printer Brother_QL_820NWB_USB is idle.  enabled since ..."); got != "idle" {
		t.Fatalf("expected idle, got %q", got)
	}
}
