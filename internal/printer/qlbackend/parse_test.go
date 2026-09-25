package qlbackend

import (
	"bytes"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The reply the QL-820NWB on office sent on 2026-09-23: 62 mm continuous
// tape, no error.
func TestParseStatus_OfficeReply(t *testing.T) {
	b, _ := hex.DecodeString("802042344130040000003e0a0000150000000000000000000001000000000000")
	st, ok := ParseStatus(b)
	if !ok {
		t.Fatal("not recognised as a status frame")
	}
	if st.MediaWidth != 62 || st.MediaType != 0x0A || st.Type != TypeReply || len(st.Problems()) != 0 {
		t.Fatalf("status = %+v", st)
	}
	if _, bad := st.MediaMismatch(Media{Known: true, Type: 0x0A, WidthMM: 62}); bad {
		t.Fatal("62 mm continuous must match a 62 mm continuous job")
	}
}

func TestParseStatus_RejectsNonFrames(t *testing.T) {
	for _, b := range [][]byte{nil, make([]byte, 31), make([]byte, 32)} {
		if _, ok := ParseStatus(b); ok {
			t.Fatalf("%x accepted", b)
		}
	}
}

func TestParseJob_Goldens(t *testing.T) {
	cases := []struct {
		file  string
		pages int
	}{
		{"traceable_732.bin", 1},
		{"edges_696_x2.bin", 2},
		{"edges_696_nocut.bin", 1},
		{"blank_696.bin", 1},
	}
	for _, tc := range cases {
		job, err := ParseJob(golden(t, tc.file))
		if err != nil {
			t.Fatalf("%s: %v", tc.file, err)
		}
		if job.Pages != tc.pages || job.Media != (Media{Known: true, Type: 0x0A, WidthMM: 62}) {
			t.Fatalf("%s: job = %+v", tc.file, job)
		}
	}
}

// Raster rows can hold any byte. A row full of 0x1A must not count as a page.
func TestParseJob_RasterBytesAreNotCommands(t *testing.T) {
	data := golden(t, "traceable_732.bin")
	i := bytes.IndexByte(data, 'g')
	row := data[i+3 : i+3+90]
	for j := range row {
		row[j] = 0x1A
	}
	job, err := ParseJob(data)
	if err != nil || job.Pages != 1 {
		t.Fatalf("job = %+v, err = %v", job, err)
	}
}

func TestParseJob_RejectsWhatItCannotFollow(t *testing.T) {
	data := golden(t, "traceable_732.bin")
	cases := map[string][]byte{
		"truncated": data[:len(data)-40],
		"png":       []byte("\x89PNG\r\n\x1a\n"),
		"text":      []byte("テスト印刷\n"),
		"no print":  data[:len(data)-1],
		"empty":     nil,
	}
	for name, b := range cases {
		if _, err := ParseJob(b); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestReadFrames_ResyncsOnJunkAndSplitReads(t *testing.T) {
	p := newFakePrinter()
	f1 := p.frame(TypeReply, 0, 0, 0)
	f2 := p.frame(TypePrintingCompleted, 1, 0, 0)
	stream := append(append([]byte{0x00, 0x80, 0x01}, f1...), f2...)
	r := &chunkReader{data: stream, sizes: []int{2, 5, 30, 1, 100}}
	out := make(chan Status, 4)
	readFrames(r, out)
	var got []byte
	for st := range out {
		got = append(got, st.Type)
	}
	if !bytes.Equal(got, []byte{TypeReply, TypePrintingCompleted}) {
		t.Fatalf("frames = %x", got)
	}
}

type chunkReader struct {
	data  []byte
	sizes []int
}

func (r *chunkReader) Read(b []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, os.ErrClosed
	}
	n := len(r.data)
	if len(r.sizes) > 0 {
		n = min(r.sizes[0], n)
		r.sizes = r.sizes[1:]
	}
	n = copy(b, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

func TestResolveDevice_FindsThePrinterBySerial(t *testing.T) {
	root := t.TempDir()
	dev := filepath.Join(root, "sys/devices/platform/usb3/3-1")
	iface := filepath.Join(dev, "3-1:1.0")
	if err := os.MkdirAll(iface, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(dev, "serial"), []byte("000M5G736596\n"), 0o644)
	node := filepath.Join(root, "sys/class/usbmisc/lp1")
	os.MkdirAll(node, 0o755)
	if err := os.Symlink(iface, filepath.Join(node, "device")); err != nil {
		t.Fatal(err)
	}

	got, err := ResolveDevice(DeviceURI("000M5G736596"), root)
	if err != nil || got != filepath.Join(root, "dev/usb/lp1") {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := ResolveDevice(DeviceURI("OTHER"), root); err == nil || !strings.Contains(err.Error(), "見つかりません") {
		t.Fatalf("unknown serial: %v", err)
	}
	if got, _ := ResolveDevice("rakuql:///dev/usb/lp0", root); got != "/dev/usb/lp0" {
		t.Fatalf("path form: %q", got)
	}
	if _, err := ResolveDevice("usb://Brother/QL-820NWB", root); err == nil {
		t.Fatal("another scheme must be refused")
	}
}

func TestResults_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := Result{JobID: "115", Outcome: OutcomeFailed, Message: "カバーが開いています。", Reasons: []string{"cover-open-error"}, Pages: 1}
	if err := WriteResult(dir, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := ReadResult(dir, "115")
	if err != nil || !ok || got.Outcome != want.Outcome || got.Message != want.Message {
		t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
	}
	if _, ok, _ := ReadResult(dir, "116"); ok {
		t.Fatal("a job without a result must read as missing")
	}
	for _, bad := range []string{"", "../x", "Brother_QL-1", "0"} {
		if err := WriteResult(dir, Result{JobID: bad}); err == nil {
			t.Errorf("job id %q accepted", bad)
		}
	}
}

func TestRun_WritesAFailedResultForDataItCannotPrint(t *testing.T) {
	dir := t.TempDir()
	var stderr bytes.Buffer
	code := run([]string{"42", "u", "t", "1", ""}, strings.NewReader("plain text"), &stderr, DeviceURI("X"), t.TempDir(), dir)
	if code != exitFailed {
		t.Fatalf("exit = %d", code)
	}
	res, ok, _ := ReadResult(dir, "42")
	if !ok || res.Outcome != OutcomeFailed || !strings.Contains(stderr.String(), "ERROR: ") {
		t.Fatalf("result = %+v ok=%v\n%s", res, ok, stderr.String())
	}
}

func TestRun_MissingPrinterFailsTheJob(t *testing.T) {
	openWait = 0
	defer func() { openWait = defaultOpenWait }()
	dir := t.TempDir()
	var stderr bytes.Buffer
	code := run([]string{"43", "u", "t", "1", ""}, bytes.NewReader(golden(t, "traceable_732.bin")), &stderr, DeviceURI("NOPE"), t.TempDir(), dir)
	res, _, _ := ReadResult(dir, "43")
	if code != exitFailed || !strings.Contains(res.Message, "USB に見つかりません") {
		t.Fatalf("exit = %d result = %+v", code, res)
	}
}

func TestRun_ListsNoDevices(t *testing.T) {
	if code := run(nil, nil, &bytes.Buffer{}, "", "", ""); code != exitOK {
		t.Fatalf("exit = %d", code)
	}
}

// usblp returns 0 bytes (io.EOF from *os.File) when the printer answers with a
// zero-length packet. That must not end reading: on office it made the backend
// report a lost connection while the labels were printing.
func TestReadFrames_ZeroLengthReadsAreNotTheEnd(t *testing.T) {
	p := newFakePrinter()
	r := &eofThenData{eofs: 3, data: p.frame(TypePrintingCompleted, 1, 0, 0)}
	out := make(chan Status, 2)
	readFrames(r, out)
	st, ok := <-out
	if !ok || st.Type != TypePrintingCompleted {
		t.Fatalf("frame after zero-length reads was lost: %+v %v", st, ok)
	}
}

type eofThenData struct {
	eofs int
	data []byte
}

func (r *eofThenData) Read(b []byte) (int, error) {
	if r.eofs > 0 {
		r.eofs--
		return 0, io.EOF
	}
	if len(r.data) == 0 {
		return 0, os.ErrClosed
	}
	n := copy(b, r.data)
	r.data = r.data[n:]
	return n, nil
}
