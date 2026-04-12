package printer

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
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
}

type PrintResult struct {
	State        string
	JobID        string
	Message      string
	PrinterState string
	JobState     string
}

type QueueSnapshot struct {
	PrinterName  string
	PrinterState string
	Jobs         []QueueJobStatus
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
	if err != nil {
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

	switch {
	case b.name != "":
		status.SelectedName = b.name
		status.Source = "configured"
	case defaultName != "":
		status.SelectedName = defaultName
		status.Source = "cups-default"
	case len(available) > 0:
		status.SelectedName = available[0]
		status.Source = "first-available"
	default:
		status.Source = "unresolved"
	}

	return status, nil
}

// LogStatus logs the configured and discovered printers.
func (b *Brother) LogStatus(context string) {
	status, err := b.Status()
	if err != nil {
		b.logger.Warn("printer status (%s): configured=%q, error=%v", context, b.name, err)
		return
	}
	b.logger.Info(
		"printer status (%s): configured=%q, selected=%q, source=%s, default=%q, available=%s",
		context,
		status.ConfiguredName,
		status.SelectedName,
		status.Source,
		status.DefaultName,
		formatPrinters(status.Available),
	)
}

// TestPrint sends a test print job to the printer.
func (b *Brother) TestPrint() error {
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

	media := fmt.Sprintf("custom_%dx%dmm_%dx%dmm", result.WidthMM, result.HeightMM, result.WidthMM, result.HeightMM)
	b.logger.Info("label media: %s (%dx%d mm)", media, result.WidthMM, result.HeightMM)

	args := []string{
		"-d", status.SelectedName,
		"-n", fmt.Sprintf("%d", copies),
		"-o", "media=" + media,
		"-o", "fit-to-page",
		"-o", "CutMedia=Auto",
	}
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
	return nil
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
		time.Sleep(2 * time.Second)
	}
	snapshot, err := readQueueSnapshot(printerName)
	if err != nil {
		return PrintResult{}, fmt.Errorf("PRINTER_ERROR: 印刷キューの確認に失敗しました: %w", err)
	}
	if job, ok := snapshot.findJob(jobID); ok {
		lastState = job.State
	}
	return PrintResult{
		State:        "pending",
		JobID:        jobID,
		Message:      "印刷ジョブは送信済みですが、プリンタの復帰待ちです。",
		PrinterState: snapshot.PrinterState,
		JobState:     lastState,
	}, nil
}

func readQueueSnapshot(printerName string) (QueueSnapshot, error) {
	out, err := exec.Command("lpstat", "-W", "not-completed", "-o", printerName).CombinedOutput()
	if err != nil && strings.TrimSpace(string(out)) != "" {
		return QueueSnapshot{}, fmt.Errorf("lpstat queue failed: %s", strings.TrimSpace(string(out)))
	}
	printerState := ""
	if stateOut, stateErr := exec.Command("lpstat", "-p", printerName, "-l").CombinedOutput(); stateErr == nil {
		printerState = parsePrinterState(string(stateOut))
	}
	return QueueSnapshot{
		PrinterName:  printerName,
		PrinterState: printerState,
		Jobs:         parseQueueJobs(string(out)),
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
	normalized := normalizeMediaName(option)
	if normalized == "" {
		return -1
	}

	best := -1
	for rank, candidate := range labelMediaCandidates {
		candidate = normalizeMediaName(candidate)
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

func normalizeMediaName(s string) string {
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
