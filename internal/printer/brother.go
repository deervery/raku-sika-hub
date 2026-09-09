package printer

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

type PrinterStatus struct {
	ConfiguredName string
	SelectedName   string
	DefaultName    string
	Available      []string
	Source         string
	// Model is the Brother QL model implied by SelectedName ("QL-800",
	// "QL-820NWB"), or "" when the queue name carries no known model.
	Model        string
	DeviceURI    string
	CUPSState    string
	BackendReady bool
	BackendError string
}

type PrintResult struct {
	State        string
	JobID        string
	Message      string
	PrinterState string
	JobState     string
}

type QueueSnapshot struct {
	PrinterName   string
	PrinterState  string
	QueueState    string
	BackendReady  bool
	BackendError  string
	BackendDevice string
	Jobs          []QueueJobStatus
}

type QueueJobStatus struct {
	ID          string
	User        string
	Size        string
	SubmittedAt string
	State       string
}

// Brother manages printing to a Brother label printer via CUPS lp command.
type Brother struct {
	// mu serializes PrintLabel / TestPrint so that concurrent print
	// requests don't trigger parallel label rendering. Parallel renders on a
	// 4GB RPi 5 caused OOM-like freezes when CUPS stalls (e.g., paper jam):
	// each goroutine held a ~10MB RGBA buffer plus a temp PNG for 12s while
	// verifySubmittedJob polled lpstat. See #283.
	mu       sync.Mutex
	name     string
	renderer *LabelRenderer
	logger   *logging.Logger
}

var labelMediaCandidates = []string{
	"roll62",
	"roll-62",
	"roll_62",
	"62mm-roll",
	"62mm_continuous",
	"62mmcontinuous",
	"62mmx100mm",
	"62x100mm",
	"w62h100",
	"62mm",
	"62",
}

// NewBrother creates a new Brother printer driver.
// fontPath is optional; if empty, system fonts are searched.
// If font loading fails, label printing is disabled but test printing still works.
func NewBrother(name string, fontPath string, assetsDir string, logger *logging.Logger) *Brother {
	b := &Brother{name: strings.TrimSpace(name), logger: logger}

	renderer, err := NewLabelRenderer(fontPath, assetsDir)
	if err != nil {
		logger.Warn("label renderer unavailable: %s", err)
	} else {
		b.renderer = renderer
		logger.Info("label renderer ready (font loaded)")
	}

	b.LogStatus("startup")
	return b
}

// IsAvailable checks whether the printer is registered in CUPS.
func (b *Brother) IsAvailable() bool {
	status, err := b.Status()
	if err != nil {
		return false
	}
	return validateStatus(status) == nil
}

// Status returns the current CUPS printer resolution.
func (b *Brother) Status() (PrinterStatus, error) {
	availableOut, err := exec.Command("lpstat", "-p").CombinedOutput()
	// `lpstat -p` exits 1 with "No destinations added." when CUPS holds no queue
	// at all. That is a printer *configuration* state, not a failure to query
	// CUPS, so it must fall through to validateStatus() → PRINTER_NOT_CONFIGURED
	// ("PRINTER_NAME を実在する CUPS 名に…") instead of the opaque
	// "CUPS の状態確認に失敗しました". Observed at raku-sika-hub-office on
	// 2026-09-08, where the queue had been deleted and the operator was told the
	// status check had failed rather than that no printer was registered.
	if err != nil && !isNoDestinationsOutput(string(availableOut)) {
		return PrinterStatus{}, fmt.Errorf("lpstat -p failed: %w: %s", err, strings.TrimSpace(string(availableOut)))
	}

	defaultOut, err := exec.Command("lpstat", "-d").CombinedOutput()
	defaultName := ""
	if err == nil {
		defaultName = parseDefaultPrinter(string(defaultOut))
	}

	available := parseAvailablePrinters(string(availableOut))
	status := PrinterStatus{
		ConfiguredName: b.name,
		DefaultName:    defaultName,
		Available:      available,
	}

	configuredNames := parseConfiguredPrinterNames(b.name)
	status.SelectedName, status.Source = resolvePrinter(configuredNames, available, defaultName)
	status.Model = PrinterModel(status.SelectedName)

	if status.SelectedName != "" {
		status.DeviceURI = readPrinterDeviceURI(status.SelectedName)
		status.CUPSState, status.BackendError = readPrinterState(status.SelectedName)
		status.BackendReady = status.BackendError == ""
		if status.BackendReady {
			status.BackendReady, status.BackendError = checkPrinterBackend(status.DeviceURI)
		}
	}

	return status, nil
}

