package printer

import (
	"fmt"
	"image"
	"image/png"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/deervery/raku-sika-hub/internal/printer/qlbackend"
	"github.com/deervery/raku-sika-hub/internal/printer/qlraster"
)

// Raw printing: the hub encodes the Brother raster itself (qlraster) and hands
// the bytes to a CUPS raw queue, which passes them to the usb backend
// untouched (verified byte-for-byte on office, 2026-09-24).
//
// Why a raw queue rather than a driver: with printer-driver-ptouch the CUPS
// filters shrank labels by 12%, rotated some by 90°, and declared a roll width
// other than 62 mm for most label heights — which the printer refuses with
// 「ロール種類の不一致」. None of those layers exist on a raw queue.
//
// Why CUPS at all rather than writing /dev/usb/lp0: the usb backend is the
// transport that has printed reliably on every station, and keeping CUPS jobs
// means the tablet's queue view, cancel and 再送信 keep working unchanged.
//
// The path is chosen per queue, not by a setting: a queue whose make-and-model
// is CUPS's "Local Raw Printer" can only mean the site was switched to raw
// (raku-sika-ops switch-printer-backend.sh --to raw). Any other queue keeps
// the existing driver path, so one binary serves every station.

// rawQueueMakeAndModel is what CUPS reports for a queue created with -m raw.
const rawQueueMakeAndModel = "local raw printer"

// isRawQueue reports whether `lpoptions -p <queue>` describes a raw queue.
func isRawQueue(lpoptionsOutput string) bool {
	const key = "printer-make-and-model="
	i := strings.Index(lpoptionsOutput, key)
	if i < 0 {
		return false
	}
	v := lpoptionsOutput[i+len(key):]
	if strings.HasPrefix(v, "'") {
		if end := strings.Index(v[1:], "'"); end >= 0 {
			v = v[1 : end+1]
		}
	} else if sp := strings.IndexByte(v, ' '); sp >= 0 {
		v = v[:sp]
	}
	return strings.EqualFold(strings.TrimSpace(v), rawQueueMakeAndModel)
}

func readQueueIsRaw(printerName string) bool {
	out, err := exec.Command("lpoptions", "-p", printerName).CombinedOutput()
	if err != nil {
		return false
	}
	return isRawQueue(string(out))
}

// rawSupportedModel limits raw printing to the models qlraster is verified
// against. Sending QL-800 series raster to anything else would print garbage
// or wedge the printer, so an unknown model is refused instead.
func rawSupportedModel(model string) bool {
	switch model {
	case "QL-800", "QL-820NWB":
		return true
	}
	return false
}

