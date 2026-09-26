package printer

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strings"

	"golang.org/x/image/font"
)

// 加工品ラベル: P-touch テンプレート processed.lbx（シクヌ PC 直結用）と同じ
// 横長レイアウト（62mm × 113.7mm）。枠の位置・寸法は lbx の値（pt）をそのまま使う。
//
//	┌ 区分（加熱食肉製品）──────────┐ 栄養成分表示（100gあたり）
//	│ 名称     │                    │ ┌熱量──────┬──────┐
//	│ 原材料名 │                    │ │たんぱく質│      │ …
//	│ 内容量   │              右寄せ│ └食塩相当量┴──────┘
//	│ 期限     │                    │ ┌販売者┬──────────┐
//	│ 保存方法 │                    │ │製造所│          │
//	└──────────┴────────────────────┘ └──────┴──────────┘
//	  注意事項
//
// 文字は lbx と同じく 8pt から枠に収まるまで縮める（折り返し + 縮小）。
// 枠からはみ出す分を「…」で落とすことはしない。
const (
	procWidthPt  = 322.3 // テープ送り方向
	procHeightPt = 175.7 // テープ幅 62mm

	procFontSize = 8.0
	// 収まらないときに限り、切らずにここまで縮める。通常の量なら 5.5pt
	// （表示可能面積 150cm² 以下の食品表示で認められる最小）より大きく収まる。
	procFontFloor = 4.0
	// 折り返さずに縮めるのはここまで（lbx の項目名は 7.2〜7.4pt）。
	procNoWrapMin   = 6.5
	procLineGap     = 1.1
	procCellInsetPt = 1.7 // lbx のセル内テキスト枠の余白
)

func pt(v float64) int { return int(math.Round(v * labelDPI / 72)) }

type procTable struct {
	xPt, yPt float64
	colsPt   []float64 // 列の境界（表の左端からの位置）
	rowsPt   []float64 // 行の境界（表の上端からの位置）
}

type procCell struct {
	text       string
	alignRight bool
	maxPt      float64 // largest size; 0 means procFontSize (8pt)
}

func isProcessedLandscape(data LabelData) bool {
	return data.Template == "processed"
}

func (r *LabelRenderer) renderProcessed(data LabelData) (RenderResult, error) {
	img := image.NewRGBA(image.Rect(0, 0, pt(procWidthPt), pt(procHeightPt)))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	trim := strings.TrimSpace
	loc := data.Locale

	// 区分（加熱食肉製品）: 表の上の 1 行。
	if v := trim(data.IsHeatedMeatProducts); v != "" && v != "false" {
		r.drawInFrame(img, v, pt(8.4), pt(8.4), pt(160), pt(9), false, false, 0)
	}

	main := procTable{xPt: 8.4, yPt: 18.4,
		colsPt: []float64{0, 33.6, 161.3},
		rowsPt: []float64{0, 19.5, 75.9, 93, 110.2, 127.3}}
	r.drawProcTable(img, main, [][2]procCell{
		{{text: localizedCaption(loc, "名称", "Name")}, {text: trim(data.ProductName)}},
		{{text: localizedCaption(loc, "原材料名", "Ingredients")}, {text: trim(data.ProductIngredient)}},
		{{text: localizedCaption(loc, "内容量", "Net Weight")}, {text: trim(data.ProductQuantity), alignRight: true}},
		{{text: deadlineCaption(data, "賞味期限", "Best Before")}, {text: trim(data.DeadlineDate)}},
		{{text: localizedCaption(loc, "保存方法", "Storage")}, {text: trim(data.StorageTemperature)}},
	})

	// 注意事項（表の下）。プラマークはその右端、表の右の罫線に揃える。
	attentionW := pt(168)
	if data.PlaMark {
		w, h := r.plaBadgeSize()
		right := pt(main.xPt + main.colsPt[len(main.colsPt)-1])
		r.drawPlaBadge(img, right-w, pt(148.4)+(pt(20)-h)/2)
		attentionW = right - w - pt(4) - pt(10.4)
	}
	if v := trim(data.AttentionText); v != "" {
		r.drawInFrame(img, v, pt(10.4), pt(148.4), attentionW, pt(20), false, false, 0)
	}

	// 見出しは表の左の罫線に揃える（区分の行と同じ）。
	r.drawInFrame(img, nutritionTitle(data), pt(174.4), pt(4.9), pt(138), pt(13.5), false, true, 0)
	nutrition := procTable{xPt: 174.4, yPt: 18.4,
		colsPt: []float64{0, 62.7, 135.3},
		rowsPt: []float64{0, 16.1, 31.6, 47.3, 63.2, 79.3}}
	r.drawProcTable(img, nutrition, [][2]procCell{
		{{text: localizedCaption(loc, "熱量", "Energy")}, {text: trim(data.CaloriesQuantity)}},
		{{text: localizedCaption(loc, "たんぱく質", "Protein")}, {text: trim(data.ProteinQuantity)}},
		{{text: localizedCaption(loc, "脂質", "Fat")}, {text: trim(data.FatQuantity)}},
		{{text: localizedCaption(loc, "炭水化物", "Carbohydrate")}, {text: trim(data.CarbohydratesQuantity)}},
		{{text: localizedCaption(loc, "食塩相当量", "Salt")}, {text: trim(data.SaltEquivalentQuantity)}},
	})

	company := trim(data.CompanyBlock)
	if company == "" {
		company = trim(data.ProcessorName)
	}
	parties := procTable{xPt: 174.4, yPt: 100.4,
		colsPt: []float64{0, 29.8, 137.3},
		rowsPt: []float64{0, 34.8, 67.3}}
	r.drawProcTable(img, parties, [][2]procCell{
		{{text: localizedCaption(loc, "販売者", "Seller")}, {text: company}},
		{{text: localizedCaption(loc, "製造所", "Plant")}, {text: trim(data.FacilityBlock)}},
	})

	return saveLandscape(img)
}

