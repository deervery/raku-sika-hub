package printer

// LabelData holds the data for label printing.
// Field names match the lite frontend's LabelData type in lib/labels/constants.ts.
type LabelData struct {
	Template           string `json:"template"`           // traceable, non_traceable, processed, pet
	Copies             int    `json:"copies"`             // number of copies (default 1, max 30)
	ProductName        string `json:"productName"`        // 品名
	ProductQuantity    string `json:"productQuantity"`    // 内容量 e.g. "2.35 kg"
	DeadlineDate       string `json:"deadlineDate"`       // 消費期限 e.g. "2026年3月18日"
	StorageTemperature string `json:"storageTemperature"` // 保存温度 e.g. "-18℃以下"

	// Traceable fields
	IndividualNumber string `json:"individualNumber"` // 個体識別番号 e.g. "1234-56-78-90"
	CaptureLocation  string `json:"captureLocation"`  // 捕獲地
	QRCode           string `json:"qrCode"`           // QRコード URL

	// Processed / Pet fields
	ProductIngredient      string `json:"productIngredient"`      // 原材料名
	NutritionUnit          string `json:"nutritionUnit"`          // 栄養成分表示単位 e.g. "100gあたり"
	CaloriesQuantity       string `json:"caloriesQuantity"`       // エネルギー
	ProteinQuantity        string `json:"proteinQuantity"`        // たんぱく質
	FatQuantity            string `json:"fatQuantity"`            // 脂質
	CarbohydratesQuantity  string `json:"carbohydratesQuantity"`  // 炭水化物
	SaltEquivalentQuantity string `json:"saltEquivalentQuantity"` // 食塩相当量

	// Block fields (v0.8.2) - 改行区切りの複数行テキスト
	CompanyBlock  string `json:"companyBlock"`  // 加工者情報ブロック（name/address/phone、改行区切り）
	FacilityBlock string `json:"facilityBlock"` // 加工所情報ブロック（name/address/phone、改行区切り）

	// Misc
	AttentionText         string `json:"attentionText"`         // 注意書き
	FacilityName          string `json:"facilityName"`          // 加工施設名
	Ingredient            string `json:"ingredient"`            // 原材料
	LogoFile              string `json:"logoFile"`              // 企業ロゴ画像へのパス（assetsDir からの相対パスまたは絶対パス）
	CertificationMarkFile string `json:"certificationMarkFile"` // 認証マーク画像へのパス（assetsDir からの相対パスまたは絶対パス）
	ProcessorName         string `json:"processorName"`         // 旧API互換の加工者名
	ProcessorLocation     string `json:"processorLocation"`     // 旧API互換の加工所所在地

	// Carcass label fields
	Species       string `json:"species"`       // 獣種
	Sex           string `json:"sex"`           // 性別
	ReceivingDate string `json:"receivingDate"` // 搬入日
}

// ValidTemplates lists the supported template keys.
var ValidTemplates = map[string]bool{
	"traceable":          true,
	"traceable_deer":     true,
	"traceable_bear":     true,
	"traceable_boar":     true,
	"traceable_raccoon":  true,
	"non_traceable":      true,
	"non_traceable_deer": true,
	"processed":          true,
	"pet":                true,
	"individual_qr":      true,
	"carcass_deer":       true,
	"carcass_bear":       true,
}

// RequiredFields returns the required field names for each template.
func RequiredFields(template string) []string {
	common := []string{
		"productName",
		"productQuantity",
		"deadlineDate",
		"storageTemperature",
	}
	switch template {
	case "traceable", "traceable_deer", "traceable_bear", "traceable_boar", "traceable_raccoon":
		return append(common, "individualNumber", "captureLocation", "qrCode")
	case "carcass_deer", "carcass_bear":
		return []string{"individualNumber"}
	default:
		return common
	}
}

// MaxCopies is the maximum number of copies per print job.
const MaxCopies = 30
