package printer

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

// recoverPrintStack restarts the CUPS/ipp-usb/avahi print stack.
// It clears all pending CUPS jobs first, then restarts services.
// This is the equivalent of running restart-print-stack.sh.
func recoverPrintStack(printerName string, logger *logging.Logger) error {
	logger.Warn("printer recovery: starting print stack restart (printer=%q)", printerName)

	// Step 1: Cancel all pending CUPS jobs
	if printerName != "" {
		out, err := exec.Command("cancel", "-a", printerName).CombinedOutput()
		if err != nil {
			logger.Warn("printer recovery: cancel -a %s failed: %s %s", printerName, err, strings.TrimSpace(string(out)))
		} else {
			logger.Info("printer recovery: cancel -a %s OK", printerName)
		}
	} else {
		out, err := exec.Command("cancel", "-a").CombinedOutput()
		if err != nil {
			logger.Warn("printer recovery: cancel -a failed: %s %s", err, strings.TrimSpace(string(out)))
		} else {
			logger.Info("printer recovery: cancel -a OK")
		}
	}

	// Step 2: Restart services (best-effort — some may not exist on all systems)
	services := []string{"cups", "ipp-usb", "avahi-daemon"}
	for _, svc := range services {
		out, err := exec.Command("systemctl", "restart", svc).CombinedOutput()
		if err != nil {
			logger.Warn("printer recovery: systemctl restart %s failed: %s %s", svc, err, strings.TrimSpace(string(out)))
		} else {
			logger.Info("printer recovery: systemctl restart %s OK", svc)
		}
	}

	// Step 3: Wait for services to stabilize
	logger.Info("printer recovery: waiting 3s for services to stabilize")
	time.Sleep(3 * time.Second)

	logger.Info("printer recovery: print stack restart completed")
	return nil
}

// isRecoverableError returns true if the error is likely fixable by restarting
// the print stack (CUPS/ipp-usb/avahi). Config and permission errors are not recoverable.
func isRecoverableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "PRINTER_NOT_CONFIGURED:"):
		return false
	case strings.HasPrefix(msg, "PRINTER_PERMISSION_DENIED:"):
		return false
	case strings.HasPrefix(msg, "PRINTER_PAPER_ERROR:"):
		return false
	case strings.Contains(msg, "JSONパース"),
		strings.Contains(msg, "不明なテンプレート"),
		strings.Contains(msg, "必須フィールド"),
		strings.Contains(msg, "印刷部数"),
		strings.Contains(msg, "ラベルレンダラが初期化されていません"),
		strings.Contains(msg, "template map に"):
		return false
	default:
		return true
	}
}

// RecoverAndRetry attempts printer recovery then retries the print function once.
// Returns the original error if recovery or retry also fails.
func RecoverAndRetry(printerName string, logger *logging.Logger, printFn func() error, originalErr error) error {
	if !isRecoverableError(originalErr) {
		return originalErr
	}

	logger.Warn("printer recovery: attempting auto-recover after error: %s", originalErr)

	if err := recoverPrintStack(printerName, logger); err != nil {
		logger.Warn("printer recovery: stack restart failed: %s", err)
		return originalErr
	}

	logger.Info("printer recovery: retrying print (1/1)")
	retryErr := printFn()
	if retryErr != nil {
		logger.Warn("printer recovery: retry also failed: %s", retryErr)
		return fmt.Errorf("%w (自動復旧後の再試行でも失敗しました)", retryErr)
	}

	logger.Info("printer recovery: retry succeeded after auto-recover")
	return nil
}
