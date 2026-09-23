package printer

import (
	"fmt"
	"net"
	"strings"
	"testing"
)

func TestParseAvailablePrinters(t *testing.T) {
	output := "printer Brother_QL-820NWB is idle. enabled since Thu 01 Jan 1970 00:00:00 AM JST\nprinter Brother_QL_820NWB_USB disabled since Thu 01 Jan 1970 00:00:00 AM JST\n"

	got := parseAvailablePrinters(output)
	if len(got) != 2 {
		t.Fatalf("expected 2 printers, got %d", len(got))
	}
	if got[0] != "Brother_QL-820NWB" {
		t.Fatalf("expected first printer to preserve lpstat order, got %q", got[0])
	}
	if got[1] != "Brother_QL_820NWB_USB" {
		t.Fatalf("expected second printer, got %q", got[1])
	}
}

func TestParseDefaultPrinter(t *testing.T) {
	output := "system default destination: Brother_QL-820NWB\n"
	if got := parseDefaultPrinter(output); got != "Brother_QL-820NWB" {
		t.Fatalf("expected default printer, got %q", got)
	}
}

func TestValidateStatus_ConfiguredMismatch(t *testing.T) {
	status := PrinterStatus{
		ConfiguredName: "Brother_QL-800",
		SelectedName:   "Brother_QL-800",
		DefaultName:    "Brother_QL-820NWB",
		Available:      []string{"Brother_QL-820NWB", "Brother_QL_820NWB_USB"},
		Source:         "configured",
	}

	err := validateStatus(status)
	if err == nil {
		t.Fatal("expected mismatch error")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "PRINTER_NOT_CONFIGURED:") {
		t.Fatalf("expected PRINTER_NOT_CONFIGURED, got %q", msg)
	}
}

func TestValidateStatus_BackendUnavailable(t *testing.T) {
	status := PrinterStatus{
		ConfiguredName: "Brother_QL_820NWB_USB",
		SelectedName:   "Brother_QL_820NWB_USB",
		Available:      []string{"Brother_QL_820NWB_USB"},
		Source:         "configured",
		DeviceURI:      "ipp://localhost:60000/ipp/print",
		BackendReady:   false,
		BackendError:   "connect: connection refused",
	}

	err := validateStatus(status)
	if err == nil {
		t.Fatal("expected backend unavailable error")
	}
	if !strings.HasPrefix(err.Error(), "PRINTER_UNAVAILABLE:") {
		t.Fatalf("expected PRINTER_UNAVAILABLE, got %q", err.Error())
	}
	if status.Ready() {
		t.Fatal("expected status to be not ready")
	}
}

func TestParsePrinterDeviceURI(t *testing.T) {
	output := "device for Brother_QL_820NWB_USB: ipp://localhost:60000/ipp/print\n"
	if got := parsePrinterDeviceURI(output); got != "ipp://localhost:60000/ipp/print" {
		t.Fatalf("expected device uri, got %q", got)
	}
}

func TestCheckPrinterBackend_LocalhostPort(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ready, msg := checkPrinterBackend(fmt.Sprintf("ipp://localhost:%d/ipp/print", ln.Addr().(*net.TCPAddr).Port))
	if !ready || msg != "" {
		t.Fatalf("expected ready backend, ready=%v msg=%q", ready, msg)
	}
}

func TestCheckPrinterBackend_LocalhostPortClosed(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()

	ready, msg := checkPrinterBackend(fmt.Sprintf("ipp://localhost:%d/ipp/print", port))
	if ready || msg == "" {
		t.Fatalf("expected closed backend, ready=%v msg=%q", ready, msg)
	}
}

func TestParseConfiguredPrinterNames(t *testing.T) {
	got := parseConfiguredPrinterNames(" Brother_QL_800_USB, Brother_QL_820NWB_USB ; Brother_QL_800_USB ")
	want := []string{"Brother_QL_800_USB", "Brother_QL_820NWB_USB"}
	if len(got) != len(want) {
		t.Fatalf("expected %d names, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected name[%d]=%q, got %q", i, want[i], got[i])
		}
	}
}

