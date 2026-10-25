package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
)

// DurablePrint protects opt-in print requests against retries and process restarts.
// A claim is fsynced BEFORE touching the printer. An interrupted claim is unknown,
// never an invitation to print again. Existing calls without a key are unchanged.
// dir must be local persistent storage, shared by all processes serving this printer.
func DurablePrint(dir string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("Idempotency-Key")
		if key == "" || r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}
		if !printRequestKey.MatchString(key) {
			writeError(w, 400, "INVALID_REQUEST", "印刷要求IDが不正です。")
			return
		}
		if dir == "" {
			writeError(w, 503, "PRINT_RECEIPTS_UNAVAILABLE", "永続的な印刷受付記録が未設定です。印刷していません。")
			return
		}
		body, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024+1))
		if err != nil || len(body) > 1024*1024 {
			writeError(w, 400, "INVALID_REQUEST", "印刷要求を読み取れません。")
			return
		}
		var value any
		decoder := json.NewDecoder(bytes.NewReader(body))
		decoder.UseNumber()
		if decoder.Decode(&value) != nil {
			writeError(w, 400, "INVALID_REQUEST", "印刷要求JSONが不正です。")
			return
		}
		var trailing any
		if decoder.Decode(&trailing) != io.EOF {
			writeError(w, 400, "INVALID_REQUEST", "印刷要求JSONが不正です。")
			return
		}
		canonical, err := json.Marshal(value)
		if err != nil {
			writeError(w, 400, "INVALID_REQUEST", "印刷要求JSONが不正です。")
			return
		}
		digest := sha256.Sum256(canonical)
		keyHash := sha256.Sum256([]byte(key))
		path := filepath.Join(dir, hex.EncodeToString(keyHash[:])+".json")
		if err = os.MkdirAll(dir, 0700); err != nil {
			receiptUnavailable(w)
			return
		}
		claim := printReceipt{PayloadHash: hex.EncodeToString(digest[:]), State: "unknown"}
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if errors.Is(err, os.ErrExist) {
			replayPrint(w, path, claim.PayloadHash)
			return
		}
		if err != nil {
			receiptUnavailable(w)
			return
		}
		err = json.NewEncoder(file).Encode(claim)
		if err == nil {
			err = file.Sync()
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = syncReceiptDir(dir)
		}
		if err != nil {
			receiptUnavailable(w)
			return
		}
		// Never remove the claim, including panics or errors: output may already exist.
		r.Body = io.NopCloser(bytes.NewReader(body))
		recorder := httptest.NewRecorder()
		next.ServeHTTP(recorder, r)
		claim.State = "recorded"
		claim.Status = recorder.Code
		claim.Body = recorder.Body.Bytes()
		claim.ContentType = recorder.Header().Get("Content-Type")
		if err = savePrintReceipt(dir, path, claim); err != nil {
			printUnknown(w)
			return
		}
		sendPrintReceipt(w, claim)
	})
}

var printRequestKey = regexp.MustCompile(`^[A-Za-z0-9:_-]{16,128}$`)

type printReceipt struct {
	PayloadHash string `json:"payloadHash"`
	State       string `json:"state"`
	Status      int    `json:"status,omitempty"`
	Body        []byte `json:"body,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

func syncReceiptDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer f.Close()
	return f.Sync()
}
func savePrintReceipt(dir, path string, receipt printReceipt) error {
	f, err := os.CreateTemp(dir, ".receipt-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	err = json.NewEncoder(f).Encode(receipt)
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	return syncReceiptDir(dir)
}
func replayPrint(w http.ResponseWriter, path, digest string) {
	raw, err := os.ReadFile(path)
	var receipt printReceipt
	if err != nil || json.Unmarshal(raw, &receipt) != nil {
		printUnknown(w)
		return
	}
	if receipt.PayloadHash != digest {
		writeError(w, 409, "PRINT_REQUEST_CONFLICT", "同じ印刷要求IDで内容が変更されています。再印刷していません。")
		return
	}
	if receipt.State != "recorded" || receipt.Status < 100 || receipt.Status > 599 {
		printUnknown(w)
		return
	}
	sendPrintReceipt(w, receipt)
}
func sendPrintReceipt(w http.ResponseWriter, receipt printReceipt) {
	if receipt.ContentType != "" {
		w.Header().Set("Content-Type", receipt.ContentType)
	}
	w.WriteHeader(receipt.Status)
	_, _ = w.Write(receipt.Body)
}
func receiptUnavailable(w http.ResponseWriter) {
	writeError(w, 503, "PRINT_RECEIPTS_UNAVAILABLE", "印刷受付記録を保存できません。印刷していません。")
}
func printUnknown(w http.ResponseWriter) {
	writeError(w, 409, "PRINT_OUTCOME_UNKNOWN", "印刷結果が未確定です。再印刷せず、用紙と印刷キューを確認してください。")
}
