package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/deervery/raku-sika-hub/internal/logging"
	"github.com/deervery/raku-sika-hub/internal/printer"
	"github.com/deervery/raku-sika-hub/internal/scale"
)

// ScannerClient is the interface for the barcode scanner.
type ScannerClient interface {
	Connected() bool
	DevicePath() string
	Consume() (value string, scannedAt string, ok bool)
}

// PrintRequest is the JSON body for POST /printer/print and /printer/preview.
type PrintRequest struct {
	Template string            `json:"template"`
	Copies   int               `json:"copies"`
	Data     map[string]string `json:"data"`
}

// Handler holds references to all service components.
type Handler struct {
	scaleClient       *scale.Client
	printer           *printer.Brother
	scanner           ScannerClient
	logger            *logging.Logger
	version           string
	commit            string
	buildDate         string
	assetsDir         string
	processorName     string
	processorLocation string
	captureLocation   string
}

// NewHandler creates a Handler.
func NewHandler(
	scaleClient *scale.Client,
	prn *printer.Brother,
	scanner ScannerClient,
	logger *logging.Logger,
	version, commit, buildDate, assetsDir, processorName, processorLocation, captureLocation string,
) *Handler {
	return &Handler{
		scaleClient:       scaleClient,
		printer:           prn,
		scanner:           scanner,
		logger:            logger,
		version:           version,
		commit:            commit,
		buildDate:         buildDate,
		assetsDir:         assetsDir,
		processorName:     processorName,
		processorLocation: processorLocation,
		captureLocation:   captureLocation,
	}
}

// HandleHealth handles GET /health.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	status, err := h.printer.Status()
	printerConnected := err == nil && status.SelectedName != ""
	printerName := ""
	if err == nil {
		printerName = status.SelectedName
	}

	scannerConnected := false
	scannerDevice := ""
	if h.scanner != nil {
		scannerConnected = h.scanner.Connected()
		scannerDevice = h.scanner.DevicePath()
	}

	resp := HealthResponse{
		Status: "ok",
		Scale: ScaleHealth{
			Connected: h.scaleClient.Connected(),
			Port:      h.scaleClient.PortName(),
		},
		Printer: PrinterHealth{
			Connected: printerConnected,
			Name:      printerName,
		},
		Scanner: ScannerHealth{
			Connected: scannerConnected,
			Device:    scannerDevice,
		},
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleVersion handles GET /version.
func (h *Handler) HandleVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, VersionResponse{
		Version:   h.version,
		Commit:    h.commit,
		BuildDate: h.buildDate,
	})
}

// HandleScaleWeigh handles POST /scale/weigh.
func (h *Handler) HandleScaleWeigh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.scaleClient.Connected() {
		writeError(w, http.StatusServiceUnavailable, "SCALE_NOT_CONNECTED",
			"スケールが接続されていません。USBケーブルを確認してください。")
		return
	}

	// Use a variable to capture the last progress state for the response.
	var lastRetry, lastMaxRetry int
	var weighingInProgress bool

	result, err := h.scaleClient.Weigh(r.Context(), func(retry, maxRetry int) {
		lastRetry = retry
		lastMaxRetry = maxRetry
		weighingInProgress = true
	})
	if err != nil {
		// If we got progress updates and the error is UNSTABLE, return weighing status.
		if weighingInProgress && strings.Contains(err.Error(), "UNSTABLE") {
			writeJSON(w, http.StatusOK, WeighingResponse{
				Status:   "weighing",
				Retry:    lastRetry,
				MaxRetry: lastMaxRetry,
			})
			return
		}
		code, message := classifyScaleError(err)
		writeError(w, http.StatusInternalServerError, code, message)
		return
	}

	writeJSON(w, http.StatusOK, WeighResponse{
		Status: "ok",
		Value:  result.Value,
		Unit:   result.Unit,
		Stable: result.Stable,
	})
}

// HandleScaleTare handles POST /scale/tare.
func (h *Handler) HandleScaleTare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.scaleClient.Connected() {
		writeError(w, http.StatusServiceUnavailable, "SCALE_NOT_CONNECTED",
			"スケールが接続されていません。USBケーブルを確認してください。")
		return
	}

	if err := h.scaleClient.Tare(r.Context()); err != nil {
		code, message := classifyScaleError(err)
		writeError(w, http.StatusInternalServerError, code, message)
		return
	}
	writeJSON(w, http.StatusOK, SuccessResponse{Status: "ok"})
}

// HandleScaleZero handles POST /scale/zero.
func (h *Handler) HandleScaleZero(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if !h.scaleClient.Connected() {
		writeError(w, http.StatusServiceUnavailable, "SCALE_NOT_CONNECTED",
			"スケールが接続されていません。USBケーブルを確認してください。")
		return
	}

	if err := h.scaleClient.Zero(r.Context()); err != nil {
		code, message := classifyScaleError(err)
		writeError(w, http.StatusInternalServerError, code, message)
		return
	}
	writeJSON(w, http.StatusOK, SuccessResponse{Status: "ok"})
}

