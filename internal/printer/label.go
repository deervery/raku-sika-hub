package printer

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	qrcode "github.com/skip2/go-qrcode"
)

// Layout constants.
const (
	labelWidthPx     = 732 // 62mm at 300 DPI — fixed (tape width for CUPS)
	labelDPI         = 300
	marginPx         = 16
	tableCellPadding = 6
	tableBorderGray  = 120
)

// Font size and spacing.
const (
	lineSpacingRatio = 1.45
	dpiScale         = lineSpacingRatio * float64(labelDPI) / 72.0 // ≈ 6.04
)

const (
	fixedPetStorageMethod          = "直射日光と高温多湿を避けて保管してください。"
	fixedTraceableStorageMethod    = "-4℃以下で保存"
	fixedProcessorName             = "(株)札幌カネシン水産"
	fixedProcessorFacilityLocation = "北海道訓子府町大町113"
	fixedMetalDetectorStatus       = "検査済み"
	fixedHeatedInstruction         = "加熱用"
)

// LabelRenderer generates label images for printing.
type LabelRenderer struct {
	fontRegular *opentype.Font
}

// NewLabelRenderer creates a renderer by loading a font.
func NewLabelRenderer(fontPath string) (*LabelRenderer, error) {
	f, err := loadFont(fontPath)
	if err != nil {
		return nil, err
	}
	return &LabelRenderer{fontRegular: f}, nil
}

// Render generates a label PNG image and returns the temporary file path.
// Width is fixed at 732px (62mm) to match the Brother QL tape width.
func (r *LabelRenderer) Render(data LabelData) (string, error) {
	tmpFile, err := os.CreateTemp("", "label-*.png")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	img, err := r.renderImage(data)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}

	if err := png.Encode(tmpFile, img); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("encode png: %w", err)
	}
	return tmpFile.Name(), nil
}

// EncodePNG writes the label as PNG for preview.
func (r *LabelRenderer) EncodePNG(w io.Writer, data LabelData) error {
	img, err := r.renderImage(data)
	if err != nil {
		return err
	}
	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}
	return nil
}

// ── Layout computation ──

// labelLayout holds the computed dimensions for rendering.
type labelLayout struct {
	labelWidthPx int // total image width
	contentWidth int // labelWidthPx - 2*marginPx
	labelColPx   int // left column width (including padding)
	valueColPx   int // right column width (including padding)
}

// measureString returns the pixel width of text rendered with the given face.
func measureString(face font.Face, text string) int {
	return font.MeasureString(face, text).Ceil()
}

// computeLayout measures all table rows and determines the minimum label width
// so that no text wraps within either column.
func (r *LabelRenderer) computeLayout(rows []row) labelLayout {
	// Determine font size from the first tableRow (all rows share the same size).
	fontSize := 8.0
	for _, row := range rows {
		if v, ok := row.(tableRow); ok && v.fontSize > 0 {
			fontSize = v.fontSize
			break
		}
	}

	face := r.makeFace(fontSize)
	defer face.Close()

	var maxLabelPx, maxValuePx int
	var maxSepPx int

	for _, row := range rows {
		switch v := row.(type) {
		case tableRow:
			lw := measureString(face, v.label)
			vw := measureString(face, v.value)
			if lw > maxLabelPx {
				maxLabelPx = lw
			}
			if vw > maxValuePx {
				maxValuePx = vw
			}
		case qrTableRow:
			lw := measureString(face, v.label)
			if lw > maxLabelPx {
				maxLabelPx = lw
			}
			qrW := v.size + tableCellPadding*2
			if qrW > maxValuePx {
				maxValuePx = qrW
			}
		case separatorRow:
			sw := measureString(face, v.text)
			if sw > maxSepPx {
				maxSepPx = sw
			}
		}
	}

	// Add cell padding to each column.
	labelColPx := maxLabelPx + tableCellPadding*2
	valueColPx := maxValuePx + tableCellPadding*2

	// Content width = label col + value col.
	contentWidth := labelColPx + valueColPx

	// Ensure separator text fits within content area.
	sepNeeded := maxSepPx + 24
	if sepNeeded > contentWidth {
		valueColPx += sepNeeded - contentWidth
		contentWidth = labelColPx + valueColPx
	}

	labelWidthPx := contentWidth + 2*marginPx

	return labelLayout{
		labelWidthPx: labelWidthPx,
		contentWidth: contentWidth,
		labelColPx:   labelColPx,
		valueColPx:   valueColPx,
	}
}

