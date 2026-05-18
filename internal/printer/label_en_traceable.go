package printer

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// EN bilingual traceable label (issue #271).
//
// Layout: 101mm × 62mm landscape PNG, 2 columns side by side.
//
// Left column (8 rows, bilingual captions and values):
//   - Name of Product / 製品名
//   - Best-by Date / 賞味期限
//   - ID Number / 個体識別番号
//   - Preservation / 保存方法
//   - Net Weight / 正味重量
//   - Species of Origin / 品種
//   - Country of Origin / 原産地
//   - Metal Check / 金属検査
//
// Right column:
//   - Top: 2-row facility table (Processing Plant, Address)
//   - Middle: bilingual warning text
//   - Bottom: certification mark logo + Hokkaido HACCP logo + QR code

const (
	// Landscape canvas dimensions: 101mm × 62mm at 300 DPI.
	enLandWidthMM  = 101.0
	enLandHeightMM = 62.0

	enLandMarginPx       = 12
	enLandColumnGapPx    = 8
	enLandFontBody       = 8.5
	enLandFontMin        = 6.0
	enLandFontJaScale    = 0.85
	enLandFontWarning    = 10.0
	enLandFontWarningJa  = 8.5
	enLandTablePaddingPx = 4
	enLandLineGap        = 0.98

	// Default fallback values for traceable_deer.
	defaultSpeciesEnDeer = "Cervus Nippon"
	defaultSpeciesJaDeer = "日本鹿"
	defaultSpeciesEnBear = "Ursus thibetanus"
	defaultSpeciesJaBear = "ツキノワグマ"
	defaultSpeciesEnBoar = "Sus scrofa"
	defaultSpeciesJaBoar = "イノシシ"
	defaultCountryEn     = "Product of Japan"
	defaultCountryJa     = "日本"
)

var (
	enLandWidthPx  = int(math.Round(enLandWidthMM / 25.4 * labelDPI))
	enLandHeightPx = int(math.Round(enLandHeightMM / 25.4 * labelDPI))
)

// fitLinesBilingual wraps an "EN/JA" combined string. It uses width-aware
// estimation that treats non-ASCII (CJK) characters as roughly 1.7x wider than
// ASCII, since they render closer to em-width. It prefers to break:
//   1. at "/" boundaries (after the slash)
//   2. at whitespace (between words)
//   3. between any two CJK characters
//   4. mid-word as a last resort
func fitLinesBilingual(text string, baseSize, minSize float64, maxLines, maxWidth int) ([]string, float64) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []string{""}, baseSize
	}
	size := baseSize
	for size >= minSize {
		lines := wrapBilingual(trimmed, size, maxWidth)
		if maxLines <= 0 || len(lines) <= maxLines {
			return lines, size
		}
		if size == minSize {
			return clampLines(lines, maxLines, minSize, maxWidth), minSize
		}
		size -= 0.5
		if size < minSize {
			size = minSize
		}
	}
	lines := wrapBilingual(trimmed, minSize, maxWidth)
	return clampLines(lines, maxLines, minSize, maxWidth), minSize
}

// asciiCharPx / cjkCharPx estimate the rendered width per character class.
func asciiCharPx(fontSize float64) float64 { return fontSize * float64(labelDPI) / 72 * 0.55 }
func cjkCharPx(fontSize float64) float64   { return fontSize * float64(labelDPI) / 72 * 0.95 }

// estimateRunePx returns approximate width in pixels for a single rune.
func estimateRunePx(rune_ rune, fontSize float64) float64 {
	if rune_ > 0x7F {
		return cjkCharPx(fontSize)
	}
	return asciiCharPx(fontSize)
}

func sumRunesPx(runes []rune, fontSize float64) float64 {
	total := 0.0
	for _, r := range runes {
		total += estimateRunePx(r, fontSize)
	}
	return total
}

