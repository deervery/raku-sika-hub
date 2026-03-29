package printer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	// NOTE: All label templates follow the same layout policy:
	// - Width is fixed to 62mm (horizontal is the priority).
	// - Height is driven by table height + image section height.
	// - Overall layout is intentionally horizontal (wide) rather than tall.
	labelWidthMM                = 62.0
	labelHeightMM               = 60.0
	labelDPI                    = 300
	marginXPx                   = 24
	marginYPx                   = 0
	imageSlotGap                = 6
	fontSizeBody                = 9.5
	minFontSize                 = 8.0
	lineSpacingRatio            = 1.1
	tableLabelWidthRatio        = 0.3
	tableLabelWidthTraceable    = 0.317
	tableLabelWidthNonTraceable = 0.295
	tableLabelWidthProcessed    = 0.208
	tableLabelWidthPet          = 0.295
	tableCellPadding            = 3
	maxTableLines               = 2
	logoWidthRatio              = 1.5
	imageSectionScale           = 0.89
	contentWidthScale           = 1.0
	minImageSizePx              = 90
)

var (
	labelWidthPx  = int(math.Round(labelWidthMM / 25.4 * labelDPI))
	labelHeightPx = int(math.Round(labelHeightMM / 25.4 * labelDPI))
	contentWidth  = int(math.Round(float64(labelWidthPx-2*marginXPx) * contentWidthScale))
	contentLeft   = (labelWidthPx - contentWidth) / 2
)

type row interface {
	height() int
	draw(img *image.RGBA, r *LabelRenderer, y int) int
}

type tableEntry struct {
	label string
	value string
}

// LabelRenderer generates printed labels.
type LabelRenderer struct {
	fontRegular *opentype.Font
	assetsDir   string
}

// NewLabelRenderer loads fonts and assets references.
func NewLabelRenderer(fontPath, assetsDir string) (*LabelRenderer, error) {
	f, err := loadFont(fontPath)
	if err != nil {
		return nil, err
	}
	return &LabelRenderer{fontRegular: f, assetsDir: strings.TrimSpace(assetsDir)}, nil
}

// RenderResult holds the output of Render.
type RenderResult struct {
	Path     string
	WidthMM  int
	HeightMM int
}

