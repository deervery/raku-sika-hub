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