// wrapBilingual tokenises the text and greedily packs tokens into lines that
// fit within maxWidth, never splitting an ASCII word mid-character. Allowed
// break points are between tokens (space, "/", CJK char). If a single token is
// longer than maxWidth, it's placed alone on its own line (overflowing) — the
// caller is expected to retry with a smaller font.
//
// Returns at least one line.
func wrapBilingual(text string, fontSize float64, maxWidth int) []string {
	maxPx := float64(maxWidth)
	if maxPx <= 0 {
		return []string{text}
	}
	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		tokens := tokenizeBilingual(paragraph)
		var current []string
		currentW := 0.0
		for _, tok := range tokens {
			tokW := sumRunesPx([]rune(tok), fontSize)
			if tok == " " {
				if len(current) == 0 {
					continue // skip leading whitespace
				}
				if currentW+tokW > maxPx {
					lines = append(lines, strings.Join(current, ""))
					current = nil
					currentW = 0
					continue
				}
				current = append(current, tok)
				currentW += tokW
				continue
			}
			if currentW+tokW > maxPx && len(current) > 0 {
				lines = append(lines, strings.TrimRight(strings.Join(current, ""), " "))
				current = []string{tok}
				currentW = tokW
				continue
			}
			current = append(current, tok)
			currentW += tokW
		}
		if len(current) > 0 {
			lines = append(lines, strings.TrimRight(strings.Join(current, ""), " "))
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// tokenizeBilingual splits a bilingual string into atomic tokens:
//   - Each whitespace is its own token (" ").
//   - Each "/" is appended to the preceding token so that it stays attached
//     to the EN side when wrapping ("Preservation/" rather than "Preservation"
//     followed by "/保存方法"). This matches p-touch convention.
//   - Runs of CJK characters are kept together as a single token (so short JA
//     words like "日本" don't get split between characters).
//   - Runs of ASCII non-whitespace, non-"/" are kept together as a word token.
//
// If a CJK run is too long to fit on one line, the caller is expected to apply
// fallback character-level wrapping for that line.
func tokenizeBilingual(text string) []string {
	var tokens []string
	var ascii []rune
	var cjk []rune
	flushAscii := func() {
		if len(ascii) > 0 {
			tokens = append(tokens, string(ascii))
			ascii = ascii[:0]
		}
	}
	flushCJK := func() {
		if len(cjk) > 0 {
			tokens = append(tokens, string(cjk))
			cjk = cjk[:0]
		}
	}
	appendSlash := func() {
		// Attach "/" to the immediately preceding non-whitespace token so the
		// slash never starts a new line. If the previous token was a space or
		// the string starts with "/", emit it standalone.
		if len(ascii) > 0 {
			ascii = append(ascii, '/')
			return
		}
		if len(cjk) > 0 {
			cjk = append(cjk, '/')
			return
		}
		if len(tokens) > 0 && tokens[len(tokens)-1] != " " {
			tokens[len(tokens)-1] = tokens[len(tokens)-1] + "/"
			return
		}
		tokens = append(tokens, "/")
	}
	for _, r := range text {
		switch {
		case r == ' ':
			flushAscii()
			flushCJK()
			tokens = append(tokens, " ")
		case r == '/':
			appendSlash()
		case r > 0x7F:
			flushAscii()
			cjk = append(cjk, r)
		default:
			flushCJK()
			ascii = append(ascii, r)
		}
	}
	flushAscii()
	flushCJK()
	return tokens
}

// fitLinesWordAware is the EN-text variant of fitLines that prefers word
// boundaries when wrapping. Falls back to character wrapping only if a single
// word doesn't fit in the cell width.
func fitLinesWordAware(text string, baseSize, minSize float64, maxLines, maxWidth int) ([]string, float64) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []string{""}, baseSize
	}
	size := baseSize
	for size >= minSize {
		lines := wrapWords(trimmed, size, maxWidth)
		if maxLines <= 0 || len(lines) <= maxLines {
			return lines, size
		}
		if size == minSize {
			return clampLines(lines, maxLines, minSize, maxWidth), minSize
		}
		size -= 0.5
		if size < minSize {
			size = minSize
		}
	}
	lines := wrapWords(trimmed, minSize, maxWidth)
	return clampLines(lines, maxLines, minSize, maxWidth), minSize
}

