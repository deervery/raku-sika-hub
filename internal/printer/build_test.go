package printer

import "testing"

func TestBuildLabelDataFromMap_UsesAPIDataOnly(t *testing.T) {
	got := BuildLabelDataFromMap("traceable", 2, map[string]string{
		"locale":             "en",
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
	if got.Locale != "en" {
		t.Fatalf("expected locale from API, got %q", got.Locale)
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

func TestBuildLabelDataFromMap_InferLocaleFromEnglishData(t *testing.T) {
	got := BuildLabelDataFromMap("traceable", 1, map[string]string{
		"deadlineDate":  "April 30, 2026",
		"companyBlock":  "Manoir Foods\n1-2-3 Sapporo\nTel: 011-000-0000",
		"facilityBlock": "Plant A\n4-5-6 Hakodate\nTel: 0138-000-0000",
	}, "assets")

	if got.Locale != "en" {
		t.Fatalf("expected inferred locale en, got %q", got.Locale)
	}
}
