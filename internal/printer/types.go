package printer

// LabelData holds the data for label printing.
// Field names match the lite frontend's LabelData type in lib/labels/constants.ts.
type LabelData struct {
	Template           string `json:"template"`           // traceable_deer, pet
	Copies             int    `json:"copies"`             // number of copies (default 1, max 30)
	ProductName        string `json:"productName"`        // 品名
	ProductQuantity    string `json:"productQuantity"`    // 内容量 e.g. "2.35 kg"
	DeadlineDate       string `json:"deadlineDate"`       // 消費期限 e.g. "2026年3月18日"
	StorageTemperature string `json:"storageTemperature"` // 保存温度 e.g. "-18℃以下"

	// Traceable fields
	IndividualID     string `json:"individualId"`     // b-PAC placeholder alias for formatted display value
	IndividualNumber string `json:"individualNumber"` // 個体識別番号 e.g. "1234-56-78-90"
	CaptureLocation  string `json:"captureLocation"`  // 捕獲地
	QRCode           string `json:"qrCode"`           // QRコード URL

	// Legacy template fields kept for template registry compatibility.
	ProductIngredient      string `json:"productIngredient"`
	NutritionUnit          string `json:"nutritionUnit"`
	CaloriesQuantity       string `json:"caloriesQuantity"`
	ProteinQuantity        string `json:"proteinQuantity"`
	FatQuantity            string `json:"fatQuantity"`
	CarbohydratesQuantity  string `json:"carbohydratesQuantity"`
	SaltEquivalentQuantity string `json:"saltEquivalentQuantity"`
	IsHeatedMeatProducts   string `json:"isHeatedMeatProducts"`
	AttentionText          string `json:"attentionText"`

	StorageMethod string `json:"storageMethod"` // 保存方法
}

// ValidTemplates lists the supported template keys.
var ValidTemplates = map[string]bool{
	"traceable_deer": true,
	"pet":            true,
}

// RequiredFields returns the required field names for each template.
func RequiredFields(template string) []string {
	switch template {
	case "traceable_deer":
		return []string{"productName", "captureLocation", "productQuantity", "deadlineDate", "individualId", "qrCode"}
	case "pet":
		return []string{"productName", "productQuantity", "deadlineDate"}
	default:
		return nil
	}
}

// MaxCopies is the maximum number of copies per print job.
const MaxCopies = 30
