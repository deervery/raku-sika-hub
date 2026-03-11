package printer

import (
	"image/png"
	"os"
	"testing"
)

// TestBuildRows_AllTemplates verifies that buildRows produces non-empty rows for each template.
func TestBuildRows_AllTemplates(t *testing.T) {
	r := &LabelRenderer{} // font not needed for buildRows

	templates := []struct {
		name    string
		data    LabelData
		minRows int
	}{
		{
			name: "traceable_deer",
			data: LabelData{
				Template:           "traceable_deer",
				ProductName:        "鹿肉（モモ）",
				ProductQuantity:    "2.35 kg",
				DeadlineDate:       "2026年3月18日",
				StorageTemperature: "-18℃以下",
				IndividualNumber:   "1234-56-78-90",
				CaptureLocation:    "長野県信濃町",
				QRCode:             "https://rakusika.com/t/abc/def",
			},
			minRows: 7, // name + qty + deadline + storage + sep + individual + capture + spacer + qr
		},
		{
			name: "traceable_bear",
			data: LabelData{
				Template:           "traceable_bear",
				ProductName:        "クマ肉（モモ）",
				ProductQuantity:    "2.35 kg",
				DeadlineDate:       "2026年3月18日",
				StorageTemperature: "-18℃以下",
				IndividualNumber:   "1234-56-78-90",
				CaptureLocation:    "長野県信濃町",
				QRCode:             "https://rakusika.com/t/abc/def",
			},
			minRows: 7,
		},
		{
			name: "non_traceable_deer",
			data: LabelData{
				Template:           "non_traceable_deer",
				ProductName:        "鹿肉（ロース）",
				ProductQuantity:    "1.80 kg",
				DeadlineDate:       "2026年3月20日",
				StorageTemperature: "4℃以下",
			},
			minRows: 4,
		},
		{
			name: "processed",
			data: LabelData{
				Template:               "processed",
				ProductName:            "鹿肉カレー",
				ProductQuantity:        "200g",
				DeadlineDate:           "2026年6月30日",
				StorageTemperature:     "常温",
				ProductIngredient:      "鹿肉、玉ねぎ、にんじん",
				NutritionUnit:          "1食(200g)あたり",
				CaloriesQuantity:       "250 kcal",
				ProteinQuantity:        "15.0 g",
				FatQuantity:            "8.0 g",
				CarbohydratesQuantity:  "30.0 g",
				SaltEquivalentQuantity: "2.5 g",
			},
			minRows: 10,
		},
		{
			name: "pet",
			data: LabelData{
				Template:           "pet",
				ProductName:        "ペット用 鹿肉ジャーキー",
				ProductQuantity:    "50g",
				DeadlineDate:       "2026年12月31日",
				StorageTemperature: "常温",
				ProductIngredient:  "鹿肉",
				AttentionText:      "ペット用です。人間の食品ではありません。",
			},
			minRows: 5,
		},
	}

	for _, tt := range templates {
		t.Run(tt.name, func(t *testing.T) {
			rows := r.buildRows(tt.data)
			if len(rows) < tt.minRows {
				t.Errorf("expected at least %d rows, got %d", tt.minRows, len(rows))
			}

			// Verify all rows have positive height.
			totalHeight := 0
			for i, row := range rows {
				h := row.height()
				if h <= 0 {
					t.Errorf("row %d has non-positive height: %d", i, h)
				}
				totalHeight += h
			}
			if totalHeight == 0 {
				t.Error("total height is 0")
			}
		})
	}
}

// TestRequiredFields verifies required field lists per template.
func TestRequiredFields(t *testing.T) {
	tests := []struct {
		template string
		expected []string
	}{
		{"traceable_deer", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature", "individualNumber"}},
		{"traceable_bear", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature", "individualNumber"}},
		{"non_traceable_deer", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature"}},
		{"processed", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature"}},
		{"pet", []string{"productName", "productQuantity", "deadlineDate", "storageTemperature"}},
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

// TestValidTemplates checks template key validation.
func TestValidTemplates(t *testing.T) {
	for _, key := range []string{"traceable", "traceable_deer", "traceable_bear", "non_traceable", "non_traceable_deer", "processed", "pet"} {
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
	renderer, err := NewLabelRenderer("")
	if err != nil {
		t.Skipf("skipping render test: %v", err)
	}

	data := LabelData{
		Template:           "traceable_deer",
		ProductName:        "鹿肉（モモ）",
		ProductQuantity:    "2.35 kg",
		DeadlineDate:       "2026年3月18日",
		StorageTemperature: "-18℃以下",
		IndividualNumber:   "1234-56-78-90",
		CaptureLocation:    "長野県信濃町",
		QRCode:             "https://rakusika.com/t/abc/def",
	}

	path, err := renderer.Render(data)
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	defer os.Remove(path)

	// Verify the output is a valid PNG.
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