func wrapWords(text string, fontSize float64, maxWidth int) []string {
	charWidth := fontSize * float64(labelDPI) / 72 * 0.55
	if charWidth <= 0 {
		charWidth = 1
	}
	maxChars := int(float64(maxWidth) / charWidth)
	if maxChars < 1 {
		maxChars = 1
	}

	var lines []string
	for _, paragraph := range strings.Split(text, "\n") {
		words := strings.Fields(paragraph)
		if len(words) == 0 {
			lines = append(lines, "")
			continue
		}
		current := ""
		for _, word := range words {
			// If word itself is longer than the line, fall back to char-split.
			if len([]rune(word)) > maxChars {
				if current != "" {
					lines = append(lines, current)
					current = ""
				}
				runes := []rune(word)
				for len(runes) > 0 {
					end := maxChars
					if end > len(runes) {
						end = len(runes)
					}
					lines = append(lines, string(runes[:end]))
					runes = runes[end:]
				}
				continue
			}
			candidate := word
			if current != "" {
				candidate = current + " " + word
			}
			if len([]rune(candidate)) <= maxChars {
				current = candidate
			} else {
				lines = append(lines, current)
				current = word
			}
		}
		if current != "" {
			lines = append(lines, current)
		}
	}
	if len(lines) == 0 {
		lines = []string{""}
	}
	return lines
}

// isENBilingualTraceable reports whether the data should be rendered with the
// bilingual landscape EN traceable layout.
func isENBilingualTraceable(data LabelData) bool {
	if !strings.EqualFold(strings.TrimSpace(data.Locale), "en") {
		return false
	}
	return isTraceableTemplate(data.Template)
}

type enLandRow struct {
	labelEn string
	labelJa string
	valueEn string
	valueJa string // optional; empty for single-language values
}

// renderENTraceable renders the bilingual landscape EN traceable label.
func (r *LabelRenderer) renderENTraceable(data LabelData) (RenderResult, error) {
	img := image.NewRGBA(image.Rect(0, 0, enLandWidthPx, enLandHeightPx))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	contentLeftX := enLandMarginPx
	contentTopY := enLandMarginPx
	contentRightX := enLandWidthPx - enLandMarginPx
	contentBottomY := enLandHeightPx - enLandMarginPx
	contentW := contentRightX - contentLeftX
	contentH := contentBottomY - contentTopY

	// Split content into left and right columns (52% : 48%).
	leftW := (contentW - enLandColumnGapPx) * 52 / 100
	rightX := contentLeftX + leftW + enLandColumnGapPx

	r.drawENLeftColumn(img, data, contentLeftX, contentTopY, leftW, contentH)
	r.drawENRightColumn(img, data, rightX, contentTopY, contentRightX-rightX, contentH)

	tmpFile, err := os.CreateTemp("", "label-*.png")
	if err != nil {
		return RenderResult{}, err
	}
	defer tmpFile.Close()
	if err := png.Encode(tmpFile, img); err != nil {
		os.Remove(tmpFile.Name())
		return RenderResult{}, err
	}

	return RenderResult{
		Path:     tmpFile.Name(),
		WidthMM:  int(enLandWidthMM),
		HeightMM: int(enLandHeightMM),
	}, nil
}

