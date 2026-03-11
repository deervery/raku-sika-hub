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

// Label dimensions for Brother QL-800/QL-820 (62mm continuous tape at 300 DPI).
const (
	labelWidthPx = 732 // 62mm at 300 DPI
	labelDPI     = 300
	marginPx     = 24
	contentWidth = labelWidthPx - 2*marginPx
)

// Font sizes at 300 DPI.
const (
	fontSizeTitle    = 13 // tuned to fit 62mm labels reliably
	fontSizeBody     = 10
	fontSizeSmall    = 9
	fontSizeSep      = 10 // separator text
	lineSpacingRatio = 1.45
)

const (
	fixedPetStorageMethod          = "直射日光と高温多湿を避けて保管してください。"
	fixedTraceableStorageMethod    = "-4℃以下で保存"
	fixedProcessorName             = "(株)札幌カネシン水産"
	fixedProcessorFacilityLocation = "北海道訓子府町大町113"
	fixedMetalDetectorStatus       = "検査済み"
	fixedHeatedInstruction         = "加熱用"
)

const (
	tableLabelWidthPx = 188
	tableCellPadding  = 8
	tableBorderGray   = 120
)

// LabelRenderer generates label images for printing.
type LabelRenderer struct {
	fontRegular *opentype.Font
}

// NewLabelRenderer creates a renderer by loading a font.
// fontPath can be empty; the renderer will search well-known system paths.
func NewLabelRenderer(fontPath string) (*LabelRenderer, error) {
	f, err := loadFont(fontPath)
	if err != nil {
		return nil, err
	}
	return &LabelRenderer{fontRegular: f}, nil
}

// Render generates a label PNG image and returns the temporary file path.
func (r *LabelRenderer) Render(data LabelData) (string, error) {
	tmpFile, err := os.CreateTemp("", "label-*.png")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	if err := r.EncodePNG(tmpFile, data); err != nil {
		os.Remove(tmpFile.Name())
		return "", err
	}
	return tmpFile.Name(), nil
}

func (r *LabelRenderer) EncodePNG(w io.Writer, data LabelData) error {
	return r.encodePNG(w, data)
}

func (r *LabelRenderer) encodePNG(w io.Writer, data LabelData) error {
	img, err := r.renderImage(data)
	if err != nil {
		return err
	}
	if err := png.Encode(w, img); err != nil {
		return fmt.Errorf("encode png: %w", err)
	}
	return nil
}

