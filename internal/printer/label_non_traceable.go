package printer

import (
	"image"
	"image/color"
	"image/draw"
	"strings"
)

// 非トレサ精肉ラベル: P-touch テンプレート non_traceable.lbx（シクヌ PC 直結用）
// と同じ横長レイアウト（62mm × 51.3mm）。8 行の表だけで、警告文やロゴは無い。
// 加熱用であることは表の最終行で示す。
//
//	┌──────────────┬───────────────────┐
//	│ 商品名       │ （11pt）          │
//	│ 内容量       │ （11pt）          │
//	│ 消費期限     │ （11pt）          │
//	│ 保存方法     │                   │
//	│ 加工者名     │ 施設名            │
//	│ 加工施設     │ 施設の所在地      │
//	│ 所在地       │                   │
//	│ 金属探知機   │ 検査済み          │
//	│ 加熱用       │ 加熱用            │
//	│ である旨     │                   │
//	└──────────────┴───────────────────┘
const (
	nonTrWidthPt  = 145.4 // テープ送り方向
	nonTrHeightPt = 175.7 // テープ幅 62mm
	nonTrLargePt  = 11.0  // 商品名・内容量・期限（lbx と同じ）
	nonTrPlaGapPt = 4.0   // 表とプラマークの間
)

func isNonTraceableLandscape(data LabelData) bool {
	return data.Template == "non_traceable" || data.Template == "non_traceable_deer"
}

func (r *LabelRenderer) renderNonTraceable(data LabelData) (RenderResult, error) {
	// プラマークは表の右に細い列を足して、下端に置く（ラベルが約 7mm 長くなる）。
	width := pt(nonTrWidthPt)
	var plaW, plaH int
	if data.PlaMark {
		plaW, plaH = r.plaStackSize()
		width += plaW + pt(nonTrPlaGapPt)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, pt(nonTrHeightPt)))
	draw.Draw(img, img.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)
	trim := strings.TrimSpace
	loc := data.Locale
	name, address := processorOf(data)

	table := procTable{xPt: 8.4, yPt: 8.4,
		colsPt: []float64{0, 37.7, 127.9},
		rowsPt: []float64{0, 20.4, 40.7, 60.7, 81.4, 101.4, 121.8, 141.9, 162.3}}
	r.drawProcTable(img, table, [][2]procCell{
		{{text: localizedCaption(loc, "商品名", "Product Name")}, {text: trim(data.ProductName), maxPt: nonTrLargePt}},
		{{text: localizedCaption(loc, "内容量", "Net Weight")}, {text: trim(data.ProductQuantity), maxPt: nonTrLargePt}},
		{{text: deadlineCaption(data, "消費期限", "Use By")}, {text: trim(data.DeadlineDate), maxPt: nonTrLargePt}},
		{{text: localizedCaption(loc, "保存方法", "Storage")}, {text: trim(data.StorageTemperature)}},
		{{text: localizedCaption(loc, "加工者名", "Processor")}, {text: name}},
		{{text: localizedCaption(loc, "加工施設\n所在地", "Address")}, {text: address}},
		{{text: localizedCaption(loc, "金属探知機", "Metal Detection")}, {text: localizedCaption(loc, "検査済み", "Passed")}},
		{{text: localizedCaption(loc, "加熱用\nである旨", "Note")}, {text: localizedCaption(loc, "加熱用", "For cooking only")}},
	})
	if data.PlaMark {
		tableRight := pt(table.xPt + table.colsPt[len(table.colsPt)-1])
		tableBottom := pt(table.yPt + table.rowsPt[len(table.rowsPt)-1])
		r.drawPlaStack(img, tableRight+pt(nonTrPlaGapPt), tableBottom-plaH, plaW)
	}
	return saveLandscape(img)
}

// processorOf returns the 加工者名 and 加工施設所在地 rows: the facility that
// processed the meat, as the lbx prints (施設名 / 施設住所). facilityBlock is
// "name\naddress…"; without it the processor fields, then the company block.
func processorOf(data LabelData) (name, address string) {
	trim := strings.TrimSpace
	split := func(block string) (string, string) {
		first, rest, _ := strings.Cut(trim(block), "\n")
		return trim(first), trim(rest)
	}
	switch {
	case trim(data.FacilityBlock) != "":
		return split(data.FacilityBlock)
	case trim(data.ProcessorName) != "":
		return trim(data.ProcessorName), trim(data.ProcessorLocation)
	default:
		return split(data.CompanyBlock)
	}
}
