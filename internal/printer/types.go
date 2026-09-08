package printer

// LabelData holds the data for label printing.
// Field names match the lite frontend's LabelData type in lib/labels/constants.ts.
type LabelData struct {
	Template           string `json:"template"`           // traceable, non_traceable, processed, pet
	Copies             int    `json:"copies"`             // number of copies (default 1, max 30)
	Locale             string `json:"locale"`             // ja or en
	ProductName        string `json:"productName"`        // 品名
	ProductQuantity    string `json:"productQuantity"`    // 内容量 e.g. "2.35 kg"
	DeadlineDate       string `json:"deadlineDate"`       // 消費期限 e.g. "2026年3月18日"
	DeadlineLabel      string `json:"deadlineLabel"`      // 期限呼称 e.g. "賞味期限" / "消費期限"
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

	// Processed-specific
	IsHeatedMeatProducts string `json:"isHeatedMeatProducts"` // 加熱食肉製品区分

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

	// EzoshikaCertified: 施設がエゾシカ協会認証を取得している場合のみ ninsyo_logo を
	// ラベルに描画する。false (デフォルト) なら認証マークは表示しない。
	// JA traceable / EN bilingual traceable 両方で参照される。
	EzoshikaCertified bool `json:"ezoshikaCertified"`

	// EN bilingual traceable label (issue #271): JA companion values for "EN / JA" display.
	// Used only when Locale == "en" and template is traceable_*.
	ProductNameJa        string `json:"productNameJa"`        // 商品名の JA 値 (例: "ウデ")
	ProductQuantityJa    string `json:"productQuantityJa"`    // 内容量の JA 値 (通常は EN と共通なので省略可)
	DeadlineDateJa       string `json:"deadlineDateJa"`       // 賞味期限の JA 表記 (例: "2028年5月1日")
	StorageTemperatureJa string `json:"storageTemperatureJa"` // 保存方法の JA 値 (例: "要冷凍")

	// EN bilingual traceable additional fields
	SpeciesOfOrigin     string `json:"speciesOfOrigin"`     // 品種 EN 値 (例: "Cervus Nippon")
	SpeciesOfOriginJa   string `json:"speciesOfOriginJa"`   // 品種 JA 値 (例: "日本鹿")
	CountryOfOrigin     string `json:"countryOfOrigin"`     // 原産地 EN (例: "Product of Japan")
	CountryOfOriginJa   string `json:"countryOfOriginJa"`   // 原産地 JA (例: "日本")
	ProcessingPlantName string `json:"processingPlantName"` // 製造所名 (バイリンガル時に CompanyBlock の代替)
	Address             string `json:"address"`             // 住所 EN (Address 行用)
	AddressJa           string `json:"addressJa"`           // 住所 JA
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