// Render produces a PNG label. Width is fixed at 62mm; height is content-driven.
func (r *LabelRenderer) Render(data LabelData) (RenderResult, error) {
	rows := r.buildRows(data)

	height := 0
	for _, row := range rows {
		height += row.height()
	}

	img := image.NewRGBA(image.Rect(0, 0, labelWidthPx, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	y := 0
	for _, row := range rows {
		y = row.draw(img, r, y)
	}

	tmpFile, err := os.CreateTemp("", "label-*.png")
	if err != nil {
		return RenderResult{}, fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	if err := png.Encode(tmpFile, img); err != nil {
		os.Remove(tmpFile.Name())
		return RenderResult{}, fmt.Errorf("encode png: %w", err)
	}

	bounds := img.Bounds()
	return RenderResult{
		Path:     tmpFile.Name(),
		WidthMM:  bounds.Dx() * 254 / (labelDPI * 10),
		HeightMM: bounds.Dy() * 254 / (labelDPI * 10),
	}, nil
}

func (r *LabelRenderer) buildRows(data LabelData) []row {
	// Carcass templates have their own dedicated layout.
	if data.Template == "carcass_deer" || data.Template == "carcass_bear" {
		return r.buildCarcassRows(data)
	}

	entries := buildTableEntries(data)
	fontSize := float64(fontSizeBody)
	spacer := 2

	// Table + warning + image define the whole label height for all templates.
	tableRow := tableBlockRow{
		entries:         entries,
		fontSize:        fontSize,
		maxLines:        maxTableLines,
		labelWidthRatio: labelWidthRatioForTemplate(data.Template),
	}
	warningRow1 := textRow{value: "加熱して", fontSize: fontSize}
	warningRow2 := textRow{value: "お召し上がりください", fontSize: fontSize}
	baseHeight := tableRow.height() + spacer + warningRow1.height() + warningRow2.height()
	targetContentHeight := labelHeightPx - 2*marginYPx
	availableHeight := targetContentHeight - baseHeight
	if availableHeight < 0 {
		availableHeight = 0
	}
	imageSize := calcImageSizeForData(data, contentWidth, availableHeight)
	imageRow := imageSectionRow{data: data, size: imageSize}

	rows := []row{
		tableRow,
		spacerRow{px: spacer},
	}

	// Traceable templates: QR is in textQRRow, skip imageSectionRow QR.
	if isTraceableTemplate(data.Template) {
		rows = append(rows, textQRRow{
			lines: []string{"加熱して", "お召し上がりください"},
			qrURL: data.QRCode,
		})
	} else {
		rows = append(rows,
			textRow{value: "加熱してお召し上がりください", fontSize: fontSize},
			imageRow,
		)
	}

	return rows
}

// buildCarcassRows creates the carcass label layout:
// large bold individualNumber at top, then text list (left) + QR (right).
func (r *LabelRenderer) buildCarcassRows(data LabelData) []row {
	indNum := strings.TrimSpace(data.IndividualNumber)
	return []row{
		largeTextRow{value: indNum, fontSize: 40},
		spacerRow{px: 10},
		carcassRow{
			texts: []string{
				strings.TrimSpace(data.Species),
				strings.TrimSpace(data.Sex),
				strings.TrimSpace(data.ReceivingDate),
				strings.TrimSpace(data.FacilityName),
			},
			qrURL:    strings.TrimSpace(data.QRCode),
			qrSize:   280,
			fontSize: 11.0,
		},
	}
}

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

	baseline := y + int(t.fontSize*float64(labelDPI)/72)
	drawString(img, face, text, contentLeft, baseline)
	return y + t.height()
}

type spacerRow struct {
	px int
}

func (s spacerRow) height() int { return s.px }

func (s spacerRow) draw(_ *image.RGBA, _ *LabelRenderer, y int) int {
	return y + s.px
}

type tableBlockRow struct {
	entries         []tableEntry
	fontSize        float64
	maxLines        int
	labelWidthRatio float64
}

type tableLayout struct {
	labelWidth  int
	valueWidth  int
	rows        []tableRowLayout
	totalHeight int
}

type tableRowLayout struct {
	labelLines    []string
	valueLines    []string
	labelFontSize float64
	valueFontSize float64
	height        int
}

func (t tableBlockRow) layout() tableLayout {
	ratio := tableLabelWidthRatio
	if t.labelWidthRatio > 0 {
		ratio = t.labelWidthRatio
	}
	labelWidth := int(float64(contentWidth) * ratio)
	if labelWidth < 1 {
		labelWidth = 1
	}
	valueWidth := contentWidth - labelWidth
	if valueWidth < 1 {
		valueWidth = contentWidth
	}

	rows := make([]tableRowLayout, 0, len(t.entries))
	totalHeight := 0
	for _, entry := range t.entries {
		labelLines, labelSize := fitLines(entry.label, t.fontSize, minFontSize, t.maxLines, labelWidth-2*tableCellPadding)
		valueLines, valueSize := fitLines(entry.value, t.fontSize, minFontSize, t.maxLines, valueWidth-2*tableCellPadding)

		if len(labelLines) == 0 {
			labelLines = []string{""}
		}
		if len(valueLines) == 0 {
			valueLines = []string{""}
		}

		linesCount := len(labelLines)
		if len(valueLines) > linesCount {
			linesCount = len(valueLines)
		}
		labelHeight := linesCount * lineHeight(labelSize)
		valueHeight := linesCount * lineHeight(valueSize)
		rowHeight := labelHeight
		if valueHeight > rowHeight {
			rowHeight = valueHeight
		}
		rowHeight += 2 * tableCellPadding
		minHeight := lineHeight(minFontSize) + 2*tableCellPadding
		if rowHeight < minHeight {
			rowHeight = minHeight
		}

		rows = append(rows, tableRowLayout{
			labelLines:    labelLines,
			valueLines:    valueLines,
			labelFontSize: labelSize,
			valueFontSize: valueSize,
			height:        rowHeight,
		})
		totalHeight += rowHeight
	}

	if len(rows) == 0 {
		rowHeight := lineHeight(t.fontSize) + 2*tableCellPadding
		rows = append(rows, tableRowLayout{
			labelLines:    []string{""},
			valueLines:    []string{""},
			labelFontSize: t.fontSize,
			valueFontSize: t.fontSize,
			height:        rowHeight,
		})
		totalHeight += rowHeight
	}

	return tableLayout{
		labelWidth:  labelWidth,
		valueWidth:  valueWidth,
		rows:        rows,
		totalHeight: totalHeight,
	}
}

func (t tableBlockRow) height() int {
	return t.layout().totalHeight
}

func (t tableBlockRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	layout := t.layout()
	if len(layout.rows) == 0 {
		return y
	}

	borderColor := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	topY := y
	bottomY := y + layout.totalHeight
	drawHLine(img, contentLeft, contentLeft+contentWidth, topY, borderColor)
	drawHLine(img, contentLeft, contentLeft+contentWidth, bottomY, borderColor)
	drawVLine(img, contentLeft, topY, bottomY, borderColor)
	drawVLine(img, contentLeft+layout.labelWidth, topY, bottomY, borderColor)
	drawVLine(img, contentLeft+contentWidth, topY, bottomY, borderColor)

	currY := y
	for _, row := range layout.rows {
		labelFace := r.makeFace(row.labelFontSize)
		valueFace := r.makeFace(row.valueFontSize)

		labelBaseline := currY + tableCellPadding + lineHeight(row.labelFontSize)
		valueBaseline := currY + tableCellPadding + lineHeight(row.valueFontSize)

		labelX := contentLeft + tableCellPadding
		valueX := contentLeft + layout.labelWidth + tableCellPadding

		for _, line := range row.labelLines {
			drawStringFitWidth(img, labelFace, line, labelX, labelBaseline, layout.labelWidth-2*tableCellPadding)
			labelBaseline += lineHeight(row.labelFontSize)
		}
		for _, line := range row.valueLines {
			drawStringFitWidth(img, valueFace, line, valueX, valueBaseline, layout.valueWidth-2*tableCellPadding)
			valueBaseline += lineHeight(row.valueFontSize)
		}

		labelFace.Close()
		valueFace.Close()

		currY += row.height
		drawHLine(img, contentLeft, contentLeft+contentWidth, currY, borderColor)
	}

	return y + layout.totalHeight
}

type imageSectionRow struct {
	data LabelData
	size int
}

func (row imageSectionRow) height() int {
	return row.cardSize() + imageSlotGap
}

func (row imageSectionRow) cardSize() int {
	if row.size > 0 {
		return row.size
	}
	return calcImageSizeForData(row.data, contentWidth, 0)
}

func (row imageSectionRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	top := y + imageSlotGap/2
	size := row.cardSize()
	if size <= 0 {
		return y + row.height()
	}

	switch row.data.Template {
	case "individual_qr":
		if row.showQRCode() {
			rect := image.Rect(contentLeft, top, contentLeft+contentWidth, top+size)
			r.drawQRCodeIntoRect(img, rect, strings.TrimSpace(row.data.QRCode))
		}
		return y + row.height()
	default:
		if row.showLogoOnly() {
			row.drawLogoFullWidth(img, r, top, size)
			return y + row.height()
		}
		row.drawTraceableImages(img, r, top, size)
		return y + row.height()
	}
}

func (row imageSectionRow) showLogoOnly() bool {
	switch row.data.Template {
	case "processed", "pet", "non_traceable", "non_traceable_deer":
		return true
	default:
		return false
	}
}

func (row imageSectionRow) showQRCode() bool {
	if strings.TrimSpace(row.data.QRCode) == "" {
		return false
	}
	switch row.data.Template {
	case "traceable", "traceable_deer", "traceable_bear", "individual_qr":
		return true
	default:
		return false
	}
}

func (row imageSectionRow) showCertification() bool {
	if row.data.Template == "traceable_bear" {
		return false
	}
	if strings.TrimSpace(row.data.CertificationMarkFile) == "" {
		return false
	}
	return row.showQRCode()
}

func (row imageSectionRow) logoPath() string {
	return strings.TrimSpace(row.data.LogoFile)
}

func (row imageSectionRow) certPath() string {
	return strings.TrimSpace(row.data.CertificationMarkFile)
}

func (row imageSectionRow) drawLogoFullWidth(img *image.RGBA, r *LabelRenderer, top, size int) {
	if path := row.logoPath(); path != "" {
		if logo, err := r.loadAssetImage(path); err == nil && logo != nil {
			rect := image.Rect(contentLeft, top, contentLeft+contentWidth, top+size)
			r.drawImageWithinRect(img, logo, rect)
		}
	}
}

func (row imageSectionRow) drawTraceableImages(img *image.RGBA, r *LabelRenderer, top, size int) {
	if !row.showQRCode() {
		row.drawLogoFullWidth(img, r, top, size)
		return
	}

	showCert := row.showCertification()
	slotCount := 0
	if showCert {
		slotCount++
	}
	if row.showQRCode() {
		slotCount++
	}

	logoWidth := contentWidth - slotCount*(size+imageSlotGap)
	if logoWidth < size {
		logoWidth = size
	}

	cursor := contentLeft
	if logoWidth > 0 {
		row.drawLogoAt(img, r, cursor, top, logoWidth, size)
	}
	cursor += logoWidth

	if showCert {
		cursor += imageSlotGap
		row.drawCertificationAt(img, r, cursor, top, size)
		cursor += size
	}

	if row.showQRCode() {
		cursor += imageSlotGap
		row.drawQRCodeAt(img, r, cursor, top, size)
	}
}

func (row imageSectionRow) drawLogoAt(img *image.RGBA, r *LabelRenderer, x, top, width, size int) {
	if width <= 0 {
		return
	}
	path := row.logoPath()
	if path == "" {
		return
	}
	if logo, err := r.loadAssetImage(path); err == nil && logo != nil {
		rect := image.Rect(x, top, x+width, top+size)
		r.drawImageWithinRect(img, logo, rect)
	}
}

func (row imageSectionRow) drawCertificationAt(img *image.RGBA, r *LabelRenderer, x, top, size int) {
	path := row.certPath()
	if path == "" {
		return
	}
	if cert, err := r.loadAssetImage(path); err == nil && cert != nil {
		rect := image.Rect(x, top, x+size, top+size)
		r.drawImageWithinRect(img, cert, rect)
	}
}

func (row imageSectionRow) drawQRCodeAt(img *image.RGBA, r *LabelRenderer, x, top, size int) {
	if !row.showQRCode() || size <= 0 {
		return
	}
	rect := image.Rect(x, top, x+size, top+size)
	r.drawQRCodeIntoRect(img, rect, strings.TrimSpace(row.data.QRCode))
}

func buildTableEntries(data LabelData) []tableEntry {
	trim := strings.TrimSpace
	if data.Template == "individual_qr" {
		entries := []tableEntry{}
		if name := trim(data.ProductName); name != "" {
			entries = append(entries, tableEntry{label: "品名", value: name})
		}
		entries = append(entries, tableEntry{label: "個体識別番号", value: trim(data.IndividualNumber)})
		return entries
	}

	switch data.Template {
	case "traceable", "traceable_deer", "traceable_bear":
		return []tableEntry{
			{label: "商品名", value: trim(data.ProductName)},
			{label: "捕獲地", value: trim(data.CaptureLocation)},
			{label: "内容量", value: trim(data.ProductQuantity)},
			{label: "消費期限", value: trim(data.DeadlineDate)},
			{label: "保存方法", value: trim(data.StorageTemperature)},
			{label: "加工者名", value: trim(data.ProcessorName)},
			{label: "加工施設\n所在地", value: trim(data.ProcessorLocation)},
			{label: "金属探知機", value: "検査済み"},
			{label: "個体識別番号", value: trim(data.IndividualNumber)},
		}
	case "non_traceable", "non_traceable_deer":
		return []tableEntry{
			{label: "商品名", value: trim(data.ProductName)},
			{label: "内容量", value: trim(data.ProductQuantity)},
			{label: "消費期限", value: trim(data.DeadlineDate)},
			{label: "保存方法", value: trim(data.StorageTemperature)},
			{label: "加工者名", value: trim(data.ProcessorName)},
			{label: "加工施設\n所在地", value: trim(data.ProcessorLocation)},
			{label: "金属探知機", value: "検査済み"},
		}
	case "processed":
		return []tableEntry{
			{label: "名称", value: trim(data.ProductName)},
			{label: "原材料名", value: trim(data.ProductIngredient)},
			{label: "内容量", value: trim(data.ProductQuantity)},
			{label: "賞味期限", value: trim(data.DeadlineDate)},
			{label: "保存方法", value: trim(data.StorageTemperature)},
		}
	case "pet":
		return []tableEntry{
			{label: "商品名", value: trim(data.ProductName)},
			{label: "内容量", value: trim(data.ProductQuantity)},
			{label: "消費期限", value: trim(data.DeadlineDate)},
			{label: "保存方法", value: trim(data.StorageTemperature)},
			{label: "加工者名", value: trim(data.ProcessorName)},
			{label: "加工施設\n所在地", value: trim(data.ProcessorLocation)},
			{label: "金属探知機", value: "検査済み"},
		}
	}
	return []tableEntry{
		{label: "商品名", value: trim(data.ProductName)},
		{label: "内容量", value: trim(data.ProductQuantity)},
		{label: "消費期限", value: trim(data.DeadlineDate)},
		{label: "保存方法", value: trim(data.StorageTemperature)},
		{label: "加工者名", value: trim(data.ProcessorName)},
		{label: "加工施設所在地", value: trim(data.ProcessorLocation)},
		{label: "金属探知機", value: "検査済み"},
	}
}

func labelWidthRatioForTemplate(template string) float64 {
	switch template {
	case "traceable", "traceable_deer", "traceable_bear":
		return tableLabelWidthTraceable
	case "non_traceable", "non_traceable_deer":
		return tableLabelWidthNonTraceable
	case "processed":
		return tableLabelWidthProcessed
	case "pet":
		return tableLabelWidthPet
	default:
		return tableLabelWidthRatio
	}
}

func isTraceableTemplate(template string) bool {
	switch template {
	case "traceable", "traceable_deer", "traceable_bear":
		return true
	default:
		return false
	}
}

func (r *LabelRenderer) drawQRCodeIntoRect(img *image.RGBA, rect image.Rectangle, url string) {
	if rect.Empty() || strings.TrimSpace(url) == "" {
		return
	}
	qrPng, err := qrcode.Encode(url, qrcode.Medium, rect.Dx())
	if err != nil {
		return
	}
	qrImg, err := png.Decode(strings.NewReader(string(qrPng)))
	if err != nil || qrImg.Bounds().Empty() {
		return
	}
	draw.Draw(img, rect, qrImg, image.Point{}, draw.Over)
}

func lineHeight(size float64) int {
	return int(size * lineSpacingRatio * float64(labelDPI) / 72)
}

func wrapText(text string, fontSize float64, maxWidth int) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []string{""}
	}
	if maxWidth <= 0 {
		return []string{trimmed}
	}

	charWidth := fontSize * float64(labelDPI) / 72 * 0.55
	if charWidth <= 0 {
		charWidth = 1
	}
	maxChars := int(float64(maxWidth) / charWidth)
	if maxChars < 1 {
		maxChars = 1
	}

	parts := strings.Split(trimmed, "\n")
	var lines []string
	for _, part := range parts {
		runes := []rune(part)
		if len(runes) == 0 {
			lines = append(lines, "")
			continue
		}
		for len(runes) > 0 {
			end := maxChars
			if end > len(runes) {
				end = len(runes)
			}
			lines = append(lines, string(runes[:end]))
			runes = runes[end:]
		}
	}
	return lines
}