// printRaw encodes the rendered label and submits it to the raw queue.
func (b *Brother) printRaw(status PrinterStatus, pngPath string, copies int) (PrintResult, error) {
	if !rawSupportedModel(status.Model) {
		return PrintResult{}, fmt.Errorf(
			"PRINTER_ERROR: raw 印刷は QL-800 / QL-820NWB のみ対応しています。キュー名 %q から機種を判別できません",
			status.SelectedName)
	}

	img, err := decodePNG(pngPath)
	if err != nil {
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: ラベル画像を読めません: %s", err)
	}
	pages := make([]image.Image, copies)
	for i := range pages {
		pages[i] = img
	}
	data, err := qlraster.EncodePages(pages, qlraster.Continuous62, qlraster.Options{Cut: true})
	if err != nil {
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: ラスタの生成に失敗しました: %s", err)
	}

	f, err := os.CreateTemp("", "label-*.ql")
	if err != nil {
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: 一時ファイルを作れません: %s", err)
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(data); err != nil {
		f.Close()
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: 一時ファイルに書けません: %s", err)
	}
	if err := f.Close(); err != nil {
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: 一時ファイルに書けません: %s", err)
	}

	b.logger.Info("raw print: printer=%q model=%s copies=%d size=%dx%d bytes=%d",
		status.SelectedName, status.Model, copies, img.Bounds().Dx(), img.Bounds().Dy(), len(data))

	// -o raw: no filters at all; the bytes reach the backend as written.
	out, err := exec.Command("lp", "-d", status.SelectedName, "-o", "raw", f.Name()).CombinedOutput()
	b.logger.Info("lp output (raw label print, printer=%q): %s", status.SelectedName, strings.TrimSpace(string(out)))
	if err != nil {
		return PrintResult{}, classifyLpError(string(out), status)
	}
	jobID := parseSubmittedJobID(string(out))
	if jobID == "" {
		return PrintResult{State: "done", Message: "印刷ジョブを送信しました。"}, nil
	}
	result, err := b.verifySubmittedJob(status.SelectedName, jobID, 12*time.Second)
	if err != nil {
		return PrintResult{}, err
	}
	b.logger.Info("raw print state: job=%s state=%s printer_state=%s", result.JobID, result.State, result.PrinterState)
	if result.State == "done" && usesQLBackend(status.DeviceURI) {
		return b.confirmWithPrinter(result)
	}
	return result, nil
}

// qlResultDir is where the rakuql backend leaves its per-job results.
var qlResultDir = qlbackend.ResultDir

// qlBackendWrapper is the CUPS backend raku-sika-ops installs for rakuql://.
var qlBackendWrapper = "/usr/lib/cups/backend/" + qlbackend.Scheme

// usesQLBackend reports whether the queue sends through raku-sika-hub's own
// CUPS backend, which follows each job until the printer confirms it.
func usesQLBackend(deviceURI string) bool {
	u, err := url.Parse(deviceURI)
	return err == nil && u.Scheme == qlbackend.Scheme
}

// confirmWithPrinter replaces "the job left the CUPS queue" — which is all a
// CUPS job state can say — with what the printer itself reported.
func (b *Brother) confirmWithPrinter(result PrintResult) (PrintResult, error) {
	number := result.JobID[strings.LastIndex(result.JobID, "-")+1:]
	var (
		res qlbackend.Result
		ok  bool
		err error
	)
	// The backend writes its result before it exits and CUPS only then drops
	// the job, so the file is normally already there.
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(200 * time.Millisecond) {
		res, ok, err = qlbackend.ReadResult(qlResultDir, number)
		if ok || err != nil || time.Now().After(deadline) {
			break
		}
	}
	if err != nil || !ok {
		b.logger.Warn("no printer result for job %s (err=%v); reporting the CUPS state only", result.JobID, err)
		return result, nil
	}
	b.logger.Info("printer result: job=%s outcome=%s completed=%d/%d message=%s", result.JobID, res.Outcome, res.Completed, res.Pages, res.Message)
	switch res.Outcome {
	case qlbackend.OutcomePrinted:
		result.Message = "印刷しました。"
		return result, nil
	case qlbackend.OutcomeUnconfirmed:
		// Not "done": the tablet shows "done" as 印刷しました. Report it as a
		// blocking state so the operator checks the tray.
		result.State = "pending"
		result.Message = res.Message
		result.Diagnosis = &Diagnosis{
			Code:     DiagPrintUnconfirmed,
			Blocking: true,
			Title:    "印刷できたか確認できません",
			Detail:   res.Message,
			Action:   "ラベルが出ていなければ、プリンタの電源を入れ直してから再送信してください。",
		}
		return result, nil
	case qlbackend.OutcomeCanceled:
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: 印刷が取り消されました。")
	default:
		if isPaperReason(res.Reasons) {
			return PrintResult{}, fmt.Errorf("PRINTER_PAPER_ERROR: %s", res.Message)
		}
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: %s", res.Message)
	}
}

func isPaperReason(reasons []string) bool {
	for _, r := range reasons {
		switch r {
		case "media-empty-error", "media-needed-error", "media-jam-error", "cover-open-error":
			return true
		}
	}
	return false
}

// testPrintRaw prints a small real label: plain text would reach a raw queue
// as bytes the printer cannot interpret.
func (b *Brother) testPrintRaw(status PrinterStatus) error {
	if b.renderer == nil {
		return fmt.Errorf("PRINTER_ERROR: ラベルレンダラが初期化されていません。" +
			"日本語フォントをインストールしてください: sudo apt-get install fonts-noto-cjk")
	}
	data := BuildLabelDataFromMap("processed", 1, map[string]string{
		"productName":        "テスト印刷",
		"productQuantity":    "-",
		"deadlineDate":       time.Now().Format("2006-01-02 15:04"),
		"storageTemperature": "RakuSika Hub",
	}, "")
	result, err := b.renderer.Render(data)
	if err != nil {
		return fmt.Errorf("PRINTER_ERROR: テストラベルの生成に失敗しました: %s", err)
	}
	defer os.Remove(result.Path)
	_, err = b.printRaw(status, result.Path, 1)
	return err
}

func decodePNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}
