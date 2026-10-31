package printer

import (
	"image"
	"image/color"
	"image/draw"
	"strings"

	"golang.org/x/image/font"
)

// ペットラベル: 縦向きの表（行を足せばラベルが伸びる）。
//
//	┌──────────┬─────────────────────┐
//	│ 商品名   │                     │
//	│ 内容量   │                     │
//	│ 消費期限 │                     │
//	│ 保存方法 │                     │
//	│ 加工者   │ 会社名・住所・TEL   │
//	│ 加工所   │ 施設名・住所        │
//	│ 金属探知機│ 検査済み           │
//	└──────────┴─────────────────────┘
//	                  [プラ 外装] [ロゴ]
//
// 文字はすべて実寸 8pt。縮めずに枠の幅で折り返す。ipp-usb で印刷する施設は
// CUPS がラベル全体を縮めて印刷するので（LabelData.ShrunkToFit）、その分だけ
// 大きく描いて、印刷された文字が 8pt になるようにする。
const (
	petFontPt     = 8.0
	petLogoWPt    = 39.0 // ソバージュのペット lbx のロゴ枠
	petLogoHPt    = 19.5
	petFooterGap  = 3.0
	petCellPadPx  = tableCellPadding + 1
	petLineGapPct = lineSpacingRatio
)

const ippModelMargin = 1.005

func isPetLabel(data LabelData) bool { return data.Template == "pet" }

func (r *LabelRenderer) renderPet(data LabelData) (RenderResult, error) {
	return r.renderRows(r.petRows(data, r.petFont(data)))
}

// petFont is the size to draw the pet label's text at so that it prints at
// petFontPt. On an ipp-usb queue the shrink ratio depends on the label's
// length, which grows with the text, and jumps as lines wrap and the media
// length rounds to whole mm, so the size is found by stepping up from the
// least it can be (the width-limited ratio) to the first that prints at
// petFontPt or more. ippFitScale matched office's prints to within 0.3%;
// aiming 0.5% high keeps the printed text from falling under 8pt.
func (r *LabelRenderer) petFont(data LabelData) float64 {
	if !data.ShrunkToFit {
		return petFontPt
	}
	const step = 0.02
	f := petFontPt / ippWidthScale
	for i := 0; i < 200; i++ {
		if f*ippFitScale(rowsHeight(r.petRows(data, f))) >= petFontPt*ippModelMargin {
			return f
		}
		f += step
	}
	return f
}

func rowsHeight(rows []row) int {
	h := 0
	for _, rw := range rows {
		h += rw.height()
	}
	return h
}

func (r *LabelRenderer) petRows(data LabelData, fontPt float64) []row {
	trim := strings.TrimSpace
	var cells [][2]string
	for _, e := range buildTableEntries(data) {
		cells = append(cells, [2]string{e.label, trim(e.value)})
	}
	rows := []row{petTableRow{r: r, cells: cells, fontPt: fontPt}}

	logo := strings.TrimSpace(data.LogoFile)
	if data.PlaMark || logo != "" {
		rows = append(rows, petFooterRow{r: r, pla: data.PlaMark, logo: logo})
	}
	return rows
}

// petTableRow is a two-column table whose text is all one size, wrapped at
// the column width (measured) and never shrunk.
type petTableRow struct {
	r      *LabelRenderer
	cells  [][2]string
	fontPt float64
}

type petTableLayout struct {
	xs    []int
	rows  [][2][]string
	hs    []int
	lh    int
	total int
}