func clampLines(lines []string, maxLines int, fontSize float64, maxWidth int) []string {
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines
	}
	clamped := append([]string{}, lines[:maxLines]...)
	maxChars := maxCharsForWidth(fontSize, maxWidth)
	last := clamped[len(clamped)-1]
	if maxChars > 3 {
		runes := []rune(last)
		if len(runes) > maxChars-3 {
			last = string(runes[:maxChars-3])
		}
		last = strings.TrimRight(last, " ") + "..."
	}
	clamped[len(clamped)-1] = last
	return clamped
}

func maxCharsForWidth(fontSize float64, maxWidth int) int {
	if maxWidth <= 0 {
		return 1
	}
	charWidth := fontSize * float64(labelDPI) / 72 * 0.55
	if charWidth <= 0 {
		return 1
	}
	maxChars := int(float64(maxWidth) / charWidth)
	if maxChars < 1 {
		return 1
	}
	return maxChars
}

func fitLines(text string, baseSize, minSize float64, maxLines, maxWidth int) ([]string, float64) {
	if strings.TrimSpace(text) == "" {
		return []string{""}, baseSize
	}
	size := baseSize
	for size >= minSize {
		lines := wrapText(text, size, maxWidth)
		if maxLines <= 0 || len(lines) <= maxLines {
			return lines, size
		}
		size -= 0.5
		if size < minSize {
			size = minSize
		}
	}
	lines := wrapText(text, minSize, maxWidth)
	lines = clampLines(lines, maxLines, minSize, maxWidth)
	return lines, minSize
}