// applyFontSize sets the font size on all scalable rows.
func applyFontSize(rows []row, size float64) {
	for i, row := range rows {
		switch v := row.(type) {
		case tableRow:
			v.fontSize = size
			rows[i] = v
		case textRow:
			v.fontSize = size
			rows[i] = v
		case multiLineRow:
			v.fontSize = size
			rows[i] = v
		case separatorRow:
			v.fontSize = size
			rows[i] = v
		case qrTableRow:
			v.fontSize = size
			rows[i] = v
		}
	}
}

// computeOptimalFontSize finds the largest font size (in 0.5pt steps)
// where the table content fits within the 62mm (732px) label width.
func (r *LabelRenderer) computeOptimalFontSize(rows []row) float64 {
	best := 6.0
	for size := 10.0; size >= 6.0; size -= 0.5 {
		applyFontSize(rows, size)
		layout := r.computeLayout(rows)
		if layout.labelWidthPx <= labelWidthPx {
			best = size
			break
		}
	}
	return best
}

func (r *LabelRenderer) renderImage(data LabelData) (*image.RGBA, error) {
	rows := r.buildRows(data)

	// Auto-fit: largest font size that fills 62mm width without overflow.
	fontSize := r.computeOptimalFontSize(rows)
	applyFontSize(rows, fontSize)

	layout := r.computeLayout(rows)

	// Expand value column to fill exactly 732px width.
	if layout.labelWidthPx < labelWidthPx {
		extra := labelWidthPx - layout.labelWidthPx
		layout.valueColPx += extra
		layout.contentWidth += extra
		layout.labelWidthPx = labelWidthPx
	}

	// Height is variable — grows with content.
	height := marginPx
	for _, row := range rows {
		height += row.height(r, layout)
	}
	height += marginPx

	img := image.NewRGBA(image.Rect(0, 0, layout.labelWidthPx, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	y := marginPx
	for _, row := range rows {
		y = row.draw(img, r, y, layout)
	}
	return img, nil
}

// row is a renderable label element.
type row interface {
	height(r *LabelRenderer, l labelLayout) int
	draw(img *image.RGBA, r *LabelRenderer, y int, l labelLayout) int
}

// ── textRow ──

type textRow struct {
	label    string
	value    string
	fontSize float64
}

func (t textRow) lineHeight() int {
	return int(t.fontSize * lineSpacingRatio * float64(labelDPI) / 72)
}

func (t textRow) height(_ *LabelRenderer, _ labelLayout) int {
	return t.lineHeight()
}

func (t textRow) draw(img *image.RGBA, r *LabelRenderer, y int, l labelLayout) int {
	face := r.makeFace(t.fontSize)
	defer face.Close()

	text := t.value
	if t.label != "" {
		text = t.label + ": " + t.value
	}

	baseline := y + int(t.fontSize*float64(labelDPI)/72)
	drawString(img, face, text, marginPx, baseline)
	return y + t.lineHeight()
}

// ── multiLineRow ──

type multiLineRow struct {
	label    string
	value    string
	fontSize float64
}

func (m multiLineRow) lineHeight() int {
	return int(m.fontSize * lineSpacingRatio * float64(labelDPI) / 72)
}

func (m multiLineRow) height(r *LabelRenderer, l labelLayout) int {
	face := r.makeFace(m.fontSize)
	defer face.Close()
	lines := wrapTextWithFace(face, m.value, l.contentWidth)
	count := len(lines)
	if m.label != "" {
		count++
	}
	return m.lineHeight() * count
}

func (m multiLineRow) draw(img *image.RGBA, r *LabelRenderer, y int, l labelLayout) int {
	face := r.makeFace(m.fontSize)
	defer face.Close()

	lh := m.lineHeight()
	baseline := y + int(m.fontSize*float64(labelDPI)/72)

	if m.label != "" {
		drawString(img, face, m.label+":", marginPx, baseline)
		baseline += lh
	}

	indent := marginPx + 20
	for _, line := range wrapTextWithFace(face, m.value, l.contentWidth) {
		drawString(img, face, line, indent, baseline)
		baseline += lh
	}
	return y + m.height(r, l)
}

// ── separatorRow ──

type separatorRow struct {
	text     string
	fontSize float64
}

func (s separatorRow) height(_ *LabelRenderer, _ labelLayout) int {
	if s.text != "" && s.fontSize > 0 {
		return int(s.fontSize * dpiScale)
	}
	return 10
}

func (s separatorRow) draw(img *image.RGBA, r *LabelRenderer, y int, l labelLayout) int {
	h := s.height(r, l)
	lineY := y + h/2

	gray := color.RGBA{R: 180, G: 180, B: 180, A: 255}
	for x := marginPx; x < l.labelWidthPx-marginPx; x++ {
		img.Set(x, lineY, gray)
	}

	if s.text != "" && s.fontSize > 0 {
		face := r.makeFace(s.fontSize)
		defer face.Close()
		adv := font.MeasureString(face, s.text)
		textX := (l.labelWidthPx - adv.Round()) / 2
		baseline := y + int(s.fontSize*float64(labelDPI)/72)
		clearW := adv.Round() + 12
		clearRect := image.Rect(textX-6, y, textX+clearW-6, y+h)
		draw.Draw(img, clearRect, &image.Uniform{color.White}, image.Point{}, draw.Src)
		drawString(img, face, s.text, textX, baseline)
	}

	return y + h
}

// ── qrRow ──

type qrRow struct {
	url        string
	size       int
	alignRight bool
}

func (q qrRow) height(_ *LabelRenderer, _ labelLayout) int {
	return q.size + 10
}

func (q qrRow) draw(img *image.RGBA, r *LabelRenderer, y int, l labelLayout) int {
	if q.url == "" {
		return y
	}

	qrPng, err := qrcode.Encode(q.url, qrcode.Medium, q.size)
	if err != nil {
		return y + q.height(r, l)
	}

	qrImg, err := png.Decode(bytes.NewReader(qrPng))
	if err != nil {
		return y + q.height(r, l)
	}

	x := (l.labelWidthPx - q.size) / 2
	if q.alignRight {
		x = l.labelWidthPx - marginPx - q.size
	}
	offset := image.Pt(x, y+5)
	draw.Draw(img, image.Rect(offset.X, offset.Y, offset.X+q.size, offset.Y+q.size),
		qrImg, image.Point{}, draw.Over)

	return y + q.height(r, l)
}

// ── spacerRow ──

type spacerRow struct {
	px int
}

func (s spacerRow) height(_ *LabelRenderer, _ labelLayout) int { return s.px }
func (s spacerRow) draw(_ *image.RGBA, _ *LabelRenderer, y int, _ labelLayout) int {
	return y + s.px
}

// ── tableRow ──

type tableRow struct {
	label    string
	value    string
	fontSize float64
}

func (t tableRow) lineHeight() int {
	return int(t.fontSize * lineSpacingRatio * float64(labelDPI) / 72)
}

func (t tableRow) height(_ *LabelRenderer, _ labelLayout) int {
	// With dynamic layout, all text fits on one line. Height = 1 line + padding.
	return t.lineHeight() + tableCellPadding*2
}

func (t tableRow) draw(img *image.RGBA, r *LabelRenderer, y int, l labelLayout) int {
	face := r.makeFace(t.fontSize)
	defer face.Close()

	rowHeight := t.height(r, l)
	left := marginPx
	right := l.labelWidthPx - marginPx
	splitX := left + l.labelColPx
	bottom := y + rowHeight

	border := color.RGBA{R: tableBorderGray, G: tableBorderGray, B: tableBorderGray, A: 255}
	for x := left; x < right; x++ {
		img.Set(x, y, border)
		img.Set(x, bottom-1, border)
	}
	for py := y; py < bottom; py++ {
		img.Set(left, py, border)
		img.Set(right-1, py, border)
		img.Set(splitX, py, border)
	}

	baseline := y + tableCellPadding + int(t.fontSize*float64(labelDPI)/72)
	drawString(img, face, t.label, left+tableCellPadding, baseline)
	drawString(img, face, t.value, splitX+tableCellPadding, baseline)

	return bottom
}

// ── qrTableRow ──

type qrTableRow struct {
	label    string
	url      string
	size     int
	fontSize float64
}

func (q qrTableRow) height(_ *LabelRenderer, _ labelLayout) int {
	return q.size + tableCellPadding*2
}

func (q qrTableRow) draw(img *image.RGBA, r *LabelRenderer, y int, l labelLayout) int {
	rowHeight := q.height(r, l)
	left := marginPx
	right := l.labelWidthPx - marginPx
	splitX := left + l.labelColPx
	bottom := y + rowHeight

	border := color.RGBA{R: tableBorderGray, G: tableBorderGray, B: tableBorderGray, A: 255}
	for x := left; x < right; x++ {
		img.Set(x, y, border)
		img.Set(x, bottom-1, border)
	}
	for py := y; py < bottom; py++ {
		img.Set(left, py, border)
		img.Set(right-1, py, border)
		img.Set(splitX, py, border)
	}

	// Draw label text vertically centered in the QR cell.
	face := r.makeFace(q.fontSize)
	defer face.Close()
	baseline := y + rowHeight/2 + int(q.fontSize*float64(labelDPI)/72)/2
	drawString(img, face, q.label, left+tableCellPadding, baseline)

	if q.url == "" {
		return bottom
	}

	qrPng, err := qrcode.Encode(q.url, qrcode.Medium, q.size)
	if err != nil {
		return bottom
	}
	qrImg, err := png.Decode(bytes.NewReader(qrPng))
	if err != nil {
		return bottom
	}

	x := right - tableCellPadding - q.size
	offsetY := y + (rowHeight-q.size)/2
	draw.Draw(img, image.Rect(x, offsetY, x+q.size, offsetY+q.size), qrImg, image.Point{}, draw.Over)
	return bottom
}

// ── Text wrapping (only needed for multiLineRow) ──

func wrapTextWithFace(face font.Face, text string, widthPx int) []string {
	if text == "" {
		return []string{""}
	}
	maxWidth := fixed.I(widthPx)

	var lines []string
	runes := []rune(text)
	start := 0
	for i := 1; i <= len(runes); i++ {
		segment := string(runes[start:i])
		w := font.MeasureString(face, segment)
		if w > maxWidth && i-1 > start {
			lines = append(lines, string(runes[start:i-1]))
			start = i - 1
		}
	}
	if start < len(runes) {
		lines = append(lines, string(runes[start:]))
	}
	return lines
}

// ── Storage method resolution ──

func resolveStorageMethod(data LabelData, fallback string) string {
	method := data.StorageMethod
	if method == "" {
		method = data.StorageTemperature
	}
	if method == "" {
		method = fallback
	}
	return NormalizeStorageTemperature(method)
}

// ── Template builders ──

func (r *LabelRenderer) buildRows(data LabelData) []row {
	// fontSize is set to 0 as a placeholder; computeOptimalFontSize + applyFontSize
	// will determine and apply the actual size to fill the 62mm height.
	switch data.Template {
	case "pet":
		return []row{
			tableRow{label: "商品名", value: data.ProductName},
			tableRow{label: "内容量", value: data.ProductQuantity},
			tableRow{label: "消費期限", value: data.DeadlineDate},
			tableRow{label: "保存方法", value: fixedPetStorageMethod},
			tableRow{label: "加工者名", value: fixedProcessorName},
			tableRow{label: "加工施設所在地", value: fixedProcessorFacilityLocation},
			tableRow{label: "金属探知機", value: fixedMetalDetectorStatus},
		}
	case "traceable_deer", "traceable_bear":
		individualID := data.IndividualID
		if individualID == "" {
			individualID = data.IndividualNumber
		}
		storageMethod := resolveStorageMethod(data, fixedTraceableStorageMethod)
		return []row{
			tableRow{label: "商品名", value: data.ProductName},
			tableRow{label: "捕獲地", value: data.CaptureLocation},
			tableRow{label: "内容量", value: data.ProductQuantity},
			tableRow{label: "消費期限", value: data.DeadlineDate},
			tableRow{label: "保存方法", value: storageMethod},
			tableRow{label: "加工者名", value: fixedProcessorName},
			tableRow{label: "加工施設所在地", value: fixedProcessorFacilityLocation},
			tableRow{label: "金属探知機", value: fixedMetalDetectorStatus},
			tableRow{label: "加熱用である旨", value: fixedHeatedInstruction},
			tableRow{label: "個体識別番号", value: individualID},
			qrTableRow{label: "QR", url: data.QRCode, size: 132},
		}
	case "non_traceable_deer":
		storageMethod := resolveStorageMethod(data, fixedTraceableStorageMethod)
		rows := []row{
			tableRow{label: "商品名", value: data.ProductName},
			tableRow{label: "内容量", value: data.ProductQuantity},
			tableRow{label: "消費期限", value: data.DeadlineDate},
			tableRow{label: "保存方法", value: storageMethod},
			tableRow{label: "加工者名", value: fixedProcessorName},
			tableRow{label: "加工施設所在地", value: fixedProcessorFacilityLocation},
			tableRow{label: "金属探知機", value: fixedMetalDetectorStatus},
		}
		if data.AttentionText != "" {
			rows = append(rows, tableRow{label: "注意事項", value: data.AttentionText})
		}
		return rows
	case "processed":
		storageMethod := resolveStorageMethod(data, fixedTraceableStorageMethod)
		rows := []row{
			tableRow{label: "商品名", value: data.ProductName},
			tableRow{label: "内容量", value: data.ProductQuantity},
			tableRow{label: "消費期限", value: data.DeadlineDate},
			tableRow{label: "保存方法", value: storageMethod},
		}
		if data.ProductIngredient != "" {
			rows = append(rows, tableRow{label: "原材料名", value: data.ProductIngredient})
		}
		if data.CaloriesQuantity != "" || data.ProteinQuantity != "" {
			unit := data.NutritionUnit
			if unit == "" {
				unit = "100gあたり"
			}
			rows = append(rows, separatorRow{text: "栄養成分表示（" + unit + "）"})
			if data.CaloriesQuantity != "" {
				rows = append(rows, tableRow{label: "熱量", value: data.CaloriesQuantity})
			}
			if data.ProteinQuantity != "" {
				rows = append(rows, tableRow{label: "たんぱく質", value: data.ProteinQuantity})
			}
			if data.FatQuantity != "" {
				rows = append(rows, tableRow{label: "脂質", value: data.FatQuantity})
			}
			if data.CarbohydratesQuantity != "" {
				rows = append(rows, tableRow{label: "炭水化物", value: data.CarbohydratesQuantity})
			}
			if data.SaltEquivalentQuantity != "" {
				rows = append(rows, tableRow{label: "食塩相当量", value: data.SaltEquivalentQuantity})
			}
		}
		rows = append(rows,
			tableRow{label: "加工者名", value: fixedProcessorName},
			tableRow{label: "加工施設所在地", value: fixedProcessorFacilityLocation},
			tableRow{label: "金属探知機", value: fixedMetalDetectorStatus},
		)
		if data.IsHeatedMeatProducts != "" {
			rows = append(rows, tableRow{label: "加熱用である旨", value: data.IsHeatedMeatProducts})
		}
		if data.AttentionText != "" {
			rows = append(rows, tableRow{label: "注意事項", value: data.AttentionText})
		}
		return rows
	default:
		return []row{
			tableRow{label: "商品名", value: data.ProductName},
			tableRow{label: "内容量", value: data.ProductQuantity},
			tableRow{label: "消費期限", value: data.DeadlineDate},
		}
	}
}

// ── Font helpers ──

func (r *LabelRenderer) makeFace(sizePt float64) font.Face {
	face, err := opentype.NewFace(r.fontRegular, &opentype.FaceOptions{
		Size:    sizePt,
		DPI:     labelDPI,
		Hinting: font.HintingFull,
	})
	if err != nil {
		panic(fmt.Sprintf("create font face: %v", err))
	}
	return face
}

func drawString(img *image.RGBA, face font.Face, text string, x, y int) {
	d := &font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{color.Black},
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}

func loadFont(configPath string) (*opentype.Font, error) {
	paths := []string{}
	if configPath != "" {
		paths = append(paths, configPath)
	}
	paths = append(paths,
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/fonts-japanese-gothic.ttf",
		"/usr/share/fonts/truetype/vlgothic/VL-Gothic-Regular.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	)

	for _, p := range paths {
		f, err := tryLoadFont(p)
		if err == nil {
			return f, nil
		}
	}

	return nil, fmt.Errorf("FONT_NOT_FOUND: 日本語フォントが見つかりません。" +
		"sudo apt-get install fonts-noto-cjk を実行するか、config.json に fontPath を設定してください")
}

func tryLoadFont(path string) (*opentype.Font, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".ttc" {
		col, err := opentype.ParseCollection(data)
		if err != nil {
			return nil, err
		}
		f, err := col.Font(0)
		return f, err
	}

	return opentype.Parse(data)
}