func (t petTableRow) layout(face font.Face) petTableLayout {
	// The caption column is as wide as the longest caption needs, so that
	// 金属探知機 stays on one line at any size.
	labelW := int(float64(contentWidth) * tableLabelWidthPet)
	for _, c := range t.cells {
		for _, p := range strings.Split(c[0], "\n") {
			if w := font.MeasureString(face, p).Ceil() + 2*petCellPadPx + pt(1); w > labelW {
				labelW = w
			}
		}
	}
	l := petTableLayout{
		xs: []int{contentLeft, contentLeft + labelW, contentLeft + contentWidth},
		lh: lineHeightWithRatio(t.fontPt, petLineGapPct),
	}
	for _, c := range t.cells {
		var cell [2][]string
		n := 1
		for j := 0; j < 2; j++ {
			w := l.xs[j+1] - l.xs[j] - 2*petCellPadPx
			if strings.TrimSpace(c[j]) != "" {
				cell[j] = wrapParagraphs(face, c[j], w)
			}
			if len(cell[j]) > n {
				n = len(cell[j])
			}
		}
		h := n*l.lh + 2*petCellPadPx
		l.rows = append(l.rows, cell)
		l.hs = append(l.hs, h)
		l.total += h
	}
	return l
}

func (t petTableRow) height() int {
	face := t.r.makeFace(t.fontPt)
	defer face.Close()
	return t.layout(face).total
}

func (t petTableRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	face := r.makeFace(t.fontPt)
	defer face.Close()
	l := t.layout(face)
	ys := []int{y}
	top := y
	for i, cell := range l.rows {
		for j, lines := range cell {
			// Centred in the row, as the lbx text frames are.
			ty := top + (l.hs[i]-l.lh*len(lines))/2
			for k, line := range lines {
				drawString(img, face, line, l.xs[j]+petCellPadPx, baselineInSlot(face, ty+k*l.lh, l.lh))
			}
		}
		top += l.hs[i]
		ys = append(ys, top)
	}
	drawGrid(img, l.xs, ys, ruleDots)
	return y + l.total
}

// petFooterRow puts the プラ mark and the facility's logo at the right end,
// below the table (where the pet templates have them).
type petFooterRow struct {
	r    *LabelRenderer
	pla  bool
	logo string
}

func (f petFooterRow) logoImage() image.Image {
	if f.logo == "" {
		return nil
	}
	img, err := f.r.loadAssetImage(f.logo)
	if err != nil || img == nil || img.Bounds().Empty() {
		return nil
	}
	return img
}

func (f petFooterRow) height() int {
	h := 0
	if f.pla {
		_, h = f.r.plaBadgeSize()
	}
	if f.logoImage() != nil && pt(petLogoHPt) > h {
		h = pt(petLogoHPt)
	}
	if h == 0 {
		return 0
	}
	return h + pt(petFooterGap)
}

func (f petFooterRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	h := f.height()
	if h == 0 {
		return y
	}
	bottom := y + h
	right := contentLeft + contentWidth
	if logo := f.logoImage(); logo != nil {
		w := pt(petLogoWPt)
		rect := image.Rect(right-w, bottom-pt(petLogoHPt), right, bottom)
		// Right-aligned in its box, keeping the logo's proportions.
		b := logo.Bounds()
		if lw := b.Dx() * rect.Dy() / b.Dy(); lw < w {
			rect.Min.X = right - lw
		}
		r.drawImageWithinRectAligned(img, logo, rect, true)
		right = rect.Min.X - pt(petFooterGap)
	}
	if f.pla {
		bw, bh := r.plaBadgeSize()
		r.drawPlaBadge(img, right-bw, bottom-bh)
	}
	return bottom
}

// renderRows renders the given rows on a label 62mm wide.
func (r *LabelRenderer) renderRows(rows []row) (RenderResult, error) {
	height := rowsHeight(rows)
	img := image.NewRGBA(image.Rect(0, 0, labelWidthPx, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	y := 0
	for _, rw := range rows {
		y = rw.draw(img, r, y)
	}
	return saveLabelPNG(img)
}

// wrapParagraphs wraps each line of text to width, measuring the glyphs.
func wrapParagraphs(face font.Face, text string, width int) []string {
	var lines []string
	for _, p := range strings.Split(strings.TrimSpace(text), "\n") {
		lines = append(lines, wrapJapanese(face, strings.TrimRight(p, " \t\r"), width)...)
	}
	return lines
}