func (r *LabelRenderer) renderImage(data LabelData) (*image.RGBA, error) {
	rows := r.buildRows(data)

	height := marginPx
	for _, row := range rows {
		height += row.height()
	}
	height += marginPx

	img := image.NewRGBA(image.Rect(0, 0, labelWidthPx, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	y := marginPx
	for _, row := range rows {
		y = row.draw(img, r, y)
	}
	return img, nil
}

// row is a renderable label element.
type row interface {
	height() int
	draw(img *image.RGBA, r *LabelRenderer, y int) int
}

// textRow renders a single "label: value" line or just text.
type textRow struct {
	label    string
	value    string
	fontSize float64
}

func (t textRow) height() int {
	return int(t.fontSize * lineSpacingRatio * float64(labelDPI) / 72)
}

func (t textRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	face := r.makeFace(t.fontSize)
	defer face.Close()

	text := t.value
	if t.label != "" {
		text = t.label + ": " + t.value
	}

	h := t.height()
	baseline := y + int(t.fontSize*float64(labelDPI)/72)
	drawString(img, face, text, marginPx, baseline)
	return y + h
}

// multiLineRow renders text that may wrap across multiple lines.
type multiLineRow struct {
	label    string
	value    string
	fontSize float64
}

func (m multiLineRow) lineHeight() int {
	return int(m.fontSize * lineSpacingRatio * float64(labelDPI) / 72)
}

func (m multiLineRow) height() int {
	lines := m.wrapLines()
	if m.label != "" {
		return m.lineHeight() * (len(lines) + 1) // label line + value lines
	}
	return m.lineHeight() * len(lines)
}

func (m multiLineRow) wrapLines() []string {
	if m.value == "" {
		return nil
	}
	// Estimate chars per line based on font size and content width.
	// Japanese characters are roughly square at the font size.
	charWidth := m.fontSize * float64(labelDPI) / 72 * 0.55
	charsPerLine := int(float64(contentWidth) / charWidth)
	if charsPerLine < 1 {
		charsPerLine = 1
	}

	var lines []string
	runes := []rune(m.value)
	for len(runes) > 0 {
		end := charsPerLine
		if end > len(runes) {
			end = len(runes)
		}
		lines = append(lines, string(runes[:end]))
		runes = runes[end:]
	}
	return lines
}

func (m multiLineRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	face := r.makeFace(m.fontSize)
	defer face.Close()

	lh := m.lineHeight()
	baseline := y + int(m.fontSize*float64(labelDPI)/72)

	if m.label != "" {
		drawString(img, face, m.label+":", marginPx, baseline)
		baseline += lh
	}

	indent := marginPx + 20
	for _, line := range m.wrapLines() {
		drawString(img, face, line, indent, baseline)
		baseline += lh
	}
	return y + m.height()
}

// separatorRow draws a horizontal line with optional centered text.
type separatorRow struct {
	text string
}

func (s separatorRow) height() int {
	if s.text != "" {
		sz := float64(fontSizeSep)
		return int(sz * lineSpacingRatio * float64(labelDPI) / 72)
	}
	return 10
}

func (s separatorRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	h := s.height()
	lineY := y + h/2

	// Draw line.
	gray := color.RGBA{R: 180, G: 180, B: 180, A: 255}
	for x := marginPx; x < labelWidthPx-marginPx; x++ {
		img.Set(x, lineY, gray)
	}

	if s.text != "" {
		face := r.makeFace(fontSizeSep)
		defer face.Close()
		// Center text.
		adv := font.MeasureString(face, s.text)
		textX := (labelWidthPx - adv.Round()) / 2
		sz := float64(fontSizeSep)
		baseline := y + int(sz*float64(labelDPI)/72)
		// Clear background behind text.
		clearW := adv.Round() + 12
		clearRect := image.Rect(textX-6, y, textX+clearW-6, y+h)
		draw.Draw(img, clearRect, &image.Uniform{color.White}, image.Point{}, draw.Src)
		drawString(img, face, s.text, textX, baseline)
	}

	return y + h
}

// qrRow renders a QR code image.
type qrRow struct {
	url        string
	size       int // pixels
	alignRight bool
}

func (q qrRow) height() int {
	return q.size + 10 // small padding
}

func (q qrRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	if q.url == "" {
		return y
	}

	qrPng, err := qrcode.Encode(q.url, qrcode.Medium, q.size)
	if err != nil {
		return y + q.height()
	}

	// Decode QR PNG to image.
	qrImg, err := png.Decode(bytes.NewReader(qrPng))
	if err != nil {
		return y + q.height()
	}

	x := (labelWidthPx - q.size) / 2
	if q.alignRight {
		x = labelWidthPx - marginPx - q.size
	}
	offset := image.Pt(x, y+5)
	draw.Draw(img, image.Rect(offset.X, offset.Y, offset.X+q.size, offset.Y+q.size),
		qrImg, image.Point{}, draw.Over)

	return y + q.height()
}

// spacerRow adds vertical space.
type spacerRow struct {
	px int
}

func (s spacerRow) height() int { return s.px }
func (s spacerRow) draw(_ *image.RGBA, _ *LabelRenderer, y int) int {
	return y + s.px
}

// tableRow renders a two-column table row with borders.
type tableRow struct {
	label    string
	value    string
	fontSize float64
}

func (t tableRow) lineHeight() int {
	return int(t.fontSize * lineSpacingRatio * float64(labelDPI) / 72)
}

func (t tableRow) wrapLines(text string, width int) []string {
	if text == "" {
		return []string{""}
	}

	charWidth := t.fontSize * float64(labelDPI) / 72 * 0.55
	charsPerLine := int(float64(width) / charWidth)
	if charsPerLine < 1 {
		charsPerLine = 1
	}

	var lines []string
	runes := []rune(text)
	for len(runes) > 0 {
		end := charsPerLine
		if end > len(runes) {
			end = len(runes)
		}
		lines = append(lines, string(runes[:end]))
		runes = runes[end:]
	}
	return lines
}

func (t tableRow) height() int {
	labelWidth := tableLabelWidthPx - 2*tableCellPadding
	valueWidth := contentWidth - tableLabelWidthPx - 2*tableCellPadding

	labelLines := t.wrapLines(t.label, labelWidth)
	valueLines := t.wrapLines(t.value, valueWidth)
	lineCount := len(labelLines)
	if len(valueLines) > lineCount {
		lineCount = len(valueLines)
	}
	if lineCount < 1 {
		lineCount = 1
	}
	return lineCount*t.lineHeight() + tableCellPadding*2
}

func (t tableRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	face := r.makeFace(t.fontSize)
	defer face.Close()

	rowHeight := t.height()
	left := marginPx
	right := labelWidthPx - marginPx
	splitX := left + tableLabelWidthPx
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

	labelLines := t.wrapLines(t.label, tableLabelWidthPx-2*tableCellPadding)
	valueLines := t.wrapLines(t.value, contentWidth-tableLabelWidthPx-2*tableCellPadding)
	lineHeight := t.lineHeight()
	baseline := y + tableCellPadding + int(t.fontSize*float64(labelDPI)/72)
	for _, line := range labelLines {
		drawString(img, face, line, left+tableCellPadding, baseline)
		baseline += lineHeight
	}

	baseline = y + tableCellPadding + int(t.fontSize*float64(labelDPI)/72)
	for _, line := range valueLines {
		drawString(img, face, line, splitX+tableCellPadding, baseline)
		baseline += lineHeight
	}

	return bottom
}

// qrTableRow keeps the QR code inside a bordered table cell.
type qrTableRow struct {
	label string
	url   string
	size  int
}

func (q qrTableRow) height() int {
	size := q.size + tableCellPadding*2
	minHeight := textRow{value: "A", fontSize: fontSizeBody}.height() + tableCellPadding*2
	if size < minHeight {
		return minHeight
	}
	return size
}

func (q qrTableRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	row := tableRow{label: q.label, value: "", fontSize: fontSizeBody}
	rowHeight := q.height()
	left := marginPx
	right := labelWidthPx - marginPx
	splitX := left + tableLabelWidthPx
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

	face := r.makeFace(fontSizeBody)
	defer face.Close()
	baseline := y + tableCellPadding + int(row.fontSize*float64(labelDPI)/72)
	for _, line := range row.wrapLines(q.label, tableLabelWidthPx-2*tableCellPadding) {
		drawString(img, face, line, left+tableCellPadding, baseline)
		baseline += row.lineHeight()
	}

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

// buildRows creates the row list for the given template and data.
func (r *LabelRenderer) buildRows(data LabelData) []row {
	switch data.Template {
	case "pet":
		return []row{
			tableRow{label: "商品名", value: data.ProductName, fontSize: fontSizeTitle},
			tableRow{label: "内容量", value: data.ProductQuantity, fontSize: fontSizeBody},
			tableRow{label: "消費期限", value: data.DeadlineDate, fontSize: fontSizeBody},
			tableRow{label: "保存方法", value: fixedPetStorageMethod, fontSize: fontSizeBody},
			tableRow{label: "加工者名", value: fixedProcessorName, fontSize: fontSizeBody},
			tableRow{label: "加工施設所在地", value: fixedProcessorFacilityLocation, fontSize: fontSizeBody},
			tableRow{label: "金属探知機", value: fixedMetalDetectorStatus, fontSize: fontSizeBody},
		}
	case "traceable_deer":
		individualID := data.IndividualID
		if individualID == "" {
			individualID = data.IndividualNumber
		}
		storageMethod := data.StorageMethod
		if storageMethod == "" {
			storageMethod = data.StorageTemperature
		}
		if storageMethod == "" {
			storageMethod = fixedTraceableStorageMethod
		}
		return []row{
			tableRow{label: "商品名", value: data.ProductName, fontSize: fontSizeTitle},
			tableRow{label: "捕獲地", value: data.CaptureLocation, fontSize: fontSizeBody},
			tableRow{label: "内容量", value: data.ProductQuantity, fontSize: fontSizeBody},
			tableRow{label: "消費期限", value: data.DeadlineDate, fontSize: fontSizeBody},
			tableRow{label: "保存方法", value: storageMethod, fontSize: fontSizeBody},
			tableRow{label: "加工者名", value: fixedProcessorName, fontSize: fontSizeBody},
			tableRow{label: "加工施設所在地", value: fixedProcessorFacilityLocation, fontSize: fontSizeBody},
			tableRow{label: "金属探知機", value: fixedMetalDetectorStatus, fontSize: fontSizeBody},
			tableRow{label: "加熱用である旨", value: fixedHeatedInstruction, fontSize: fontSizeBody},
			tableRow{label: "個体識別番号", value: individualID, fontSize: fontSizeBody},
			qrTableRow{label: "QR", url: data.QRCode, size: 132},
		}
	default:
		return []row{
			tableRow{label: "商品名", value: data.ProductName, fontSize: fontSizeTitle},
			tableRow{label: "内容量", value: data.ProductQuantity, fontSize: fontSizeBody},
			tableRow{label: "消費期限", value: data.DeadlineDate, fontSize: fontSizeBody},
		}
	}
}

// makeFace creates a font.Face at the given point size.
func (r *LabelRenderer) makeFace(sizePt float64) font.Face {
	face, err := opentype.NewFace(r.fontRegular, &opentype.FaceOptions{
		Size:    sizePt,
		DPI:     labelDPI,
		Hinting: font.HintingFull,
	})
	if err != nil {
		// Should not happen with a valid font.
		panic(fmt.Sprintf("create font face: %v", err))
	}
	return face
}

// drawString draws a string on the image.
func drawString(img *image.RGBA, face font.Face, text string, x, y int) {
	d := &font.Drawer{
		Dst:  img,
		Src:  &image.Uniform{color.Black},
		Face: face,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)},
	}
	d.DrawString(text)
}

// loadFont searches for a Japanese-capable font file.
func loadFont(configPath string) (*opentype.Font, error) {
	paths := []string{}
	if configPath != "" {
		paths = append(paths, configPath)
	}
	// Well-known system font paths (Raspberry Pi OS / Debian).
	paths = append(paths,
		"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
		"/usr/share/fonts/truetype/fonts-japanese-gothic.ttf",
		"/usr/share/fonts/truetype/vlgothic/VL-Gothic-Regular.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", // fallback (no CJK)
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
