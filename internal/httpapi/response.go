package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/deervery/raku-sika-hub/internal/printer"
)

// SuccessResponse is a generic success JSON response.
type SuccessResponse struct {
	Status     string `json:"status"`
	PrintState string `json:"printState,omitempty"`
	JobID      string `json:"jobId,omitempty"`
	Message    string `json:"message,omitempty"`
	Copies     int    `json:"copies,omitempty"`
	// Diagnosis is attached when a print did not complete, so the tablet can
	// tell a wedged USB link from a backed-up queue instead of showing the
	// same "送信しました" for both.
	Diagnosis *printer.Diagnosis `json:"diagnosis,omitempty"`
}

// ErrorBody is the standard error response format.
type ErrorBody struct {
	Status  string `json:"status"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WeighResponse is returned by POST /scale/weigh on success.
type WeighResponse struct {
	Status string  `json:"status"`
	Value  float64 `json:"value"`
	Unit   string  `json:"unit"`
	Stable bool    `json:"stable"`
}

// WeighingResponse is returned when the scale is still unstable.
// Value/Unit contain the last unstable reading (may be useful as approximation).
type WeighingResponse struct {
	Status   string  `json:"status"`
	Retry    int     `json:"retry"`
	MaxRetry int     `json:"maxRetry"`
	Value    float64 `json:"value,omitempty"`
	Unit     string  `json:"unit,omitempty"`
}

// HealthResponse is the structured /health response.
type HealthResponse struct {
	Status  string        `json:"status"`
	Scale   ScaleHealth   `json:"scale"`
	Printer PrinterHealth `json:"printer"`
	Scanner ScannerHealth `json:"scanner"`
}

// ScaleHealth is the scale section of the health response.
type ScaleHealth struct {
	Connected bool   `json:"connected"`
	Port      string `json:"port,omitempty"`
}

// PrinterHealth is the printer section of the health response.
type PrinterHealth struct {
	Connected bool   `json:"connected"`
	Name      string `json:"name,omitempty"`
	// Model is the detected Brother QL model ("QL-800", "QL-820NWB"); empty
	// when the CUPS queue name carries no recognizable model.
	Model        string `json:"model,omitempty"`
	State        string `json:"state,omitempty"`
	DeviceURI    string `json:"deviceUri,omitempty"`
	BackendReady bool   `json:"backendReady"`
	BackendError string `json:"backendError,omitempty"`
	// Transport is "raw" when the hub encodes the raster itself for a CUPS
	// raw queue, "driver" when a CUPS driver renders the label.
	Transport string `json:"transport,omitempty"`
}

// ScannerHealth is the scanner section of the health response.
type ScannerHealth struct {
	Connected bool   `json:"connected"`
	Device    string `json:"device,omitempty"`
}

// WSStatusResponse is the structured HTTP fallback response for GET /ws/status.
type WSStatusResponse struct {
	Connected         bool     `json:"connected"`
	Port              string   `json:"port"`
	PrinterConnected  bool     `json:"printerConnected"`
	ConfiguredPrinter string   `json:"configuredPrinter"`
	SelectedPrinter   string   `json:"selectedPrinter"`
	SelectedModel     string   `json:"selectedModel,omitempty"`
	PrinterSource     string   `json:"printerSource,omitempty"`
	AvailablePrinters []string `json:"availablePrinters"`
}

// VersionResponse is returned by GET /version.
type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
	// Features lets raku-sika-ops check what this binary can do without
	// running it. "rakuql": it serves as the CUPS backend for rakuql:// queues.
	Features []string `json:"features"`
}

// hubFeatures is what VersionResponse.Features reports.
var hubFeatures = []string{"rakuql"}

// ScanResponse is returned by GET /scanner/scan.
type ScanResponse struct {
	Status    string  `json:"status"`
	Value     *string `json:"value"`
	ScannedAt *string `json:"scannedAt,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeSuccess(w http.ResponseWriter, msg string) {
	writeJSON(w, http.StatusOK, SuccessResponse{Status: "ok", Message: msg})
}

// QueueResponse is returned by GET /printer/queue.
type QueueResponse struct {
	Status       string     `json:"status"`
	Printer      string     `json:"printer,omitempty"`
	PrinterState string     `json:"printerState,omitempty"`
	DeviceURI    string     `json:"deviceUri,omitempty"`
	BackendReady bool       `json:"backendReady"`
	BackendError string     `json:"backendError,omitempty"`
	QueueState   string     `json:"queueState,omitempty"`
	JobCount     int        `json:"jobCount"`
	Clearable    bool       `json:"clearable"`
	Message      string     `json:"message,omitempty"`
	Jobs         []QueueJob `json:"jobs"`
	// RecentJobs are the printer's most recent finished jobs, newest first.
	// A label that printed wrong is already "completed" for CUPS, so it never
	// appears in Jobs — the tablet needs these to offer 再送信.
	RecentJobs []QueueJob `json:"recentJobs,omitempty"`
	// Diagnosis classifies the queue state for the tablet and the admin GUI.
	Diagnosis *printer.Diagnosis `json:"diagnosis,omitempty"`
}

// QueueJob represents a single CUPS print job.
type QueueJob struct {
	ID          string `json:"id"`
	Printer     string `json:"printer"`
	User        string `json:"user"`
	Size        string `json:"size"`
	SubmittedAt string `json:"submittedAt"`
	State       string `json:"state,omitempty"`
	// AgeSec is how long the hub has seen this job waiting.
	AgeSec int `json:"ageSec"`
}

func writeError(w http.ResponseWriter, httpStatus int, code, message string) {
	writeJSON(w, httpStatus, ErrorBody{Status: "error", Code: code, Message: message})
}
