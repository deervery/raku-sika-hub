package printer

import (
	"bytes"
	_ "embed"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
)

// プラマーク＋「外装」: 施設ごとの設定（lite の施設マスタ plaMark）で表示する
// プラスチック製容器包装の識別表示。テンプレートラベル（P-touch）では
// マーク 12〜14pt の右に「外装」を置いている。
//
// マークはバイナリに埋め込む（リリースは実行ファイルだけで、assets は配らない）。
//
//go:embed marks/pla.png
var plaMarkPNG []byte

const (
	plaMarkPt     = 13.0 // マークの一辺
	plaMarkTextPt = 8.0  // 「外装」
	plaMarkGapPt  = 1.5  // マークと「外装」の間
	plaMarkText   = "外装"
)

// plaMarkMask is the mark as an alpha mask (ink = opaque), trimmed to its
// ink, so drawing it never paints white over what is around it.
var plaMarkMask = sync.OnceValue(func() *image.Alpha {
	src, err := png.Decode(bytes.NewReader(plaMarkPNG))
	if err != nil {
		return nil
	}
	b := src.Bounds()
	ink := image.Rectangle{}
	mask := image.NewAlpha(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			g := color.GrayModel.Convert(src.At(x, y)).(color.Gray).Y
			a := 255 - g
			mask.SetAlpha(x, y, color.Alpha{A: a})
			if a > 64 {
				ink = ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	if ink.Empty() {
		return nil
	}
	return mask.SubImage(ink).(*image.Alpha)
})

// drawPlaMark draws the mark scaled into an s×s square at (x, y).
func drawPlaMark(img *image.RGBA, x, y, s int) {
	m := plaMarkMask()
	if m == nil || s <= 0 {
		return
	}
	scaled := image.NewAlpha(image.Rect(0, 0, s, s))
	xdraw.CatmullRom.Scale(scaled, scaled.Bounds(), m, m.Bounds(), xdraw.Src, nil)
	draw.DrawMask(img, image.Rect(x, y, x+s, y+s), image.Black, image.Point{}, scaled, image.Point{}, draw.Over)
}

// plaBadgeSize is the size of the mark with 「外装」 beside it.
func (r *LabelRenderer) plaBadgeSize() (w, h int) {
	face := r.makeFace(plaMarkTextPt)
	defer face.Close()
	return pt(plaMarkPt) + pt(plaMarkGapPt) + font.MeasureString(face, plaMarkText).Ceil(), pt(plaMarkPt)
}

// drawPlaBadge draws the mark at (x, y) with 「外装」 beside it, centred on
// the mark.
func (r *LabelRenderer) drawPlaBadge(img *image.RGBA, x, y int) {
	s := pt(plaMarkPt)
	drawPlaMark(img, x, y, s)
	face := r.makeFace(plaMarkTextPt)
	defer face.Close()
	drawString(img, face, plaMarkText, x+s+pt(plaMarkGapPt), baselineInSlot(face, y, s))
}

// plaStackSize is the size of the mark with 「外装」 below it, for a narrow
// strip beside a table.
func (r *LabelRenderer) plaStackSize() (w, h int) {
	face := r.makeFace(plaMarkTextPt)
	defer face.Close()
	w = font.MeasureString(face, plaMarkText).Ceil()
	if s := pt(plaMarkPt); s > w {
		w = s
	}
	return w, pt(plaMarkPt) + lineHeight(plaMarkTextPt)
}

// drawPlaStack draws the mark with 「外装」 below it, centred in a column w
// wide starting at x.
func (r *LabelRenderer) drawPlaStack(img *image.RGBA, x, y, w int) {
	s := pt(plaMarkPt)
	drawPlaMark(img, x+(w-s)/2, y, s)
	face := r.makeFace(plaMarkTextPt)
	defer face.Close()
	tw := font.MeasureString(face, plaMarkText).Ceil()
	drawString(img, face, plaMarkText, x+(w-tw)/2, baselineInSlot(face, y+s, lineHeight(plaMarkTextPt)))
}

// plaBadgeRow puts the badge at the right end of its own row, below the
// table (as the pet template does).
type plaBadgeRow struct {
	r *LabelRenderer
}

func (p plaBadgeRow) height() int {
	_, h := p.r.plaBadgeSize()
	return h + pt(3)
}

func (p plaBadgeRow) draw(img *image.RGBA, r *LabelRenderer, y int) int {
	w, _ := r.plaBadgeSize()
	r.drawPlaBadge(img, contentLeft+contentWidth-w, y+pt(3))
	return y + p.height()
}
