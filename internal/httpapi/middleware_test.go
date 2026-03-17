package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLANOnly_PrivateIPs(t *testing.T) {
	handler := LANOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		remoteAddr string
		wantCode   int
	}{
		{"127.0.0.1:12345", http.StatusOK},
		{"192.168.1.100:12345", http.StatusOK},
		{"10.0.0.1:12345", http.StatusOK},
		{"172.16.0.1:12345", http.StatusOK},
		{"172.31.255.255:12345", http.StatusOK},
		{"[::1]:12345", http.StatusOK},
		{"8.8.8.8:12345", http.StatusForbidden},
		{"203.0.113.1:12345", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.remoteAddr, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			req.RemoteAddr = tt.remoteAddr
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != tt.wantCode {
				t.Errorf("got %d, want %d", rec.Code, tt.wantCode)
			}
		})
	}
}

func TestCORS_Headers(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Errorf("ACAO = %q, want *", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Methods"); got != "GET, POST, OPTIONS" {
		t.Errorf("ACAM = %q", got)
	}
}

func TestCORS_Preflight(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/scale/weigh", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("OPTIONS response = %d, want %d", rec.Code, http.StatusNoContent)
	}
}
