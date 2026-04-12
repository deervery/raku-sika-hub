package ws

// Request represents an incoming WebSocket message from a client.
type Request struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
}

// PrintRequest represents a label print request.
type PrintRequest struct {
	Type      string            `json:"type"`
	RequestID string            `json:"requestId,omitempty"`
	Template  string            `json:"template"`  // traceable, non_traceable, processed, pet
	Copies    int               `json:"copies"`    // 1-30
	Data      map[string]string `json:"data"`      // label field values
}

// PrintOKResponse is sent when a label print succeeds.
type PrintOKResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Message   string `json:"message,omitempty"`
	Copies    int    `json:"copies"`
}

// PrintErrorResponse is sent when a label print fails.
type PrintErrorResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// WeightResponse is sent when a stable weight is obtained.
type WeightResponse struct {
	Type      string  `json:"type"`
	RequestID string  `json:"requestId,omitempty"`
	Value     float64 `json:"value"`
	Unit      string  `json:"unit"`
	Stable    bool    `json:"stable"`
}

// WeighingProgress is sent during retry loops while waiting for stable weight.
type WeighingProgress struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Retry     int    `json:"retry"`
	MaxRetry  int    `json:"maxRetry"`
}

// TareOKResponse is sent when tare completes successfully.
type TareOKResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
}

// ZeroOKResponse is sent when zero reset completes successfully.
type ZeroOKResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
}

// HealthResponse is sent when an on-demand health check succeeds.
type HealthResponse struct {
	Type             string `json:"type"`
	RequestID        string `json:"requestId,omitempty"`
	Connected        bool   `json:"connected"`
	Port             string `json:"port,omitempty"`
	PrinterConnected bool   `json:"printerConnected"`
}

// ErrorResponse is sent when an operation fails.
type ErrorResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message"`
}

// ConnectionStatus is pushed when the scale connection state changes.
type ConnectionStatus struct {
	Type      string `json:"type"`
	Connected bool   `json:"connected"`
	Port      string `json:"port,omitempty"`
}

// PrintTestOKResponse is sent when a test print succeeds.
type PrintTestOKResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Message   string `json:"message,omitempty"`
}

// PrintTestErrorResponse is sent when a test print fails.
type PrintTestErrorResponse struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId,omitempty"`
	Message   string `json:"message"`
}

// PrintProgressEvent is broadcast to all clients when a print job completes or fails.
type PrintProgressEvent struct {
	Type     string `json:"type"`               // "print_progress"
	Status   string `json:"status"`             // "done" | "failed"
	Template string `json:"template"`           // テンプレートID
	Copies   int    `json:"copies"`             // 印刷部数
	Error    string `json:"error,omitempty"`    // エラーメッセージ（失敗時のみ）
}

// PrinterStatusEvent is broadcast to all clients when printer connection state changes.
type PrinterStatusEvent struct {
	Type             string `json:"type"`             // "printer_status"
	PrinterConnected bool   `json:"printerConnected"`
	PrinterName      string `json:"printerName"`
}

// Error codes.
const (
	ErrCodeUnstable           = "UNSTABLE"
	ErrCodeTimeout            = "TIMEOUT"
	ErrCodeOverload           = "OVERLOAD"
	ErrCodePortError          = "PORT_ERROR"
	ErrCodeScaleNotConnected  = "SCALE_NOT_CONNECTED"
	ErrCodeScaleBusy          = "SCALE_BUSY"
	ErrCodeSerialWriteError   = "SERIAL_WRITE_ERROR"
	ErrCodePermissionDenied   = "PERMISSION_DENIED"
	ErrCodePortInUse          = "PORT_IN_USE"
	ErrCodePortNotFound       = "PORT_NOT_FOUND"
	ErrCodeFTDINotFound       = "FTDI_NOT_FOUND"
	ErrCodeScaleNoResponse    = "SCALE_NO_RESPONSE"
	ErrCodeUnexpectedResponse = "UNEXPECTED_RESPONSE"
	ErrCodeTareFailed         = "TARE_FAILED"
	ErrCodeZeroFailed         = "ZERO_FAILED"
	ErrCodeInvalidRequest     = "INVALID_REQUEST"
	ErrCodeUnknownType        = "UNKNOWN_TYPE"
	ErrCodeUnknownError       = "UNKNOWN_ERROR"
)