// Ready reports whether the selected CUPS printer resolves to a usable backend.
func (s PrinterStatus) Ready() bool {
	if s.SelectedName == "" {
		return false
	}
	if s.Source == "configured" && !contains(s.Available, s.SelectedName) {
		return false
	}
	return s.BackendReady
}

func (b *Brother) QueueSnapshot() (QueueSnapshot, error) {
	status, err := b.Status()
	if err != nil {
		return QueueSnapshot{}, err
	}
	if status.SelectedName == "" {
		return QueueSnapshot{
			PrinterName:   status.SelectedName,
			PrinterState:  status.CUPSState,
			QueueState:    "cleared",
			BackendReady:  status.BackendReady,
			BackendError:  status.BackendError,
			BackendDevice: status.DeviceURI,
		}, nil
	}

	snapshot, err := readQueueSnapshot(status.SelectedName)
	if err != nil {
		return QueueSnapshot{}, err
	}
	snapshot.PrinterState = status.CUPSState
	snapshot.BackendReady = status.BackendReady
	snapshot.BackendError = status.BackendError
	snapshot.BackendDevice = status.DeviceURI
	if !status.BackendReady && len(snapshot.Jobs) > 0 {
		snapshot.QueueState = "stalled"
		for i := range snapshot.Jobs {
			snapshot.Jobs[i].State = "stalled"
		}
	}
	return snapshot, nil
}

// LogStatus logs the configured and discovered printers.
func (b *Brother) LogStatus(context string) {
	status, err := b.Status()
	if err != nil {
		b.logger.Warn("printer status (%s): configured=%q, error=%v", context, b.name, err)
		return
	}
	b.logger.Info(
		"printer status (%s): configured=%q, selected=%q, model=%q, source=%s, default=%q, available=%s, device_uri=%q, cups_state=%q, backend_ready=%t, backend_error=%q",
		context,
		status.ConfiguredName,
		status.SelectedName,
		status.Model,
		status.Source,
		status.DefaultName,
		formatPrinters(status.Available),
		status.DeviceURI,
		status.CUPSState,
		status.BackendReady,
		status.BackendError,
	)
}

// TestPrint sends a test print job to the printer.
func (b *Brother) TestPrint() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	status, err := b.Status()
	if err != nil {
		return fmt.Errorf("PRINTER_ERROR: CUPS の状態確認に失敗しました: %s", err)
	}
	b.logger.Info(
		"test print requested: configured=%q, selected=%q, source=%s, available=%s",
		status.ConfiguredName,
		status.SelectedName,
		status.Source,
		formatPrinters(status.Available),
	)

	if err := validateStatus(status); err != nil {
		return err
	}

	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`echo "RakuSika Hub Test Print\n$(date)" | lp -d "%s" -`, status.SelectedName))
	out, err := cmd.CombinedOutput()
	b.logger.Info("lp output (test print, printer=%q): %s", status.SelectedName, strings.TrimSpace(string(out)))
	if err != nil {
		return classifyLpError(string(out), status)
	}

	b.logger.Info("test print sent via lp (printer=%q)", status.SelectedName)
	return nil
}

