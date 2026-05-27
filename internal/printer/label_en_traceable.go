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
	// Landscape canvas dimensions (#271): 102.5mm × 62mm at 300 DPI.
	// 101 → 102.5 で +1.5 mm (≈ 18 px) 横拡張、その分を左テーブル label 列に振る。
	// Brother QL-820 の連続テープは 62mm 幅 × 任意長なので W 拡張は印刷可能。
	enLandWidthMM  = 102.5
	enLandHeightMM = 62.0

	enLandMarginPx       = 8
	enLandColumnGapPx    = 12
	enLandFontBody = 8.0
	enLandFontMin  = 5.5
	enLandFontJaScale = 0.85
	// #271: user 要望で警告文 (Please cook thoroughly... / 加熱してお召し上がりください) も
	// 本文と同じ 8pt に統一。
	enLandFontWarning   = 8.0
	enLandFontWarningJa = 8.0
	enLandTablePaddingPx = 5
	enLandLineGap        = 1.05
	// Border thickness in pixels for table grid lines (drawn as multiple
	// parallel pixel lines to match p-touch's thicker borders).
	enLandBorderPx = 2

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
	// #271: ユーザ要望でフォントサイズ統一 — 動的縮小を止めて baseSize 固定。
	// baseSize で wrap し、maxLines を超えても clamp で対応 (font 縮小しない)。
	lines := wrapBilingual(trimmed, baseSize, maxWidth)
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines, baseSize
	}
	return clampLines(lines, maxLines, baseSize, maxWidth), baseSize
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

// drawThickHLine draws an `enLandBorderPx`-pixel-thick horizontal line.
func drawThickHLine(img *image.RGBA, x1, x2, y int, c color.Color) {
	for dy := 0; dy < enLandBorderPx; dy++ {
		drawHLine(img, x1, x2, y+dy, c)
	}
}

// drawThickVLine draws an `enLandBorderPx`-pixel-thick vertical line.
func drawThickVLine(img *image.RGBA, x, y1, y2 int, c color.Color) {
	for dx := 0; dx < enLandBorderPx; dx++ {
		drawVLine(img, x+dx, y1, y2, c)
	}
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

	// Split content into left and right columns (50% : 50%) — left は base 50% に対し
	// canvas 拡張分 (+18 px = 1 文字分) を leftW に全振り (labelExtraPx)。これにより
	// 項目名列だけ 1 文字分広がり、value 列・右セクションは絶対幅を維持できる。
	const labelExtraPx = 18 // 1 文字分 (8pt ASCII) の追加幅、すべて label 列に充当
	leftW := (contentW-enLandColumnGapPx-labelExtraPx)/2 + labelExtraPx
	rightX := contentLeftX + leftW + enLandColumnGapPx

	r.drawENLeftColumn(img, data, contentLeftX, contentTopY, leftW, contentH)
	r.drawENRightColumn(img, data, rightX, contentTopY, contentRightX-rightX, contentH)

	// Brother QL-820NWB は連続テープを 62mm 幅 × 任意長で印刷する物理制約がある。
	// landscape (101mm × 62mm) PNG をそのまま lp に渡すと、ドライバが 62mm 幅に
	// 縮小して印字するため、ラベル全体が小さくなる。
	// 解決: 90° 回転した portrait (62mm × 101mm) PNG にして送ることで、テープを
	// 101mm 長ぶん流して 62mm 幅いっぱいに印字する。
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

	return RenderResult{
		Path: tmpFile.Name(),
		// 物理的に印字される向き: 62mm 幅 × N mm 長 (テープ送り方向)。
		// enLandWidthMM が小数を含む可能性があるため math.Round で四捨五入。
		WidthMM:  int(math.Round(enLandHeightMM)),
		HeightMM: int(math.Round(enLandWidthMM)),
	}, nil
}

// rotate90 returns a new image rotated 90° clockwise.
// The output dimensions are (src.height, src.width).
func rotate90(src *image.RGBA) *image.RGBA {
	b := src.Bounds()
	w := b.Dx()
	h := b.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, h, w))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dst.Set(h-1-y, x, src.At(x, y))
		}
	}
	return dst
}

