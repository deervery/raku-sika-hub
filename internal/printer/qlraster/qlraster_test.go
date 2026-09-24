package qlraster

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func loadPNG(t *testing.T, name string) image.Image {
	t.Helper()
	f, err := os.Open(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

// The goldens are brother_ql's output for the same images (testdata/gen_golden.py).
// brother_ql is known to drive QL-820NWB correctly, so byte equality here means
// the printer receives exactly what a working implementation would send.
func TestEncode_MatchesBrotherQL(t *testing.T) {
	cases := []struct {
		png, golden string
		cut         bool
	}{
		{"traceable_732.png", "traceable_732.bin", true}, // real hub render: crop, RGB luma, anti-aliasing
		{"edges_696.png", "edges_696.bin", true},         // 1-dot edges: placement and mirroring
		{"gradient_696.png", "gradient_696.bin", true},   // every gray level: the ink threshold
		{"blank_696.png", "blank_696.bin", true},         // no ink at all
		{"edges_696.png", "edges_696_nocut.bin", false},  // header without cutting
	}
	for _, tc := range cases {
		t.Run(tc.golden, func(t *testing.T) {
			want, err := os.ReadFile(filepath.Join("testdata", tc.golden))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Encode(loadPNG(t, tc.png), Continuous62, Options{Cut: tc.cut})
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				i := 0
				for i < len(got) && i < len(want) && got[i] == want[i] {
					i++
				}
				t.Fatalf("differs from brother_ql at byte %d (got %d bytes, want %d)", i, len(got), len(want))
			}
		})
	}
}

// Copies: brother_ql initializes once and repeats information, raster and
// print per page. Re-initializing between pages could drop a label that is
// still in the printer's buffer.
func TestEncodePages_MatchesBrotherQLForCopies(t *testing.T) {
	want, err := os.ReadFile(filepath.Join("testdata", "edges_696_x2.bin"))
	if err != nil {
		t.Fatal(err)
	}
	img := loadPNG(t, "edges_696.png")
	got, err := EncodePages([]image.Image{img, img}, Continuous62, Options{Cut: true})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("differs from brother_ql (got %d bytes, want %d)", len(got), len(want))
	}
	if n := bytes.Count(got, []byte{0x1B, 0x40}); n != 1 {
		t.Fatalf("printer initialized %d times, want once", n)
	}
}

func TestEncodePages_RejectsNoPages(t *testing.T) {
	if _, err := EncodePages(nil, Continuous62, Options{}); err == nil {
		t.Fatal("expected an error for an empty job")
	}
}

// The office failure: ptouch declared 58 mm for a 62 mm roll and the printer
// stopped with 「ロール種類の不一致」. Whatever the label height, the declared
// width must be the roll's.
func TestEncode_DeclaresRollWidthForEveryHeight(t *testing.T) {
	for _, h := range []int{150, 218, 290, 547, 622, 822, 1200} {
		img := image.NewGray(image.Rect(0, 0, 732, h))
		got, err := Encode(img, Continuous62, Options{Cut: true})
		if err != nil {
			t.Fatal(err)
		}
		i := bytes.Index(got, []byte{0x1B, 0x69, 0x7A})
		if i < 0 {
			t.Fatalf("h=%d: no print information command", h)
		}
		pi := got[i+3 : i+13]
		if pi[1] != 0x0A || pi[2] != 62 || pi[3] != 0 {
			t.Errorf("h=%d: declared type=0x%02x width=%d length=%d, want continuous 62mm", h, pi[1], pi[2], pi[3])
		}
		if pi[0]&0x04 == 0 {
			t.Errorf("h=%d: width not marked valid (flags 0x%02x)", h, pi[0])
		}
		if rows := binary.LittleEndian.Uint32(pi[4:8]); int(rows) != h {
			t.Errorf("h=%d: declared %d rows", h, rows)
		}
	}
}

// Independent of brother_ql: decoding the stream must give back the input,
// dot for dot, at its original size. This is the property ptouch broke.
func TestEncode_RoundTripsAtOriginalSize(t *testing.T) {
	src := loadPNG(t, "traceable_732.png")
	out, err := Encode(src, Continuous62, Options{Cut: true})
	if err != nil {
		t.Fatal(err)
	}
	rows := decodeRows(t, out)
	if len(rows) != src.Bounds().Dy() {
		t.Fatalf("got %d rows, want %d", len(rows), src.Bounds().Dy())
	}
	m := Continuous62
	left := (m.DotsTotal - m.DotsPrintable) / 2
	pad := headDots - m.DotsPrintable - m.OffsetRight
	mismatch := 0
	for y, row := range rows {
		for x := 0; x < m.DotsPrintable; x++ {
			hx := headDots - 1 - (pad + x)
			got := row[hx/8]&(0x80>>(hx%8)) != 0
			if got != isInk(src.At(left+x, y)) {
				mismatch++
			}
		}
	}
	if mismatch != 0 {
		t.Fatalf("%d dots differ from the rendered label", mismatch)
	}
}

func TestEncode_RejectsWidthsItWouldHaveToResize(t *testing.T) {
	for _, w := range []int{700, 720, 1200} {
		if _, err := Encode(image.NewGray(image.Rect(0, 0, w, 300)), Continuous62, Options{}); err == nil {
			t.Errorf("width %d: expected an error instead of a silent resize", w)
		}
	}
}

func TestEncode_RejectsLabelsTooShortToFeed(t *testing.T) {
	if _, err := Encode(image.NewGray(image.Rect(0, 0, 696, MinRows-1)), Continuous62, Options{}); err == nil {
		t.Fatal("expected an error for a label shorter than the printer can feed")
	}
}

func TestIsInk_Threshold(t *testing.T) {
	// brother_ql threshold=50: luma 128 prints, 129 does not.
	if !isInk(color.Gray{Y: 128}) {
		t.Error("luma 128 should print")
	}
	if isInk(color.Gray{Y: 129}) {
		t.Error("luma 129 should not print")
	}
	if !isInk(color.RGBA{0, 0, 0, 255}) || isInk(color.RGBA{255, 255, 255, 255}) {
		t.Error("black must print and white must not")
	}
}

// decodeRows pulls the raster rows back out of an encoded stream.
func decodeRows(t *testing.T, data []byte) [][]byte {
	t.Helper()
	i := bytes.Index(data, []byte{0x1B, 0x69, 0x64}) // margins: last command before rows
	if i < 0 {
		t.Fatal("no margin command")
	}
	i += 5
	var rows [][]byte
	for i < len(data) && data[i] == 0x67 {
		n := int(data[i+2])
		rows = append(rows, data[i+3:i+3+n])
		i += 3 + n
	}
	if i >= len(data) || data[i] != 0x1A {
		t.Fatalf("stream does not end with a print command at byte %d", i)
	}
	return rows
}
