package printer

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

type PtouchTemplate struct {
	name         string
	address      string
	registryPath string
	registry     *TemplateRegistry
	logger       *logging.Logger
}

func NewPtouchTemplate(cfg DriverConfig, logger *logging.Logger) (*PtouchTemplate, error) {
	path := strings.TrimSpace(cfg.TemplateMapPath)
	if path == "" {
		path = DefaultTemplateMapPath()
	}

	registry, err := LoadTemplateRegistry(path)
	if err != nil {
		return nil, err
	}

	p := &PtouchTemplate{
		name:         strings.TrimSpace(cfg.PrinterName),
		address:      strings.TrimSpace(cfg.PrinterAddress),
		registryPath: path,
		registry:     registry,
		logger:       logger,
	}
	p.LogStatus("startup")
	return p, nil
}

func (p *PtouchTemplate) IsAvailable() bool {
	status, err := p.Status()
	if err != nil {
		return false
	}
	if err := validateTemplateStatus(status, p.address); err != nil {
		return false
	}
	return p.probeReachable(status.SelectedName) == nil
}

func (p *PtouchTemplate) Status() (PrinterStatus, error) {
	if p.address != "" {
		return PrinterStatus{
			ConfiguredName: p.name,
			SelectedName:   p.name,
			DefaultName:    "",
			Available:      []string{p.name},
			Source:         "ptouch-template-tcp",
		}, nil
	}

	status, err := resolveCUPSPrinterStatus(p.name)
	if err != nil {
		return PrinterStatus{}, err
	}
	if status.Source == "configured" {
		status.Source = "ptouch-template-cups"
	}
	return status, nil
}

func (p *PtouchTemplate) LogStatus(context string) {
	status, err := p.Status()
	if err != nil {
		p.logger.Warn("printer status (%s): configured=%q, address=%q, registry=%q, error=%v", context, p.name, p.address, p.registryPath, err)
		return
	}
	p.logger.Info(
		"printer status (%s): driver=ptouch_template, configured=%q, address=%q, selected=%q, source=%s, registry=%q, available=%s",
		context,
		status.ConfiguredName,
		p.address,
		status.SelectedName,
		status.Source,
		p.registryPath,
		formatPrinters(status.Available),
	)
}

func (p *PtouchTemplate) TestPrint() error {
	templateName := firstTemplateName(p.registry)
	if templateName == "" {
		return fmt.Errorf("PRINTER_ERROR: template map に印刷可能なテンプレートがありません")
	}
	data, err := p.registry.TestLabelData(templateName)
	if err != nil {
		return fmt.Errorf("PRINTER_ERROR: テスト印刷データの生成に失敗しました: %s", err)
	}

	p.logger.Info("test print requested: template=%s, configured=%q, address=%q", templateName, p.name, p.address)
	return p.PrintLabel(data)
}

func (p *PtouchTemplate) PrintLabel(data LabelData) error {
	status, err := p.Status()
	if err != nil {
		return fmt.Errorf("PRINTER_ERROR: プリンタ状態確認に失敗しました: %s", err)
	}
	p.logger.Info(
		"print label requested: driver=ptouch_template, template=%s, copies=%d, product=%s, configured=%q, address=%q, selected=%q, source=%s",
		data.Template,
		data.Copies,
		data.ProductName,
		status.ConfiguredName,
		p.address,
		status.SelectedName,
		status.Source,
	)

	if err := validateTemplateStatus(status, p.address); err != nil {
		return err
	}
	if err := p.probeReachable(status.SelectedName); err != nil {
		return err
	}

	entry, ok := p.registry.Entry(data.Template)
	if !ok {
		return fmt.Errorf("PRINTER_ERROR: template map にテンプレート %q がありません", data.Template)
	}

	payload, err := p.registry.RenderPayload(entry, data)
	if err != nil {
		return fmt.Errorf("PRINTER_ERROR: P-touch Template コマンド生成に失敗しました: %s", err)
	}

	copies := data.Copies
	if copies < 1 {
		copies = 1
	}
	for i := 0; i < copies; i++ {
		if err := p.sendPayload(status.SelectedName, payload); err != nil {
			return err
		}
	}

	p.logger.Info("label printed via ptouch_template: template=%s copies=%d printer=%q address=%q", data.Template, copies, status.SelectedName, p.address)
	return nil
}

func (p *PtouchTemplate) PreviewLabel(data LabelData) ([]byte, error) {
	return nil, fmt.Errorf("PRINTER_ERROR: ptouch_template ではプレビューを生成できません。printerDriver=cups_png を使用してください")
}

func (p *PtouchTemplate) CanPrintLabels() bool {
	return p.registry != nil
}