// buildENLeftRows builds the left-column 8 bilingual rows.
func buildENLeftRows(data LabelData) []enLandRow {
	trim := strings.TrimSpace
	speciesEn, speciesJa := resolveSpeciesOfOrigin(data)
	countryEn, countryJa := resolveCountryOfOrigin(data)

	return []enLandRow{
		// productName が lite 側で bilingual ("EN\n/JA") に組み立てられて送られてくる
		// 後方互換ケースに備え、EN 部分のみ抽出。productNameJa が別途渡されていれば
		// それを JA 側として使い、"EN/JA/JA" の重複表示を防ぐ。
		{labelEn: "Name of Product", labelJa: "製品名", valueEn: extractENOnly(trim(data.ProductName)), valueJa: trim(data.ProductNameJa)},
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
	// labelW を 0.42 比率 (#271): leftW に追加した +18 px (canvas 拡張分) を
	// label 列に振る。0.40 比率 (base) → 0.42 で leftW の +18 px がほぼ label に
	// 充当され、value 列の絶対幅は維持される。
	labelW := int(float64(w) * 0.42)
	valueW := w - labelW
	tableH := rowH * len(rows)

	border := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	drawThickHLine(img, x, x+w, y, border)
	drawThickHLine(img, x, x+w, y+tableH, border)
	drawThickVLine(img, x, y, y+tableH, border)
	drawThickVLine(img, x+labelW, y, y+tableH, border)
	drawThickVLine(img, x+w-enLandBorderPx, y, y+tableH, border)

	for i, row := range rows {
		rowY := y + i*rowH
		if i > 0 {
			drawThickHLine(img, x, x+w, rowY, border)
		}
		// 8pt 統一: baseSize=minSize=enLandFontBody で動的縮小を回避し、maxLines を
		// label 3 / value 4 まで許可 (Preservation の "Keep Frozen below -18C/要冷凍"
		// のような長い保存方法表記が 2 行で切れないように)。
		r.drawENCellBilingualBounded(img, row.labelEn, row.labelJa, x+enLandTablePaddingPx, rowY, labelW-2*enLandTablePaddingPx, rowH, enLandFontBody, 3)
		r.drawENCellBilingualBounded(img, row.valueEn, row.valueJa, x+labelW+enLandTablePaddingPx, rowY, valueW-2*enLandTablePaddingPx, rowH, enLandFontBody, 4)
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

// extractENOnly returns the EN-only portion of a bilingual "EN\n/JA" or "EN/JA"
// string. lite (raku-sika-lite) historically composes productName as
// "Shoulder\n/ウデ" when sending to bPAC, and now also sends productNameJa
// separately. Treating the combined string verbatim duplicates the JA portion.
func extractENOnly(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Drop everything from the first newline onward (lite uses "\n/" as a
	// hint for p-touch label wrapping; the line after is the JA half).
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = s[:idx]
	}
	// Drop trailing "/" if present (some legacy payloads have "EN/" only).
	s = strings.TrimRight(strings.TrimSpace(s), "/")
	// Drop everything from the first "/" onward — but only if what follows
	// contains CJK characters (= JA half). Plain ASCII "/" inside an EN string
	// (e.g. product code "A/B") must be preserved.
	if idx := strings.IndexByte(s, '/'); idx >= 0 {
		tail := s[idx+1:]
		for _, r := range tail {
			if r > 0x7F {
				s = s[:idx]
				break
			}
		}
	}
	return strings.TrimSpace(s)
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
	//   - Warning text:   3% (8pt 1 行で足りる)
	//   - Logos + QR:     残り (画像を +10% さらに大きく取るため warning を圧縮 #271)
	tableH := h*45/100 + 70
	warningH := h * 3 / 100
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

	// #271: Processing Plant 行は label が "Processing Plant/製造所名" の 3 行 wrap に
	// 必要な固定高 (8pt × 3 lines × line-gap = 約 110px) とし、残りを Address 行に
	// 振る。drawENRightColumn が tableH を +70px しているため、その追加分は全部
	// Address row (rowH2) に流れて住所行が 2 行分大きくなる。
	const rowH1Fixed = 110
	rowH1 := rowH1Fixed
	if rowH1 > h/2 {
		rowH1 = h / 2
	}
	rowH2 := h - rowH1
	rowHs := []int{rowH1, rowH2}

	// #271: labelW 比率を 0.32 → 0.40 に拡張。"Processing Plant" が 8pt で 3 行に収まり、
	// かつ value 側にも余裕を持たせる。
	labelW := int(float64(w) * 0.40)

	border := color.RGBA{R: 0, G: 0, B: 0, A: 255}
	drawThickHLine(img, x, x+w, y, border)
	drawThickHLine(img, x, x+w, y+h, border)
	drawThickVLine(img, x, y, y+h, border)
	drawThickVLine(img, x+labelW, y, y+h, border)
	drawThickVLine(img, x+w-enLandBorderPx, y, y+h, border)

	curY := y
	for i, row := range rows {
		rowH := rowHs[i]
		if i > 0 {
			drawThickHLine(img, x, x+w, curY, border)
		}
		// Facility table label cells need up to 3 lines for "Processing Plant/製造所名".
		r.drawENCellBilingualBounded(img, row.labelEn, row.labelJa, x+enLandTablePaddingPx, curY, labelW-2*enLandTablePaddingPx, rowH, enLandFontBody, 3)
		// #271: 製造所名・住所ともに 8pt 統一 (user 要望)。Processing Plant の cell は
		// 元の rowH1 拡張 (3/8) で縦に余裕があるが、フォント自体は 8pt に揃える。
		valueFontSize := enLandFontBody
		valueMaxLines := 6
		r.drawENCellBilingualBounded(img, row.valueEn, row.valueJa, x+labelW+enLandTablePaddingPx, curY, w-labelW-2*enLandTablePaddingPx, rowH, valueFontSize, valueMaxLines)
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
	// 3 slots: ninsyo logo (cert), Hokkaido HACCP (1.5x #271), QR.
	// 比率は ninsyo:haccp:qr = 1 : 1.5 : 1 で配分。
	// #271: user 要望で CSS space-around 相当の配置にする。
	// 端 1 単位 / 要素間 2 単位 / 端 1 単位 = 合計 6 単位の余白を確保する。
	// #271: 余白なしレベルで画像を最大化。画像自体に白縁があるので gap 小さくて OK。
	gap := 3 // ロゴ間 2*gap = 6 px (ごくわずか)
	// space-around: 端=gap, 要素間=2*gap, 端=gap → 合計余白 = 6*gap。
	// 比率変更 (#271): ninsyo:HACCP:QR = 1:1:1 (3 つとも同じ slot 幅)。
	// HACCP は assets 側で trim 済みなので 1x slot でも実ロゴが大きく描画される。
	// cert / QR とも視覚的に同サイズに揃う。
	usableW := w - 6*gap
	unitW := usableW / 3
	haccpW := unitW
	qrW := unitW
	// #271: slot を正方形 (W=H=unitW) にして 3 つの画像を最大サイズで表示する。
	// HACCP は trim 済みで square、QR は元から square、ninsyo もほぼ square なので
	// 正方形 slot にすれば縦余白がほぼ消えて全画像が同じ大きさで並ぶ。
	// canvas 縦余裕 (h - slotH) は slot top 側に出て警告文と画像の間で吸収される。
	slotH := unitW
	if slotH > h {
		slotH = h
	}
	slotY := y + (h - slotH)

	certW := unitW

	// Slot 1: 認証マーク (ninsyo_logo.jpg) — fall back to data.CertificationMarkFile.
	certPath := strings.TrimSpace(data.CertificationMarkFile)
	if certPath == "" {
		certPath = "ninsyo_logo.jpg"
	}
	// space-around 配置: 左端 gap → ninsyo → 2*gap → HACCP → 2*gap → QR → 右端 gap。
	certX := x + gap
	if !strings.EqualFold(strings.TrimSpace(data.Template), "traceable_bear") {
		if certImg, err := r.loadAssetImage(certPath); err == nil && certImg != nil {
			rect := image.Rect(certX, slotY, certX+certW, slotY+slotH)
			r.drawImageWithinRectAligned(img, certImg, rect, true)
		}
	}

	// Slot 2: 北海道HACCP (1.5x 幅 #271).
	haccpX := certX + certW + 2*gap
	if haccpImg, err := r.loadAssetImage("hokkaido_haccp.png"); err == nil && haccpImg != nil {
		rect := image.Rect(haccpX, slotY, haccpX+haccpW, slotY+slotH)
		r.drawImageWithinRectAligned(img, haccpImg, rect, true)
	}

	// Slot 3: QR code (kept square — pick the smaller dimension). Bottom align (#271).
	qrSlotX := haccpX + haccpW + 2*gap
	qrSize := qrW
	if qrSize > slotH {
		qrSize = slotH
	}
	qrX := qrSlotX + (qrW-qrSize)/2
	qrY := slotY + slotH - qrSize // bottom align
	qrURL := strings.TrimSpace(data.QRCode)
	if qrURL != "" {
		qrPng, err := qrcode.Encode(qrURL, qrcode.Medium, qrSize)
		if err == nil {
			qrImg, err := png.Decode(strings.NewReader(string(qrPng)))
			if err == nil {
				rect := image.Rect(qrX, qrY, qrX+qrSize, qrY+qrSize)
				draw.Draw(img, rect, qrImg, image.Point{}, draw.Over)
			}
		}
	}
}
