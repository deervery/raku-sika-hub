package printer

import (
	"image/png"
	"os"
	"testing"
)

// TestBuildRows_SupportedTemplates verifies that buildRows produces non-empty rows for each supported template.
func TestBuildRows_SupportedTemplates(t *testing.T) {
	r, err := NewLabelRenderer("")
	if err != nil {
		t.Skipf("skipping: %v", err)
	}

	templates := []struct {
		name    string
		data    LabelData
		minRows int
	}{
		{
			name: "traceable_deer",
			data: LabelData{
				Template:        "traceable_deer",
				ProductName:     "鹿肉（モモ）",
				ProductQuantity: "2.35 kg",
				DeadlineDate:    "2026年3月18日",
				StorageMethod:   "-4℃以下で保存",
				IndividualID:    "1234-56-78-90",
				CaptureLocation: "長野県信濃町",
				QRCode:          "https://rakusika.com/t/abc/def",
			},
			minRows: 11,
		},
		{
			name: "traceable_bear",
			data: LabelData{
				Template:           "traceable_bear",
				ProductName:        "熊肉（モモ）",
				ProductQuantity:    "1.50 kg",
				DeadlineDate:       "2026年4月10日",
				StorageTemperature: "frozen",
				IndividualID:       "9876-54-32-10",
				CaptureLocation:    "北海道斜里町",
				QRCode:             "https://rakusika.com/t/xyz/ghi",
			},
			minRows: 11,
		},
		{
			name: "non_traceable_deer",
			data: LabelData{
				Template:           "non_traceable_deer",
				ProductName:        "鹿肉ミンチ",
				ProductQuantity:    "500g",
				DeadlineDate:       "2026年5月1日",
				StorageTemperature: "refrigerated",
			},
			minRows: 7,
		},
		{
			name: "processed",
			data: LabelData{
				Template:               "processed",
				ProductName:            "鹿肉カレー",
				ProductQuantity:        "200g",
				DeadlineDate:           "2027年1月1日",
				StorageTemperature:     "ambient",
				ProductIngredient:      "鹿肉、玉ねぎ、じゃがいも、カレールー",
				NutritionUnit:          "1食(200g)あたり",
				CaloriesQuantity:       "250kcal",
				ProteinQuantity:        "15g",
				FatQuantity:            "10g",
				CarbohydratesQuantity:  "25g",
				SaltEquivalentQuantity: "2.5g",
			},
			minRows: 10,
		},
		{
			name: "pet",
			data: LabelData{
				Template:        "pet",
				ProductName:     "ペット用 鹿肉ジャーキー",
				ProductQuantity: "50g",
				DeadlineDate:    "2026年12月31日",
			},
			minRows: 7,
		},
	}

	for _, tt := range templates {
		t.Run(tt.name, func(t *testing.T) {
			rows := r.buildRows(tt.data)
			if len(rows) < tt.minRows {
				t.Errorf("expected at least %d rows, got %d", tt.minRows, len(rows))
			}

			applyFontSize(rows, labelFontSize)
			layout := r.computeLayout(rows)

			totalHeight := 0
			for i, row := range rows {
				h := row.height(r, layout)
				if h <= 0 {
					t.Errorf("row %d has non-positive height: %d", i, h)
				}
				totalHeight += h
			}
			if totalHeight == 0 {
				t.Error("total height is 0")
			}
			t.Logf("fontSize=%.1f layout: width=%d, totalHeight=%d, labelCol=%d, valueCol=%d",
				labelFontSize, layout.labelWidthPx, totalHeight, layout.labelColPx, layout.valueColPx)
		})
	}
}

// TestRequiredFields verifies required field lists per template.
func TestRequiredFields(t *testing.T) {
	tests := []struct {
		template string
		expected []string
	}{
		{"traceable_deer", []string{"productName", "captureLocation", "productQuantity", "deadlineDate", "individualId", "qrCode"}},
		{"traceable_bear", []string{"productName", "captureLocation", "productQuantity", "deadlineDate", "individualId", "qrCode"}},
		{"non_traceable_deer", []string{"productName", "productQuantity", "deadlineDate"}},
		{"processed", []string{"productName", "productQuantity", "deadlineDate"}},
		{"pet", []string{"productName", "productQuantity", "deadlineDate"}},
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
	for _, key := range []string{"traceable_deer", "traceable_bear", "non_traceable_deer", "processed", "pet"} {
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

// TestNormalizeStorageTemperature verifies English-to-Japanese conversion.
func TestNormalizeStorageTemperature(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"frozen", "-18℃以下で保存"},
		{"refrigerated", "10℃以下で保存"},
		{"ambient", "直射日光と高温多湿を避けて保管してください。"},
		{"-4℃以下で保存", "-4℃以下で保存"},
		{"", ""},
	}
	for _, tt := range tests {
		got := NormalizeStorageTemperature(tt.input)
		if got != tt.expected {
			t.Errorf("NormalizeStorageTemperature(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// TestRender_WithFont tests actual image rendering if a system font is available.
func TestRender_WithFont(t *testing.T) {
	renderer, err := NewLabelRenderer("")
	if err != nil {
		t.Skipf("skipping render test: %v", err)
	}

	data := LabelData{
		Template:        "traceable_deer",
		ProductName:     "鹿肉（モモ）",
		ProductQuantity: "2.35 kg",
		DeadlineDate:    "2026年3月18日",
		StorageMethod:   "-4℃以下で保存",
		IndividualID:    "1234-56-78-90",
		CaptureLocation: "長野県信濃町",
		QRCode:          "https://rakusika.com/t/abc/def",
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
		t.Errorf("expected width = %d (62mm), got %d", labelWidthPx, bounds.Dx())
	}
	if bounds.Dy() < 100 {
		t.Errorf("height too small: %d", bounds.Dy())
	}

	t.Logf("rendered label: %dx%d px → %s", bounds.Dx(), bounds.Dy(), path)
}
