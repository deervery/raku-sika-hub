package printer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadTemplateRegistryAndRenderPayload(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "template-map.json")
	content := `{
  "delimiter": "\t",
  "printStartCommand": "^FF",
  "templates": {
    "traceable": {
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

	entry, ok := registry.Entry("traceable")
	if !ok {
		t.Fatal("expected traceable template")
	}

	payload, err := registry.RenderPayload(entry, LabelData{
		Template:         "traceable",
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