// PrintLabel renders a label image and sends it to the printer.
func (b *Brother) PrintLabel(data LabelData) (PrintResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	status, err := b.Status()
	if err != nil {
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: CUPS の状態確認に失敗しました: %s", err)
	}
	b.logger.Info(
		"print label requested: template=%s, copies=%d, product=%s, configured=%q, selected=%q, source=%s, available=%s",
		data.Template,
		data.Copies,
		data.ProductName,
		status.ConfiguredName,
		status.SelectedName,
		status.Source,
		formatPrinters(status.Available),
	)

	if err := validateStatus(status); err != nil {
		return PrintResult{}, err
	}

	if b.renderer == nil {
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: ラベルレンダラが初期化されていません。" +
			"日本語フォントをインストールしてください: sudo apt-get install fonts-noto-cjk")
	}

	// Render the label image.
	result, err := b.renderer.Render(data)
	if err != nil {
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: ラベル画像の生成に失敗しました: %s", err)
	}
	defer os.Remove(result.Path)

	// Print via CUPS lp command with dynamic media size for auto-cut.
	copies := data.Copies
	if copies < 1 {
		copies = 1
	}

	// Keep width fixed to 62mm and adjust only height per rendered label.
	const mediaWidthMM = 62
	mediaHeightMM := result.HeightMM
	if mediaHeightMM < 1 {
		mediaHeightMM = 1
	}
	media := fmt.Sprintf("Custom.%dx%dmm", mediaWidthMM, mediaHeightMM)
	b.logger.Info(
		"label media resolved: template=%s copies=%d media=%s rendered=%dx%dmm",
		data.Template,
		copies,
		media,
		result.WidthMM,
		result.HeightMM,
	)

	args := []string{
		"-d", status.SelectedName,
		"-n", fmt.Sprintf("%d", copies),
		"-o", "media=" + media,
		"-o", "PageSize=" + media,
		"-o", "fit-to-page",
		"-o", "CutMedia=EndOfPage",
	}
	b.logger.Info("lp args: %s", strings.Join(args, " "))
	args = append(args, result.Path)
	out, err := exec.Command("lp", args...).CombinedOutput()
	b.logger.Info("lp output (label print, printer=%q): %s", status.SelectedName, strings.TrimSpace(string(out)))
	if err != nil {
		return PrintResult{}, classifyLpError(string(out), status)
	}
	jobID := parseSubmittedJobID(string(out))
	if jobID != "" {
		printResult, err := b.verifySubmittedJob(status.SelectedName, jobID, 12*time.Second)
		if err != nil {
			return PrintResult{}, err
		}
		b.logger.Info("label print state: job=%s state=%s printer_state=%s", printResult.JobID, printResult.State, printResult.PrinterState)
		return printResult, nil
	}

	b.logger.Info("label printed: %d copies via lp (printer=%q)", copies, status.SelectedName)
	return PrintResult{
		State:   "done",
		Message: "印刷ジョブを送信しました。",
	}, nil
}

// CanRenderLabels reports whether the label renderer is available.
func (b *Brother) CanRenderLabels() bool {
	return b.renderer != nil
}

// RenderLabel generates a label PNG image and returns the temporary file path.
// The caller is responsible for removing the file.
func (b *Brother) RenderLabel(data LabelData) (string, error) {
	if b.renderer == nil {
		return "", fmt.Errorf("PRINTER_ERROR: ラベルレンダラが初期化されていません")
	}
	result, err := b.renderer.Render(data)
	if err != nil {
		return "", err
	}
	return result.Path, nil
}

// classifyLpError maps lp output to specific error codes with Japanese messages.
func classifyLpError(output string, status PrinterStatus) error {
	output = strings.TrimSpace(output)
	switch {
	case strings.Contains(output, "does not exist") || strings.Contains(output, "unknown destination"):
		return printerConfigError(status)
	case strings.Contains(output, "Permission denied") || strings.Contains(output, "EACCES"):
		return fmt.Errorf("PRINTER_PERMISSION_DENIED: プリンタへのアクセス権限がありません。sudo usermod -aG lpadmin $USER を実行してください")
	case strings.Contains(output, "not accepting") || strings.Contains(output, "disabled"):
		return fmt.Errorf("PRINTER_DISABLED: プリンタが無効化されています。CUPSの管理画面でプリンタを有効にしてください")
	case strings.Contains(output, "paper") || strings.Contains(output, "media"):
		return fmt.Errorf("PRINTER_PAPER_ERROR: ラベル用紙を確認してください。用紙切れまたはジャムの可能性があります")
	default:
		return fmt.Errorf("PRINTER_ERROR: 印刷エラー: %s", output)
	}
}

func validateStatus(status PrinterStatus) error {
	if status.SelectedName == "" {
		return printerConfigError(status)
	}
	if status.Source == "configured" && !contains(status.Available, status.SelectedName) {
		return printerConfigError(status)
	}
	if !status.BackendReady && status.BackendError != "" {
		return fmt.Errorf("PRINTER_UNAVAILABLE: プリンタの送信先に接続できません。 printer=%q device_uri=%q state=%q error=%s",
			status.SelectedName,
			status.DeviceURI,
			status.CUPSState,
			status.BackendError,
		)
	}
	return nil
}

func readPrinterDeviceURI(printerName string) string {
	out, err := exec.Command("lpstat", "-v", printerName).CombinedOutput()
	if err != nil {
		return ""
	}
	return parsePrinterDeviceURI(string(out))
}

func parsePrinterDeviceURI(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.Index(line, ":"); idx >= 0 {
			return strings.TrimSpace(line[idx+1:])
		}
	}
	return ""
}

