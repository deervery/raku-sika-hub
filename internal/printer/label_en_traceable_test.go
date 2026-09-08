package printer

import (
	"os"
	"strings"
	"testing"
)

func TestIsENBilingualTraceable(t *testing.T) {
	cases := []struct {
		name string
		data LabelData
		want bool
	}{
		{
			name: "en + traceable_deer",
			data: LabelData{Template: "traceable_deer", Locale: "en"},
			want: true,
		},
		{
			name: "en + traceable",
			data: LabelData{Template: "traceable", Locale: "en"},
			want: true,
		},
		{
			name: "ja + traceable",
			data: LabelData{Template: "traceable_deer", Locale: "ja"},
			want: false,
		},
		{
			name: "en + non_traceable",
			data: LabelData{Template: "non_traceable_deer", Locale: "en"},
			want: false,
		},
		{
			name: "en + processed",
			data: LabelData{Template: "processed", Locale: "en"},
			want: false,
		},
		{
			name: "empty locale",
			data: LabelData{Template: "traceable_deer", Locale: ""},
			want: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isENBilingualTraceable(c.data); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestBuildENLeftRows_AllEightRowsPresent(t *testing.T) {
	data := LabelData{
		Template:             "traceable_deer",
		Locale:               "en",
		ProductName:          "Shoulder",
		ProductNameJa:        "ウデ",
		ProductQuantity:      "0.00 kg",
		DeadlineDate:         "1 May 2028",
		StorageTemperature:   "Keep Frozen",
		StorageTemperatureJa: "要冷凍",
		IndividualNumber:     "0000000000",
	}
	rows := buildENLeftRows(data)
	if len(rows) != 8 {
		t.Fatalf("expected 8 rows, got %d", len(rows))
	}
	want := []string{
		"Name of Product",
		"Best-by Date",
		"ID Number",
		"Preservation",
		"Net Weight",
		"Species of Origin",
		"Country of Origin",
		"Metal Check",
	}
	for i, w := range want {
		if rows[i].labelEn != w {
			t.Errorf("row %d label = %q, want %q", i, rows[i].labelEn, w)
		}
	}
}

func TestResolveSpeciesOfOrigin_DefaultsByTemplate(t *testing.T) {
	cases := []struct {
		template string
		wantEn   string
		wantJa   string
	}{
		{"traceable", "Cervus Nippon", "日本鹿"},
		{"traceable_deer", "Cervus Nippon", "日本鹿"},
		{"traceable_bear", "Ursus thibetanus", "ツキノワグマ"},
		{"traceable_boar", "Sus scrofa", "イノシシ"},
	}
	for _, c := range cases {
		t.Run(c.template, func(t *testing.T) {
			en, ja := resolveSpeciesOfOrigin(LabelData{Template: c.template})
			if en != c.wantEn || ja != c.wantJa {
				t.Errorf("got (%q, %q), want (%q, %q)", en, ja, c.wantEn, c.wantJa)
			}
		})
	}
}

func TestResolveSpeciesOfOrigin_CallerOverride(t *testing.T) {
	en, ja := resolveSpeciesOfOrigin(LabelData{
		Template:          "traceable_deer",
		SpeciesOfOrigin:   "Custom Species",
		SpeciesOfOriginJa: "カスタム",
	})
	if en != "Custom Species" || ja != "カスタム" {
		t.Errorf("got (%q, %q), want override values", en, ja)
	}
}

func TestCombineENJA(t *testing.T) {
	cases := []struct {
		en, ja, want string
	}{
		{"Shoulder", "ウデ", "Shoulder/ウデ"},
		{"Shoulder", "", "Shoulder"},
		{"", "ウデ", "ウデ"},
		{"", "", ""},
		{"  Shoulder  ", "  ウデ  ", "Shoulder/ウデ"},
	}
	for _, c := range cases {
		if got := combineENJA(c.en, c.ja); got != c.want {
			t.Errorf("combineENJA(%q, %q) = %q, want %q", c.en, c.ja, got, c.want)
		}
	}
}

func TestTokenizeBilingual_KeepsCJKRunsTogether(t *testing.T) {
	toks := tokenizeBilingual("Product of Japan/日本")
	// "/" stays attached to the preceding ASCII word so wrap puts it on the EN line.
	want := []string{"Product", " ", "of", " ", "Japan/", "日本"}
	if len(toks) != len(want) {
		t.Fatalf("got %d tokens %v, want %d %v", len(toks), toks, len(want), want)
	}
	for i := range want {
		if toks[i] != want[i] {
			t.Errorf("token %d = %q, want %q", i, toks[i], want[i])
		}
	}
}

func TestWrapBilingual_PrefersWordAndSlashBoundaries(t *testing.T) {
	// At 8.5pt body, narrow width should force "Preservation/" + "保存方法" wrap.
	// Verify the break is at "/" (attached to EN side) rather than mid-word.
	lines := wrapBilingual("Preservation/保存方法", 8.5, 220)
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %v", lines)
	}
	if !strings.HasSuffix(lines[0], "/") {
		t.Errorf("expected first line to end with '/', got %q", lines[0])
	}
	// At narrow width "Preservation" stays whole (no mid-word break).
	for _, line := range lines {
		if strings.Contains(line, "Preservati") && !strings.Contains(line, "Preservation") {
			t.Errorf("found mid-word break in line %q", line)
		}
	}
}

func TestRenderENTraceable_ProducesLandscapePNG(t *testing.T) {
	renderer, err := NewLabelRenderer("", "")
	if err != nil {
		t.Skipf("skipping render test: %v", err)
	}
	data := LabelData{
		Template:             "traceable_deer",
		Locale:               "en",
		ProductName:          "Shoulder",
		ProductNameJa:        "ウデ",
		ProductQuantity:      "0.00 kg",
		DeadlineDate:         "1 May 2028",
		StorageTemperature:   "Keep Frozen",
		StorageTemperatureJa: "要冷凍",
		IndividualNumber:     "0000000000",
		QRCode:               "https://rakusika.com/t/SAMPLE",
		ProcessingPlantName:  "Sauvage de Hakodate",
		Address:              "342-3 Zenigame-cho",
		AddressJa:            "北海道函館市銭亀町342-3",
	}
	res, err := renderer.Render(data)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	defer func() {
		if res.Path != "" {
			_ = os.Remove(res.Path)
		}
	}()

	// Portrait 62mm × 103mm (Brother QL-820 tape feeds in 62mm width × variable length;
	// renderENTraceable rotates the landscape canvas 90° to portrait so lp can send
	// it on a 62mm-wide roll without scaling. See label_en_traceable.go #271).
	// canvas 横は 102.5 mm (label 列拡張分 +1.5mm)、四捨五入で 103mm を返す。
	if res.WidthMM != 62 {
		t.Errorf("WidthMM = %d, want 62 (post-rotate)", res.WidthMM)
	}
	if res.HeightMM != 103 {
		t.Errorf("HeightMM = %d, want 103 (post-rotate)", res.HeightMM)
	}
}