func calcImageSizeForData(data LabelData, widthPx, availableHeight int) int {
	row := imageSectionRow{data: data}
	if row.showLogoOnly() {
		return calcImageSize(widthPx, availableHeight, 0, 1)
	}
	if data.Template == "individual_qr" {
		return calcImageSize(widthPx, availableHeight, 0, 1)
	}

	slotCount := 0
	if row.showCertification() {
		slotCount++
	}
	if row.showQRCode() {
		slotCount++
	}
	return calcImageSize(widthPx, availableHeight, logoWidthRatio, slotCount)
}

func calcImageSize(widthPx, availableHeight int, logoRatio float64, slotCount int) int {
	if slotCount < 0 {
		slotCount = 0
	}
	available := widthPx - slotCount*imageSlotGap
	if available <= 0 {
		available = widthPx
	}
	denom := logoRatio + float64(slotCount)
	if denom <= 0 {
		denom = 1
	}
	size := int(math.Floor(float64(available) / denom))
	if availableHeight > 0 {
		maxSize := availableHeight - imageSlotGap
		if maxSize < 1 {
			maxSize = 1
		}
		if size > maxSize {
			size = maxSize
		}
	}
	size = int(math.Floor(float64(size) * imageSectionScale))
	if size < minImageSizePx {
		size = minImageSizePx
	}
	if size < 1 {
		size = 1
	}
	return size
}