// saveLandscape turns a label laid out landscape (as the lbx templates are)
// by 90° and writes it for the 62mm tape, as the EN traceable label is sent.
func saveLandscape(img *image.RGBA) (RenderResult, error) {
	rotated := rotate90(img)
	tmpFile, err := os.CreateTemp("", "label-*.png")
	if err != nil {
		return RenderResult{}, err
	}
	defer tmpFile.Close()
	if err := png.Encode(tmpFile, rotated); err != nil {
		os.Remove(tmpFile.Name())
		return RenderResult{}, err
	}
	b := rotated.Bounds()
	return RenderResult{
		Path:     tmpFile.Name(),
		WidthMM:  int(math.Round(float64(b.Dx()) * 25.4 / labelDPI)),
		HeightMM: int(math.Round(float64(b.Dy()) * 25.4 / labelDPI)),
	}, nil
}

// nutritionTitle follows lite: 「栄養成分表示（100gあたり）」. lite sends it
// already formatted; a bare unit ("100g" / "100gあたり") is completed here.
func nutritionTitle(data LabelData) string {
	nu := strings.TrimSpace(data.NutritionUnit)
	if strings.EqualFold(strings.TrimSpace(data.Locale), "en") {
		if nu == "" {
			return "Nutrition Facts"
		}
		return "Nutrition Facts (" + nu + ")"
	}
	switch {
	case nu == "":
		return "栄養成分表示"
	case strings.Contains(nu, "栄養成分表示"):
		return nu
	default:
		return "栄養成分表示（" + strings.TrimSuffix(nu, "あたり") + "あたり）"
	}
}

func (r *LabelRenderer) drawProcTable(img *image.RGBA, t procTable, rows [][2]procCell) {
	x0, y0 := pt(t.xPt), pt(t.yPt)
	xs := make([]int, len(t.colsPt))
	for i, c := range t.colsPt {
		xs[i] = x0 + pt(c)
	}
	ys := make([]int, len(t.rowsPt))
	for i, c := range t.rowsPt {
		ys[i] = y0 + pt(c)
	}
	inset := pt(procCellInsetPt)
	// Column 0 holds the captions, which stay on one line.
	for i, row := range rows {
		if i+1 >= len(ys) {
			break
		}
		for j, cell := range row {
			if j+1 >= len(xs) {
				break
			}
			r.drawInFrame(img, cell.text,
				xs[j]+inset, ys[i]+inset,
				xs[j+1]-xs[j]-2*inset, ys[i+1]-ys[i]-2*inset, cell.alignRight, j == 0, cell.maxPt)
		}
	}
	drawGrid(img, xs, ys, ruleDots)
}

// drawGrid draws a table's rules thick dots wide. The outer frame is drawn
// inside the table's bounds, as P-touch's INSIDEFRAME pen does.
func drawGrid(img *image.RGBA, xs, ys []int, thick int) {
	if len(xs) < 2 || len(ys) < 2 {
		return
	}
	left, right := xs[0], xs[len(xs)-1]
	top, bottom := ys[0], ys[len(ys)-1]
	offset := func(i, n int) int {
		switch i {
		case 0:
			return 0
		case n - 1:
			return -(thick - 1)
		default:
			return -thick / 2
		}
	}
	for i, y := range ys {
		for d := 0; d < thick; d++ {
			drawHLine(img, left, right, y+offset(i, len(ys))+d, color.Black)
		}
	}
	for i, x := range xs {
		for d := 0; d < thick; d++ {
			drawVLine(img, x+offset(i, len(xs))+d, top, bottom, color.Black)
		}
	}
}