// buildENLeftRows builds the left-column 8 bilingual rows.
func buildENLeftRows(data LabelData) []enLandRow {
	trim := strings.TrimSpace
	speciesEn, speciesJa := resolveSpeciesOfOrigin(data)
	countryEn, countryJa := resolveCountryOfOrigin(data)

	return []enLandRow{
		{labelEn: "Name of Product", labelJa: "製品名", valueEn: trim(data.ProductName), valueJa: trim(data.ProductNameJa)},
		// Best-by Date: per p-touch convention, JA date representation is omitted
		// from the value (Japanese readers can parse "1 May 2028"). Provide JA
		// only if explicitly set, otherwise show EN alone.
		{labelEn: "Best-by Date", labelJa: "賞味期限", valueEn: trim(data.DeadlineDate)},
		{labelEn: "ID Number", labelJa: "個体識別番号", valueEn: trim(data.IndividualNumber)},
		{labelEn: "Preservation", labelJa: "保存方法", valueEn: trim(data.StorageTemperature), valueJa: trim(data.StorageTemperatureJa)},
		// Net Weight: quantity (e.g. "0.00kg") doesn't have a JA variant.
		{labelEn: "Net Weight", labelJa: "正味重量", valueEn: trim(data.ProductQuantity)},
		{labelEn: "Species of Origin", labelJa: "品種", valueEn: speciesEn, valueJa: speciesJa},
		{labelEn: "Country of Origin", labelJa: "原産地", valueEn: countryEn, valueJa: countryJa},
		{labelEn: "Metal Check", labelJa: "金属検査", valueEn: "Inspected", valueJa: "検査済み"},
	}
}

func resolveSpeciesOfOrigin(data LabelData) (string, string) {
	en := strings.TrimSpace(data.SpeciesOfOrigin)
	ja := strings.TrimSpace(data.SpeciesOfOriginJa)
	if en != "" || ja != "" {
		return en, ja
	}
	switch data.Template {
	case "traceable_deer", "traceable":
		return defaultSpeciesEnDeer, defaultSpeciesJaDeer
	case "traceable_bear":
		return defaultSpeciesEnBear, defaultSpeciesJaBear
	case "traceable_boar":
		return defaultSpeciesEnBoar, defaultSpeciesJaBoar
	}
	return "", ""
}

func resolveCountryOfOrigin(data LabelData) (string, string) {
	en := strings.TrimSpace(data.CountryOfOrigin)
	ja := strings.TrimSpace(data.CountryOfOriginJa)
	if en != "" || ja != "" {
		return en, ja
	}
	return defaultCountryEn, defaultCountryJa
}

func (r *LabelRenderer) drawENLeftColumn(img *image.RGBA, data LabelData, x, y, w, h int) {
	rows := buildENLeftRows(data)
	if len(rows) == 0 {
		return
	}

	rowH := h / len(rows)
	labelW := int(float64(w) * 0.35)
	valueW := w - labelW

	border := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	drawHLine(img, x, x+w, y, border)
	drawHLine(img, x, x+w, y+rowH*len(rows), border)
	drawVLine(img, x, y, y+rowH*len(rows), border)
	drawVLine(img, x+labelW, y, y+rowH*len(rows), border)
	drawVLine(img, x+w, y, y+rowH*len(rows), border)

	for i, row := range rows {
		rowY := y + i*rowH
		if i > 0 {
			drawHLine(img, x, x+w, rowY, border)
		}
		r.drawENCellBilingual(img, row.labelEn, row.labelJa, x+enLandTablePaddingPx, rowY, labelW-2*enLandTablePaddingPx, rowH, enLandFontBody)
		r.drawENCellBilingual(img, row.valueEn, row.valueJa, x+labelW+enLandTablePaddingPx, rowY, valueW-2*enLandTablePaddingPx, rowH, enLandFontBody)
	}
}

// drawENCellBilingual renders an "EN/JA" bilingual string in a cell, mimicking
// the p-touch template style where EN and JA are separated by "/" and wrap as
// a single inline string. If JA is empty, only EN is drawn.
func (r *LabelRenderer) drawENCellBilingual(img *image.RGBA, en, ja string, x, y, w, h int, baseFontSize float64) {
	r.drawENCellBilingualBounded(img, en, ja, x, y, w, h, baseFontSize, 2)
}

