package printer

import "testing"

func TestDetectBrotherQL(t *testing.T) {
	tests := []struct {
		name      string
		available []string
		want      string
	}{
		{
			name:      "QL-800 registered under an unconfigured queue name",
			available: []string{"Brother_QL-800"},
			want:      "Brother_QL-800",
		},
		{
			name:      "QL-820NWB USB queue",
			available: []string{"Brother_QL_820NWB_USB"},
			want:      "Brother_QL_820NWB_USB",
		},
		{
			name:      "known model wins over an unrelated printer",
			available: []string{"Canon_LBP", "PDF", "Brother_QL_800_USB"},
			want:      "Brother_QL_800_USB",
		},
		{
			name:      "known model wins over a generic Brother QL",
			available: []string{"Brother_QL_1100", "Brother_QL-820NWB"},
			want:      "Brother_QL-820NWB",
		},
		{
			name:      "other Brother QL models are still usable",
			available: []string{"Canon_LBP", "Brother_QL_1110NWB"},
			want:      "Brother_QL_1110NWB",
		},
		{
			name:      "first match wins when several share the top score",
			available: []string{"Brother_QL_800_USB", "Brother_QL-820NWB"},
			want:      "Brother_QL_800_USB",
		},
		{
			name:      "no Brother QL printer",
			available: []string{"Canon_LBP", "PDF"},
			want:      "",
		},
		{
			name:      "no printers at all",
			available: nil,
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectBrotherQL(tt.available); got != tt.want {
				t.Fatalf("DetectBrotherQL(%v) = %q, want %q", tt.available, got, tt.want)
			}
		})
	}
}

func TestPrinterModel(t *testing.T) {
	tests := []struct {
		queue string
		want  string
	}{
		{"Brother_QL_800_USB", "QL-800"},
		{"Brother_QL-800", "QL-800"},
		{"Brother QL-800 (USB)", "QL-800"},
		{"Brother_QL_820NWB_USB", "QL-820NWB"},
		{"Brother_QL-820NWB", "QL-820NWB"},
		{"Brother_QL_1100", ""},
		{"Canon_LBP", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := PrinterModel(tt.queue); got != tt.want {
			t.Errorf("PrinterModel(%q) = %q, want %q", tt.queue, got, tt.want)
		}
	}
}

// A QL-820NWB queue name must never be read as a QL-800 (or the reverse),
// since "QL-800" is not a substring of "QL-820NWB" only after normalization.
func TestPrinterModelDoesNotCrossMatch(t *testing.T) {
	if got := PrinterModel("Brother_QL_820NWB_USB"); got != "QL-820NWB" {
		t.Fatalf("QL-820NWB queue resolved to %q", got)
	}
	if got := PrinterModel("Brother_QL_800_USB"); got != "QL-800" {
		t.Fatalf("QL-800 queue resolved to %q", got)
	}
}

func TestResolvePrinter(t *testing.T) {
	tests := []struct {
		name        string
		configured  []string
		available   []string
		defaultName string
		wantName    string
		wantSource  string
	}{
		{
			name:       "configured name that exists wins",
			configured: []string{"Brother_QL_800_USB", "Brother_QL_820NWB_USB"},
			available:  []string{"Brother_QL_820NWB_USB", "Brother_QL_800_USB"},
			wantName:   "Brother_QL_800_USB",
			wantSource: "configured",
		},
		{
			name:       "second configured name is used when the first is absent",
			configured: []string{"Brother_QL_800_USB", "Brother_QL_820NWB_USB"},
			available:  []string{"Brother_QL_820NWB_USB"},
			wantName:   "Brother_QL_820NWB_USB",
			wantSource: "configured",
		},
		{
			name:       "QL-800 under an unconfigured queue name is auto-detected",
			configured: []string{"Brother_QL_820NWB_USB"},
			available:  []string{"Brother_QL-800"},
			wantName:   "Brother_QL-800",
			wantSource: "auto-detected",
		},
		{
			name:        "CUPS default is preferred when it is a Brother QL",
			configured:  []string{"Brother_QL_820NWB_USB"},
			available:   []string{"Brother_QL-800", "Brother_QL_1100"},
			defaultName: "Brother_QL_1100",
			wantName:    "Brother_QL_1100",
			wantSource:  "cups-default",
		},
		{
			name:        "a non-QL default printer is never used for labels",
			configured:  []string{"Brother_QL_820NWB_USB"},
			available:   []string{"Canon_LBP", "Brother_QL-800"},
			defaultName: "Canon_LBP",
			wantName:    "Brother_QL-800",
			wantSource:  "auto-detected",
		},
		{
			name:       "unmatched config with no QL printer keeps the configured name",
			configured: []string{"Brother_QL_820NWB_USB"},
			available:  []string{"Canon_LBP"},
			wantName:   "Brother_QL_820NWB_USB",
			wantSource: "configured",
		},
		{
			name:        "no config falls back to the CUPS default",
			available:   []string{"Canon_LBP"},
			defaultName: "Canon_LBP",
			wantName:    "Canon_LBP",
			wantSource:  "cups-default",
		},
		{
			name:       "no config and no default falls back to the first printer",
			available:  []string{"Canon_LBP", "PDF"},
			wantName:   "Canon_LBP",
			wantSource: "first-available",
		},
		{
			name:       "nothing to resolve",
			wantName:   "",
			wantSource: "unresolved",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, source := resolvePrinter(tt.configured, tt.available, tt.defaultName)
			if name != tt.wantName || source != tt.wantSource {
				t.Fatalf("resolvePrinter() = (%q, %q), want (%q, %q)", name, source, tt.wantName, tt.wantSource)
			}
		})
	}
}
