package printer

import (
	"image"
	"image/color"
	"image/draw"
	"os"
	"strings"
	"testing"

	"golang.org/x/image/font"
)

func whiteImage(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	return img
}

func isDark(img *image.RGBA, x, y int) bool {
	c := img.RGBAAt(x, y)
	return int(c.R)+int(c.G)+int(c.B) < 3*128
}

func testRenderer(t *testing.T) *LabelRenderer {
	t.Helper()
	r, err := NewLabelRenderer("", "")
	if err != nil {
		t.Skipf("skipping render test: %v", err)
	}
	return r
}

// A line too wide for its cell is squeezed into it. The squeezed text used to
// be pasted as an opaque white box taller than the line, erasing the bottom
// of the line above (「加熱して」 on the traceable label) and the table rule.
func TestDrawStringFitWidth_KeepsWhatIsUnderIt(t *testing.T) {
	r := testRenderer(t)
	face := r.makeFace(fontSizeBody)
	defer face.Close()

	const x, maxWidth, baseline = 10, 200, 100
	text := "加熱してお召し上がりください。加熱してお召し上がりください。"
	if w := font.MeasureString(face, text).Ceil(); w <= maxWidth {
		t.Fatalf("text is %dpx wide; it must be wider than %dpx to be squeezed", w, maxWidth)
	}
	ascent := face.Metrics().Ascent.Ceil()
	img := whiteImage(maxWidth+2*x, baseline+ascent)
	ruleY := baseline - ascent + 1 // inside the squeezed text's box
	drawHLine(img, 0, img.Bounds().Dx(), ruleY, color.Black)

	drawStringFitWidth(img, face, text, x, baseline, maxWidth)

	for px := 0; px < img.Bounds().Dx(); px++ {
		if !isDark(img, px, ruleY) {
			t.Fatalf("the line under the text was erased at x=%d", px)
		}
	}
}

// Table text must sit clear of the rules above and below it.
func TestTableBlock_TextClearOfRules(t *testing.T) {
	r := testRenderer(t)
	tbl := tableBlockRow{
		entries: []tableEntry{
			{label: "名称", value: "鹿肉（モモ）ブロック"},
			{label: "内容量", value: "2.35 kg"},
			{label: "加工者", value: "株式会社サンプル食肉加工センター 北海道札幌市中央区北一条西一丁目一番地", maxValueLines: 2},
			{label: "保存方法", value: "-18℃以下で保存してください", valueLineGap: 1.35, keepValueFont: true},
		},
		fontSize: fontSizeBody,
		maxLines: maxTableLines,
	}
	layout := tbl.layout()
	img := whiteImage(labelWidthPx, layout.totalHeight+20)
	const top = 10
	tbl.draw(img, r, top)

	vertical := map[int]bool{
		contentLeft:                     true,
		contentLeft + layout.labelWidth: true,
		contentLeft + contentWidth:      true,
	}
	rule := top
	for i, row := range layout.rows {
		for _, y := range []int{rule + 1, rule + row.height - 1} {
			for x := contentLeft; x <= contentLeft+contentWidth; x++ {
				if !vertical[x] && isDark(img, x, y) {
					t.Fatalf("row %d: text touches the rule at (%d,%d)", i, x, y)
				}
			}
		}
		rule += row.height
	}
	rule = top
	for i, row := range append([]tableRowLayout{{}}, layout.rows...) {
		rule += row.height
		for x := contentLeft; x <= contentLeft+contentWidth; x++ {
			if !isDark(img, x, rule) {
				t.Fatalf("rule %d is broken at x=%d", i, x)
			}
		}
	}
}

// Nothing in the table is cut with "...": ingredients with their allergens,
// nutrition and the processor's TEL used to be dropped when they ran long.
func TestTableLayout_PrintsEveryCharacter(t *testing.T) {
	ingredients := "鹿肉（北海道産）、豚脂、食塩、砂糖、香辛料、ポークエキス／調味料（アミノ酸等）、リン酸塩（Na）、酸化防止剤（ビタミンC）、発色剤（亜硝酸Na）、（一部に豚肉を含む）"
	company := "株式会社サンプルジビエファクトリー北海道\n北海道川上郡標茶町字虹別原野基線123番地の45\nTEL 015-000-0000"
	data := LabelData{
		Template:               "processed",
		ProductName:            "鹿肉ソーセージ",
		ProductIngredient:      ingredients,
		ProductQuantity:        "200 g",
		DeadlineDate:           "2026年11月1日",
		StorageTemperature:     "10℃以下",
		NutritionUnit:          "100gあたり",
		CaloriesQuantity:       "210kcal",
		ProteinQuantity:        "18.2g",
		FatQuantity:            "14.1g",
		CarbohydratesQuantity:  "2.3g",
		SaltEquivalentQuantity: "1.8g",
		CompanyBlock:           company,
	}
	entries := buildTableEntries(data)
	tbl := tableBlockRow{entries: entries, fontSize: fontSizeBody, maxLines: maxTableLines}
	for i, row := range tbl.layout().rows {
		got := strings.Join(row.valueLines, "")
		want := strings.ReplaceAll(entries[i].value, "\n", "")
		if got != want {
			t.Errorf("%s: printed %q, want %q", entries[i].label, got, want)
		}
	}
}

// Every template keeps its ink inside the 696 dots the QL-800 head prints
// (the centre of the 732-dot canvas). Raw printing crops the rest, which cut
// the EN label's outer border.
func TestRender_EveryTemplateFitsThePrintHead(t *testing.T) {
	r := testRenderer(t)
	const headDots = 696
	edge := (labelWidthPx - headDots) / 2
	common := map[string]string{
		"productName": "エゾシカ ロース", "productQuantity": "0.52 kg",
		"deadlineDate": "2026年10月10日", "storageTemperature": "-18℃以下",
		"individualNumber": "0123-45-67-89", "captureLocation": "北海道 標茶町",
		"qrCode": "https://rakusika.com/t/0123456789", "ezoshikaCertified": "true",
		"companyBlock":  "株式会社サンプル\n北海道川上郡標茶町1-2-3\nTEL 015-000-0000",
		"facilityBlock": "サンプル処理施設\n北海道川上郡標茶町4-5-6\nTEL 015-000-0001",
		"species":       "エゾシカ", "sex": "メス", "receivingDate": "2026年9月20日", "facilityName": "サンプル処理施設",
		"productIngredient": "鹿肉（北海道産）、食塩", "nutritionUnit": "100gあたり", "caloriesQuantity": "210kcal",
	}
	cases := map[string]map[string]string{}
	for tpl := range ValidTemplates {
		cases[tpl] = common
	}
	en := map[string]string{"locale": "en", "productName": "Venison Loin", "productNameJa": "エゾシカ ロース"}
	for k, v := range common {
		if _, ok := en[k]; !ok {
			en[k] = v
		}
	}
	cases["traceable (en)"] = en

	for name, fields := range cases {
		tpl := strings.TrimSuffix(name, " (en)")
		res, err := r.Render(BuildLabelDataFromMap(tpl, 1, fields, ""))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		img, err := decodePNG(res.Path)
		os.Remove(res.Path)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for _, x := range []int{edge - 1, labelWidthPx - edge} {
				if c := color.GrayModel.Convert(img.At(x, y)).(color.Gray); c.Y <= 128 {
					t.Errorf("%s: ink at (%d,%d) is outside the print head", name, x, y)
					break
				}
			}
		}
	}
}