func TestSelectConfiguredPrinter_PicksFirstAvailableCandidate(t *testing.T) {
	configured := []string{"Brother_QL_800_USB", "Brother_QL_820NWB_USB"}
	available := []string{"Brother_QL_820NWB_USB", "Brother_QL_800_USB"}
	if got := selectConfiguredPrinter(configured, available); got != "Brother_QL_800_USB" {
		t.Fatalf("expected first configured available printer, got %q", got)
	}
}

func TestSelectConfiguredPrinter_FallsBackToLaterCandidate(t *testing.T) {
	configured := []string{"Brother_QL_800_USB", "Brother_QL_820NWB_USB"}
	available := []string{"Brother_QL_820NWB_USB"}
	if got := selectConfiguredPrinter(configured, available); got != "Brother_QL_820NWB_USB" {
		t.Fatalf("expected later configured printer, got %q", got)
	}
}

func TestSelectConfiguredPrinter_ReturnsEmptyWhenNoCandidateAvailable(t *testing.T) {
	configured := []string{"Brother_QL_800_USB", "Brother_QL_820NWB_USB"}
	available := []string{"Other_Printer"}
	if got := selectConfiguredPrinter(configured, available); got != "" {
		t.Fatalf("expected empty selection, got %q", got)
	}
}

func TestParseMediaOptions(t *testing.T) {
	output := strings.Join([]string{
		"PageSize/Media Size: *w62h100/62 mm x 100 mm w62h29/62 mm x 29 mm roll-62/62 mm Continuous",
		"media/Media Tracking: *continuous die-cut",
		"",
	}, "\n")

	got := parseMediaOptions(output)
	want := []string{"roll-62", "w62h100", "w62h29"}
	if len(got) != len(want) {
		t.Fatalf("expected %d media options, got %d: %v", len(want), len(got), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("expected media[%d]=%q, got %q", i, want[i], got[i])
		}
	}
}

func TestSelectPreferredMediaOption_PrefersContinuous62mm(t *testing.T) {
	options := []string{"w62h100", "roll-62", "w62h29"}
	if got := selectPreferredMediaOption(options); got != "roll-62" {
		t.Fatalf("expected roll-62, got %q", got)
	}
}

func TestSelectPreferredMediaOption_FallsBackTo62mmOption(t *testing.T) {
	options := []string{"w62h100", "w62h29"}
	if got := selectPreferredMediaOption(options); got != "w62h100" {
		t.Fatalf("expected w62h100, got %q", got)
	}
}

func TestSelectPreferredMediaOption_ReturnsEmptyWhen62mmUnavailable(t *testing.T) {
	options := []string{"A4", "Letter", "w29h90"}
	if got := selectPreferredMediaOption(options); got != "" {
		t.Fatalf("expected empty selection, got %q", got)
	}
}

func TestParseSubmittedJobID(t *testing.T) {
	output := "request id is Brother_QL_820NWB_USB-17 (1 file(s))"
	if got := parseSubmittedJobID(output); got != "Brother_QL_820NWB_USB-17" {
		t.Fatalf("expected job id, got %q", got)
	}
}

func TestParseQueueJobs(t *testing.T) {
	output := "Brother_QL_820NWB_USB-8 rakusika 104448 Sun Apr 12 22:37:10 2026\n"
	jobs := parseQueueJobs(output)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 job, got %d", len(jobs))
	}
	if jobs[0].ID != "Brother_QL_820NWB_USB-8" {
		t.Fatalf("unexpected job id %q", jobs[0].ID)
	}
	if jobs[0].State != "queued" {
		t.Fatalf("unexpected job state %q", jobs[0].State)
	}
}

func TestParsePrinterState(t *testing.T) {
	if got := parsePrinterState("printer Brother_QL_820NWB_USB now printing Brother_QL_820NWB_USB-8. enabled since ..."); got != "printing" {
		t.Fatalf("expected printing, got %q", got)
	}
	if got := parsePrinterState("printer Brother_QL_820NWB_USB is idle. enabled since ..."); got != "idle" {
		t.Fatalf("expected idle, got %q", got)
	}
	if got := parsePrinterState("プリンター Brother_QL_820NWB_USB は待機中です。2026年08月17日 11時05分14秒 以来有効です"); got != "idle" {
		t.Fatalf("expected Japanese idle, got %q", got)
	}
	if got := parsePrinterState("プリンター Brother_QL_820NWB_USB は Brother_QL_820NWB_USB-3798 を印刷しています。"); got != "printing" {
		t.Fatalf("expected Japanese printing, got %q", got)
	}
}

