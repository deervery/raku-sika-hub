package printer

import (
	"image/png"
	"os"
	"testing"
)

func TestBuildTableEntriesTraceable(t *testing.T) {
	data := LabelData{
		Template:           "traceable",
		ProductName:        "鹿肉（モモ）",
		ProductQuantity:    "1.50 kg",
		DeadlineDate:       "2026年5月31日",
		StorageTemperature: "-18℃以下",
		IndividualNumber:   "IND-0001",
		CaptureLocation:    "北海道",
		ProcessorName:      "工場A",
		ProcessorLocation:  "函館市",
	}

	entries := buildTableEntries(data)
	if _, ok := findEntry(entries, "個体識別番号"); !ok {
		t.Fatalf("traceable entries missing 個体識別番号")
	}
	if entry, ok := findEntry(entries, "捕獲地"); !ok || entry.value != "北海道" {
		t.Fatalf("capture row missing or wrong: %+v", entry)
	}
	if _, ok := findEntry(entries, "金属探知機"); !ok {
		t.Fatalf("metal detector row missing")
	}
}

func TestBuildTableEntriesProcessed(t *testing.T) {
	data := LabelData{
		Template:           "processed",
		ProductName:        "鹿肉カレー",
		ProductQuantity:    "200g",
		DeadlineDate:       "2026年6月30日",
		StorageTemperature: "常温",
		ProductIngredient:  "鹿肉、玉ねぎ、にんじん",
		NutritionUnit:      "100gあたり",
		CaloriesQuantity:   "250 kcal",
		ProcessorName:      "施設B",
		ProcessorLocation:  "札幌市",
	}

	entries := buildTableEntries(data)
	if entry, ok := findEntry(entries, "原材料名"); !ok || entry.value != "鹿肉、玉ねぎ、にんじん" {
		t.Fatalf("原材料名 row missing or wrong: %+v", entry)
	}
	if _, ok := findEntry(entries, "栄養成分表示（100gあたり）"); ok {
		t.Fatalf("nutrition heading should not be included for processed")
	}
}

func findEntry(entries []tableEntry, label string) (tableEntry, bool) {
	for _, entry := range entries {
		if entry.label == label {
			return entry, true
		}
	}
	return tableEntry{}, false
}

func TestRequiredFields(t *testing.T) {
	tests := []struct {
		template string
		expected []string
	}{
		{"traceable", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature", "processorName", "processorLocation", "individualNumber", "captureLocation", "qrCode"}},
		{"traceable_deer", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature", "processorName", "processorLocation", "individualNumber", "captureLocation", "qrCode"}},
		{"traceable_bear", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature", "processorName", "processorLocation", "individualNumber", "captureLocation", "qrCode"}},
		{"non_traceable", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature", "processorName", "processorLocation"}},
		{"non_traceable_deer", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature", "processorName", "processorLocation"}},
		{"processed", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature", "processorName", "processorLocation"}},
		{"pet", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature", "processorName", "processorLocation"}},
	}

	for _, tt := range tests {
		t.Run(tt.template, func(t *testing.T) {
			got := RequiredFields(tt.template)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %d fields, got %d", len(tt.expected), len(got))
			}
			for i, f := range got {
				if f != tt.expected[i] {
					t.Errorf("field %d: expected %q, got %q", i, tt.expected[i], f)
				}
			}
		})
	}
}

func TestValidTemplates(t *testing.T) {
	for _, key := range []string{"traceable", "traceable_deer", "traceable_bear", "non_traceable", "non_traceable_deer", "processed", "pet", "individual_qr"} {
		if !ValidTemplates[key] {
			t.Errorf("expected %q to be valid", key)
		}
	}
	for _, key := range []string{"", "unknown", "TRACEABLE"} {
		if ValidTemplates[key] {
			t.Errorf("expected %q to be invalid", key)
		}
	}
}

// TestRender_WithFont tests actual image rendering if a system font is available.
// This test is skipped if no font is found (e.g., in CI without Japanese fonts).
func TestRender_WithFont(t *testing.T) {
	renderer, err := NewLabelRenderer("", "")
	if err != nil {
		t.Skipf("skipping render test: %v", err)
	}

	data := LabelData{
		Template:           "traceable",
		ProductName:        "鹿肉（モモ）",
		ProductQuantity:    "2.35 kg",
		DeadlineDate:       "2026年3月18日",
		StorageTemperature: "-18℃以下",
		IndividualNumber:   "1234-56-78-90",
		CaptureLocation:    "長野県信濃町",
		QRCode:             "https://rakusika.com/t/abc/def",
		ProcessorName:      "工場Z",
		ProcessorLocation:  "札幌市",
	}

	path, err := renderer.Render(data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	defer os.Remove(path)

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open output: %v", err)
	}
	defer f.Close()

	img, err := png.Decode(f)
	if err != nil {
		t.Fatalf("decode png: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != labelWidthPx {
		t.Errorf("expected width %d, got %d", labelWidthPx, bounds.Dx())
	}
	if bounds.Dy() < 100 {
		t.Errorf("height too small: %d", bounds.Dy())
	}

	t.Logf("rendered label: %dx%d px → %s", bounds.Dx(), bounds.Dy(), path)
}
