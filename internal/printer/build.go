package printer

import "strings"

// NormalizeRequestData trims request fields and applies known aliases in-place.
func NormalizeRequestData(data map[string]string) map[string]string {
	if data == nil {
		return map[string]string{}
	}
	normalized := map[string]string{}
	for k, v := range data {
		normalized[k] = strings.TrimSpace(v)
	}
	if normalized["storageTemperature"] == "" && normalized["storageMethod"] != "" {
		normalized["storageTemperature"] = normalized["storageMethod"]
	}
	return normalized
}

// BuildLabelDataFromMap converts print request payloads into LabelData.
// The caller is responsible for validating required fields and copy limits.
func BuildLabelDataFromMap(template string, copies int, data map[string]string, assetsDir string) LabelData {
	normalized := NormalizeRequestData(data)
	locale := resolveLocale(normalized)

	return LabelData{
		Template:               template,
		Copies:                 copies,
		Locale:                 locale,
		ProductName:            normalized["productName"],
		ProductQuantity:        normalized["productQuantity"],
		DeadlineDate:           normalized["deadlineDate"],
		DeadlineLabel:          normalized["deadlineLabel"],
		StorageTemperature:     normalized["storageTemperature"],
		IndividualNumber:       normalized["individualNumber"],
		CaptureLocation:        normalized["captureLocation"],
		QRCode:                 normalized["qrCode"],
		ProductIngredient:      normalized["productIngredient"],
		NutritionUnit:          normalized["nutritionUnit"],
		CaloriesQuantity:       normalized["caloriesQuantity"],
		ProteinQuantity:        normalized["proteinQuantity"],
		FatQuantity:            normalized["fatQuantity"],
		CarbohydratesQuantity:  normalized["carbohydratesQuantity"],
		SaltEquivalentQuantity: normalized["saltEquivalentQuantity"],
		IsHeatedMeatProducts:   normalized["isHeatedMeatProducts"],
		AttentionText:          normalized["attentionText"],
		FacilityName:           normalized["facilityName"],
		Ingredient:             normalized["ingredient"],
		LogoFile:               resolveLogoField(normalized["logoFile"], assetsDir),
		CertificationMarkFile:  normalized["certificationMarkFile"],
		ProcessorName:          normalized["processorName"],
		ProcessorLocation:      normalized["processorLocation"],
		EzoshikaCertified:      strings.EqualFold(strings.TrimSpace(normalized["ezoshikaCertified"]), "true"),
		CompanyBlock:           normalized["companyBlock"],
		FacilityBlock:          normalized["facilityBlock"],
		Species:                normalized["species"],
		Sex:                    normalized["sex"],
		ReceivingDate:          normalized["receivingDate"],

		// EN bilingual traceable (issue #271)
		ProductNameJa:        normalized["productNameJa"],
		ProductQuantityJa:    normalized["productQuantityJa"],
		DeadlineDateJa:       normalized["deadlineDateJa"],
		StorageTemperatureJa: normalized["storageTemperatureJa"],
		SpeciesOfOrigin:      normalized["speciesOfOrigin"],
		SpeciesOfOriginJa:    normalized["speciesOfOriginJa"],
		CountryOfOrigin:      normalized["countryOfOrigin"],
		CountryOfOriginJa:    normalized["countryOfOriginJa"],
		ProcessingPlantName:  normalized["processingPlantName"],
		Address:              normalized["address"],
		AddressJa:            normalized["addressJa"],
	}
}

func resolveLocale(data map[string]string) string {
	if data == nil {
		return "ja"
	}
	switch strings.ToLower(strings.TrimSpace(data["locale"])) {
	case "en", "en-us", "en-gb":
		return "en"
	case "ja", "ja-jp":
		return "ja"
	}
	switch strings.ToLower(strings.TrimSpace(data["language"])) {
	case "en", "english":
		return "en"
	case "ja", "japanese":
		return "ja"
	}
	deadline := strings.TrimSpace(data["deadlineDate"])
	if deadline != "" && looksEnglishDate(deadline) {
		return "en"
	}
	for _, key := range []string{"companyBlock", "facilityBlock"} {
		if strings.Contains(data[key], "Tel:") {
			return "en"
		}
	}
	return "ja"
}

func looksEnglishDate(value string) bool {
	for _, month := range []string{
		"January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	} {
		if strings.Contains(value, month) {
			return true
		}
	}
	return false
}

func resolveLogoField(input string, assetsDir string) string {
	if trimmed := strings.TrimSpace(input); trimmed != "" {
		return trimmed
	}
	return DefaultLogoFile(assetsDir)
}
