package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/deervery/raku-sika-hub/internal/logging"
	"github.com/deervery/raku-sika-hub/internal/printer"
	"github.com/deervery/raku-sika-hub/internal/scale"
)

// Handler processes incoming WebSocket messages.
type Handler struct {
	scaleClient *scale.Client
	printer     printer.Driver
	hub         *Hub
	logger      *logging.Logger
}

type HealthSnapshot struct {
	Connected         bool     `json:"connected"`
	Port              string   `json:"port"`
	PrinterConnected  bool     `json:"printerConnected"`
	ConfiguredPrinter string   `json:"configuredPrinter"`
	SelectedPrinter   string   `json:"selectedPrinter"`
	AvailablePrinters []string `json:"availablePrinters"`
}

// NewHandler creates a new Handler.
func NewHandler(scaleClient *scale.Client, printer printer.Driver, hub *Hub, logger *logging.Logger) *Handler {
	return &Handler{
		scaleClient: scaleClient,
		printer:     printer,
		hub:         hub,
		logger:      logger,
	}
}

func (h *Handler) PrinterQueue() (printer.QueueStatus, error) {
	return h.printer.Queue()
}

func (h *Handler) ClearPrinterQueue() (printer.QueueStatus, error) {
	return h.printer.ClearQueue()
}

func (h *Handler) PreviewLabel(raw []byte) ([]byte, error) {
	_, data, err := h.parsePrintRequest(raw)
	if err != nil {
		return nil, err
	}
	if !h.printer.CanPrintLabels() {
		return nil, fmt.Errorf("PRINTER_ERROR: 印刷ドライバがラベルプレビューを処理できません。printerDriver / fontPath 設定を確認してください。")
	}
	return h.printer.PreviewLabel(data)
}

// HandleMessage parses and dispatches a WebSocket request.
func (h *Handler) HandleMessage(ctx context.Context, client *WSClient, raw []byte) {
	// Empty message check
	if len(raw) == 0 || strings.TrimSpace(string(raw)) == "" {
		client.Send(ErrorResponse{
			Type:    "error",
			Code:    ErrCodeInvalidRequest,
			Message: "空のメッセージです。JSON形式で送信してください。",
		})
		return
	}

	var req Request
	if err := json.Unmarshal(raw, &req); err != nil {
		client.Send(ErrorResponse{
			Type:    "error",
			Code:    ErrCodeInvalidRequest,
			Message: "JSONパースエラー。正しいJSON形式で送信してください。",
		})
		return
	}

	// type field validation
	if req.Type == "" {
		client.Send(ErrorResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Code:      ErrCodeInvalidRequest,
			Message:   "type フィールドが必要です。例: {\"type\": \"weigh\", \"requestId\": \"1\"}",
		})
		return
	}

	switch req.Type {
	case "weigh":
		go h.handleWeigh(ctx, client, req)
	case "tare":
		go h.handleTare(ctx, client, req)
	case "zero":
		go h.handleZero(ctx, client, req)
	case "health":
		go h.handleHealth(ctx, client, req)
	case "status":
		h.SendCurrentStatus(client)
	case "print_test":
		go h.handlePrintTest(ctx, client, req)
	case "print":
		go h.handlePrint(ctx, client, raw)
	default:
		client.Send(ErrorResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Code:      ErrCodeUnknownType,
			Message:   "不明なリクエストタイプ: \"" + req.Type + "\"。使用可能: weigh, tare, zero, health, status, print_test, print",
		})
	}
}

// SendCurrentStatus sends the current connection status to a single client.
func (h *Handler) SendCurrentStatus(client *WSClient) {
	client.Send(ConnectionStatus{
		Type:      "connection_status",
		Connected: h.scaleClient.Connected(),
		Port:      h.scaleClient.PortName(),
	})
}