func (p *PtouchTemplate) Queue() (QueueStatus, error) {
	status, err := p.Status()
	if err != nil {
		return QueueStatus{}, err
	}
	if p.address != "" {
		return queueStatusForDirect(status.SelectedName, status.Source, p.address), nil
	}
	if status.SelectedName == "" {
		return QueueStatus{}, printerConfigError(status)
	}
	return readCUPSQueue(status.SelectedName, status.Source)
}

func (p *PtouchTemplate) ClearQueue() (QueueStatus, error) {
	status, err := p.Status()
	if err != nil {
		return QueueStatus{}, err
	}
	if p.address != "" {
		return queueStatusForDirect(status.SelectedName, status.Source, p.address), nil
	}
	if status.SelectedName == "" {
		return QueueStatus{}, printerConfigError(status)
	}
	return clearCUPSQueue(status.SelectedName, status.Source)
}

func (p *PtouchTemplate) sendPayload(selectedName string, payload []byte) error {
	p.logger.Info("ptouch payload: printer=%q address=%q bytes=%d", selectedName, p.address, len(payload))

	if p.address != "" {
		conn, err := net.DialTimeout("tcp", p.address, 3*time.Second)
		if err != nil {
			return fmt.Errorf("PRINTER_ERROR: プリンタ %q への TCP 接続に失敗しました: %s", p.address, err)
		}
		defer conn.Close()

		if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
			return fmt.Errorf("PRINTER_ERROR: プリンタ書き込み期限の設定に失敗しました: %s", err)
		}
		if _, err := conn.Write(payload); err != nil {
			return fmt.Errorf("PRINTER_ERROR: P-touch Template コマンド送信に失敗しました: %s", err)
		}
		return nil
	}

	cmd := exec.Command("lp", "-d", selectedName, "-o", "raw")
	cmd.Stdin = bytes.NewReader(payload)
	out, err := cmd.CombinedOutput()
	outText := strings.TrimSpace(string(out))
	p.logger.Info("lp output (ptouch template, printer=%q): %s", selectedName, outText)
	if err != nil {
		return classifyLpError(string(out), PrinterStatus{
			ConfiguredName: p.name,
			SelectedName:   selectedName,
			Available:      []string{selectedName},
			Source:         "ptouch-template-cups",
		})
	}
	jobID := parseLPRequestID(outText)
	if jobID == "" {
		return fmt.Errorf("PRINTER_ERROR: CUPS ジョブIDを取得できませんでした: %s", outText)
	}
	if err := waitForCUPSJobToLeaveQueue(selectedName, jobID, 12*time.Second); err != nil {
		return err
	}
	return nil
}

func (p *PtouchTemplate) probeReachable(selectedName string) error {
	if p.address != "" {
		return probeTCPAddress(p.address)
	}

	deviceURI, err := lookupCUPSDeviceURI(selectedName)
	if err != nil {
		return fmt.Errorf("PRINTER_ERROR: CUPS デバイスURIの取得に失敗しました: %s", err)
	}
	if deviceURI == "" {
		return nil
	}

	address, err := endpointFromDeviceURI(deviceURI)
	if err != nil {
		p.logger.Warn("printer probe skipped: printer=%q uri=%q err=%v", selectedName, deviceURI, err)
		return nil
	}
	if err := probeTCPAddress(address); err != nil {
		return fmt.Errorf(
			"PRINTER_OFFLINE: プリンタとの通信確認に失敗しました。 printer=%q uri=%q address=%q err=%s",
			selectedName,
			deviceURI,
			address,
			err,
		)
	}
	return nil
}

func probeTCPAddress(address string) error {
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		return err
	}
	defer conn.Close()
	return nil
}

func lookupCUPSDeviceURI(selectedName string) (string, error) {
	out, err := exec.Command("lpstat", "-v", selectedName).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}

	line := strings.TrimSpace(string(out))
	prefix := "device for " + selectedName + ":"
	if !strings.HasPrefix(line, prefix) {
		return "", fmt.Errorf("unexpected lpstat -v output: %s", line)
	}
	return strings.TrimSpace(strings.TrimPrefix(line, prefix)), nil
}

func endpointFromDeviceURI(deviceURI string) (string, error) {
	u, err := url.Parse(deviceURI)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("uri has no host")
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "ipp", "ipps":
			port = "631"
		case "socket":
			port = "9100"
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			return "", fmt.Errorf("unsupported scheme: %s", u.Scheme)
		}
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("invalid port: %s", port)
	}
	return net.JoinHostPort(host, port), nil
}

func validateTemplateStatus(status PrinterStatus, address string) error {
	if address != "" {
		return nil
	}
	return validateStatus(status)
}

func firstTemplateName(registry *TemplateRegistry) string {
	if registry == nil {
		return ""
	}
	for _, name := range []string{"traceable_deer", "traceable_bear", "non_traceable_deer", "traceable", "non_traceable", "processed", "pet"} {
		if _, ok := registry.Entry(name); ok {
			return name
		}
	}
	for name := range registry.Templates {
		return name
	}
	return ""
}