// drawInFrame prints text inside a w×h frame the way the lbx's shrink-to-fit
// text frames do, in the largest size (8pt at most) at which it fits, and
// centred vertically. Captions stay on one line and shrink (原材料名 goes to
// 7.5pt, as the lbx's 期限 and 保存方法 do); values also may wrap at the
// frame width, whichever prints larger.
func (r *LabelRenderer) drawInFrame(img *image.RGBA, text string, x, y, w, h int, alignRight, caption bool, maxPt float64) {
	text = strings.TrimSpace(text)
	if text == "" || w <= 0 || h <= 0 {
		return
	}
	paragraphs := strings.Split(text, "\n")
	for i, p := range paragraphs {
		paragraphs[i] = strings.TrimRight(p, " \t\r")
	}
	if maxPt <= 0 {
		maxPt = procFontSize
	}
	size, lines := r.fitUnwrapped(paragraphs, w, h, maxPt)
	if !caption || size == 0 {
		ws, wl := r.fitWrapped(paragraphs, w, h, maxPt)
		// In a cell one or two lines tall (a name, an address) a slightly
		// smaller single line reads better than a name broken in two.
		short := h < 3*lineHeightWithRatio(maxPt, procLineGap)
		keep := size > 0 && short && size >= 0.85*ws
		if ws > size && !keep {
			size, lines = ws, wl
		}
	}
	face := r.makeFace(size)
	defer face.Close()
	lh := lineHeightWithRatio(size, procLineGap)
	top := y + (h-lh*len(lines))/2
	for i, line := range lines {
		lx := x
		if alignRight {
			lx = x + w - font.MeasureString(face, line).Ceil()
		}
		drawString(img, face, line, lx, baselineInSlot(face, top+i*lh, lh))
	}
}

// fitUnwrapped returns the largest size, down to procNoWrapMin, at which
// every paragraph fits on one line; 0 if none does.
func (r *LabelRenderer) fitUnwrapped(paragraphs []string, w, h int, maxPt float64) (float64, []string) {
	for size := maxPt; size >= procNoWrapMin; size -= 0.25 {
		face := r.makeFace(size)
		fits := lineHeightWithRatio(size, procLineGap)*len(paragraphs) <= h
		for _, p := range paragraphs {
			if fits && font.MeasureString(face, p).Ceil() > w {
				fits = false
			}
		}
		face.Close()
		if fits {
			return size, paragraphs
		}
	}
	return 0, nil
}

// fitWrapped returns the largest size at which the wrapped text fits. Below
// the legal minimum only when it would not fit at all, and never cut.
func (r *LabelRenderer) fitWrapped(paragraphs []string, w, h int, maxPt float64) (float64, []string) {
	for size := maxPt; ; size -= 0.25 {
		face := r.makeFace(size)
		var lines []string
		for _, p := range paragraphs {
			lines = append(lines, wrapJapanese(face, p, w)...)
		}
		face.Close()
		if lineHeightWithRatio(size, procLineGap)*len(lines) <= h || size <= procFontFloor {
			return size, lines
		}
	}
}

// Characters that must not start a line (、。）etc.) and must not end one
// (（「 etc.) — the usual Japanese line-breaking rules.
const (
	noLineStart = "、。，．,.)）」』】〕〉》・：；:;！？!?ー～ぁぃぅぇぉっゃゅょァィゥェォッャュョ％%"
	noLineEnd   = "(（「『【〔〈《"
)

// wrapJapanese wraps text to width. Runs of letters and digits ("23-203",
// "TEL:", "1.8g") are kept together, and punctuation stays with the
// character it belongs to.
func wrapJapanese(face font.Face, text string, width int) []string {
	if text == "" {
		return []string{""}
	}
	var tokens []string
	isWord := func(r rune) bool { return r < 0x80 && r != ' ' }
	for _, r := range text {
		n := len(tokens)
		switch {
		case n > 0 && isWord(r) && isWord(lastRune(tokens[n-1])) && !strings.ContainsRune(noLineEnd, lastRune(tokens[n-1])):
			tokens[n-1] += string(r)
		case n > 0 && strings.ContainsRune(noLineStart, r):
			tokens[n-1] += string(r)
		case n > 0 && strings.ContainsRune(noLineEnd, lastRune(tokens[n-1])):
			tokens[n-1] += string(r)
		default:
			tokens = append(tokens, string(r))
		}
	}

	var lines []string
	line := ""
	for _, tok := range tokens {
		if line != "" && font.MeasureString(face, line+tok).Ceil() > width {
			lines = append(lines, strings.TrimRight(line, " "))
			line = strings.TrimLeft(tok, " ")
		} else {
			line += tok
		}
		// A token wider than the frame is split where it has to be.
		for font.MeasureString(face, line).Ceil() > width {
			parts := wrapTextWithFace(face, line, width)
			lines = append(lines, parts[:len(parts)-1]...)
			line = parts[len(parts)-1]
			if len(parts) == 1 {
				break
			}
		}
	}
	return append(lines, strings.TrimRight(line, " "))
}

func lastRune(s string) rune {
	r := []rune(s)
	if len(r) == 0 {
		return 0
	}
	return r[len(r)-1]
}
