package printer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnoseQueueNoPrinter(t *testing.T) {
	d := DiagnoseQueue(QueueSnapshot{})
	if d.Code != DiagPrinterMissing {
		t.Fatalf("expected %s, got %s", DiagPrinterMissing, d.Code)
	}
	if !d.Blocking {
		t.Fatal("a missing printer must be reported as blocking")
	}
}

func TestDiagnoseQueueUnreachableOutranksBacklog(t *testing.T) {
	// The 2026-09-22 シクヌ shape: jobs piled up *because* the USB link was
	// dead. Clearing the queue would not have helped, so the diagnosis must
	// point at the printer, not at the jobs.
	d := DiagnoseQueue(QueueSnapshot{
		PrinterName:  "Brother_QL_820NWB_USB",
		BackendReady: false,
		BackendError: "backend unavailable",
		QueueState:   "stalled",
		Jobs:         []QueueJobStatus{{ID: "q-1"}, {ID: "q-2"}},
	})
	if d.Code != DiagPrinterUnreachable {
		t.Fatalf("expected %s, got %s", DiagPrinterUnreachable, d.Code)
	}
	if d.ClearQueue {
		t.Fatal("clearing the queue must not be offered when the printer is unreachable")
	}
	if !strings.Contains(d.Detail, "backend unavailable") {
		t.Fatalf("backend error should reach the operator, got %q", d.Detail)
	}
}

func TestDiagnoseQueueEmpty(t *testing.T) {
	d := DiagnoseQueue(QueueSnapshot{PrinterName: "p", BackendReady: true})
	if d.Code != DiagOK {
		t.Fatalf("expected %s, got %s", DiagOK, d.Code)
	}
	if d.Blocking {
		t.Fatal("an empty queue is not blocking")
	}
}

func TestDiagnoseQueueBacklogVersusStalled(t *testing.T) {
	base := QueueSnapshot{
		PrinterName:  "p",
		BackendReady: true,
		QueueState:   "printing",
		Jobs:         []QueueJobStatus{{ID: "q-1"}},
	}

	if got := DiagnoseQueue(base); got.Code != DiagQueueBacklog {
		t.Fatalf("a freshly queued job while printing is a backlog, got %s", got.Code)
	}

	aged := base
	aged.OldestJobAgeSec = stalledAfterSec
	got := DiagnoseQueue(aged)
	if got.Code != DiagQueueStalled {
		t.Fatalf("a job waiting past the threshold is stalled, got %s", got.Code)
	}
	if !got.ClearQueue {
		t.Fatal("a stalled queue should offer clearing as the remedy")
	}

	idle := base
	idle.QueueState = "stalled"
	if got := DiagnoseQueue(idle); got.Code != DiagQueueStalled {
		t.Fatalf("an idle printer holding jobs is stalled, got %s", got.Code)
	}
}

func TestVerifyRenderedLabelRejectsEmptyFile(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.png")
	if err := os.WriteFile(empty, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	err := verifyRenderedLabel(empty)
	if err == nil {
		t.Fatal("a zero-byte render must not be submitted to CUPS")
	}
	if !strings.HasPrefix(err.Error(), "PRINTER_ERROR:") {
		t.Fatalf("error should carry the PRINTER_ERROR prefix, got %q", err)
	}

	good := filepath.Join(dir, "good.png")
	if err := os.WriteFile(good, make([]byte, minRenderedLabelBytes), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyRenderedLabel(good); err != nil {
		t.Fatalf("a full-size render must pass: %v", err)
	}

	if err := verifyRenderedLabel(filepath.Join(dir, "missing.png")); err == nil {
		t.Fatal("a missing render must not be submitted to CUPS")
	}
}