func drawHLine(img *image.RGBA, x1, x2, y int, c color.Color) {
	bounds := img.Bounds()
	if y < bounds.Min.Y || y >= bounds.Max.Y {
		return
	}
	if x1 < bounds.Min.X {
		x1 = bounds.Min.X
	}
	if x2 > bounds.Max.X {
		x2 = bounds.Max.X
	}
	for x := x1; x <= x2; x++ {
		img.Set(x, y, c)
	}
}

func drawVLine(img *image.RGBA, x, y1, y2 int, c color.Color) {
	bounds := img.Bounds()
	if x < bounds.Min.X || x >= bounds.Max.X {
		return
	}
	if y1 < bounds.Min.Y {
		y1 = bounds.Min.Y
	}
	if y2 > bounds.Max.Y {
		y2 = bounds.Max.Y
	}
	for y := y1; y <= y2; y++ {
		img.Set(x, y, c)
	}
}

func (r *LabelRenderer) loadAssetImage(file string) (image.Image, error) {
	if file == "" {
		return nil, nil
	}
	resolved := r.resolveAssetPath(file)
	if resolved == "" {
		return nil, nil
	}
	f, err := os.Open(resolved)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, err
	}
	return img, nil
}

func (r *LabelRenderer) resolveAssetPath(file string) string {
	if file == "" {
		return ""
	}
	if filepath.IsAbs(file) || r.assetsDir == "" {
		return file
	}
	return filepath.Join(r.assetsDir, file)
}