func (r *LabelRenderer) drawENCellBilingualBounded(img *image.RGBA, en, ja string, x, y, w, h int, baseFontSize float64, maxLines int) {
	en = strings.TrimSpace(en)
	ja = strings.TrimSpace(ja)
	combined := combineENJA(en, ja)
	if combined == "" {
		return
	}

	lines, size := fitLinesBilingual(combined, baseFontSize, enLandFontMin, maxLines, w)
	lineH := lineHeightWithRatio(size, enLandLineGap)
	totalH := lineH * len(lines)
	startY := y + (h-totalH)/2
	if startY < y {
		startY = y
	}

	face := r.makeFace(size)
	defer face.Close()
	curY := startY
	for _, line := range lines {
		baseline := curY + int(size*float64(labelDPI)/72)
		drawString(img, face, line, x, baseline)
		curY += lineH
	}
}

// combineENJA joins EN and JA with "/" if both present; returns either alone.
// Empty strings are dropped.
func combineENJA(en, ja string) string {
	en = strings.TrimSpace(en)
	ja = strings.TrimSpace(ja)
	switch {
	case en != "" && ja != "":
		return en + "/" + ja
	case en != "":
		return en
	case ja != "":
		return ja
	default:
		return ""
	}
}

func (r *LabelRenderer) drawENRightColumn(img *image.RGBA, data LabelData, x, y, w, h int) {
	// Allocate vertical regions to match p-touch proportions more closely:
	//   - Facility table: 45% of right column height (needs space for wrapped address)
	//   - Warning text:   13%
	//   - Logos + QR:     42%
	tableH := h * 45 / 100
	warningH := h * 13 / 100
	logosY := y + tableH + warningH
	logosH := h - tableH - warningH

	r.drawENFacilityTable(img, data, x, y, w, tableH)
	r.drawENWarning(img, x, y+tableH, w, warningH)
	r.drawENLogosAndQR(img, data, x, logosY, w, logosH)
}

func (r *LabelRenderer) drawENFacilityTable(img *image.RGBA, data LabelData, x, y, w, h int) {
	trim := strings.TrimSpace
	plantEn := trim(data.ProcessingPlantName)
	if plantEn == "" {
		plantEn = trim(data.FacilityName)
	}
	if plantEn == "" && trim(data.CompanyBlock) != "" {
		// fallback: first line of CompanyBlock
		plantEn = strings.SplitN(trim(data.CompanyBlock), "\n", 2)[0]
	}
	addressEn := trim(data.Address)
	addressJa := trim(data.AddressJa)
	if addressEn == "" && trim(data.CompanyBlock) != "" {
		// fallback: second line (and below) of CompanyBlock minus phone
		parts := strings.Split(trim(data.CompanyBlock), "\n")
		if len(parts) > 1 {
			var addrLines []string
			for _, p := range parts[1:] {
				if strings.HasPrefix(strings.ToLower(strings.TrimSpace(p)), "tel:") {
					continue
				}
				addrLines = append(addrLines, p)
			}
			addressEn = strings.Join(addrLines, ", ")
		}
	}

	rows := []enLandRow{
		{labelEn: "Processing Plant", labelJa: "製造所名", valueEn: plantEn},
		{labelEn: "Address", labelJa: "住所", valueEn: addressEn, valueJa: addressJa},
	}

	rowH1 := h / 4       // Processing Plant row gets 1/4 (single short line typically)
	rowH2 := h - rowH1   // Address row gets 3/4 (multi-line content)
	rowHs := []int{rowH1, rowH2}

	labelW := int(float64(w) * 0.34)

	border := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	drawHLine(img, x, x+w, y, border)
	drawHLine(img, x, x+w, y+h, border)
	drawVLine(img, x, y, y+h, border)
	drawVLine(img, x+labelW, y, y+h, border)
	drawVLine(img, x+w, y, y+h, border)

	curY := y
	for i, row := range rows {
		rowH := rowHs[i]
		if i > 0 {
			drawHLine(img, x, x+w, curY, border)
		}
		r.drawENCellBilingual(img, row.labelEn, row.labelJa, x+enLandTablePaddingPx, curY, labelW-2*enLandTablePaddingPx, rowH, enLandFontBody)
		r.drawENCellMultiLine(img, row.valueEn, row.valueJa, x+labelW+enLandTablePaddingPx, curY, w-labelW-2*enLandTablePaddingPx, rowH, enLandFontBody)
		curY += rowH
	}
}

