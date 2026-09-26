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
	labelWidthMM             = 62.0
	labelHeightMM            = 60.0
	labelDPI                 = 300
	marginXPx                = 24
	marginYPx                = 0
	imageSlotGap             = 6
	fontSizeBody             = 9.5
	minFontSize              = 8.0
	lineSpacingRatio         = 1.1
	tableLabelWidthRatio     = 0.3
	tableLabelWidthTraceable = 0.317
	tableLabelWidthPet       = 0.295
	tableCellPadding         = 3
	// Table rules are 2 dots (≈0.17mm), as the P-touch templates' 0.5pt pen.
	// A 1-dot rule vanished when the image was shown scaled down, and
	// thinned out when ipp-usb shrank the label to fit.
	ruleDots          = 2
	maxTableLines     = 2
	logoWidthRatio    = 1.5
	imageSectionScale = 0.89
	contentWidthScale = 1.0
	minImageSizePx    = 90
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
	label         string
	value         string
	maxValueLines int
	minValueLines int
	keepValueFont bool
	valueLineGap  float64
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
// Exception: EN bilingual traceable labels use a fixed 101mm × 62mm landscape
// canvas (see renderENTraceable).
func (r *LabelRenderer) Render(data LabelData) (RenderResult, error) {
	if isENBilingualTraceable(data) {
		return r.renderENTraceable(data)
	}
	if isProcessedLandscape(data) {
		return r.renderProcessed(data)
	}
	if isNonTraceableLandscape(data) {
		return r.renderNonTraceable(data)
	}
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
	warningHeight := warningRow1.height() + warningRow2.height()
	if data.Template == "pet" {
		warningHeight = 0 // Pet labels don't have warning text
	}
	baseHeight := tableRow.height() + spacer + warningHeight
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
		// #271: JA traceable では警告文の右側にエゾシカ認証ロゴを配置 (HACCP は廃止)。
		// EN bilingual は別 renderer (renderENTraceable) で完結している。
		// lite は certificationMarkFile を送らないため空のときは ninsyo_logo.jpg にフォールバック。
		certPath := ""
		if data.Locale != "en" &&
			data.EzoshikaCertified &&
			data.Template != "traceable_bear" {
			certPath = strings.TrimSpace(data.CertificationMarkFile)
			if certPath == "" {
				certPath = "ninsyo_logo.jpg"
			}
		}
		rows = append(rows, textQRRow{
			lines:    warningLines(data.Locale, certPath != ""),
			qrURL:    data.QRCode,
			certPath: certPath,
			plaMark:  data.PlaMark,
		})
	} else if data.Template == "pet" {
		// Pet: no warning text, no image section
		if data.PlaMark {
			rows = append(rows, plaBadgeRow{r: r})
		}
	} else {
		rows = append(rows,
			textRow{value: warningText(data.Locale), fontSize: fontSize},
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
			fontSize: 18.0,
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

	drawString(img, face, text, contentLeft, baselineInSlot(face, y, t.height()))
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
	labelLines      []string
	valueLines      []string
	labelFontSize   float64
	valueFontSize   float64
	valueLineHeight int
	height          int
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
		labelLines, labelSize := fitAllLines(entry.label, t.fontSize, minFontSize, t.maxLines, labelWidth-2*tableCellPadding)
		valueMaxLines := t.maxLines
		if entry.maxValueLines > 0 {
			valueMaxLines = entry.maxValueLines
		}
		var valueLines []string
		var valueSize float64
		if entry.keepValueFont {
			// A block's lines are printed in full: a long address used to
			// push the TEL line out behind "...".
			valueLines = wrapText(entry.value, t.fontSize, valueWidth-2*tableCellPadding)
			valueSize = t.fontSize
		} else {
			valueLines, valueSize = fitAllLines(entry.value, t.fontSize, minFontSize, valueMaxLines, valueWidth-2*tableCellPadding)
		}
		valueLineHeight := lineHeight(valueSize)
		if entry.valueLineGap > 0 {
			valueLineHeight = lineHeightWithRatio(valueSize, entry.valueLineGap)
		}

		if len(labelLines) == 0 {
			labelLines = []string{""}
		}
		if len(valueLines) == 0 {
			valueLines = []string{""}
		}
		for len(valueLines) < entry.minValueLines {
			valueLines = append(valueLines, "")
		}

		linesCount := len(labelLines)
		if len(valueLines) > linesCount {
			linesCount = len(valueLines)
		}
		labelHeight := linesCount * lineHeight(labelSize)
		valueHeight := linesCount * valueLineHeight
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
			labelLines:      labelLines,
			valueLines:      valueLines,
			labelFontSize:   labelSize,
			valueFontSize:   valueSize,
			valueLineHeight: valueLineHeight,
			height:          rowHeight,
		})
		totalHeight += rowHeight
	}

	if len(rows) == 0 {
		rowHeight := lineHeight(t.fontSize) + 2*tableCellPadding
		rows = append(rows, tableRowLayout{
			labelLines:      []string{""},
			valueLines:      []string{""},
			labelFontSize:   t.fontSize,
			valueFontSize:   t.fontSize,
			valueLineHeight: lineHeight(t.fontSize),
			height:          rowHeight,
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

	ys := []int{y}
	currY := y
	for _, row := range layout.rows {
		labelFace := r.makeFace(row.labelFontSize)
		valueFace := r.makeFace(row.valueFontSize)

		labelX := contentLeft + tableCellPadding
		valueX := contentLeft + layout.labelWidth + tableCellPadding

		labelLH := lineHeight(row.labelFontSize)
		slotTop := currY + tableCellPadding
		for _, line := range row.labelLines {
			drawStringFitWidth(img, labelFace, line, labelX, baselineInSlot(labelFace, slotTop, labelLH), layout.labelWidth-2*tableCellPadding)
			slotTop += labelLH
		}
		slotTop = currY + tableCellPadding
		for _, line := range row.valueLines {
			drawStringFitWidth(img, valueFace, line, valueX, baselineInSlot(valueFace, slotTop, row.valueLineHeight), layout.valueWidth-2*tableCellPadding)
			slotTop += row.valueLineHeight
		}

		labelFace.Close()
		valueFace.Close()

		currY += row.height
		ys = append(ys, currY)
	}
	xs := []int{contentLeft, contentLeft + layout.labelWidth, contentLeft + contentWidth}
	drawGrid(img, xs, ys, ruleDots)

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
	case "pet":
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
	// ezoshika 認証マークは施設が認証取得済みのときだけ描画 (#271)。
	// CertificationMarkFile が空の場合も skip (後方互換)。
	if !row.data.EzoshikaCertified {
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

// padBlockToMinLines ensures a block text has at least minLines lines (padding with empty lines).
func padBlockToMinLines(block string, minLines int) string {
	lines := strings.Split(block, "\n")
	for len(lines) < minLines {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func companyEntry(data LabelData) (tableEntry, bool) {
	trim := strings.TrimSpace
	switch {
	case trim(data.CompanyBlock) != "":
		return tableEntry{
			label:         localizedCaption(data.Locale, "加工者", "Processor"),
			value:         padBlockToMinLines(trim(data.CompanyBlock), 3),
			minValueLines: 3,
			maxValueLines: 3,
			keepValueFont: true,
			valueLineGap:  1.35,
		}, true
	case trim(data.ProcessorName) != "":
		return tableEntry{label: localizedCaption(data.Locale, "加工者", "Processor"), value: trim(data.ProcessorName)}, true
	default:
		return tableEntry{}, false
	}
}

func facilityEntry(data LabelData) (tableEntry, bool) {
	trim := strings.TrimSpace
	switch {
	case trim(data.FacilityBlock) != "":
		return tableEntry{
			label:         localizedCaption(data.Locale, "加工所", "Facility"),
			value:         padBlockToMinLines(trim(data.FacilityBlock), 3),
			minValueLines: 3,
			maxValueLines: 3,
			keepValueFont: true,
			valueLineGap:  1.35,
		}, true
	case trim(data.ProcessorLocation) != "":
		return tableEntry{label: localizedCaption(data.Locale, "加工所", "Facility"), value: trim(data.ProcessorLocation)}, true
	case trim(data.FacilityName) != "":
		return tableEntry{label: localizedCaption(data.Locale, "加工所", "Facility"), value: trim(data.FacilityName)}, true
	default:
		return tableEntry{}, false
	}
}

func deadlineCaption(data LabelData, fallbackJa, fallbackEn string) string {
	if label := strings.TrimSpace(data.DeadlineLabel); label != "" {
		return label
	}
	return localizedCaption(data.Locale, fallbackJa, fallbackEn)
}

func buildTableEntries(data LabelData) []tableEntry {
	trim := strings.TrimSpace
	if data.Template == "individual_qr" {
		entries := []tableEntry{}
		if name := trim(data.ProductName); name != "" {
			entries = append(entries, tableEntry{label: localizedCaption(data.Locale, "品名", "Product"), value: name})
		}
		entries = append(entries, tableEntry{label: localizedCaption(data.Locale, "個体識別番号", "Individual ID"), value: trim(data.IndividualNumber)})
		return entries
	}

	switch data.Template {
	case "traceable", "traceable_deer", "traceable_bear", "traceable_boar", "traceable_raccoon":
		entries := []tableEntry{
			{label: localizedCaption(data.Locale, "商品名", "Product Name"), value: trim(data.ProductName)},
			{label: localizedCaption(data.Locale, "捕獲地", "Capture Location"), value: trim(data.CaptureLocation)},
			{label: localizedCaption(data.Locale, "内容量", "Net Weight"), value: trim(data.ProductQuantity)},
			{label: deadlineCaption(data, "消費期限", "Use By"), value: trim(data.DeadlineDate)},
			{label: localizedCaption(data.Locale, "保存方法", "Storage"), value: trim(data.StorageTemperature)},
		}
		if entry, ok := companyEntry(data); ok {
			entries = append(entries, entry)
		}
		if entry, ok := facilityEntry(data); ok {
			entries = append(entries, entry)
		}
		entries = append(entries,
			tableEntry{label: localizedCaption(data.Locale, "金属探知機", "Metal Detection"), value: localizedCaption(data.Locale, "検査済み", "Passed")},
			tableEntry{label: localizedCaption(data.Locale, "個体識別番号", "Individual ID"), value: trim(data.IndividualNumber)},
		)
		return entries
	case "pet":
		entries := []tableEntry{
			{label: localizedCaption(data.Locale, "商品名", "Product Name"), value: trim(data.ProductName)},
			{label: localizedCaption(data.Locale, "内容量", "Net Weight"), value: trim(data.ProductQuantity)},
			{label: deadlineCaption(data, "消費期限", "Use By"), value: trim(data.DeadlineDate)},
			{label: localizedCaption(data.Locale, "保存方法", "Storage"), value: trim(data.StorageTemperature)},
		}
		if entry, ok := companyEntry(data); ok {
			entries = append(entries, entry)
		}
		if entry, ok := facilityEntry(data); ok {
			entries = append(entries, entry)
		}
		return entries
	}
	entries := []tableEntry{
		{label: localizedCaption(data.Locale, "商品名", "Product Name"), value: trim(data.ProductName)},
		{label: localizedCaption(data.Locale, "内容量", "Net Weight"), value: trim(data.ProductQuantity)},
		{label: deadlineCaption(data, "消費期限", "Use By"), value: trim(data.DeadlineDate)},
		{label: localizedCaption(data.Locale, "保存方法", "Storage"), value: trim(data.StorageTemperature)},
	}
	if entry, ok := companyEntry(data); ok {
		entries = append(entries, entry)
	}
	if entry, ok := facilityEntry(data); ok {
		entries = append(entries, entry)
	}
	entries = append(entries, tableEntry{label: localizedCaption(data.Locale, "金属探知機", "Metal Detection"), value: localizedCaption(data.Locale, "検査済み", "Passed")})
	return entries
}

func localizedCaption(locale, ja, en string) string {
	if strings.EqualFold(strings.TrimSpace(locale), "en") {
		return en
	}
	return ja
}

func warningText(locale string) string {
	return localizedCaption(locale, "加熱してお召し上がりください", "Cook thoroughly before eating")
}

// warningLines returns the "cook thoroughly" warning shown next to the QR.
//
// withCertLogo=true splits the JA text into 3 shorter lines so that the
// warning, the ezoshika certification logo and the QR all fit on one row
// (#271). Facilities without the certification keep the original 2-line
// wording, so their labels are unchanged.
func warningLines(locale string, withCertLogo bool) []string {
	if strings.EqualFold(strings.TrimSpace(locale), "en") {
		return []string{"Cook thoroughly", "before eating"}
	}
	if withCertLogo {
		return []string{"加熱して", "お召し上がり", "ください"}
	}
	return []string{"加熱して", "お召し上がりください"}
}

func labelWidthRatioForTemplate(template string) float64 {
	switch template {
	case "traceable", "traceable_deer", "traceable_bear", "traceable_boar", "traceable_raccoon":
		return tableLabelWidthTraceable
	case "pet":
		return tableLabelWidthPet
	default:
		return tableLabelWidthRatio
	}
}

func isTraceableTemplate(template string) bool {
	switch template {
	case "traceable", "traceable_deer", "traceable_bear", "traceable_boar", "traceable_raccoon":
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

// baselineInSlot returns the baseline that centers a line of text in a slot
// of the given height, measured on a full-width ideograph. A baseline one
// line height (or one em) below the slot top, as before, left the bottom of
// the glyphs on the table rule below and on the top of the next line.
func baselineInSlot(face font.Face, slotTop, slotHeight int) int {
	b, _ := font.BoundString(face, "国")
	top, bottom := b.Min.Y.Floor(), b.Max.Y.Ceil()
	if bottom <= top {
		return slotTop + slotHeight
	}
	return slotTop + (slotHeight-(bottom-top))/2 - top
}

func lineHeightWithRatio(size, ratio float64) int {
	if ratio <= 0 {
		ratio = lineSpacingRatio
	}
	return int(size * ratio * float64(labelDPI) / 72)
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
		if size == minSize {
			lines = clampLines(lines, maxLines, minSize, maxWidth)
			return lines, minSize
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

// fitAllLines is fitLines without the "..." cut: text that does not fit in
// maxLines even at minSize takes as many lines as it needs. What the table
// shows (ingredients and allergens, nutrition, the processor's address) must
// be printed in full; the label grows instead.
func fitAllLines(text string, baseSize, minSize float64, maxLines, maxWidth int) ([]string, float64) {
	lines, size := fitLines(text, baseSize, minSize, maxLines, maxWidth)
	if all := wrapText(text, size, maxWidth); len(all) > len(lines) {
		return all, size
	}
	return lines, size
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
	r.drawImageWithinRectAligned(dst, src, rect, false)
}

// drawImageWithinRectAligned scales src into rect (aspect preserved) and
// centers horizontally; vertically, centers by default or bottom-aligns when
// alignBottom=true.
func (r *LabelRenderer) drawImageWithinRectAligned(dst *image.RGBA, src image.Image, rect image.Rectangle, alignBottom bool) {
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
	var offsetY int
	if alignBottom {
		offsetY = rect.Max.Y - scaledH
	} else {
		offsetY = rect.Min.Y + (maxH-scaledH)/2
	}
	draw.Draw(dst, image.Rect(offsetX, offsetY, offsetX+scaledW, offsetY+scaledH), scaled, image.Point{}, draw.Over)
}

// ── textQRRow — text on the left, QR on the right ──

type textQRRow struct {
	lines    []string
	qrURL    string
	fontSize float64
	// certPath: 空でなければ警告文と QR の間にエゾシカ認証ロゴを描画する (#271)。
	certPath string
	// plaMark: 警告文の下にプラマーク＋「外装」を置く。
	plaMark bool
}

func (t textQRRow) effectiveFontSize() float64 {
	if t.fontSize > 0 {
		return t.fontSize
	}
	return fontSizeBody
}

func (t textQRRow) qrSizePx() int {
	// 認証ロゴを併置する行では、警告文(3 行)+ロゴ+QR を 1 行に納めるため
	// QR を控えめにする (#271)。ロゴを描かないラベルでは従来の大きさ
	// (62mm 幅で約 23mm 角) を維持し、読み取り性を落とさない。
	if strings.TrimSpace(t.certPath) != "" {
		return contentWidth * 33 / 100
	}
	return contentWidth * 40 / 100
}

// plaMarkAbovePt is the space between the warning and the プラ badge.
const plaMarkAbovePt = 2.0

func (t textQRRow) plaMarkHeight() int {
	if !t.plaMark {
		return 0
	}
	return pt(plaMarkAbovePt) + pt(plaMarkPt)
}

func (t textQRRow) height() int {
	fs := t.effectiveFontSize()
	lh := lineHeight(fs)
	textH := lh*len(t.lines) + t.plaMarkHeight()
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
	qs := t.qrSizePx()
	showCert := strings.TrimSpace(t.certPath) != ""

	// レイアウト: [警告文(左)] [認証ロゴ(中央, optional)] [QR(右)]。
	// 認証ロゴは QR と同サイズの正方形スロットに収める。
	var certSize, textWidth int
	if showCert {
		certSize = qs
		textWidth = contentWidth - qs - certSize - 2*imageSlotGap
	} else {
		textWidth = contentWidth - qs - imageSlotGap
	}
	if textWidth < 1 {
		textWidth = 1
	}

	textTotalH := lh*len(t.lines) + t.plaMarkHeight()
	ty := y + (rowHeight-textTotalH)/2
	for _, line := range t.lines {
		drawStringFitWidth(img, face, line, contentLeft, baselineInSlot(face, ty, lh), textWidth)
		ty += lh
	}
	if t.plaMark {
		r.drawPlaBadge(img, contentLeft, ty+pt(plaMarkAbovePt))
	}

	if showCert {
		if certImg, err := r.loadAssetImage(strings.TrimSpace(t.certPath)); err == nil && certImg != nil {
			certX := contentLeft + textWidth + imageSlotGap
			certY := y + (rowHeight-certSize)/2
			rect := image.Rect(certX, certY, certX+certSize, certY+certSize)
			r.drawImageWithinRect(img, certImg, rect)
		}
	}

	if strings.TrimSpace(t.qrURL) != "" {
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
		drawStringFitWidth(img, face, text, left, baselineInSlot(face, ty, lh), textWidth)
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

	// tmp and scaled stay transparent so only the glyphs reach img. The box
	// is taller than a line (ascent + descent); filled white, it erased the
	// bottom of the line above and the table rule it overlapped.
	tmp := image.NewRGBA(image.Rect(0, 0, width, height))
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