func (r *LabelRenderer) drawImageWithinRect(dst *image.RGBA, src image.Image, rect image.Rectangle) {
	if src == nil || rect.Empty() {
		return
	}
	bounds := src.Bounds()
	if bounds.Empty() {
		return
	}
	maxW := rect.Dx()
	maxH := rect.Dy()
	if maxW <= 0 || maxH <= 0 {
		return
	}
	scale := math.Min(float64(maxW)/float64(bounds.Dx()), float64(maxH)/float64(bounds.Dy()))
	if scale <= 0 {
		return
	}
	scaledW := int(math.Round(float64(bounds.Dx()) * scale))
	scaledH := int(math.Round(float64(bounds.Dy()) * scale))
	if scaledW <= 0 || scaledH <= 0 {
		return
	}
	scaled := image.NewRGBA(image.Rect(0, 0, scaledW, scaledH))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), src, bounds, draw.Over, nil)
	offsetX := rect.Min.X + (maxW-scaledW)/2
	offsetY := rect.Min.Y + (maxH-scaledH)/2
	draw.Draw(dst, image.Rect(offsetX, offsetY, offsetX+scaledW, offsetY+scaledH), scaled, image.Point{}, draw.Over)
}

// ── textQRRow — text on the left, QR on the right ──

type textQRRow struct {
	lines    []string
	qrURL    string
	fontSize float64
}

