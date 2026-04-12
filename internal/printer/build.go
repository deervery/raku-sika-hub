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

	return LabelData{
		Template:               template,
		Copies:                 copies,
		ProductName:            normalized["productName"],
		ProductQuantity:        normalized["productQuantity"],
		DeadlineDate:           normalized["deadlineDate"],
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
		AttentionText:          normalized["attentionText"],
		FacilityName:           normalized["facilityName"],
		Ingredient:             normalized["ingredient"],
		LogoFile:               resolveLogoField(normalized["logoFile"], assetsDir),
		CertificationMarkFile:  normalized["certificationMarkFile"],
		ProcessorName:          normalized["processorName"],
		ProcessorLocation:      normalized["processorLocation"],
		CompanyBlock:           normalized["companyBlock"],
		FacilityBlock:          normalized["facilityBlock"],
		Species:                normalized["species"],
		Sex:                    normalized["sex"],
		ReceivingDate:          normalized["receivingDate"],
	}
}

func resolveLogoField(input string, assetsDir string) string {
	if trimmed := strings.TrimSpace(input); trimmed != "" {
		return trimmed
	}
	return DefaultLogoFile(assetsDir)
}