func readPrinterState(printerName string) (string, string) {
	out, err := exec.Command("lpstat", "-p", printerName, "-l").CombinedOutput()
	if err != nil && strings.TrimSpace(string(out)) == "" {
		return "", err.Error()
	}
	return parsePrinterStateAndBackendError(string(out))
}

func parsePrinterStateAndBackendError(output string) (string, string) {
	state := parsePrinterState(output)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		lower := strings.ToLower(line)
		if strings.Contains(lower, "unavailable") ||
			strings.Contains(lower, "may not exist") ||
			strings.Contains(lower, "not connected") {
			if state == "" {
				state = "unavailable"
			}
			return state, line
		}
	}
	return state, ""
}

func checkPrinterBackend(deviceURI string) (bool, string) {
	if strings.TrimSpace(deviceURI) == "" {
		return false, "device URI is empty"
	}

	parsed, err := url.Parse(deviceURI)
	if err != nil {
		return false, err.Error()
	}

	host := parsed.Hostname()
	if !isLocalhost(host) {
		return true, ""
	}

	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "ipp", "ipps":
			port = "631"
		default:
			return true, ""
		}
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 1500*time.Millisecond)
	if err != nil {
		return false, err.Error()
	}
	_ = conn.Close()
	return true, ""
}

func isLocalhost(host string) bool {
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "localhost", "127.0.0.1", "::1":
		return true
	default:
		return false
	}
}

func parseSubmittedJobID(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	const marker = "request id is "
	idx := strings.Index(output, marker)
	if idx < 0 {
		return ""
	}
	rest := output[idx+len(marker):]
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return ""
	}
	return strings.TrimSpace(fields[0])
}

func (b *Brother) verifySubmittedJob(printerName, jobID string, timeout time.Duration) (PrintResult, error) {
	deadline := time.Now().Add(timeout)
	lastState := ""
	lastQueueState := ""
	for time.Now().Before(deadline) {
		snapshot, err := readQueueSnapshot(printerName)
		if err != nil {
			b.logger.Warn("queue poll failed for %s: %v", jobID, err)
			time.Sleep(2 * time.Second)
			continue
		}
		job, ok := snapshot.findJob(jobID)
		if !ok {
			return PrintResult{
				State:        "done",
				JobID:        jobID,
				Message:      "印刷ジョブを送信しました。",
				PrinterState: snapshot.PrinterState,
				JobState:     "cleared",
			}, nil
		}
		lastState = job.State
		lastQueueState = snapshot.QueueState
		b.logger.Info("queue poll: job=%s queue_state=%s printer_state=%s job_state=%s", jobID, snapshot.QueueState, snapshot.PrinterState, job.State)
		time.Sleep(2 * time.Second)
	}
	snapshot, err := readQueueSnapshot(printerName)
	if err != nil {
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: 印刷キューの確認に失敗しました: %w", err)
	}
	if job, ok := snapshot.findJob(jobID); ok {
		lastState = job.State
	}
	if snapshot.QueueState == "" {
		snapshot.QueueState = lastQueueState
	}
	message := "印刷ジョブは送信済みですが、プリンタの復帰待ちです。"
	if snapshot.QueueState == "stalled" {
		message = "印刷ジョブは送信済みですが、キューが停滞しています。プリンタ状態とキューを確認してください。"
	}
	return PrintResult{
		State:        "pending",
		JobID:        jobID,
		Message:      message,
		PrinterState: snapshot.PrinterState,
		JobState:     lastState,
	}, nil
}

func readQueueSnapshot(printerName string) (QueueSnapshot, error) {
	out, err := exec.Command("lpstat", "-W", "not-completed", "-o", printerName).CombinedOutput()
	if err != nil && strings.TrimSpace(string(out)) != "" {
		return QueueSnapshot{}, fmt.Errorf("lpstat queue failed: %s", strings.TrimSpace(string(out)))
	}
	jobs := parseQueueJobs(string(out))
	printerState := ""
	if stateOut, stateErr := exec.Command("lpstat", "-p", printerName, "-l").CombinedOutput(); stateErr == nil {
		printerState = parsePrinterState(string(stateOut))
	}
	return QueueSnapshot{
		PrinterName:   printerName,
		PrinterState:  printerState,
		QueueState:    normalizePrinterQueueState(printerState, len(jobs)),
		BackendReady:  true,
		BackendDevice: readPrinterDeviceURI(printerName),
		Jobs:          jobs,
	}, nil
}

func (q QueueSnapshot) findJob(jobID string) (QueueJobStatus, bool) {
	for _, job := range q.Jobs {
		if job.ID == jobID {
			return job, true
		}
	}
	return QueueJobStatus{}, false
}