func (t textQRRow) effectiveFontSize() float64 {
	if t.fontSize > 0 {
		return t.fontSize
	}
	return fontSizeBody
}

func (t textQRRow) qrSizePx() int {
	return contentWidth * 40 / 100
}

func (t textQRRow) height() int {
	fs := t.effectiveFontSize()
	lh := lineHeight(fs)
	textH := lh * len(t.lines)
	qrH := t.qrSizePx() + 4
	if qrH > textH {
		return qrH
	}
	return textH
}

func (t textQRRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	fs := t.effectiveFontSize()
	face := r.makeFace(fs)
	defer face.Close()

	rowHeight := t.height()
	lh := lineHeight(fs)

	textTotalH := lh * len(t.lines)
	ty := y + (rowHeight-textTotalH)/2
	for _, line := range t.lines {
		baseline := ty + int(fs*float64(labelDPI)/72)
		drawString(img, face, line, contentLeft, baseline)
		ty += lh
	}

	if strings.TrimSpace(t.qrURL) != "" {
		qs := t.qrSizePx()
		qrPng, err := qrcode.Encode(t.qrURL, qrcode.Medium, qs)
		if err == nil {
			qrImg, err := png.Decode(strings.NewReader(string(qrPng)))
			if err == nil {
				qrX := contentLeft + contentWidth - qs
				qrY := y + (rowHeight-qs)/2
				draw.Draw(img, image.Rect(qrX, qrY, qrX+qs, qrY+qs),
					qrImg, image.Point{}, draw.Over)
			}
		}
	}
	return y + rowHeight
}

// ── carcassRow — text list (left 60%) + QR (right 40%) ──

type carcassRow struct {
	texts    []string
	qrURL    string
	qrSize   int
	fontSize float64
}

func (c carcassRow) height() int {
	lh := lineHeight(c.fontSize)
	textH := lh * len(c.texts)
	qrH := c.qrSize + marginXPx
	if qrH > textH {
		return qrH
	}
	return textH
}

