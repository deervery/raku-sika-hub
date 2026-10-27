package printer

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const SellerLabelFeature = "seller-label-ja-traceable-v1"

// ValidateSellerLabel leaves all existing requests unchanged when no seller is given.
func ValidateSellerLabel(template string, data map[string]string) error {
	block := strings.TrimSpace(data["sellerBlock"])
	if block == "" {
		return nil
	}
	if !isTraceableTemplate(template) || resolveLocale(data) != "ja" {
		return fmt.Errorf("販売者欄は日本語の精肉ラベルのみ対応しています。")
	}
	if utf8.RuneCountInString(block) > 800 {
		return fmt.Errorf("販売者の情報は800文字以内で指定してください。")
	}
	lines := strings.Split(strings.ReplaceAll(block, "\r\n", "\n"), "\n")
	if len(lines) < 2 || len(lines) > 3 {
		return fmt.Errorf("販売者の名称・住所・任意の電話番号を2〜3行で指定してください。")
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			return fmt.Errorf("販売者の名称・住所を入力してください。")
		}
		for _, char := range line {
			if unicode.IsControl(char) {
				return fmt.Errorf("販売者に制御文字は指定できません。")
			}
		}
	}
	return nil
}

func sellerEntry(data LabelData) (tableEntry, bool) {
	value := strings.TrimSpace(data.SellerBlock)
	if value == "" || !isTraceableTemplate(data.Template) || data.Locale != "ja" {
		return tableEntry{}, false
	}
	return tableEntry{
		label: "販売者", value: value,
		// Wrap the complete name/address rather than clamp and lose the end.
		maxValueLines: utf8.RuneCountInString(value) + 1,
		keepValueFont: true,
	}, true
}