func (h *Handler) handleWeigh(ctx context.Context, client *WSClient, req Request) {
	// Pre-check
	if !h.scaleClient.Connected() {
		client.Send(ErrorResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Code:      ErrCodeScaleNotConnected,
			Message:   "スケールが接続されていません。USBケーブルを確認してください。再接続ループが自動で試行中です。",
		})
		return
	}

	result, err := h.scaleClient.Weigh(ctx, func(retry, maxRetry int) {
		client.Send(WeighingProgress{
			Type:      "weighing",
			RequestID: req.RequestID,
			Retry:     retry,
			MaxRetry:  maxRetry,
		})
	})
	if err != nil {
		code, message := classifyScaleError(err)
		if ctx.Err() != nil {
			code = ErrCodeTimeout
			message = "リクエストがタイムアウトしました。"
		}
		client.Send(ErrorResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Code:      code,
			Message:   message,
		})
		return
	}

	client.Send(WeightResponse{
		Type:      "weight",
		RequestID: req.RequestID,
		Value:     result.Value,
		Unit:      result.Unit,
		Stable:    result.Stable,
	})
}

func (h *Handler) handleTare(ctx context.Context, client *WSClient, req Request) {
	if !h.scaleClient.Connected() {
		client.Send(ErrorResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Code:      ErrCodeScaleNotConnected,
			Message:   "スケールが接続されていません。USBケーブルを確認してください。",
		})
		return
	}

	if err := h.scaleClient.Tare(ctx); err != nil {
		code, message := classifyScaleError(err)
		client.Send(ErrorResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Code:      code,
			Message:   message,
		})
		return
	}
	client.Send(TareOKResponse{
		Type:      "tare_ok",
		RequestID: req.RequestID,
	})
}

func (h *Handler) handleZero(ctx context.Context, client *WSClient, req Request) {
	if !h.scaleClient.Connected() {
		client.Send(ErrorResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Code:      ErrCodeScaleNotConnected,
			Message:   "スケールが接続されていません。USBケーブルを確認してください。",
		})
		return
	}

	if err := h.scaleClient.Zero(ctx); err != nil {
		code, message := classifyScaleError(err)
		client.Send(ErrorResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Code:      code,
			Message:   message,
		})
		return
	}
	client.Send(ZeroOKResponse{
		Type:      "zero_ok",
		RequestID: req.RequestID,
	})
}

func (h *Handler) handleHealth(ctx context.Context, client *WSClient, req Request) {
	if err := h.scaleClient.HealthCheck(ctx); err != nil {
		code, message := classifyScaleError(err)
		client.Send(ErrorResponse{
			Type:      "error",
			RequestID: req.RequestID,
			Code:      code,
			Message:   message,
		})
		return
	}
	client.Send(HealthResponse{
		Type:             "health_ok",
		RequestID:        req.RequestID,
		Connected:        h.scaleClient.Connected(),
		Port:             h.scaleClient.PortName(),
		PrinterConnected: h.printer.IsAvailable(),
	})
}

// SnapshotHealth returns the current health state for HTTP endpoints.
func (h *Handler) SnapshotHealth() HealthSnapshot {
	status, err := h.printer.Status()
	if err != nil {
		h.logger.Warn("printer status snapshot failed: %v", err)
	}

	return HealthSnapshot{
		Connected:         h.scaleClient.Connected(),
		Port:              h.scaleClient.PortName(),
		PrinterConnected:  err == nil && printerReady(status),
		ConfiguredPrinter: status.ConfiguredName,
		SelectedPrinter:   status.SelectedName,
		AvailablePrinters: status.Available,
	}
}

func printerReady(status printer.PrinterStatus) bool {
	if status.SelectedName == "" {
		return false
	}
	if status.Source != "configured" {
		return true
	}
	for _, name := range status.Available {
		if name == status.SelectedName {
			return true
		}
	}
	return false
}

