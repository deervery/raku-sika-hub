// Package qlraster builds the raster byte stream that Brother QL-800 series
// label printers (QL-800 / QL-810W / QL-820NWB) consume directly.
//
// Why this exists: the hub already renders each label pixel-exact at 300 dpi.
// Handing that picture to CUPS drivers lets several layers re-lay it out. On
// office (2026-09-24) the ptouch path shrank labels by 12%, rotated some by
// 90°, and — for most label heights — declared a roll width other than 62 mm,
// which the printer rejects with 「ロール種類の不一致」. Encoding the raster
// here removes every one of those layers: what the hub drew is what prints.
//
// The output is byte-for-byte identical to brother_ql 0.9.4
// (https://github.com/pklaus/brother_ql) for the same image and options; the
// golden tests in this package enforce that. See testdata/gen_golden.py.
package qlraster

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
)

// headDots is the width of one raster row on the QL-800 series: 90 bytes.
const headDots = 720

const rowBytes = headDots / 8

// Media describes a roll the way the printer needs to be told about it.
type Media struct {
	// Name is for logs and errors.
	Name string
	// WidthMM is the roll width declared in the print information command.
	// The printer compares it with the loaded roll and stops with
	// 「ロール種類の不一致」 when they differ.
	WidthMM byte
	// DotsTotal is the full tape width in dots at 300 dpi.
	DotsTotal int
	// DotsPrintable is how many of those dots the head can actually reach.
	DotsPrintable int
	// OffsetRight is the gap, in dots, between the printable area and the
	// right end of the head.
	OffsetRight int
	// FeedMargin is the blank feed before and after the label, in dots.
	FeedMargin uint16
}

// Continuous62 is DK-22205 and compatibles: 62 mm continuous-length tape.
// Every station runs this roll. Values from brother_ql's label "62".
var Continuous62 = Media{
	Name:          "62mm continuous",
	WidthMM:       62,
	DotsTotal:     732,
	DotsPrintable: 696,
	OffsetRight:   12,
	FeedMargin:    35,
}

// Length limits from brother_ql's QL-800 series model entry. The printer
// cannot feed a continuous label shorter than 12.7 mm.
const (
	MinRows = 150
	MaxRows = 11811
)

// Options controls how the label is cut.
type Options struct {
	// Cut cuts the tape after the label.
	Cut bool
}

// inkThreshold decides which gray levels become ink. It reproduces brother_ql
// with threshold=50: a pixel prints when 255-L >= int(0.5*255) = 127, i.e.
// when its luma L <= 128.
const inkThreshold = 127

// Encode turns a rendered label into the byte stream for one label.
//
// img must be either DotsTotal wide (the hub's 62 mm canvas; the centre
// DotsPrintable dots are used) or already DotsPrintable wide. Anything else
// is rejected rather than resized: resizing is exactly the silent distortion
// this package exists to avoid.
func Encode(img image.Image, m Media, opts Options) ([]byte, error) {
	return EncodePages([]image.Image{img}, m, opts)
}

// EncodePages encodes several labels into one job — how copies are printed.
//
// The printer is set up once; each page then carries its own print
// information, raster and print command. The printer is deliberately not
// re-initialized between pages: that could clear a label still in its buffer.
// This mirrors brother_ql's multi-image output byte for byte.
func EncodePages(imgs []image.Image, m Media, opts Options) ([]byte, error) {
	if len(imgs) == 0 {
		return nil, fmt.Errorf("qlraster: no pages")
	}
	var out bytes.Buffer
	out.Write([]byte{0x1B, 0x69, 0x61, 0x01}) // ESC i a: raster mode
	out.Write(make([]byte, 200))              // invalidate
	out.Write([]byte{0x1B, 0x40})             // ESC @: initialize
	out.Write([]byte{0x1B, 0x69, 0x61, 0x01}) // ESC i a: raster mode
	for i, img := range imgs {
		if err := writePage(&out, img, m, opts); err != nil {
			return nil, fmt.Errorf("page %d: %w", i+1, err)
		}
	}
	return out.Bytes(), nil
}

func writePage(out *bytes.Buffer, img image.Image, m Media, opts Options) error {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	var left int
	switch w {
	case m.DotsTotal:
		left = (m.DotsTotal - m.DotsPrintable) / 2
	case m.DotsPrintable:
		left = 0
	default:
		return fmt.Errorf("qlraster: %s needs an image %d or %d dots wide, got %d",
			m.Name, m.DotsTotal, m.DotsPrintable, w)
	}
	if h < MinRows || h > MaxRows {
		return fmt.Errorf("qlraster: label must be %d-%d rows long, got %d", MinRows, MaxRows, h)
	}

	// Where column 0 of the printable area lands on the head, before the
	// left-right mirror the head needs.
	pad := headDots - m.DotsPrintable - m.OffsetRight

	out.Grow(60 + h*(3+rowBytes))
	out.Write([]byte{0x1B, 0x69, 0x53}) // ESC i S: status request

	// ESC i z: print information. Valid: recover | quality | length | width | type.
	out.Write([]byte{0x1B, 0x69, 0x7A, 0xCE, 0x0A, m.WidthMM, 0x00})
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(h))
	out.Write(n[:])
	out.Write([]byte{0x00, 0x00}) // page flag (brother_ql always sends 0), fixed 0

	if opts.Cut {
		out.Write([]byte{0x1B, 0x69, 0x4D, 0x40}) // ESC i M: auto cut
		out.Write([]byte{0x1B, 0x69, 0x41, 0x01}) // ESC i A: cut every label
	}
	var expanded byte
	if opts.Cut {
		expanded |= 1 << 3 // cut at end
	}
	out.Write([]byte{0x1B, 0x69, 0x4B, expanded}) // ESC i K

	var margin [2]byte
	binary.LittleEndian.PutUint16(margin[:], m.FeedMargin)
	out.Write([]byte{0x1B, 0x69, 0x64})
	out.Write(margin[:])

	row := make([]byte, rowBytes)
	for y := 0; y < h; y++ {
		clear(row)
		for x := 0; x < m.DotsPrintable; x++ {
			if !isInk(img.At(b.Min.X+left+x, b.Min.Y+y)) {
				continue
			}
			// The head receives each row mirrored left to right.
			hx := headDots - 1 - (pad + x)
			row[hx/8] |= 0x80 >> (hx % 8)
		}
		out.Write([]byte{0x67, 0x00, rowBytes}) // g: raster row, uncompressed
		out.Write(row)
	}

	out.WriteByte(0x1A) // print with feed
	return nil
}

// isInk applies Pillow's RGB→L conversion and brother_ql's threshold, so the
// same anti-aliased edge pixels land on the same side as brother_ql's.
func isInk(c color.Color) bool {
	var l uint32
	switch v := c.(type) {
	case color.Gray:
		l = uint32(v.Y)
	default:
		n := color.NRGBAModel.Convert(c).(color.NRGBA)
		// Pillow: L = (R*19595 + G*38470 + B*7471 + 0x8000) >> 16
		l = (uint32(n.R)*19595 + uint32(n.G)*38470 + uint32(n.B)*7471 + 0x8000) >> 16
	}
	return 255-l >= inkThreshold
}
