package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
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
	// Assert membership rather than the exact string: the set grows as the
	// API does, and pinning the spelling makes every addition look like a
	// regression.
	got := rec.Header().Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "POST", "OPTIONS"} {
		if !strings.Contains(got, method) {
			t.Errorf("ACAM = %q, missing %s", got, method)
		}
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

// TestCORS_AllowsDeleteFromTablet pins the methods a cross-origin caller may
// use. The tablet (raku-sika-lite, served from rakusika.com) clears a stuck
// queue with DELETE /printer/queue, so a preflight that omits DELETE silently
// disables that button in the browser — curl never sees it, because curl does
// not send a preflight.
func TestCORS_AllowsDeleteFromTablet(t *testing.T) {
	handler := CORS(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/printer/queue", nil)
	req.Header.Set("Origin", "https://rakusika.com")
	req.Header.Set("Access-Control-Request-Method", "DELETE")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	allowed := rec.Header().Get("Access-Control-Allow-Methods")
	for _, method := range []string{"GET", "POST", "DELETE", "OPTIONS"} {
		if !strings.Contains(allowed, method) {
			t.Fatalf("preflight must allow %s, got %q", method, allowed)
		}
	}
}
