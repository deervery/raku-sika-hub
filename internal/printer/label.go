package printer

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
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
	marginPx     = 30
	contentWidth = labelWidthPx - 2*marginPx
)

// Font sizes at 300 DPI.
const (
	fontSizeTitle    = 16 // product name
	fontSizeBody     = 12 // regular fields
	fontSizeSmall    = 10 // sub-fields like nutrition
	fontSizeSep      = 10 // separator text
	lineSpacingRatio = 1.6
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
	rows := r.buildRows(data)

	// Calculate total height.
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

	tmpFile, err := os.CreateTemp("", "label-*.png")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	defer tmpFile.Close()

	if err := png.Encode(tmpFile, img); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("encode png: %w", err)
	}
	return tmpFile.Name(), nil
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
	url  string
	size int // pixels
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
	qrImg, err := png.Decode(strings.NewReader(string(qrPng)))
	if err != nil {
		return y + q.height()
	}

	// Center the QR code.
	x := (labelWidthPx - q.size) / 2
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

// buildRows creates the row list for the given template and data.
func (r *LabelRenderer) buildRows(data LabelData) []row {
	var rows []row

	// Common header for all templates.
	rows = append(rows,
		textRow{value: data.ProductName, fontSize: fontSizeTitle},
		textRow{label: "内容量", value: data.ProductQuantity, fontSize: fontSizeBody},
	)

	switch data.Template {
	case "traceable", "traceable_deer", "traceable_bear":
		rows = append(rows,
			textRow{label: "消費期限", value: data.DeadlineDate, fontSize: fontSizeBody},
			textRow{label: "保存方法", value: data.StorageTemperature, fontSize: fontSizeBody},
			separatorRow{},
			textRow{label: "個体識別番号", value: data.IndividualNumber, fontSize: fontSizeBody},
		)
		if data.CaptureLocation != "" {
			rows = append(rows, textRow{label: "捕獲場所", value: data.CaptureLocation, fontSize: fontSizeBody})
		}
		if data.QRCode != "" {
			rows = append(rows,
				spacerRow{px: 10},
				qrRow{url: data.QRCode, size: 200},
			)
		}
		if data.AttentionText != "" {
			rows = append(rows, multiLineRow{label: "注意", value: data.AttentionText, fontSize: fontSizeSmall})
		}

	case "non_traceable", "non_traceable_deer":
		rows = append(rows,
			textRow{label: "消費期限", value: data.DeadlineDate, fontSize: fontSizeBody},
			textRow{label: "保存方法", value: data.StorageTemperature, fontSize: fontSizeBody},
		)
		if data.AttentionText != "" {
			rows = append(rows, multiLineRow{label: "注意", value: data.AttentionText, fontSize: fontSizeSmall})
		}

	case "processed":
		if data.ProductIngredient != "" {
			rows = append(rows, multiLineRow{label: "原材料名", value: data.ProductIngredient, fontSize: fontSizeSmall})
		}
		rows = append(rows,
			textRow{label: "消費期限", value: data.DeadlineDate, fontSize: fontSizeBody},
			textRow{label: "保存方法", value: data.StorageTemperature, fontSize: fontSizeBody},
		)
		// Nutrition table.
		nutritionLabel := "栄養成分表示"
		if data.NutritionUnit != "" {
			nutritionLabel += "（" + data.NutritionUnit + "）"
		}
		rows = append(rows, separatorRow{text: nutritionLabel})
		if data.CaloriesQuantity != "" {
			rows = append(rows, textRow{label: "エネルギー", value: data.CaloriesQuantity, fontSize: fontSizeSmall})
		}
		if data.ProteinQuantity != "" {
			rows = append(rows, textRow{label: "たんぱく質", value: data.ProteinQuantity, fontSize: fontSizeSmall})
		}
		if data.FatQuantity != "" {
			rows = append(rows, textRow{label: "脂質", value: data.FatQuantity, fontSize: fontSizeSmall})
		}
		if data.CarbohydratesQuantity != "" {
			rows = append(rows, textRow{label: "炭水化物", value: data.CarbohydratesQuantity, fontSize: fontSizeSmall})
		}
		if data.SaltEquivalentQuantity != "" {
			rows = append(rows, textRow{label: "食塩相当量", value: data.SaltEquivalentQuantity, fontSize: fontSizeSmall})
		}
		rows = append(rows, separatorRow{})
		if data.AttentionText != "" {
			rows = append(rows, multiLineRow{label: "注意", value: data.AttentionText, fontSize: fontSizeSmall})
		}

	case "pet":
		if data.ProductIngredient != "" {
			rows = append(rows, multiLineRow{label: "原材料名", value: data.ProductIngredient, fontSize: fontSizeSmall})
		}
		rows = append(rows,
			textRow{label: "消費期限", value: data.DeadlineDate, fontSize: fontSizeBody},
			textRow{label: "保存方法", value: data.StorageTemperature, fontSize: fontSizeBody},
		)
		if data.AttentionText != "" {
			rows = append(rows, multiLineRow{label: "注意", value: data.AttentionText, fontSize: fontSizeSmall})
		}
	}

	return rows
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
