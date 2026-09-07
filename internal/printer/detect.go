package printer

import "strings"

// brotherQLModels lists the Brother QL label printers this hub is verified
// against. Both share the 62mm continuous roll and 300dpi head, so the label
// renderer output is identical; only the CUPS queue name differs per site.
var brotherQLModels = []string{
	"QL-800",
	"QL-820NWB",
}

const (
	// scoreKnownModel is given to a CUPS queue whose name carries one of
	// brotherQLModels (e.g. "Brother_QL_800_USB").
	scoreKnownModel = 300
	// scoreGenericQL is given to a queue that looks like some other Brother QL
	// model (e.g. "Brother_QL_1100"). Usable, but never preferred over a
	// verified model.
	scoreGenericQL = 100
)

// DetectBrotherQL returns the CUPS queue that most likely drives a Brother QL
// label printer, or "" when none of the queues look like one.
//
// Sites register the printer under whatever name their setup produced
// (`Brother_QL_800_USB`, `Brother_QL-800`, `Brother QL-820NWB`, ...), so the
// match is made on the model contained in the name rather than on an exact
// string. Ties keep the lpstat order, which makes the choice deterministic.
func DetectBrotherQL(available []string) string {
	best := ""
	bestScore := 0
	for _, name := range available {
		if score := scoreBrotherQL(name); score > bestScore {
			best, bestScore = name, score
		}
	}
	return best
}

// PrinterModel reports the Brother QL model implied by a CUPS queue name,
// or "" when the name carries no recognizable model.
func PrinterModel(name string) string {
	return brotherQLModel(normalizeAlnum(name))
}

func scoreBrotherQL(name string) int {
	normalized := normalizeAlnum(name)
	if normalized == "" {
		return 0
	}
	if brotherQLModel(normalized) != "" {
		return scoreKnownModel
	}
	if strings.Contains(normalized, "brother") && strings.Contains(normalized, "ql") {
		return scoreGenericQL
	}
	return 0
}

func brotherQLModel(normalized string) string {
	for _, model := range brotherQLModels {
		if strings.Contains(normalized, normalizeAlnum(model)) {
			return model
		}
	}
	return ""
}