func parseQueueJobs(output string) []QueueJobStatus {
	var jobs []QueueJobStatus
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		jobs = append(jobs, QueueJobStatus{
			ID:          fields[0],
			User:        fields[1],
			Size:        fields[2],
			SubmittedAt: strings.Join(fields[3:], " "),
			State:       "queued",
		})
	}
	return jobs
}

func normalizePrinterQueueState(printerState string, jobCount int) string {
	if jobCount == 0 {
		return "cleared"
	}
	state := strings.TrimSpace(strings.ToLower(printerState))
	switch {
	case state == "printing", strings.Contains(state, "connected to printer"):
		return "printing"
	case state == "idle", state == "disabled", state == "":
		return "stalled"
	default:
		return "queued"
	}
}

func parsePrinterState(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "printer ") && strings.Contains(line, " is idle"):
			return "idle"
		case strings.HasPrefix(line, "printer ") && strings.Contains(line, " now printing "):
			return "printing"
		case strings.HasPrefix(line, "printer ") && strings.Contains(line, " disabled"):
			return "disabled"
		case strings.HasPrefix(line, "プリンター ") && strings.Contains(line, " は待機中"):
			return "idle"
		case strings.HasPrefix(line, "プリンター ") && strings.Contains(line, " を印刷しています"):
			return "printing"
		case strings.HasPrefix(line, "Status:"):
			return strings.TrimSpace(strings.TrimPrefix(line, "Status:"))
		}
	}
	return ""
}

func printerConfigError(status PrinterStatus) error {
	selected := status.SelectedName
	if selected == "" {
		selected = "(none)"
	}
	configured := status.ConfiguredName
	if configured == "" {
		configured = "(not set)"
	}
	return fmt.Errorf(
		"PRINTER_NOT_CONFIGURED: 使用するプリンタ名を解決できません。 configured=%q selected=%q default=%q available=%s. PRINTER_NAME を実在する CUPS 名に合わせて設定してください",
		configured,
		selected,
		status.DefaultName,
		formatPrinters(status.Available),
	)
}

func (b *Brother) resolveLabelMedia(printerName string) (string, []string, error) {
	out, err := exec.Command("lpoptions", "-p", printerName, "-l").CombinedOutput()
	if err != nil {
		return "", nil, fmt.Errorf("PRINTER_MEDIA_ERROR: 62mm ラベル設定を解決できません。lpoptions -l 取得に失敗しました: %s", strings.TrimSpace(string(out)))
	}

	options := parseMediaOptions(string(out))
	if len(options) == 0 {
		return "", options, nil
	}
	selected := selectPreferredMediaOption(options)
	if selected == "" {
		return "", options, fmt.Errorf(
			"PRINTER_MEDIA_ERROR: 62mm ラベル設定を解決できません。printer=%q available_media=%s",
			printerName,
			formatMediaOptions(options),
		)
	}
	return selected, options, nil
}

// isNoDestinationsOutput reports whether `lpstat -p` failed only because CUPS
// has no printer queue registered. Both the C locale message and the Japanese
// translation are matched, since the Hub runs under LANG=C on some stations and
// under a ja locale on others.
func isNoDestinationsOutput(output string) bool {
	out := strings.TrimSpace(output)
	if out == "" {
		return false
	}
	return strings.Contains(out, "No destinations added") ||
		strings.Contains(out, "宛先が追加されていません")
}

func parseAvailablePrinters(output string) []string {
	lines := strings.Split(output, "\n")
	printers := make([]string, 0, len(lines))
	seen := make(map[string]struct{})
	for _, line := range lines {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 2 && fields[0] == "printer" {
			name := fields[1]
			if _, ok := seen[name]; ok {
				continue
			}
			seen[name] = struct{}{}
			printers = append(printers, name)
		}
	}
	return printers
}

func parseConfiguredPrinterNames(input string) []string {
	parts := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ';'
	})
	names := make([]string, 0, len(parts))
	seen := make(map[string]struct{})
	for _, part := range parts {
		name := strings.TrimSpace(part)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return names
}

