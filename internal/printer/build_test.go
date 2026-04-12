package printer

import "testing"

func TestBuildLabelDataFromMap_UsesAPIDataOnly(t *testing.T) {
	got := BuildLabelDataFromMap("traceable", 2, map[string]string{
		"productName":        "Venison",
		"productQuantity":    "100 g",
		"deadlineDate":       "April 30, 2026",
		"storageTemperature": "Keep frozen",
		"individualNumber":   "IND-0001",
		"captureLocation":    "Hakodate",
		"companyBlock":       "Manoir Foods",
		"facilityBlock":      "Plant A",
	}, "assets")

	if got.Copies != 2 {
		t.Fatalf("expected copies=2, got %d", got.Copies)
	}
	if got.CompanyBlock != "Manoir Foods" {
		t.Fatalf("expected companyBlock from API, got %q", got.CompanyBlock)
	}
	if got.FacilityBlock != "Plant A" {
		t.Fatalf("expected facilityBlock from API, got %q", got.FacilityBlock)
	}
	if got.CaptureLocation != "Hakodate" {
		t.Fatalf("expected captureLocation from API, got %q", got.CaptureLocation)
	}
}

func TestBuildLabelDataFromMap_NormalizesStorageMethodAlias(t *testing.T) {
	got := BuildLabelDataFromMap("non_traceable", 1, map[string]string{
		"productName":     "Test",
		"productQuantity": "1 pack",
		"deadlineDate":    "2026-04-30",
		"storageMethod":   "Keep refrigerated",
	}, "assets")

	if got.StorageTemperature != "Keep refrigerated" {
		t.Fatalf("expected storageMethod alias to populate storageTemperature, got %q", got.StorageTemperature)
	}
}
