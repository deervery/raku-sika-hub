package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
)

// HandlePrinterJob handles DELETE /printer/jobs/{id}.
//
// The whole-queue DELETE /printer/queue is the blunt instrument; this is the
// one an operator reaches for when a single job is wedged and the rest of the
// queue is healthy — the shape of the 2026-09-22 シクヌ incident, where one
// zero-byte job held up five good ones.
func (h *Handler) HandlePrinterJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID := strings.TrimSpace(r.PathValue("id"))
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "ジョブ ID が指定されていません。")
		return
	}
	if err := h.printer.CancelJob(jobID); err != nil {
		status := http.StatusInternalServerError
		code := "PRINTER_ERROR"
		if strings.HasPrefix(err.Error(), "PRINTER_NOT_FOUND") {
			status = http.StatusNotFound
			code = "PRINTER_NOT_FOUND"
		}
		writeError(w, status, code, err.Error())
		return
	}
	h.logger.Info("print job cancelled via admin API: %s", jobID)
	writeSuccess(w, "印刷ジョブを削除しました。")
}

// HandlePrinterJobRestart handles POST /printer/jobs/{id}/restart.
//
// 再送信: CUPS keeps the spooled document for a while after a job finishes, so
// a label that came out wrong (wrong roll, half-cut, blank) can be sent again
// without the operator re-entering anything on the tablet.
func (h *Handler) HandlePrinterJobRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobID := strings.TrimSpace(r.PathValue("id"))
	if jobID == "" {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "ジョブ ID が指定されていません。")
		return
	}
	if err := h.printer.RestartJob(jobID); err != nil {
		status := http.StatusInternalServerError
		code := "PRINTER_ERROR"
		if strings.HasPrefix(err.Error(), "PRINTER_NOT_FOUND") {
			status = http.StatusNotFound
			code = "PRINTER_NOT_FOUND"
		}
		writeError(w, status, code, err.Error())
		return
	}
	h.logger.Info("print job restarted via API: %s", jobID)
	writeSuccess(w, "印刷ジョブを再送信しました。")
}

// NetworkConnectRequest is the body of POST /system/network/connect.
type NetworkConnectRequest struct {
	Profile string `json:"profile"`
}

// HandleNetwork handles GET /system/network.
func (h *Handler) HandleNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	status, err := h.network.Status()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "NETWORK_ERROR",
			"ネットワーク状態の取得に失敗しました: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "network": status})
}

// HandleNetworkConnect handles POST /system/network/connect.
//
// Switching the Wi-Fi profile can drop the very connection carrying this
// request. That is expected: the caller is either the Pi's own kiosk browser
// (unaffected) or an operator who accepts the reconnect.
func (h *Handler) HandleNetworkConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req NetworkConnectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_REQUEST", "JSONパースエラー。正しいJSON形式で送信してください。")
		return
	}
	if err := h.network.Activate(req.Profile); err != nil {
		writeError(w, http.StatusInternalServerError, "NETWORK_ERROR",
			"接続の切り替えに失敗しました: "+err.Error())
		return
	}
	h.logger.Info("wifi profile activated via admin API: %s", req.Profile)
	writeSuccess(w, "接続を切り替えました。")
}
