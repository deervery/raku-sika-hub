package printer

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

// Brother manages printing to a Brother label printer via CUPS lp command.
type Brother struct {
	name   string
	logger *logging.Logger
}

// NewBrother creates a new Brother printer driver.
func NewBrother(name string, logger *logging.Logger) *Brother {
	return &Brother{name: name, logger: logger}
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
		output := strings.TrimSpace(string(out))
		// Classify printer errors
		if strings.Contains(output, "Permission denied") || strings.Contains(output, "EACCES") {
			return fmt.Errorf("PRINTER_PERMISSION_DENIED: プリンタへのアクセス権限がありません。sudo usermod -aG lpadmin $USER を実行してください")
		}
		if strings.Contains(output, "not accepting") || strings.Contains(output, "disabled") {
			return fmt.Errorf("PRINTER_DISABLED: プリンタが無効化されています。CUPSの管理画面でプリンタを有効にしてください")
		}
		if strings.Contains(output, "paper") || strings.Contains(output, "media") {
			return fmt.Errorf("PRINTER_PAPER_ERROR: ラベル用紙を確認してください。用紙切れまたはジャムの可能性があります")
		}
		return fmt.Errorf("PRINTER_ERROR: 印刷エラー: %s", output)
	}

	b.logger.Info("test print sent via lp")
	return nil
}