// HandlePrinterPrint handles POST /printer/print.
func (h *Handler) HandlePrinterPrint(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "JSONパースエラー。正しいJSON形式で送信してください。")
		return
	}

	labelData, httpStatus, errBody := h.validateAndBuildLabelData(req)
	if errBody != nil {
		writeJSON(w, httpStatus, errBody)
		return
	}

	if err := h.printer.PrintLabel(*labelData); err != nil {
		code := classifyPrinterError(err)
		writeError(w, http.StatusInternalServerError, code, err.Error())
		h.logger.Warn("print failed: [%s] %s", code, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{
		Status:  "ok",
		Copies:  labelData.Copies,
		Message: "印刷完了",
	})
}

// HandlePrinterPreview handles POST /printer/preview.
func (h *Handler) HandlePrinterPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PrintRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "JSONパースエラー。")
		return
	}

	labelData, httpStatus, errBody := h.validateAndBuildLabelData(req)
	if errBody != nil {
		writeJSON(w, httpStatus, errBody)
		return
	}

	if !h.printer.CanRenderLabels() {
		writeError(w, http.StatusInternalServerError, "PRINTER_ERROR",
			"ラベルレンダラが初期化されていません。日本語フォントをインストールしてください。")
		return
	}

	imgPath, err := h.printer.RenderLabel(*labelData)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "PRINTER_ERROR",
			"ラベル画像の生成に失敗しました: "+err.Error())
		return
	}
	defer func(path string) {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			h.logger.Warn("preview temp file cleanup failed (%s): %v", path, err)
		}
	}(imgPath)

	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, r, imgPath)
}

// HandlePrinterQueue handles GET /printer/queue.
// Returns the CUPS print job queue via `lpstat -o`.
func (h *Handler) HandlePrinterQueue(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	out, err := exec.Command("lpstat", "-o").CombinedOutput()
	if err != nil {
		// lpstat returns exit code 1 when there are no jobs — treat as empty
		writeJSON(w, http.StatusOK, QueueResponse{Status: "ok", Jobs: []QueueJob{}})
		return
	}

	jobs := parseLpstatOutput(string(out))
	writeJSON(w, http.StatusOK, QueueResponse{Status: "ok", Jobs: jobs})
}

// parseLpstatOutput parses `lpstat -o` output.
// Each line: "PrinterName-123  user  1024  Mon 24 Mar 2025 10:30:00"
func parseLpstatOutput(output string) []QueueJob {
	var jobs []QueueJob
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split by whitespace: id, user, size, rest is date
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		id := fields[0]
		// Extract printer name from job id (e.g. "Brother_QL-800-123" → "Brother_QL-800")
		printerName := id
		if idx := strings.LastIndex(id, "-"); idx > 0 {
			printerName = id[:idx]
		}
		user := fields[1]
		size := fields[2]
		submittedAt := strings.Join(fields[3:], " ")

		jobs = append(jobs, QueueJob{
			ID:          id,
			Printer:     strings.ReplaceAll(printerName, "_", " "),
			User:        user,
			Size:        size,
			SubmittedAt: submittedAt,
		})
	}
	return jobs
}

// HandlePrinterTest handles POST /printer/test.
func (h *Handler) HandlePrinterTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.printer.TestPrint(); err != nil {
		code := classifyPrinterError(err)
		writeError(w, http.StatusInternalServerError, code, err.Error())
		h.logger.Warn("test print failed: [%s] %s", code, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Status: "ok", Message: "テスト印刷完了"})
}

// HandleScannerScan handles GET /scanner/scan.
func (h *Handler) HandleScannerScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if h.scanner == nil || !h.scanner.Connected() {
		writeError(w, http.StatusServiceUnavailable, "SCANNER_NOT_CONNECTED",
			"バーコードリーダーが接続されていません")
		return
	}

	value, scannedAt, ok := h.scanner.Consume()
	if !ok {
		writeJSON(w, http.StatusOK, ScanResponse{Status: "ok", Value: nil})
		return
	}

	writeJSON(w, http.StatusOK, ScanResponse{
		Status:    "ok",
		Value:     &value,
		ScannedAt: &scannedAt,
	})
}