func (h *Handler) handlePrintTest(ctx context.Context, client *WSClient, req Request) {
	err := h.printer.TestPrint()
	if err != nil {
		errMsg := err.Error()
		// The printer driver already classifies errors with prefixes
		code := ErrCodeUnknownError
		if strings.HasPrefix(errMsg, "PRINTER_NOT_CONFIGURED:") {
			code = "PRINTER_NOT_CONFIGURED"
		} else if strings.HasPrefix(errMsg, "PRINTER_OFFLINE:") {
			code = "PRINTER_OFFLINE"
		} else if strings.HasPrefix(errMsg, "PRINTER_PERMISSION_DENIED:") {
			code = "PRINTER_PERMISSION_DENIED"
		} else if strings.HasPrefix(errMsg, "PRINTER_DISABLED:") {
			code = "PRINTER_DISABLED"
		} else if strings.HasPrefix(errMsg, "PRINTER_PAPER_ERROR:") {
			code = "PRINTER_PAPER_ERROR"
		} else if strings.HasPrefix(errMsg, "PRINTER_ERROR:") {
			code = "PRINTER_ERROR"
		}
		client.Send(PrintTestErrorResponse{
			Type:      "print_test_error",
			RequestID: req.RequestID,
			Message:   errMsg,
		})
		h.logger.Warn("print_test failed: [%s] %s", code, errMsg)
		return
	}

	client.Send(PrintTestOKResponse{
		Type:      "print_test_ok",
		RequestID: req.RequestID,
		Message:   "テスト印刷を送信しました",
	})
}

func (h *Handler) handlePrint(ctx context.Context, client *WSClient, raw []byte) {
	var req PrintRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		client.Send(PrintErrorResponse{
			Type:    "print_error",
			Code:    ErrCodeInvalidRequest,
			Message: "印刷リクエストのJSONパースに失敗しました。",
		})
		return
	}

	copies, data, err := h.parsePrintRequest(raw)
	if err != nil {
		client.Send(PrintErrorResponse{
			Type:      "print_error",
			RequestID: req.RequestID,
			Code:      errorCodeForPrintError(err),
			Message:   err.Error(),
		})
		return
	}

	err = h.printer.PrintLabel(data)
	if err != nil {
		errMsg := err.Error()
		code := errorCodeForPrintError(err)
		client.Send(PrintErrorResponse{
			Type:      "print_error",
			RequestID: req.RequestID,
			Code:      code,
			Message:   errMsg,
		})
		h.logger.Warn("print failed: [%s] %s", code, errMsg)
		return
	}

	client.Send(PrintOKResponse{
		Type:      "print_ok",
		RequestID: req.RequestID,
		Message:   fmt.Sprintf("ラベルを%d部印刷しました", copies),
		Copies:    copies,
	})
}

func (h *Handler) parsePrintRequest(raw []byte) (int, printer.LabelData, error) {
	var req PrintRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return 0, printer.LabelData{}, fmt.Errorf("印刷リクエストのJSONパースに失敗しました。")
	}

	if !printer.ValidTemplates[req.Template] {
		return 0, printer.LabelData{}, fmt.Errorf("不明なテンプレート: %q。使用可能: traceable_deer, traceable_bear, non_traceable_deer, processed, pet", req.Template)
	}

	copies := req.Copies
	if copies < 1 {
		copies = 1
	}
	if copies > printer.MaxCopies {
		return 0, printer.LabelData{}, fmt.Errorf("印刷部数は1〜%dの範囲で指定してください。", printer.MaxCopies)
	}

	var missing []string
	for _, field := range printer.RequiredFields(req.Template) {
		if missingRequiredField(req.Data, field) {
			missing = append(missing, field)
		}
	}
	if len(missing) > 0 {
		return 0, printer.LabelData{}, fmt.Errorf("必須フィールドが不足しています: %s", strings.Join(missing, ", "))
	}

	if !h.printer.CanPrintLabels() {
		return 0, printer.LabelData{}, fmt.Errorf("PRINTER_ERROR: 印刷ドライバがラベル印刷を処理できません。printerDriver / fontPath 設定を確認してください。")
	}

	data := printer.LabelData{
		Template:               req.Template,
		Copies:                 copies,
		ProductName:            req.Data["productName"],
		ProductQuantity:        req.Data["productQuantity"],
		DeadlineDate:           req.Data["deadlineDate"],
		StorageTemperature:     req.Data["storageTemperature"],
		StorageMethod:          req.Data["storageMethod"],
		IndividualID:           req.Data["individualId"],
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
		IsHeatedMeatProducts:   req.Data["isHeatedMeatProducts"],
		AttentionText:          req.Data["attentionText"],
	}
	if data.IndividualID == "" {
		data.IndividualID = data.IndividualNumber
	}
	if data.IndividualNumber == "" {
		data.IndividualNumber = data.IndividualID
	}
	return copies, data, nil
}

