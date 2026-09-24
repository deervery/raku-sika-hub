package printer

import (
	"bytes"
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deervery/raku-sika-hub/internal/logging"
	"github.com/deervery/raku-sika-hub/internal/printer/qlraster"
)

// Outputs of `lpoptions -p <queue>` taken from office (2026-09-24).
func TestIsRawQueue(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want bool
	}{
		{
			"raw queue (lpadmin -m raw)",
			"copies=1 device-uri=socket://127.0.0.1:9100 finishings=3 job-cancel-after=10800 printer-info=raw_capture printer-make-and-model='Local Raw Printer' printer-state=3",
			true,
		},
		{
			"ptouch driver queue",
			"copies=1 device-uri=usb://Brother/QL-820NWB?serial=000M5G736596 printer-info=Brother_QL_820NWB_ptouch printer-make-and-model='Brother QL-820NWB Foomatic/ptouch-ql (recommended)' printer-state=3",
			false,
		},
		{
			"ipp-usb queue, unquoted value",
			"device-uri=ipp://Brother%20QL-820NWB%20(USB)._ipp._tcp.local/ printer-info='Brother QL-820NWB (USB)' printer-location printer-make-and-model=Unknown printer-type=16785412",
			false,
		},
		{
			"the words only in printer-info must not count",
			"printer-info='Local Raw Printer' printer-make-and-model='Brother QL-820NWB Foomatic/ptouch-ql (recommended)'",
			false,
		},
		{"no make-and-model at all", "copies=1 printer-state=3", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRawQueue(tc.out); got != tc.want {
				t.Fatalf("isRawQueue = %v, want %v", got, tc.want)
			}
		})
	}
}

// qlraster emits QL-800 series raster. Any other model must be refused
// rather than fed bytes it cannot interpret.
func TestRawSupportedModel(t *testing.T) {
	for _, m := range []string{"QL-800", "QL-820NWB"} {
		if !rawSupportedModel(m) {
			t.Errorf("%s should be supported", m)
		}
	}
	for _, m := range []string{"", "QL-700", "QL-1100", "QL-810W"} {
		if rawSupportedModel(m) {
			t.Errorf("%q should not be supported", m)
		}
	}
	// The queue raku-sika-ops creates for raw printing must still name its model.
	if got := PrinterModel("Brother_QL_820NWB_raw"); got != "QL-820NWB" {
		t.Errorf("PrinterModel(raw queue) = %q, want QL-820NWB", got)
	}
}

// printRaw must hand CUPS exactly what qlraster produced, with -o raw so no
// filter can touch it. lp and lpstat are replaced by stubs on PATH, so this
// runs anywhere without a printer.
func TestPrintRaw_SubmitsQLRasterUntouched(t *testing.T) {
	dir := t.TempDir()
	argsFile := filepath.Join(dir, "lp.args")
	gotFile := filepath.Join(dir, "lp.data")
	writeStub(t, dir, "lp", `#!/bin/sh
printf '%s\n' "$@" > `+argsFile+`
for a in "$@"; do last="$a"; done
cp "$last" `+gotFile+`
echo "request id is Brother_QL_820NWB_raw-7 (1 file(s))"
`)
	// Job already gone from the queue: verifySubmittedJob reports done at once.
	writeStub(t, dir, "lpstat", `#!/bin/sh
case "$*" in *"-l"*) echo "printer Brother_QL_820NWB_raw is idle.";; esac
exit 0
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	logger, err := logging.New(t.TempDir(), logging.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	b := &Brother{logger: logger}
	status := PrinterStatus{SelectedName: "Brother_QL_820NWB_raw", Model: "QL-820NWB", Raw: true}

	png := filepath.Join("qlraster", "testdata", "traceable_732.png")
	res, err := b.printRaw(status, png, 2)
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "done" || res.JobID != "Brother_QL_820NWB_raw-7" {
		t.Fatalf("result = %+v", res)
	}

	args, _ := os.ReadFile(argsFile)
	if !strings.Contains(string(args), "-o\nraw\n") {
		t.Fatalf("lp was not called with -o raw:\n%s", args)
	}
	if !strings.Contains(string(args), "-d\nBrother_QL_820NWB_raw\n") {
		t.Fatalf("lp was not pointed at the raw queue:\n%s", args)
	}

	img, err := decodePNG(png)
	if err != nil {
		t.Fatal(err)
	}
	want, err := qlraster.EncodePages([]image.Image{img, img}, qlraster.Continuous62, qlraster.Options{Cut: true})
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(gotFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("lp received %d bytes that differ from qlraster's %d", len(got), len(want))
	}
}

func TestPrintRaw_RefusesUnknownModel(t *testing.T) {
	logger, err := logging.New(t.TempDir(), logging.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	b := &Brother{logger: logger}
	status := PrinterStatus{SelectedName: "Some_Raw_Queue", Model: "", Raw: true}
	if _, err := b.printRaw(status, filepath.Join("qlraster", "testdata", "traceable_732.png"), 1); err == nil {
		t.Fatal("expected refusal for a queue whose model is unknown")
	}
}

func writeStub(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}
