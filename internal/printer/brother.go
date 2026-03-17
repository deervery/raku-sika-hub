package printer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

type PrinterStatus struct {
	ConfiguredName string
	SelectedName   string
	DefaultName    string
	Available      []string
	Source         string
}

// Brother manages printing to a Brother label printer via CUPS lp command.
type Brother struct {
	name     string
	renderer *LabelRenderer
	logger   *logging.Logger
}

// NewBrother creates a new Brother printer driver.
// fontPath is optional; if empty, system fonts are searched.
// If font loading fails, label printing is disabled but test printing still works.
func NewBrother(name string, fontPath string, logger *logging.Logger) *Brother {
	b := &Brother{name: strings.TrimSpace(name), logger: logger}

	renderer, err := NewLabelRenderer(fontPath)
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
func (b *Brother) PrintLabel(data LabelData) error {
	status, err := b.Status()
	if err != nil {
		return fmt.Errorf("PRINTER_ERROR: CUPS の状態確認に失敗しました: %s", err)
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
		return err
	}

	if b.renderer == nil {
		return fmt.Errorf("PRINTER_ERROR: ラベルレンダラが初期化されていません。" +
			"日本語フォントをインストールしてください: sudo apt-get install fonts-noto-cjk")
	}

	// Render the label image.
	imgPath, err := b.renderer.Render(data)
	if err != nil {
		return fmt.Errorf("PRINTER_ERROR: ラベル画像の生成に失敗しました: %s", err)
	}
	defer os.Remove(imgPath)

	// Print via CUPS lp command.
	copies := data.Copies
	if copies < 1 {
		copies = 1
	}

	args := []string{"-d", status.SelectedName, "-n", fmt.Sprintf("%d", copies), imgPath}
	cmd := exec.Command("lp", args...)
	out, err := cmd.CombinedOutput()
	b.logger.Info("lp output (label print, printer=%q): %s", status.SelectedName, strings.TrimSpace(string(out)))
	if err != nil {
		return classifyLpError(string(out), status)
	}

	b.logger.Info("label printed: %d copies via lp (printer=%q)", copies, status.SelectedName)
	return nil
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
	return b.renderer.Render(data)
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
