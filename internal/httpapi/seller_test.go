package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/deervery/raku-sika-hub/internal/printer"
)

func TestSellerCapabilityIsOptIn(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		h := &Handler{sellerLabelEnabled: enabled}
		w := httptest.NewRecorder()
		h.HandleVersion(w, httptest.NewRequest(http.MethodGet, "/version", nil))
		var response VersionResponse
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if slices.Contains(response.Features, printer.SellerLabelFeature) != enabled {
			t.Fatalf("wrong features: %v", response.Features)
		}
	}
}

func TestUnenabledHubRejectsSellerBeforePrinting(t *testing.T) {
	h := &Handler{}
	_, status, body := h.validateAndBuildLabelData(PrintRequest{Template: "traceable_deer", Data: map[string]string{"sellerBlock": "販売会社\n住所"}})
	if status != http.StatusBadRequest || body == nil {
		t.Fatal("disabled Hub must reject seller")
	}
}

func TestTrialRejectsOtherUsesBeforePrinting(t *testing.T) {
	h := &Handler{sellerLabelEnabled: true}
	for _, template := range []string{"processed", "pet", "carcass_deer"} {
		_, status, body := h.validateAndBuildLabelData(PrintRequest{Template: template, Data: map[string]string{"sellerBlock": "販売会社\n住所"}})
		if status != http.StatusBadRequest || body == nil {
			t.Fatalf("%s must reject seller", template)
		}
	}
}