func TestParsePrinterStateAndBackendError_Unavailable(t *testing.T) {
	state, backendErr := parsePrinterStateAndBackendError(strings.Join([]string{
		"プリンター Brother_QL_820NWB_USB は Brother_QL_820NWB_USB-3798 を印刷しています。",
		"\tThe printer may not exist or is unavailable at this time.",
	}, "\n"))
	if state != "printing" {
		t.Fatalf("expected printing state, got %q", state)
	}
	if !strings.Contains(backendErr, "unavailable") {
		t.Fatalf("expected unavailable backend error, got %q", backendErr)
	}
}

func TestNormalizePrinterQueueState(t *testing.T) {
	if got := normalizePrinterQueueState("printing", 1); got != "printing" {
		t.Fatalf("expected printing, got %q", got)
	}
	if got := normalizePrinterQueueState("idle", 1); got != "stalled" {
		t.Fatalf("expected stalled, got %q", got)
	}
	if got := normalizePrinterQueueState("", 0); got != "cleared" {
		t.Fatalf("expected cleared, got %q", got)
	}
}

func TestIsNoDestinationsOutput(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{"c locale", "lpstat: No destinations added.", true},
		{"ja locale", "lpstat: 宛先が追加されていません。", true},
		{"empty output", "", false},
		{"other failure", "lpstat: Bad file descriptor", false},
		{"printer listed", "printer Brother_QL_820NWB_USB is idle.  enabled since Mon", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNoDestinationsOutput(tc.output); got != tc.want {
				t.Fatalf("isNoDestinationsOutput(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}

// With no CUPS queue at all, the configured name is kept so the operator is
// told what the site asked for, and the error must be the actionable
// PRINTER_NOT_CONFIGURED rather than a generic status-read failure.
func TestValidateStatus_NoDestinations(t *testing.T) {
	selected, source := resolvePrinter([]string{"Brother_QL_820NWB_USB"}, nil, "")
	status := PrinterStatus{
		ConfiguredName: "Brother_QL_820NWB_USB",
		SelectedName:   selected,
		Source:         source,
	}

	if status.Ready() {
		t.Fatal("expected Ready() == false when CUPS has no destination")
	}
	err := validateStatus(status)
	if err == nil {
		t.Fatal("expected error when no CUPS destination exists")
	}
	if !strings.HasPrefix(err.Error(), "PRINTER_NOT_CONFIGURED:") {
		t.Fatalf("expected PRINTER_NOT_CONFIGURED, got %q", err.Error())
	}
}

// 実機から採取した lpoptions -l の出力。
// office (2026-09-23) の ipp-usb キューと ptouch キューの実物。
const lpoptionsIPPUSB = `PageSize/Media Size: 12x12mm 17x54mm 17x87mm 23x23mm 24x24mm 29x42mm 29x52mm 29x54mm 29x62mm *29x90mm 38x90mm 39x48mm 58x58mm 60x86mm 62x100mm Custom.WIDTHxHEIGHT
CutMedia/CutMedia: *None Auto EndOfPage EndOfJob
Resolution/Resolution: *300dpi`

const lpoptionsPtouch = `PageSize/Page Size: Custom.WIDTHxHEIGHT 12mm 12mm-circular 17x54mm 29mm 29x90mm 38mm *62mm 62x29mm 62x100mm
Resolution/Resolution: *300dpi 300x600dpi
PrintQuality/Print Quality: *High Fast
MirrorPrint/Mirror Print: True *False
PrintDensity/Print Density: *0PrinterDefault 1VeryLight 2Light 3Medium 4Dark 5VeryDark
AutoCut/Auto Cut: *True False
AutoEject/Auto Eject: *True False
CutLabel/Cut every: *0 1 2 3 4 5 6 7 8 9 10
ExtraMargin/Extra Margin (ignored for die-cut tape): *0mm 1mm 2mm
MediaType/Media Type: Labels *Tape`

func TestParseOptionNames(t *testing.T) {
	ipp := parseOptionNames(lpoptionsIPPUSB)
	for _, want := range []string{"PageSize", "CutMedia", "Resolution"} {
		if _, ok := ipp[want]; !ok {
			t.Errorf("ipp-usb: %q missing from %v", want, ipp)
		}
	}
	if _, ok := ipp["AutoCut"]; ok {
		t.Error("ipp-usb: AutoCut should not be reported")
	}

	pt := parseOptionNames(lpoptionsPtouch)
	for _, want := range []string{"PageSize", "AutoCut", "CutLabel", "MediaType", "ExtraMargin"} {
		if _, ok := pt[want]; !ok {
			t.Errorf("ptouch: %q missing from %v", want, pt)
		}
	}
	// ptouch の PPD には CutMedia が無い。ここが移行で効く差分。
	if _, ok := pt["CutMedia"]; ok {
		t.Error("ptouch: CutMedia should not be reported; the PPD has no such option")
	}
}

func TestCutArgsFor(t *testing.T) {
	join := func(args []string) string { return strings.Join(args, " ") }

	if got := join(cutArgsFor(parseOptionNames(lpoptionsIPPUSB))); got != "-o CutMedia=EndOfPage" {
		t.Errorf("ipp-usb queue: got %q", got)
	}
	if got := join(cutArgsFor(parseOptionNames(lpoptionsPtouch))); got != "-o AutoCut=True -o CutLabel=1" {
		t.Errorf("ptouch queue: got %q", got)
	}
	// 照会できなかったときは従来どおりの挙動を保つ（カットを黙って落とさない）。
	if got := join(cutArgsFor(nil)); got != "-o CutMedia=EndOfPage" {
		t.Errorf("unknown queue: got %q", got)
	}
	// どちらの綴りも無いキューには何も送らない。未知のオプションは
	// CUPS がジョブごと弾くため。
	if got := cutArgsFor(parseOptionNames("PageSize/Page Size: A4\n")); got != nil {
		t.Errorf("queue without any cut option: got %v", got)
	}
}

func TestIsSafeJobID(t *testing.T) {
	ok := []string{"Brother_QL_820NWB_USB-763", "q-1", "A.b_c-9"}
	for _, id := range ok {
		if !isSafeJobID(id) {
			t.Errorf("expected %q to be accepted", id)
		}
	}
	bad := []string{"", " ", "a b", "job;rm -rf /", "job$(id)", "job/../x", strings.Repeat("a", 129)}
	for _, id := range bad {
		if isSafeJobID(id) {
			t.Errorf("expected %q to be rejected", id)
		}
	}
}

// lpstat prints completed jobs oldest first; the operator reprinting a bad
// label wants the most recent one, so the list is reversed and capped.
func TestNewestCompletedJobs(t *testing.T) {
	output := `Brother_QL_820NWB_ptouch-49 rakusika 61440 Wed Sep 23 07:12:01 2026
Brother_QL_820NWB_ptouch-50 rakusika 61440 Wed Sep 23 07:12:57 2026
Brother_QL_820NWB_ptouch-51 rakusika 61440 Wed Sep 23 07:27:59 2026
`
	jobs := newestCompletedJobs(output, 2)
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d", len(jobs))
	}
	if jobs[0].ID != "Brother_QL_820NWB_ptouch-51" {
		t.Errorf("expected newest job first, got %q", jobs[0].ID)
	}
	if jobs[1].ID != "Brother_QL_820NWB_ptouch-50" {
		t.Errorf("expected second newest job, got %q", jobs[1].ID)
	}
	for _, job := range jobs {
		if job.State != "completed" {
			t.Errorf("expected state completed, got %q", job.State)
		}
	}
}

func TestNewestCompletedJobs_EmptyOutput(t *testing.T) {
	if jobs := newestCompletedJobs("", 5); len(jobs) != 0 {
		t.Fatalf("expected no jobs, got %d", len(jobs))
	}
}

// A limit of 0 means "everything", which jobBelongsToPrinter relies on to
// validate a restart against the full history rather than the newest few.
func TestNewestCompletedJobs_ZeroLimitKeepsAll(t *testing.T) {
	output := `p-1 u 1 Wed Sep 23 07:00:00 2026
p-2 u 1 Wed Sep 23 07:01:00 2026
p-3 u 1 Wed Sep 23 07:02:00 2026
`
	if jobs := newestCompletedJobs(output, 0); len(jobs) != 3 {
		t.Fatalf("expected all 3 jobs, got %d", len(jobs))
	}
}