// drawENCellMultiLine is like drawENCellBilingual but allows the value to wrap
// across more lines (for the Address row).
func (r *LabelRenderer) drawENCellMultiLine(img *image.RGBA, en, ja string, x, y, w, h int, baseFontSize float64) {
	r.drawENCellBilingualBounded(img, en, ja, x, y, w, h, baseFontSize, 6)
}

func (r *LabelRenderer) drawENWarning(img *image.RGBA, x, y, w, h int) {
	enText := "Please cook thoroughly before consumption."
	jaText := "加熱してお召し上がりください"

	enLines, enSize := fitLinesBilingual(enText, enLandFontWarning, enLandFontMin, 2, w)
	jaLines, jaSize := fitLinesBilingual(jaText, enLandFontWarningJa, enLandFontMin, 2, w)

	enLineH := lineHeightWithRatio(enSize, enLandLineGap)
	jaLineH := lineHeightWithRatio(jaSize, enLandLineGap)
	totalH := enLineH*len(enLines) + jaLineH*len(jaLines)
	startY := y + (h-totalH)/2
	if startY < y {
		startY = y
	}

	faceEn := r.makeFace(enSize)
	defer faceEn.Close()
	curY := startY
	for _, line := range enLines {
		baseline := curY + int(enSize*float64(labelDPI)/72)
		drawString(img, faceEn, line, x, baseline)
		curY += enLineH
	}

	faceJa := r.makeFace(jaSize)
	defer faceJa.Close()
	for _, line := range jaLines {
		baseline := curY + int(jaSize*float64(labelDPI)/72)
		drawString(img, faceJa, line, x, baseline)
		curY += jaLineH
	}
}

func (r *LabelRenderer) drawENLogosAndQR(img *image.RGBA, data LabelData, x, y, w, h int) {
	// 3 equal slots: ninsyo logo (cert), Hokkaido HACCP, QR.
	slotCount := 3
	gap := 6
	slotW := (w - gap*(slotCount-1)) / slotCount
	slotSize := slotW
	if slotSize > h {
		slotSize = h
	}
	slotY := y + (h-slotSize)/2

	// Slot 1: 認証マーク (ninsyo_logo.jpg) — fall back to data.CertificationMarkFile.
	certPath := strings.TrimSpace(data.CertificationMarkFile)
	if certPath == "" {
		certPath = "ninsyo_logo.jpg"
	}
	if !strings.EqualFold(strings.TrimSpace(data.Template), "traceable_bear") {
		if certImg, err := r.loadAssetImage(certPath); err == nil && certImg != nil {
			rect := image.Rect(x, slotY, x+slotSize, slotY+slotSize)
			r.drawImageWithinRect(img, certImg, rect)
		}
	}

	// Slot 2: 北海道HACCP.
	haccpX := x + slotW + gap
	if haccpImg, err := r.loadAssetImage("hokkaido_haccp.png"); err == nil && haccpImg != nil {
		rect := image.Rect(haccpX, slotY, haccpX+slotSize, slotY+slotSize)
		r.drawImageWithinRect(img, haccpImg, rect)
	}

	// Slot 3: QR code.
	qrX := x + 2*(slotW+gap)
	qrURL := strings.TrimSpace(data.QRCode)
	if qrURL != "" {
		qrPng, err := qrcode.Encode(qrURL, qrcode.Medium, slotSize)
		if err == nil {
			qrImg, err := png.Decode(strings.NewReader(string(qrPng)))
			if err == nil {
				rect := image.Rect(qrX, slotY, qrX+slotSize, slotY+slotSize)
				draw.Draw(img, rect, qrImg, image.Point{}, draw.Over)
			}
		}
	}
}