func (c carcassRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	face := r.makeFace(c.fontSize)
	defer face.Close()

	rowHeight := c.height()
	left := contentLeft
	textWidth := contentWidth * 60 / 100
	qrAreaWidth := contentWidth - textWidth

	lh := lineHeight(c.fontSize)
	textTotalH := lh * len(c.texts)
	ty := y + (rowHeight-textTotalH)/2
	for _, text := range c.texts {
		baseline := ty + int(c.fontSize*float64(labelDPI)/72)
		drawString(img, face, text, left, baseline)
		ty += lh
	}

	if strings.TrimSpace(c.qrURL) != "" {
		qrPng, err := qrcode.Encode(c.qrURL, qrcode.Medium, c.qrSize)
		if err == nil {
			qrImg, err := png.Decode(strings.NewReader(string(qrPng)))
			if err == nil {
				qrX := left + textWidth + (qrAreaWidth-c.qrSize)/2
				qrY := y + (rowHeight-c.qrSize)/2
				draw.Draw(img, image.Rect(qrX, qrY, qrX+c.qrSize, qrY+c.qrSize),
					qrImg, image.Point{}, draw.Over)
			}
		}
	}
	return y + rowHeight
}

// ── largeTextRow — auto-fits bold text, multi-line support ──

type largeTextRow struct {
	value    string
	fontSize float64
}

func (t largeTextRow) fittedLayout(r *LabelRenderer) (float64, []string) {
	maxW := contentWidth
	for size := t.fontSize; size >= 6.0; size -= 1.0 {
		face := r.makeFace(size)
		lines := wrapTextWithFace(face, t.value, maxW)
		face.Close()
		if len(lines) <= 2 {
			return size, lines
		}
	}
	face := r.makeFace(6.0)
	lines := wrapTextWithFace(face, t.value, maxW)
	face.Close()
	return 6.0, lines
}

func (t largeTextRow) height() int {
	// Approximate — actual height computed in draw.
	return lineHeight(t.fontSize) * 2
}

func (t largeTextRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	size, lines := t.fittedLayout(r)
	face := r.makeFace(size)
	defer face.Close()
	lh := lineHeight(size)
	for _, line := range lines {
		baseline := y + int(size*float64(labelDPI)/72)
		drawStringBold(img, face, line, contentLeft, baseline)
		y += lh
	}
	return y
}

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

func drawStringBold(img *image.RGBA, face font.Face, text string, x, y int) {
	src := &image.Uniform{color.Black}
	for _, dx := range []int{0, 1, 2} {
		d := &font.Drawer{
			Dst:  img,
			Src:  src,
			Face: face,
			Dot:  fixed.Point26_6{X: fixed.I(x + dx), Y: fixed.I(y)},
		}
		d.DrawString(text)
	}
}

func (r *LabelRenderer) makeFace(size float64) font.Face {
	face, err := opentype.NewFace(r.fontRegular, &opentype.FaceOptions{
		Size:    size,
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

func drawStringFitWidth(img *image.RGBA, face font.Face, text string, x, y, maxWidth int) {
	if strings.TrimSpace(text) == "" {
		return
	}
	if maxWidth <= 0 {
		drawString(img, face, text, x, y)
		return
	}

	bounds, _ := font.BoundString(face, text)
	width := (bounds.Max.X - bounds.Min.X).Ceil()
	if width <= 0 || width <= maxWidth {
		drawString(img, face, text, x, y)
		return
	}

	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	descent := metrics.Descent.Ceil()
	height := ascent + descent
	if height <= 0 {
		drawString(img, face, text, x, y)
		return
	}

	tmp := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(tmp, tmp.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	d := &font.Drawer{
		Dst:  tmp,
		Src:  &image.Uniform{color.Black},
		Face: face,
		Dot: fixed.Point26_6{
			X: fixed.I(-bounds.Min.X.Ceil()),
			Y: fixed.I(ascent),
		},
	}
	d.DrawString(text)

	scaled := image.NewRGBA(image.Rect(0, 0, maxWidth, height))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), tmp, tmp.Bounds(), draw.Over, nil)

	topY := y - ascent
	draw.Draw(img, image.Rect(x, topY, x+maxWidth, topY+height), scaled, image.Point{}, draw.Over)
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
