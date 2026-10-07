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
	s.SettleTimeout = 150 * time.Millisecond
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
		{"cover open", 0, 0x10, "cover-open-error", "カバーが開いています"}, // office, 2026-09-25
		{"cannot feed", 0, 0x40, "media-jam-error", "ラベルを送れません"},
		{"fan failure", 0x80, 0, "other-error", "ファンが動いていません"},
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

// withoutRollCheck marks the job's roll fields as not valid, as qlraster does
// with NoMediaCheck.
func withoutRollCheck(t *testing.T, data []byte) []byte {
	t.Helper()
	out := append([]byte(nil), data...)
	i := bytes.Index(out, []byte{0x1B, 0x69, 0x7A})
	if i < 0 {
		t.Fatal("no print information command")
	}
	out[i+3] = 0xC0
	return out
}

// Stations using rolls the printer does not recognise turn the roll check
// off; the backend must then send the job whatever roll the printer reports.
func TestSend_JobWithoutRollCheckPrintsOnAnyRoll(t *testing.T) {
	for _, loaded := range []struct{ width, media byte }{{29, 0x0A}, {62, 0x0B}, {0, 0x00}} {
		p := newFakePrinter()
		p.width, p.media = loaded.width, loaded.media
		res, log := send(t, p, withoutRollCheck(t, golden(t, "traceable_732.bin")))
		if res.Outcome != OutcomePrinted || p.printed != 1 {
			t.Fatalf("loaded %+v: printed=%d result = %+v\n%s", loaded, p.printed, res, log)
		}
	}
}

// A printer that says the roll does not match (err2 0x01) must not stop a job
// that does not ask for the roll to be checked; other problems still do.
func TestSend_JobWithoutRollCheckIgnoresOnlyTheRollMismatch(t *testing.T) {
	p := newFakePrinter()
	p.err2 = 0x01
	res, log := send(t, p, withoutRollCheck(t, golden(t, "traceable_732.bin")))
	if p.printed != 1 {
		t.Fatalf("the job was refused: %+v\n%s", res, log)
	}

	p = newFakePrinter()
	p.err2 = 0x01 | 0x10 // and the cover is open
	res, _ = send(t, p, withoutRollCheck(t, golden(t, "traceable_732.bin")))
	if p.printed != 0 || res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "カバーが開いています") || strings.Contains(res.Message, "一致しません") {
		t.Fatalf("printed=%d result = %+v", p.printed, res)
	}

	p = newFakePrinter()
	p.err2 = 0x01
	if _, _ = send(t, p, golden(t, "traceable_732.bin")); p.printed != 0 {
		t.Fatal("a job that asks for the roll check must still be refused")
	}
}

// On a red/black roll the black-only job goes out in two-colour mode, black
// lines as they were and the red lines empty, and prints.
func TestSend_TwoColorRollGetsATwoColorJob(t *testing.T) {
	data := golden(t, "traceable_732.bin")
	p := newFakePrinter()
	p.textColor = 0x81 // hakodate, 2026-09-29
	res, log := send(t, p, data)
	if res.Outcome != OutcomePrinted || p.printed != 1 {
		t.Fatalf("printed=%d result = %+v\n%s", p.printed, res, log)
	}
	if !p.twoColorMode || p.redLines != 0 || p.blackLines == 0 || !strings.Contains(log, "2 色印刷モード") {
		t.Fatalf("twoColorMode=%t black=%d red=%d\n%s", p.twoColorMode, p.blackLines, p.redLines, log)
	}
	job, err := ParseJob(data)
	if err != nil {
		t.Fatal(err)
	}
	if p.twoColorLines != 2*rasterLines(t, data) || job.Pages != 1 {
		t.Fatalf("w lines %d for %d rows", p.twoColorLines, rasterLines(t, data))
	}
}

// Without the conversion the printer stops the job, as it did at hakodate.
func TestSend_WhiteRollJobIsNotConverted(t *testing.T) {
	p := newFakePrinter()
	res, log := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomePrinted || p.twoColorMode || p.twoColorLines != 0 {
		t.Fatalf("twoColorMode=%t lines=%d result = %+v\n%s", p.twoColorMode, p.twoColorLines, res, log)
	}
}

func rasterLines(t *testing.T, data []byte) int {
	t.Helper()
	n := 0
	for i := 0; i < len(data); {
		c := commandSize(data[i:])
		if c == 0 {
			t.Fatalf("unknown byte at %d", i)
		}
		if data[i] == 'g' {
			n++
		}
		i += c
	}
	return n
}

// Every frame the printer sends is logged as received, so that an error the
// backend cannot interpret can still be read afterwards.
func TestSend_LogsThePrintersFrames(t *testing.T) {
	p := newFakePrinter()
	_, log := send(t, p, golden(t, "traceable_732.bin"))
	if n := strings.Count(log, "DEBUG: printer status 8020"); n < 2 {
		t.Fatalf("want the reply and the completion logged, got %d:\n%s", n, log)
	}
}

