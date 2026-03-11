package printer

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

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
	b := &Brother{name: name, logger: logger}

	renderer, err := NewLabelRenderer(fontPath)
	if err != nil {
		logger.Warn("label renderer unavailable: %s", err)
	} else {
		b.renderer = renderer
		logger.Info("label renderer ready (font loaded)")
	}

	return b
}

// IsAvailable checks whether the printer is registered in CUPS.
func (b *Brother) IsAvailable() bool {
	out, err := exec.Command("lpstat", "-p").CombinedOutput()
	if err != nil {
		return false
	}
	s := string(out)
	return strings.Contains(s, b.name) || strings.Contains(s, "QL")
}

// TestPrint sends a test print job to the printer.
func (b *Brother) TestPrint() error {
	b.logger.Info("test print requested on %s", b.name)

	if !b.IsAvailable() {
		return fmt.Errorf(
			"PRINTER_NOT_CONFIGURED: プリンタ \"%s\" がCUPSに登録されていません。"+
				" sudo apt-get install printer-driver-ptouch を実行し、CUPS設定を行ってください", b.name)
	}

	cmd := exec.Command("bash", "-c",
		fmt.Sprintf(`echo "RakuSika Hub Test Print\n$(date)" | lp -d "%s" -`, b.name))
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyLpError(string(out))
	}

	b.logger.Info("test print sent via lp")
	return nil
}

// PrintLabel renders a label image and sends it to the printer.
func (b *Brother) PrintLabel(data LabelData) error {
	b.logger.Info("print label requested: template=%s, copies=%d, product=%s",
		data.Template, data.Copies, data.ProductName)

	if !b.IsAvailable() {
		return fmt.Errorf(
			"PRINTER_NOT_CONFIGURED: プリンタ \"%s\" がCUPSに登録されていません。"+
				" sudo apt-get install printer-driver-ptouch を実行し、CUPS設定を行ってください", b.name)
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

	args := []string{"-d", b.name, "-n", fmt.Sprintf("%d", copies), imgPath}
	cmd := exec.Command("lp", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return classifyLpError(string(out))
	}

	b.logger.Info("label printed: %d copies via lp", copies)
	return nil
}

// CanRenderLabels reports whether the label renderer is available.
func (b *Brother) CanRenderLabels() bool {
	return b.renderer != nil
}

// classifyLpError maps lp output to specific error codes with Japanese messages.
func classifyLpError(output string) error {
	output = strings.TrimSpace(output)
	switch {
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
