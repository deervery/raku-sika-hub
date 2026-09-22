package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMountAdminServesEmbeddedGUI(t *testing.T) {
	srv := &Server{}
	mux := http.NewServeMux()
	if err := srv.mountAdmin(mux); err != nil {
		t.Fatalf("mountAdmin: %v", err)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /admin/, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "RakuSika Hub 保守") {
		t.Fatal("admin page body was not served")
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/admin", nil))
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected /admin to redirect, got %d", rec.Code)
	}
}