// An error with no bits set points the person at the printer's own screen.
func TestSend_ErrorWithoutBitsPointsAtThePrinter(t *testing.T) {
	p := newFakePrinter()
	p.failOnPrint = &[2]byte{0x00, 0x00}
	res, _ := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "コード 0000") || !strings.Contains(res.Message, "プリンタの画面") {
		t.Fatalf("result = %+v", res)
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
	// The rest of the job must not print once the roll is replaced.
	if !bytes.HasSuffix(p.written(), cmdReset) {
		t.Fatal("a failed job must leave the printer's buffer cleared")
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

// A printer that stays quiet after the job is asked how it is. One that
// still answers is alive: the label may have come out, so the tablet says to
// look before sending again.
func TestSend_MissingCompletionFromAPrinterThatStillAnswers(t *testing.T) {
	p := newFakePrinter()
	p.noCompletion = true
	res, log := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "ラベルが出ていれば再送信は不要") {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(log, "probe: printer answered") {
		t.Fatalf("the probe is not in the log:\n%s", log)
	}
}

// One that answers nothing has stopped talking; only restarting the printer
// brings it back (hakodate, 2026-10-06 14:22). The tablet must not lead staff
// to switch off the station with it (siknue, 2026-10-06 20:03).
func TestSend_MissingCompletionFromAPrinterThatStoppedAnswering(t *testing.T) {
	p := newFakePrinter()
	p.silentAfterPrint = true
	res, log := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "応答しなくなっています") ||
		!strings.Contains(res.Message, "プリンタの電源だけを入れ直し") || !strings.Contains(res.Message, "端末の電源は切らないでください") {
		t.Fatalf("result = %+v", res)
	}
	if !strings.Contains(log, "probe: no answer") {
		t.Fatalf("the probe is not in the log:\n%s", log)
	}
}

// A completion that comes late — only once the printer is asked — means the
// label printed.
func TestSend_LateCompletionIsPrinted(t *testing.T) {
	p := newFakePrinter()
	p.noCompletion = true
	p.completeOnAsk = true
	res, log := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomePrinted || res.Completed != 1 {
		t.Fatalf("result = %+v\n%s", res, log)
	}
}

// A printer that stopped on an error without reporting it says what is wrong
// when asked.
func TestSend_ProbeReportsAnErrorThePrinterDidNotAnnounce(t *testing.T) {
	p := newFakePrinter()
	p.errAfterPrint = &[2]byte{0x00, 0x10} // cover opened
	res, _ := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "カバーが開いています") {
		t.Fatalf("result = %+v", res)
	}
}

// Every frame in the log carries how far into the job it came, so a slow
// printer can be told from a silent one afterwards.
func TestSend_LoggedFramesAreTimed(t *testing.T) {
	p := newFakePrinter()
	_, log := send(t, p, golden(t, "traceable_732.bin"))
	for _, line := range strings.Split(strings.TrimSpace(log), "\n") {
		if strings.HasPrefix(line, "DEBUG: ") && !strings.Contains(line, " (+") {
			t.Fatalf("untimed DEBUG line: %q", line)
		}
	}
	if !strings.Contains(log, "DEBUG: sent ") {
		t.Fatalf("the send is not in the log:\n%s", log)
	}
}

// A printer that stops partway through a multi-copy job is noticed one
// per-label timeout after its last label, not after the whole job's worth
// (siknue 2026-10-07 14:53: 3 copies, the verdict came after hub stopped
// waiting).
func TestSend_StallMidJobIsNoticedPerLabel(t *testing.T) {
	p := newFakePrinter()
	p.completeOnly = 1
	data := golden(t, "edges_696_x2.bin")
	job, err := ParseJob(data)
	if err != nil {
		t.Fatal(err)
	}
	// Make it an 8-label job, like the multi-copy pet labels staff send.
	for i := 0; i < 2; i++ {
		data = append(data, data...)
	}
	job.Pages *= 4
	var log bytes.Buffer
	s := testSender(&log)
	start := time.Now()
	res := s.Send(context.Background(), p, job, data)
	elapsed := time.Since(start)
	if res.Outcome != OutcomeFailed || !strings.Contains(res.Message, "8 枚中 1 枚") {
		t.Fatalf("result = %+v\n%s", res, log.String())
	}
	if limit := 3 * s.PerPageTimeout; elapsed > limit {
		t.Fatalf("took %v to notice; want about one per-label timeout (%v), well under %v", elapsed, s.PerPageTimeout, limit)
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

// Long runs make the printer stop to cool its head. That pause is not a lost
// job: the wait is extended while it cools.
func TestSend_WaitsWhileThePrinterCools(t *testing.T) {
	p := newFakePrinter()
	p.coolFor = 700 * time.Millisecond // longer than the per-page timeout
	data := golden(t, "traceable_732.bin")
	job, _ := ParseJob(data)
	var log bytes.Buffer
	s := testSender(&log)
	s.CoolingTimeout = 3 * time.Second
	res := s.Send(context.Background(), p, job, data)
	if res.Outcome != OutcomePrinted {
		t.Fatalf("result = %+v\n%s", res, log.String())
	}
	if !strings.Contains(log.String(), "ヘッドを冷やしています") {
		t.Fatalf("the operator should be told the printer is cooling:\n%s", log.String())
	}
}

// An error notification an earlier job left unread must not be taken for the
// answer to this job's status request.
func TestSend_IgnoresAStaleErrorFromAnEarlierJob(t *testing.T) {
	p := newFakePrinter()
	stale := p.frame(TypeErrorOccurred, 0x00, 0, 0x10) // cover was open back then
	p.queue(stale)
	res, log := send(t, p, golden(t, "traceable_732.bin"))
	if res.Outcome != OutcomePrinted {
		t.Fatalf("result = %+v\n%s", res, log)
	}
}
