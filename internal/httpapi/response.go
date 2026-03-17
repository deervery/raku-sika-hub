package httpapi

import (
	"encoding/json"
	"net/http"
)

// SuccessResponse is a generic success JSON response.
type SuccessResponse struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
	Copies  int    `json:"copies,omitempty"`
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
type WeighingResponse struct {
	Status   string `json:"status"`
	Retry    int    `json:"retry"`
	MaxRetry int    `json:"maxRetry"`
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
}

// ScannerHealth is the scanner section of the health response.
type ScannerHealth struct {
	Connected bool   `json:"connected"`
	Device    string `json:"device,omitempty"`
}

// VersionResponse is returned by GET /version.
type VersionResponse struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"buildDate"`
}

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

func writeError(w http.ResponseWriter, httpStatus int, code, message string) {
	writeJSON(w, httpStatus, ErrorBody{Status: "error", Code: code, Message: message})
}
