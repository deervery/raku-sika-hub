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

	vertical := map[int]bool{}
	for _, x := range []int{contentLeft, contentLeft + 1,
		contentLeft + layout.labelWidth - 1, contentLeft + layout.labelWidth,
		contentLeft + contentWidth - 1, contentLeft + contentWidth} {
		vertical[x] = true
	}
	// Rules are ruleDots wide: the top one below its line, the others above.
	rule := top
	for i, row := range layout.rows {
		for _, y := range []int{rule + ruleDots, rule + row.height - ruleDots} {
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

// Nothing in the table is cut with "...": a long address used to push the
// TEL line out, and long values were cut at 8pt.
func TestTableLayout_PrintsEveryCharacter(t *testing.T) {
	company := "株式会社サンプルジビエファクトリー北海道\n北海道川上郡標茶町字虹別原野基線123番地の45\nTEL 015-000-0000"
	data := LabelData{
		Template:           "traceable",
		ProductName:        "エゾシカ ロース ブロック（背ロース・ヒレ・内もも・外もも・肩ロース詰め合わせ）スライス用",
		ProductQuantity:    "0.52 kg",
		DeadlineDate:       "2026年10月10日",
		StorageTemperature: "-18℃以下",
		IndividualNumber:   "0123-45-67-89",
		CaptureLocation:    "北海道 標茶町",
		CompanyBlock:       company,
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

func TestProcessedLabel_IsTheLbxLandscapeLayout(t *testing.T) {
	r := testRenderer(t)
	res, err := r.Render(BuildLabelDataFromMap("processed", 1, map[string]string{
		"productName": "鹿肉ソーセージ", "productQuantity": "200 g",
		"deadlineDate": "2026年11月1日", "storageTemperature": "10℃以下",
	}, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(res.Path)
	img, err := decodePNG(res.Path)
	if err != nil {
		t.Fatal(err)
	}
	// processed.lbx: 175.7pt × 322.3pt, sent rotated like the EN label.
	if b := img.Bounds(); b.Dx() != labelWidthPx || b.Dy() != pt(procWidthPt) {
		t.Fatalf("size = %dx%d, want %dx%d", b.Dx(), b.Dy(), labelWidthPx, pt(procWidthPt))
	}
	if res.WidthMM != 62 || res.HeightMM != 114 {
		t.Fatalf("media = %dx%dmm, want 62x114mm", res.WidthMM, res.HeightMM)
	}
}

// A long ingredient list with its additives and allergens is printed in full
// in the 原材料名 frame, shrinking no further than the legal minimum.
func TestProcessedLabel_IngredientsFitWithoutCutting(t *testing.T) {
	r := testRenderer(t)
	text := "鹿肉（北海道産）、豚脂、食塩、砂糖、香辛料、ポークエキス／調味料（アミノ酸等）、リン酸塩（Na）、酸化防止剤（ビタミンC）、発色剤（亜硝酸Na）、（一部に豚肉を含む）"
	inset := pt(procCellInsetPt)
	w := pt(161.3) - pt(33.6) - 2*inset
	h := pt(75.9) - pt(19.5) - 2*inset
	size, lines := r.fitWrapped([]string{text}, w, h, procFontSize)
	if strings.Join(lines, "") != text {
		t.Fatalf("printed %q", strings.Join(lines, ""))
	}
	if size < 5.5 {
		t.Fatalf("shrunk to %.2fpt, below the 5.5pt minimum", size)
	}
}

func TestWrapJapanese_KeepsNumbersAndPunctuationTogether(t *testing.T) {
	r := testRenderer(t)
	face := r.makeFace(procFontSize)
	defer face.Close()
	text := "北海道小樽市銭函3丁目23-203、鹿肉（北海道産）、豚脂、食塩、香辛料"
	for _, width := range []int{200, 260, 330, 400} {
		lines := wrapJapanese(face, text, width)
		if strings.Join(lines, "") != text {
			t.Fatalf("width %d: lost text: %q", width, lines)
		}
		for i, line := range lines {
			if strings.HasPrefix(line, "、") || strings.HasSuffix(line, "（") {
				t.Errorf("width %d: line %d breaks at punctuation: %q", width, i, lines)
			}
			if i > 0 && strings.HasPrefix(line, "203") {
				t.Errorf("width %d: 23-203 split: %q", width, lines)
			}
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
	withPla := map[string]string{"plaMark": "true"}
	for k, v := range common {
		withPla[k] = v
	}
	for _, tpl := range []string{"traceable", "traceable_bear", "non_traceable", "processed", "pet"} {
		cases[tpl+" (pla)"] = withPla
	}

	for name, fields := range cases {
		tpl, _, _ := strings.Cut(name, " (")
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

func TestNonTraceableLabel_IsTheLbxLandscapeLayout(t *testing.T) {
	r := testRenderer(t)
	for _, tpl := range []string{"non_traceable", "non_traceable_deer"} {
		res, err := r.Render(BuildLabelDataFromMap(tpl, 1, map[string]string{
			"productName": "エゾシカ 切り落とし", "productQuantity": "0.30 kg",
			"deadlineDate": "2026年10月10日", "storageTemperature": "-18℃以下",
			"facilityBlock": "サンプル処理施設\n北海道訓子府町大町113",
		}, ""))
		if err != nil {
			t.Fatal(err)
		}
		img, err := decodePNG(res.Path)
		os.Remove(res.Path)
		if err != nil {
			t.Fatal(err)
		}
		// non_traceable.lbx: 175.7pt × 145.4pt, sent rotated.
		if b := img.Bounds(); b.Dx() != labelWidthPx || b.Dy() != pt(nonTrWidthPt) {
			t.Fatalf("%s: size = %dx%d", tpl, b.Dx(), b.Dy())
		}
		if res.WidthMM != 62 || res.HeightMM != 51 {
			t.Fatalf("%s: media = %dx%dmm, want 62x51mm", tpl, res.WidthMM, res.HeightMM)
		}
	}
}

// 加工者名 / 加工施設所在地 are the facility's, as the lbx prints them.
func TestProcessorOf(t *testing.T) {
	cases := []struct {
		data          LabelData
		name, address string
	}{
		{LabelData{FacilityBlock: "(株)サンプル(シクヌ)\n北海道訓子府町大町113", CompanyBlock: "株式会社サンプル\n札幌市\nTEL 011"},
			"(株)サンプル(シクヌ)", "北海道訓子府町大町113"},
		{LabelData{ProcessorName: "工場Z", ProcessorLocation: "札幌市"}, "工場Z", "札幌市"},
		{LabelData{CompanyBlock: "株式会社サンプル\n札幌市西区\nTEL 011"}, "株式会社サンプル", "札幌市西区\nTEL 011"},
	}
	for _, c := range cases {
		if n, a := processorOf(c.data); n != c.name || a != c.address {
			t.Errorf("processorOf(%+v) = %q, %q; want %q, %q", c.data, n, a, c.name, c.address)
		}
	}
}

// The プラ mark is printed only for a facility that enables it, on the meat,
// processed and pet labels.
func TestPlaMark_OnlyWhenTheFacilityEnablesIt(t *testing.T) {
	r := testRenderer(t)
	fields := map[string]string{
		"productName": "エゾシカ ロース", "productQuantity": "0.52 kg",
		"deadlineDate": "2026年10月10日", "storageTemperature": "-18℃以下",
		"individualNumber": "0123-45-67-89", "captureLocation": "北海道 標茶町",
		"qrCode": "https://rakusika.com/t/0123456789",
	}
	render := func(tpl string, pla bool) image.Image {
		f := map[string]string{}
		for k, v := range fields {
			f[k] = v
		}
		if pla {
			f["plaMark"] = "true"
		}
		res, err := r.Render(BuildLabelDataFromMap(tpl, 1, f, ""))
		if err != nil {
			t.Fatal(err)
		}
		defer os.Remove(res.Path)
		img, err := decodePNG(res.Path)
		if err != nil {
			t.Fatal(err)
		}
		return img
	}
	dark := func(img image.Image) int {
		n := 0
		b := img.Bounds()
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				if color.GrayModel.Convert(img.At(x, y)).(color.Gray).Y <= 128 {
					n++
				}
			}
		}
		return n
	}
	// The mark with 「外装」 is several hundred dots of ink.
	for _, tpl := range []string{"traceable", "traceable_bear", "non_traceable", "processed", "pet"} {
		without, with := dark(render(tpl, false)), dark(render(tpl, true))
		if with-without < 500 {
			t.Errorf("%s: plaMark added %d dots of ink", tpl, with-without)
		}
	}
	if !BuildLabelDataFromMap("pet", 1, map[string]string{"plaMark": "true"}, "").PlaMark {
		t.Fatal("plaMark=true is not read")
	}
	if BuildLabelDataFromMap("pet", 1, map[string]string{}, "").PlaMark {
		t.Fatal("plaMark defaults to on")
	}
}
