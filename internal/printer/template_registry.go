package printer

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type TemplateRegistry struct {
	Delimiter         string                   `json:"delimiter"`
	PrintStartCommand string                   `json:"printStartCommand"`
	Templates         map[string]TemplateEntry `json:"templates"`
}

type TemplateEntry struct {
	Key      int               `json:"key"`
	Fields   []string          `json:"fields"`
	TestData map[string]string `json:"testData"`
}

func LoadTemplateRegistry(path string) (*TemplateRegistry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template map: %w", err)
	}

	var registry TemplateRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("parse template map: %w", err)
	}

	if registry.Delimiter == "" {
		registry.Delimiter = "\t"
	}
	if registry.PrintStartCommand == "" {
		registry.PrintStartCommand = "^FF"
	}
	if len(registry.Templates) == 0 {
		return nil, fmt.Errorf("template map has no templates: %s", path)
	}

	return &registry, nil
}

func DefaultTemplateMapPath() string {
	return filepath.Join("templates", "siknue", "template-map.json")
}

func (r *TemplateRegistry) Entry(name string) (TemplateEntry, bool) {
	entry, ok := r.Templates[name]
	return entry, ok
}

func (r *TemplateRegistry) RenderPayload(entry TemplateEntry, data LabelData) ([]byte, error) {
	if entry.Key < 1 || entry.Key > 999 {
		return nil, fmt.Errorf("invalid template key: %d", entry.Key)
	}
	if len(entry.Fields) == 0 {
		return nil, fmt.Errorf("template fields are empty")
	}

	values := make([]string, 0, len(entry.Fields))
	for _, field := range entry.Fields {
		values = append(values, sanitizeTemplateValue(labelFieldValue(data, field)))
	}

	payload := fmt.Sprintf("^TS%03d%s%s", entry.Key, strings.Join(values, r.Delimiter), r.PrintStartCommand)
	return []byte(payload), nil
}

func (r *TemplateRegistry) TestLabelData(templateName string) (LabelData, error) {
	entry, ok := r.Entry(templateName)
	if !ok {
		return LabelData{}, fmt.Errorf("template not found: %s", templateName)
	}

	data := LabelData{
		Template: templateName,
		Copies:   1,
	}
	for _, field := range entry.Fields {
		value := entry.TestData[field]
		if value == "" {
			value = defaultTestValue(field)
		}
		assignLabelField(&data, field, value)
	}
	return data, nil
}

func defaultTestValue(field string) string {
	switch field {
	case "productName":
		return "RakuSika Hub Test"
	case "productQuantity":
		return "1 pack"
	case "deadlineDate":
		return "2099-12-31"
	case "storageTemperature":
		return "-18C"
	case "individualNumber":
		return "TEST-0001"
	case "captureLocation":
		return "Shinano"
	case "qrCode":
		return "https://rakusika.example/test"
	case "productIngredient":
		return "deer meat"
	case "nutritionUnit":
		return "100g"
	case "caloriesQuantity":
		return "100kcal"
	case "proteinQuantity":
		return "20g"
	case "fatQuantity":
		return "5g"
	case "carbohydratesQuantity":
		return "0g"
	case "saltEquivalentQuantity":
		return "0.1g"
	case "attentionText":
		return "TEST"
	default:
		return "TEST"
	}
}

func sanitizeTemplateValue(v string) string {
	v = strings.ReplaceAll(v, "\r\n", " ")
	v = strings.ReplaceAll(v, "\n", " ")
	v = strings.ReplaceAll(v, "\r", " ")
	v = strings.ReplaceAll(v, "\t", " ")
	return v
}

func labelFieldValue(data LabelData, field string) string {
	switch field {
	case "productName":
		return data.ProductName
	case "productQuantity":
		return data.ProductQuantity
	case "deadlineDate":
		return data.DeadlineDate
	case "storageTemperature":
		return data.StorageTemperature
	case "individualNumber":
		return data.IndividualNumber
	case "captureLocation":
		return data.CaptureLocation
	case "qrCode":
		return data.QRCode
	case "productIngredient":
		return data.ProductIngredient
	case "nutritionUnit":
		return data.NutritionUnit
	case "caloriesQuantity":
		return data.CaloriesQuantity
	case "proteinQuantity":
		return data.ProteinQuantity
	case "fatQuantity":
		return data.FatQuantity
	case "carbohydratesQuantity":
		return data.CarbohydratesQuantity
	case "saltEquivalentQuantity":
		return data.SaltEquivalentQuantity
	case "attentionText":
		return data.AttentionText
	default:
		return ""
	}
}

func assignLabelField(data *LabelData, field, value string) {
	switch field {
	case "productName":
		data.ProductName = value
	case "productQuantity":
		data.ProductQuantity = value
	case "deadlineDate":
		data.DeadlineDate = value
	case "storageTemperature":
		data.StorageTemperature = value
	case "individualNumber":
		data.IndividualNumber = value
	case "captureLocation":
		data.CaptureLocation = value
	case "qrCode":
		data.QRCode = value
	case "productIngredient":
		data.ProductIngredient = value
	case "nutritionUnit":
		data.NutritionUnit = value
	case "caloriesQuantity":
		data.CaloriesQuantity = value
	case "proteinQuantity":
		data.ProteinQuantity = value
	case "fatQuantity":
		data.FatQuantity = value
	case "carbohydratesQuantity":
		data.CarbohydratesQuantity = value
	case "saltEquivalentQuantity":
		data.SaltEquivalentQuantity = value
	case "attentionText":
		data.AttentionText = value
	}
}