func (h *Handler) validateAndBuildLabelData(req PrintRequest) (*printer.LabelData, int, *ErrorBody) {
	if !printer.ValidTemplates[req.Template] {
		return nil, http.StatusBadRequest, &ErrorBody{
			Status:  "error",
			Code:    "INVALID_REQUEST",
			Message: "不明なテンプレート: \"" + req.Template + "\"",
		}
	}

	copies := req.Copies
	if copies < 1 {
		copies = 1
	}
	if copies > printer.MaxCopies {
		return nil, http.StatusBadRequest, &ErrorBody{
			Status:  "error",
			Code:    "INVALID_REQUEST",
			Message: fmt.Sprintf("印刷部数は1〜%dの範囲で指定してください。", printer.MaxCopies),
		}
	}

	// Normalize field aliases: storageMethod → storageTemperature
	if req.Data["storageTemperature"] == "" && req.Data["storageMethod"] != "" {
		req.Data["storageTemperature"] = req.Data["storageMethod"]
	}

	required := printer.RequiredFields(req.Template)
	var missing []string
	for _, f := range required {
		if req.Data[f] == "" {
			missing = append(missing, f)
		}
	}
	if len(missing) > 0 {
		return nil, http.StatusBadRequest, &ErrorBody{
			Status:  "error",
			Code:    "INVALID_REQUEST",
			Message: "必須フィールドが不足しています: " + strings.Join(missing, ", "),
		}
	}

	if !h.printer.CanRenderLabels() {
		return nil, http.StatusInternalServerError, &ErrorBody{
			Status:  "error",
			Code:    "PRINTER_ERROR",
			Message: "ラベルレンダラが初期化されていません。日本語フォントをインストールしてください: sudo apt-get install fonts-noto-cjk",
		}
	}

	data := printer.LabelData{
		Template:               req.Template,
		Copies:                 copies,
		ProductName:            req.Data["productName"],
		ProductQuantity:        req.Data["productQuantity"],
		DeadlineDate:           req.Data["deadlineDate"],
		StorageTemperature:     req.Data["storageTemperature"],
		IndividualNumber:       req.Data["individualNumber"],
		CaptureLocation:        req.Data["captureLocation"],
		QRCode:                 req.Data["qrCode"],
		ProductIngredient:      req.Data["productIngredient"],
		NutritionUnit:          req.Data["nutritionUnit"],
		CaloriesQuantity:       req.Data["caloriesQuantity"],
		ProteinQuantity:        req.Data["proteinQuantity"],
		FatQuantity:            req.Data["fatQuantity"],
		CarbohydratesQuantity:  req.Data["carbohydratesQuantity"],
		SaltEquivalentQuantity: req.Data["saltEquivalentQuantity"],
		AttentionText:          req.Data["attentionText"],
		FacilityName:           req.Data["facilityName"],
		Ingredient:             req.Data["ingredient"],
		LogoFile:               h.resolveLogoField(req.Data["logoFile"]),
		CertificationMarkFile:  strings.TrimSpace(req.Data["certificationMarkFile"]),
		ProcessorName:          req.Data["processorName"],
		ProcessorLocation:      req.Data["processorLocation"],
	}

	// Apply defaults from config if not provided by client.
	if strings.TrimSpace(data.ProcessorName) == "" {
		data.ProcessorName = h.processorName
	}
	if strings.TrimSpace(data.ProcessorLocation) == "" {
		data.ProcessorLocation = h.processorLocation
	}
	if h.captureLocation != "" {
		data.CaptureLocation = h.captureLocation
	}

	return &data, 0, nil
}

func (h *Handler) resolveLogoField(input string) string {
	if trimmed := strings.TrimSpace(input); trimmed != "" {
		return trimmed
	}
	return printer.DefaultLogoFile(h.assetsDir)
}

// classifyScaleError maps scale errors to error codes and Japanese messages.
func classifyScaleError(err error) (string, string) {
	msg := err.Error()

	switch {
	case strings.Contains(msg, "not connected"):
		return "SCALE_NOT_CONNECTED",
			"スケールが切断されました。USBケーブルを確認してください。"
	case strings.Contains(msg, "UNSTABLE"):
		return "UNSTABLE",
			"計量値が安定しません。計量台の上の物が動いていないか確認してください。"
	case strings.Contains(msg, "OVERLOAD"):
		return "OVERLOAD",
			"スケールが過負荷状態です。最大計量（60kg）を超える荷物が乗っています。"
	case strings.Contains(msg, "PORT_ERROR"):
		return "PORT_ERROR",
			"シリアルポートエラー。USBケーブルを確認してください。"
	case strings.Contains(msg, "Permission denied"):
		return "PERMISSION_DENIED",
			"シリアルポートのアクセス権限がありません。"
	default:
		return "UNKNOWN_ERROR",
			"予期しないエラーが発生しました: " + msg
	}
}

func classifyPrinterError(err error) string {
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "PRINTER_NOT_CONFIGURED:"):
		return "PRINTER_NOT_CONFIGURED"
	case strings.HasPrefix(msg, "PRINTER_PERMISSION_DENIED:"):
		return "PRINTER_PERMISSION_DENIED"
	case strings.HasPrefix(msg, "PRINTER_DISABLED:"):
		return "PRINTER_DISABLED"
	case strings.HasPrefix(msg, "PRINTER_PAPER_ERROR:"):
		return "PRINTER_PAPER_ERROR"
	default:
		return "PRINTER_ERROR"
	}
}
