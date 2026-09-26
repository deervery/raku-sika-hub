package printer

import "math"

// ipp-usb 経路（IPP Everywhere の PPD + fit-to-page）で CUPS がラベルを縮める率。
// 用紙 Custom.62xHmm（H はラベル高さの mm 切り捨て）の印字可能域に、幅と高さの
// 両方が収まるよう縮める。office の QL-820NWB で CUPS が送るデータを捕まえて
// 実測した値（2026-09-26）:
//
//	ラベル長  44mm → 85%   51mm → 87%   60mm → 89%   76mm 以上 → 90%（幅で決まる）
//
// 幅は 685 ドットの絵柄が 617 ドットに、高さは用紙長から上下計 約 6.1mm を
// 引いた長さに収まる。
const (
	ippWidthScale    = 617.0 / 685.0
	ippVerticalMarMM = 6.1
)

// ippFitScale is the ratio CUPS prints a label heightPx dots long at on an
// ipp-usb queue.
func ippFitScale(heightPx int) float64 {
	if heightPx <= 0 {
		return ippWidthScale
	}
	mediaMM := math.Floor(float64(heightPx) * 25.4 / labelDPI)
	printable := (mediaMM - ippVerticalMarMM) / 25.4 * labelDPI
	return math.Min(ippWidthScale, printable/float64(heightPx))
}
