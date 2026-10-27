package printer

import (
	"reflect"
	"strings"
	"testing"
)

func TestSellerRowIsOptionalAndSeparate(t *testing.T) {
	for _, template := range []string{"traceable", "traceable_deer", "traceable_bear", "traceable_boar", "traceable_raccoon"} {
		t.Run(template, func(t *testing.T) {
			base := LabelData{Template: template, Locale: "ja", CompanyBlock: "加工会社\n加工会社住所", FacilityBlock: "加工施設\n加工施設住所"}
			before := buildTableEntries(base)
			base.SellerBlock = "  \n "
			if !reflect.DeepEqual(before, buildTableEntries(base)) {
				t.Fatal("empty seller changed existing rows")
			}
			base.SellerBlock = "株式会社ロピア（検証用）\n北海道テスト市1-2-3"
			after := buildTableEntries(base)
			entry, found := findEntry(after, "販売者")
			if !found || entry.value != base.SellerBlock || len(after) != len(before)+1 {
				t.Fatal("seller row missing or duplicated")
			}
			withoutSeller := []tableEntry{}
			for _, row := range after {
				if row.label != "販売者" {
					withoutSeller = append(withoutSeller, row)
				}
			}
			if !reflect.DeepEqual(before, withoutSeller) {
				t.Fatal("processor, facility, or other rows changed")
			}
		})
	}
}

func TestSellerDoesNotChangeOtherLayouts(t *testing.T) {
	for _, template := range []string{"processed", "pet", "non_traceable", "non_traceable_deer", "carcass_deer", "carcass_bear", "individual_qr", "traceable_deer"} {
		for _, locale := range []string{"ja", "en"} {
			if template == "traceable_deer" && locale == "ja" {
				continue
			}
			base := LabelData{Template: template, Locale: locale, ProductName: "テスト", IndividualNumber: "2026102701"}
			before := buildTableEntries(base)
			base.SellerBlock = "販売会社\n住所"
			if !reflect.DeepEqual(before, buildTableEntries(base)) {
				t.Fatalf("seller changed %s/%s", template, locale)
			}
		}
	}
}

func TestSellerAddressIsFullyWrappedWithoutEllipsis(t *testing.T) {
	block := "株式会社ロピア（検証用）\n" + strings.Repeat("北海道函館市テスト町", 15) + "末尾確認123"
	entry, ok := sellerEntry(LabelData{Template: "traceable_deer", Locale: "ja", SellerBlock: block})
	if !ok {
		t.Fatal("seller missing")
	}
	layout := (tableBlockRow{entries: []tableEntry{entry}, fontSize: fontSizeBody, maxLines: maxTableLines, labelWidthRatio: tableLabelWidthTraceable}).layout()
	got := strings.Join(layout.rows[0].valueLines, "")
	if got != strings.ReplaceAll(block, "\n", "") {
		t.Fatalf("seller truncated: %q", got)
	}
	if len(layout.rows[0].valueLines) <= 3 {
		t.Fatal("long address did not grow the row")
	}
}

func TestValidateSellerLabelScopeAndLimits(t *testing.T) {
	for _, tt := range []struct {
		template, locale, block string
		valid                   bool
	}{
		{"processed", "ja", "", true},
		{"traceable_deer", "ja", "会社\n住所", true},
		{"traceable_bear", "ja", "会社\n住所\nTEL: 000-0000", true},
		{"traceable_deer", "en", "会社\n住所", false},
		{"pet", "ja", "会社\n住所", false},
		{"processed", "ja", "会社\n住所", false},
		{"carcass_deer", "ja", "会社\n住所", false},
		{"traceable_deer", "ja", "会社だけ", false},
		{"traceable_deer", "ja", "会社\n\n住所", false},
		{"traceable_deer", "ja", "会社\n住所\x00", false},
		{"traceable_deer", "ja", "会社\n" + strings.Repeat("あ", 801), false},
	} {
		err := ValidateSellerLabel(tt.template, map[string]string{"locale": tt.locale, "sellerBlock": tt.block})
		if (err == nil) != tt.valid {
			t.Fatalf("%s/%s: valid=%v, err=%v", tt.template, tt.locale, tt.valid, err)
		}
	}
}
