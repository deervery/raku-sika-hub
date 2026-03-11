package printer

// LabelData holds the data for label printing.
// Field names match the lite frontend's LabelData type in lib/labels/constants.ts.
type LabelData struct {
	Template           string `json:"template"`           // traceable_deer, traceable_bear, non_traceable_deer, processed, pet
	Copies             int    `json:"copies"`             // number of copies (default 1, max 30)
	ProductName        string `json:"productName"`        // 品名
	ProductQuantity    string `json:"productQuantity"`    // 内容量 e.g. "2.35 kg"
	DeadlineDate       string `json:"deadlineDate"`       // 消費期限 e.g. "2026年3月18日"
	StorageTemperature string `json:"storageTemperature"` // 保存温度 e.g. "-18℃以下"

	// Traceable fields
	IndividualNumber string `json:"individualNumber"` // 個体識別番号 e.g. "1234-56-78-90"
	CaptureLocation  string `json:"captureLocation"`  // 捕獲場所
	QRCode           string `json:"qrCode"`           // QRコード URL

	// Processed / Pet fields
	ProductIngredient      string `json:"productIngredient"`      // 原材料名
	NutritionUnit          string `json:"nutritionUnit"`          // 栄養成分表示単位 e.g. "100gあたり"
	CaloriesQuantity       string `json:"caloriesQuantity"`       // エネルギー
	ProteinQuantity        string `json:"proteinQuantity"`        // たんぱく質
	FatQuantity            string `json:"fatQuantity"`            // 脂質
	CarbohydratesQuantity  string `json:"carbohydratesQuantity"`  // 炭水化物
	SaltEquivalentQuantity string `json:"saltEquivalentQuantity"` // 食塩相当量

	// Misc
	AttentionText string `json:"attentionText"` // 注意書き
}

// ValidTemplates lists the supported template keys.
var ValidTemplates = map[string]bool{
	"traceable":          true,
	"traceable_deer":     true,
	"traceable_bear":     true,
	"non_traceable":      true,
	"non_traceable_deer": true,
	"processed":          true,
	"pet":                true,
}

// RequiredFields returns the required field names for each template.
func RequiredFields(template string) []string {
	common := []string{"productName", "productQuantity", "deadlineDate", "storageTemperature"}
	switch template {
	case "traceable", "traceable_deer", "traceable_bear":
		return append(common, "individualNumber")
	default:
		return common
	}
}

// MaxCopies is the maximum number of copies per print job.
const MaxCopies = 30
