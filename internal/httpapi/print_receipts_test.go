package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

func receiptCall(handler http.Handler, key, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/printer/print", strings.NewReader(body))
	req.Header.Set("Idempotency-Key", key)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

const testPrintKey = "test-print-request-0001"

func TestDurablePrintRestartReplay(t *testing.T) {
	dir := t.TempDir()
	var calls atomic.Int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"printState":"done","jobId":"test-1"}`))
	})
	first := receiptCall(DurablePrint(dir, next), testPrintKey, `{"copies":1,"template":"pet"}`)
	replay := receiptCall(DurablePrint(dir, next), testPrintKey, `{"template":"pet", "copies":1}`)
	if calls.Load() != 1 || first.Body.String() != replay.Body.String() || replay.Code != 200 {
		t.Fatalf("calls=%d replay=%s", calls.Load(), replay.Body)
	}
	conflict := receiptCall(DurablePrint(dir, next), testPrintKey, `{"copies":2,"template":"pet"}`)
	if conflict.Code != 409 || calls.Load() != 1 {
		t.Fatal("changed payload executed")
	}
}
func TestDurablePrintConcurrentClaim(t *testing.T) {
	dir := t.TempDir()
	var calls atomic.Int32
	entered := make(chan struct{})
	release := make(chan struct{})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		close(entered)
		<-release
		w.Write([]byte("done"))
	})
	var group sync.WaitGroup
	group.Add(1)
	go func() { defer group.Done(); receiptCall(DurablePrint(dir, next), testPrintKey, `{}`) }()
	<-entered
	duplicate := receiptCall(DurablePrint(dir, next), testPrintKey, `{}`)
	if duplicate.Code != 409 || !strings.Contains(duplicate.Body.String(), "PRINT_OUTCOME_UNKNOWN") {
		t.Fatal(duplicate.Body)
	}
	close(release)
	group.Wait()
	if calls.Load() != 1 {
		t.Fatal("double print")
	}
}
func TestDurablePrintCrashLeavesUnknown(t *testing.T) {
	dir := t.TempDir()
	var calls int
	crashing := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++; panic("power loss after physical output") })
	func() { defer func() { _ = recover() }(); receiptCall(DurablePrint(dir, crashing), testPrintKey, `{}`) }()
	result := receiptCall(DurablePrint(dir, crashing), testPrintKey, `{}`)
	if calls != 1 || result.Code != 409 {
		t.Fatal("re-executed interrupted print")
	}
}
func TestDurablePrintFailureIsNotRetried(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(500)
		w.Write([]byte("partial print paper error"))
	})
	receiptCall(DurablePrint(dir, next), testPrintKey, `{}`)
	replay := receiptCall(DurablePrint(dir, next), testPrintKey, `{}`)
	if calls != 1 || replay.Code != 500 {
		t.Fatal("failed print was retried")
	}
	receiptCall(DurablePrint(dir, next), "explicit-new-request-02", `{}`)
	if calls != 2 {
		t.Fatal("explicit new request rejected")
	}
}
func TestDurablePrintFailClosedAndLegacy(t *testing.T) {
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ })
	if receiptCall(DurablePrint("", next), testPrintKey, `{}`).Code != 503 {
		t.Fatal("unconfigured store")
	}
	if receiptCall(DurablePrint(t.TempDir(), next), "../bad", `{}`).Code != 400 {
		t.Fatal("invalid key")
	}
	if receiptCall(DurablePrint(t.TempDir(), next), testPrintKey, `{} {}`).Code != 400 {
		t.Fatal("trailing JSON")
	}
	if calls != 0 {
		t.Fatal("printed without durable claim")
	}
	receiptCall(DurablePrint("", next), "", `{}`)
	if calls != 1 {
		t.Fatal("legacy changed")
	}
}
func TestDurablePrintCorruptReceiptNeverReprints(t *testing.T) {
	dir := t.TempDir()
	calls := 0
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ })
	receiptCall(DurablePrint(dir, next), testPrintKey, `{}`)
	paths, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(paths) != 1 {
		t.Fatal(paths)
	}
	if err := os.WriteFile(paths[0], []byte("truncated"), 0600); err != nil {
		t.Fatal(err)
	}
	if receiptCall(DurablePrint(dir, next), testPrintKey, `{}`).Code != 409 || calls != 1 {
		t.Fatal("corrupt receipt reprinted")
	}
}

func statusCall(dir, key string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", "/printer/receipts/"+key, nil)
	req.SetPathValue("id", key)
	res := httptest.NewRecorder()
	PrintReceiptStatus(dir).ServeHTTP(res, req)
	return res
}
func TestPrintReceiptStatusNeverDispatches(t *testing.T) {
	dir := t.TempDir()
	if statusCall(dir, testPrintKey).Code != 404 {
		t.Fatal("unknown receipt must not print")
	}
	var calls atomic.Int32
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(202)
		w.Write([]byte(`{"printState":"pending","jobId":"queue-1"}`))
	})
	receiptCall(RequiredDurablePrint(dir, next), testPrintKey, `{}`)
	for i := 0; i < 3; i++ {
		result := statusCall(dir, testPrintKey)
		var data struct {
			ReceiptState string
			HTTPStatus   int
			ResponseBody string
		}
		if err := json.Unmarshal(result.Body.Bytes(), &data); err != nil {
			t.Fatal(err)
		}
		if result.Code != 200 || data.ReceiptState != "recorded" || data.HTTPStatus != 202 || !strings.Contains(data.ResponseBody, "pending") {
			t.Fatal(result.Body)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("status triggered another print")
	}
}
func TestPrintReceiptStatusUnknownAndUnavailable(t *testing.T) {
	if statusCall("", testPrintKey).Code != 503 {
		t.Fatal("unconfigured store")
	}
	dir := t.TempDir()
	if statusCall(dir, "bad").Code != 400 {
		t.Fatal("invalid key")
	}
	crashing := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { panic("interrupted") })
	func() {
		defer func() { _ = recover() }()
		receiptCall(RequiredDurablePrint(dir, crashing), testPrintKey, `{}`)
	}()
	if statusCall(dir, testPrintKey).Code != 409 {
		t.Fatal("interrupted receipt is unknown")
	}
}
func TestRequiredDurablePrintRejectsMissingKeyAndWrongMethod(t *testing.T) {
	calls := 0
	handler := RequiredDurablePrint(t.TempDir(), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	if receiptCall(handler, "", `{}`).Code != 400 {
		t.Fatal("missing key")
	}
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, httptest.NewRequest("GET", "/printer/print/durable", nil))
	if res.Code != 405 || calls != 0 {
		t.Fatal("invalid request dispatched")
	}
	// A pre-v2 mux has no dedicated route, so rollback cannot execute a print.
	old := http.NewServeMux()
	old.HandleFunc("/printer/print", func(w http.ResponseWriter, r *http.Request) { calls++ })
	res = httptest.NewRecorder()
	old.ServeHTTP(res, httptest.NewRequest("POST", "/printer/print/durable", strings.NewReader(`{}`)))
	if res.Code != 404 || calls != 0 {
		t.Fatal("rollback dispatched")
	}
}
