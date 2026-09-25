package printer

import "fmt"

// Diagnosis codes. These are stable identifiers: the tablet (raku-sika-lite)
// and the hub admin GUI both branch on them, so renaming one is a breaking
// change for those clients.
const (
	// DiagOK means nothing is blocking printing.
	DiagOK = "ok"
	// DiagPrinterMissing means no CUPS queue is registered at all.
	DiagPrinterMissing = "printer_missing"
	// DiagPrinterUnreachable means CUPS holds a queue but cannot talk to the
	// printer (USB unplugged / powered off / backend gone). This is the case
	// where clearing the queue changes nothing.
	DiagPrinterUnreachable = "printer_unreachable"
	// DiagQueueStalled means jobs are waiting but the printer is not pulling
	// them. Clearing the queue is the operator's lever here.
	DiagQueueStalled = "queue_stalled"
	// DiagQueueBacklog means jobs are waiting and the printer is actively
	// printing — the operator only has to wait.
	DiagQueueBacklog = "queue_backlog"
	// DiagPrintUnconfirmed means the job was sent but the printer answered
	// nothing, so whether a label came out is unknown (rakuql queues only).
	DiagPrintUnconfirmed = "print_unconfirmed"
)

// stalledAfterSec is how long the head job may sit in the queue before we stop
// calling it a backlog and start calling it stalled. Label jobs are small; a
// healthy QL prints one in a couple of seconds.
const stalledAfterSec = 90

// Diagnosis explains, in the operator's terms, why printing is not completing.
//
// The point of this type is the distinction the 精肉入庫 screen could not make
// on 2026-09-22: a wedged USB link and a backed-up queue both surface as
// "印刷ジョブを送信しました" followed by silence, but they need opposite
// responses from the operator (re-seat the USB vs. clear the queue).
type Diagnosis struct {
	Code string `json:"code"`
	// Blocking reports whether printing is currently not progressing.
	Blocking bool `json:"blocking"`
	// Title is a short headline, safe to show in a card header.
	Title string `json:"title"`
	// Detail explains what the hub observed.
	Detail string `json:"detail"`
	// Action is the single next step the operator should take.
	Action string `json:"action"`
	// ClearQueue reports whether clearing the queue is a sensible remedy.
	// False for DiagPrinterUnreachable: the jobs are not the problem there.
	ClearQueue bool `json:"clearQueue"`
}

// DiagnoseQueue classifies a queue snapshot.
func DiagnoseQueue(s QueueSnapshot) Diagnosis {
	jobCount := len(s.Jobs)

	if s.PrinterName == "" {
		return Diagnosis{
			Code:     DiagPrinterMissing,
			Blocking: true,
			Title:    "プリンタが登録されていません",
			Detail:   "CUPS にラベルプリンタのキューがありません。",
			Action:   "Hub の設定でプリンタを登録してください。",
		}
	}

	// A dead backend outranks everything else: with no path to the printer,
	// the queue length says nothing about the cause.
	if !s.BackendReady || s.BackendError != "" {
		detail := "プリンタに接続できません（USB ケーブル・電源・接続先）。"
		if s.BackendError != "" {
			detail = fmt.Sprintf("プリンタに接続できません: %s", s.BackendError)
		}
		return Diagnosis{
			Code:     DiagPrinterUnreachable,
			Blocking: true,
			Title:    "プリンタに接続できません",
			Detail:   detail,
			Action:   "プリンタの電源を入れ直し、USB ケーブルを別のポートに挿し直してください。",
		}
	}

	if jobCount == 0 {
		return Diagnosis{
			Code:   DiagOK,
			Title:  "印刷できる状態です",
			Detail: "待機中のジョブはありません。",
		}
	}

	stalled := s.QueueState == "stalled" || s.OldestJobAgeSec >= stalledAfterSec
	if stalled {
		return Diagnosis{
			Code:     DiagQueueStalled,
			Blocking: true,
			Title:    "印刷ジョブが詰まっています",
			Detail: fmt.Sprintf(
				"%d 件のジョブが待機したまま進んでいません（最古 %d 秒経過）。",
				jobCount, s.OldestJobAgeSec,
			),
			Action:     "プリンタの画面が明るいことを確認し、直らなければキューを削除してください。",
			ClearQueue: true,
		}
	}

	return Diagnosis{
		Code:     DiagQueueBacklog,
		Blocking: false,
		Title:    "印刷中です",
		Detail:   fmt.Sprintf("%d 件のジョブを順に印刷しています。", jobCount),
		Action:   "そのままお待ちください。",
	}
}