func missingRequiredField(data map[string]string, field string) bool {
	switch field {
	case "individualId":
		return strings.TrimSpace(data["individualId"]) == "" && strings.TrimSpace(data["individualNumber"]) == ""
	default:
		return strings.TrimSpace(data[field]) == ""
	}
}

func errorCodeForPrintError(err error) string {
	errMsg := err.Error()
	code := "PRINTER_ERROR"
	switch {
	case strings.HasPrefix(errMsg, "PRINTER_NOT_CONFIGURED:"):
		code = "PRINTER_NOT_CONFIGURED"
	case strings.HasPrefix(errMsg, "PRINTER_OFFLINE:"):
		code = "PRINTER_OFFLINE"
	case strings.HasPrefix(errMsg, "PRINTER_PERMISSION_DENIED:"):
		code = "PRINTER_PERMISSION_DENIED"
	case strings.HasPrefix(errMsg, "PRINTER_DISABLED:"):
		code = "PRINTER_DISABLED"
	case strings.HasPrefix(errMsg, "PRINTER_PAPER_ERROR:"):
		code = "PRINTER_PAPER_ERROR"
	case strings.Contains(errMsg, "JSONパース"), strings.Contains(errMsg, "不明なテンプレート"), strings.Contains(errMsg, "必須フィールド"), strings.Contains(errMsg, "印刷部数"):
		code = ErrCodeInvalidRequest
	}
	return code
}

// classifyScaleError maps scale errors to specific error codes with Japanese messages.
func classifyScaleError(err error) (string, string) {
	msg := err.Error()

	switch {
	case strings.Contains(msg, "not connected"):
		return ErrCodeScaleNotConnected,
			"スケールが切断されました。USBケーブルを確認してください。自動再接続を試行中です。"

	case strings.Contains(msg, "UNSTABLE"):
		return ErrCodeUnstable,
			"計量値が安定しません（10回リトライ超過）。計量台の上の物が動いていないか、風や振動がないか確認してください。"

	case strings.Contains(msg, "OVERLOAD"):
		return ErrCodeOverload,
			"スケールが過負荷状態です。最大計量（60kg）を超える荷物が乗っています。"

	case strings.Contains(msg, "PORT_ERROR") && strings.Contains(msg, "write"):
		return ErrCodeSerialWriteError,
			"シリアルポートへの書き込みに失敗しました。USBケーブルが抜けた可能性があります。"

	case strings.Contains(msg, "PORT_ERROR") && strings.Contains(msg, "read"):
		return ErrCodePortError,
			"シリアルポートからの読み取りに失敗しました。スケールの電源とUSBケーブルを確認してください。"

	case strings.Contains(msg, "PORT_ERROR"):
		return ErrCodePortError,
			"シリアルポートエラー。USBケーブルを確認してください。詳細: " + msg

	case strings.Contains(msg, "Permission denied") || strings.Contains(msg, "EACCES"):
		return ErrCodePermissionDenied,
			"シリアルポートのアクセス権限がありません。sudo usermod -aG dialout $USER を実行して再ログインしてください。"

	case strings.Contains(msg, "EBUSY") || strings.Contains(msg, "Resource busy"):
		return ErrCodePortInUse,
			"シリアルポートが別のプロセスに使用されています。他にスケールを使用しているプログラムがないか確認してください。"

	case strings.Contains(msg, "FTDI_NOT_FOUND"):
		return ErrCodeFTDINotFound,
			msg + " USBケーブルを確認し、スケールの電源を入れてください。"

	case strings.Contains(msg, "unexpected tare"):
		return ErrCodeTareFailed,
			"風袋引きに失敗しました。スケールの状態を確認してください。詳細: " + msg

	case strings.Contains(msg, "unexpected zero"):
		return ErrCodeZeroFailed,
			"ゼロリセットに失敗しました。計量台に物が乗っていないか確認してください。詳細: " + msg

	case strings.Contains(msg, "unexpected header"):
		return ErrCodeUnexpectedResponse,
			"スケールから予期しない応答がありました。詳細: " + msg

	default:
		return ErrCodeUnknownError,
			"予期しないエラーが発生しました: " + msg
	}
}