// resolvePrinter picks the CUPS queue to print to and reports how it was found.
//
// A configured name that actually exists always wins, so a site that pins
// PRINTER_NAME keeps behaving exactly as before. Everything after that is a
// fallback for sites whose queue name does not match what was configured —
// most often a Brother QL-800 registered under a name the config never
// listed. Auto-detection only ever selects a queue that looks like a Brother
// QL label printer, so an unrelated default printer (an office laser, a PDF
// queue) is never silently used for labels.
func resolvePrinter(configured []string, available []string, defaultName string) (string, string) {
	if name := selectConfiguredPrinter(configured, available); name != "" {
		return name, "configured"
	}
	if defaultName != "" && scoreBrotherQL(defaultName) > 0 && contains(available, defaultName) {
		return defaultName, "cups-default"
	}
	if name := DetectBrotherQL(available); name != "" {
		return name, "auto-detected"
	}
	if len(configured) > 0 {
		// Nothing matched: keep the configured name so the error reported to
		// the operator names what the site asked for.
		return configured[0], "configured"
	}
	if defaultName != "" {
		return defaultName, "cups-default"
	}
	if len(available) > 0 {
		return available[0], "first-available"
	}
	return "", "unresolved"
}

func selectConfiguredPrinter(configured []string, available []string) string {
	availableSet := make(map[string]struct{}, len(available))
	for _, name := range available {
		availableSet[name] = struct{}{}
	}
	for _, name := range configured {
		if _, ok := availableSet[name]; ok {
			return name
		}
	}
	return ""
}

func parseDefaultPrinter(output string) string {
	const prefix = "system default destination:"
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func parseMediaOptions(output string) []string {
	options := make(map[string]struct{})
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lower := strings.ToLower(line)
		if !strings.HasPrefix(lower, "pagesize/") && !strings.HasPrefix(lower, "media/") {
			continue
		}

		colon := strings.Index(line, ":")
		if colon < 0 || colon == len(line)-1 {
			continue
		}

		for _, field := range strings.Fields(line[colon+1:]) {
			field = strings.TrimPrefix(field, "*")
			if field == "" {
				continue
			}
			slash := strings.Index(field, "/")
			if slash < 0 {
				continue
			}
			name := field[:slash]
			if name == "" {
				continue
			}
			options[name] = struct{}{}
		}
	}

	result := make([]string, 0, len(options))
	for option := range options {
		result = append(result, option)
	}
	sort.Strings(result)
	return result
}

func selectPreferredMediaOption(options []string) string {
	if len(options) == 0 {
		return ""
	}

	type scoredOption struct {
		name  string
		score int
	}

	best := scoredOption{score: -1}
	for _, option := range options {
		score := scoreMediaOption(option)
		if score > best.score || (score == best.score && score >= 0 && option < best.name) {
			best = scoredOption{name: option, score: score}
		}
	}

	if best.score < 0 {
		return ""
	}
	return best.name
}

func scoreMediaOption(option string) int {
	normalized := normalizeAlnum(option)
	if normalized == "" {
		return -1
	}

	best := -1
	for rank, candidate := range labelMediaCandidates {
		candidate = normalizeAlnum(candidate)
		if candidate == "" {
			continue
		}
		switch {
		case normalized == candidate:
			score := 1000 - rank
			if score > best {
				best = score
			}
		case strings.Contains(normalized, candidate):
			score := 800 - rank
			if score > best {
				best = score
			}
		}
	}

	width, height, ok := parseMediaDimensionsMM(normalized)
	if !ok {
		return best
	}

	if width == 62 && height == 0 {
		if 700 > best {
			best = 700
		}
		return best
	}
	if width == 62 {
		score := 600
		if strings.Contains(normalized, "roll") || strings.Contains(normalized, "cont") {
			score = 750
		}
		if score > best {
			best = score
		}
	}

	return best
}

func normalizeAlnum(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func parseMediaDimensionsMM(normalized string) (int, int, bool) {
	var dims []int
	var current strings.Builder

	flush := func() {
		if current.Len() == 0 {
			return
		}
		n, err := strconv.Atoi(current.String())
		if err == nil {
			dims = append(dims, n)
		}
		current.Reset()
	}

	for _, r := range normalized {
		if unicode.IsDigit(r) {
			current.WriteRune(r)
			continue
		}
		flush()
	}
	flush()

	if len(dims) == 0 {
		return 0, 0, false
	}
	if len(dims) == 1 {
		return dims[0], 0, true
	}
	return dims[0], dims[1], true
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func formatPrinters(printers []string) string {
	if len(printers) == 0 {
		return "[]"
	}
	return "[" + strings.Join(printers, ", ") + "]"
}

func formatMediaOptions(options []string) string {
	if len(options) == 0 {
		return "[]"
	}
	return "[" + strings.Join(options, ", ") + "]"
}
