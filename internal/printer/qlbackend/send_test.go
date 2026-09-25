package qlbackend

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func golden(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "qlraster", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func testSender(log *bytes.Buffer) *Sender {
	s := NewSender(log)
	s.PreflightTimeout = 100 * time.Millisecond
	s.PerPageTimeout = 300 * time.Millisecond
	s.Sleep = func(time.Duration) {}
	return s
}

func send(t *testing.T, p *fakePrinter, data []byte) (Result, string) {
	t.Helper()
	job, err := ParseJob(data)
	if err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	res := testSender(&log).Send(context.Background(), p, job, data)
	return res, log.String()
}

func TestSend_ReportsPrintedWhenEveryLabelIsConfirmed(t *testing.T) {
	p := newFakePrinter()
	data := golden(t, "edges_696_x2.bin")
	res, log := send(t, p, data)
	if res.Outcome != OutcomePrinted || res.Completed != 2 || res.Pages != 2 {
		t.Fatalf("result = %+v\n%s", res, log)
	}
	if !bytes.HasSuffix(p.written(), data) {
		t.Fatal("the job was not sent unchanged after the status check")
	}
	// A good job clears reasons an earlier failure left on the queue.
	if !strings.Contains(log, "STATE: -media-empty-error,") || strings.Contains(log, "ERROR:") {
		t.Fatalf("CUPS messages:\n%s", log)
	}
}

func TestSend_RefusesBeforeSendingWhenThePrinterReportsAProblem(t *testing.T) {
	cases := []struct {
		name       string
		err1, err2 byte
		reason     string
		message    string
	}{
		{"cover open", 0, 0x20, "cover-open-error", "カバーが開いています"},
		{"no roll", 0x01, 0, "media-empty-error", "ロールが入っていません"},
		{"roll used up", 0x02, 0, "media-empty-error", "ロールがなくなりました"},
		{"cutter jam", 0x04, 0, "media-jam-error", "カッターが詰まっています"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := newFakePrinter()
			p.err1, p.err2 = tc.err1, tc.err2
			res, log := send(t, p, golden(t, "traceable_732.bin"))
			if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, tc.message) {
				t.Fatalf("result = %+v", res)
			}
			if p.printed != 0 {
				t.Fatal("the job was sent to a printer that reported a problem")
			}
			if !strings.Contains(log, "STATE: +"+tc.reason) || !strings.Contains(log, "ERROR: ") {
				t.Fatalf("CUPS messages:\n%s", log)
			}
		})
	}
}

// The office failure ptouch caused: the loaded roll is not what the job says.
func TestSend_RefusesAJobForADifferentRoll(t *testing.T) {
	for _, loaded := range []struct{ width, media byte }{{29, 0x0A}, {62, 0x0B}} {
		p := newFakePrinter()
		p.width, p.media = loaded.width, loaded.media
		res, log := send(t, p, golden(t, "traceable_732.bin"))
		if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "62mm 長尺テープ のロールを入れてください") {
			t.Fatalf("loaded %+v: result = %+v", loaded, res)
		}
		if p.printed != 0 || !strings.Contains(log, "STATE: +media-needed-error") {
			t.Fatalf("loaded %+v: printed=%d\n%s", loaded, p.printed, log)
		}
	}
}

// A printer that answers nothing still prints (office, 2026-09-25). The job
// goes out, but it must not be reported as printed.
func TestSend_MutePrinterIsUnconfirmedNotPrinted(t *testing.T) {
	p := newFakePrinter()
	p.muted = true
	res, log := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomeUnconfirmed || !res.Muted {
		t.Fatalf("result = %+v\n%s", res, log)
	}
	if p.printed != 1 {
		t.Fatal("the job should still be sent to a mute printer")
	}
	if strings.Contains(log, "ERROR:") {
		t.Fatalf("an unconfirmed job is not a failure:\n%s", log)
	}
}

func TestSend_ReportsAnErrorRaisedWhilePrinting(t *testing.T) {
	p := newFakePrinter()
	p.failOnPrint = &[2]byte{0x02, 0x00} // roll ran out mid-job
	res, log := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "ロールがなくなりました") {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(log, "STATE: +media-empty-error") {
		t.Fatalf("CUPS messages:\n%s", log)
	}
}

// The failure the tablet could not see before: the printer took the job but
// never said it printed it.
func TestSend_MissingCompletionIsAFailure(t *testing.T) {
	p := newFakePrinter()
	p.noCompletion = true
	res, _ := send(t, p, golden(t, "edges_696_x2.bin"))
	if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "2 枚中 0 枚") {
		t.Fatalf("result = %+v", res)
	}
}

func TestSend_WaitsWhileThePrinterIsBusy(t *testing.T) {
	p := newFakePrinter()
	p.busyReplies = 2
	res, _ := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomePrinted {
		t.Fatalf("result = %+v", res)
	}
	// preflight + 2 busy retries + the one inside the job
	if p.statusRequests != 4 {
		t.Fatalf("status requests = %d, want 4", p.statusRequests)
	}
}

func TestSend_DisconnectWhilePrintingIsAFailure(t *testing.T) {
	p := newFakePrinter()
	p.dropOnPrint = true
	res, _ := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "接続が切れました") {
		t.Fatalf("result = %+v", res)
	}
}

func TestSend_CancelResetsThePrinter(t *testing.T) {
	p := newFakePrinter()
	p.noCompletion = true
	data := golden(t, "traceable_732.bin")
	job, _ := ParseJob(data)
	ctx, cancel := context.WithCancel(context.Background())
	s := testSender(&bytes.Buffer{})
	s.PerPageTimeout = 5 * time.Second
	done := make(chan Result)
	go func() { done <- s.Send(ctx, p, job, data) }()
	for p.requests() < 2 { // wait until the job itself is out
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	res := <-done
	if res.Outcome != OutcomeCanceled {
		t.Fatalf("result = %+v", res)
	}
	if !bytes.HasSuffix(p.written(), cmdReset) {
		t.Fatal("a canceled job must leave the printer reset")
	}
}
