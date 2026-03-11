package printer

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

func TestLoadTemplateRegistryAndRenderPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "template-map.json")
	content := `{
  "delimiter": "\t",
  "printStartCommand": "^FF",
  "templates": {
    "traceable_deer": {
      "key": 1,
      "fields": ["productName", "productQuantity", "individualNumber"]
    }
  }
}`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write template map: %v", err)
	}

	registry, err := LoadTemplateRegistry(path)
	if err != nil {
		t.Fatalf("LoadTemplateRegistry failed: %v", err)
	}

	entry, ok := registry.Entry("traceable_deer")
	if !ok {
		t.Fatal("expected traceable_deer template")
	}

	payload, err := registry.RenderPayload(entry, LabelData{
		Template:         "traceable_deer",
		ProductName:      "鹿肉",
		ProductQuantity:  "2kg",
		IndividualNumber: "1234",
	})
	if err != nil {
		t.Fatalf("RenderPayload failed: %v", err)
	}

	got := string(payload)
	want := "^TS001鹿肉\t2kg\t1234^FF"
	if got != want {
		t.Fatalf("unexpected payload: got %q want %q", got, want)
	}
}

func TestTestLabelDataUsesFallbackValues(t *testing.T) {
	registry := &TemplateRegistry{
		Templates: map[string]TemplateEntry{
			"pet": {
				Key:    4,
				Fields: []string{"productName", "attentionText"},
			},
		},
	}

	data, err := registry.TestLabelData("pet")
	if err != nil {
		t.Fatalf("TestLabelData failed: %v", err)
	}
	if data.ProductName == "" || data.AttentionText == "" {
		t.Fatalf("expected fallback test values, got %+v", data)
	}
}

func TestRenderPayloadEncodesShiftJIS(t *testing.T) {
	registry := &TemplateRegistry{
		Encoding:          "shift_jis",
		Delimiter:         "\t",
		PrintStartCommand: "^FF",
	}

	payload, err := registry.RenderPayload(TemplateEntry{
		Key:    1,
		Fields: []string{"productName", "individualId"},
	}, LabelData{
		ProductName:      "鹿肉",
		IndividualNumber: "1234-56-78-90",
	})
	if err != nil {
		t.Fatalf("RenderPayload failed: %v", err)
	}

	decoded, _, err := transform.String(japanese.ShiftJIS.NewDecoder(), string(payload))
	if err != nil {
		t.Fatalf("decode Shift_JIS payload: %v", err)
	}

	want := "^TS001鹿肉\t1234-56-78-90^FF"
	if decoded != want {
		t.Fatalf("unexpected decoded payload: got %q want %q", decoded, want)
	}
}

func TestRenderPayloadIncludesProcessedAndPetFields(t *testing.T) {
	registry := &TemplateRegistry{
		Delimiter:         "\t",
		PrintStartCommand: "^FF",
	}

	processedPayload, err := registry.RenderPayload(TemplateEntry{
		Key: 3,
		Fields: []string{
			"productName",
			"saltEquivalentQuantity",
			"isHeatedMeatProducts",
			"attentionText",
		},
	}, LabelData{
		ProductName:            "ソーセージ",
		SaltEquivalentQuantity: "0.3g",
		IsHeatedMeatProducts:   "加熱食肉製品",
		AttentionText:          "要冷凍",
	})
	if err != nil {
		t.Fatalf("RenderPayload(processed) failed: %v", err)
	}
	if got, want := string(processedPayload), "^TS003ソーセージ\t0.3g\t加熱食肉製品\t要冷凍^FF"; got != want {
		t.Fatalf("unexpected processed payload: got %q want %q", got, want)
	}

	petPayload, err := registry.RenderPayload(TemplateEntry{
		Key:    4,
		Fields: []string{"productName", "nutritionUnit", "attentionText"},
	}, LabelData{
		ProductName:   "ペットフード",
		NutritionUnit: "100gあたり",
		AttentionText: "加熱して与えてください",
	})
	if err != nil {
		t.Fatalf("RenderPayload(pet) failed: %v", err)
	}
	if got, want := string(petPayload), "^TS004ペットフード\t100gあたり\t加熱して与えてください^FF"; got != want {
		t.Fatalf("unexpected pet payload: got %q want %q", got, want)
	}
}
