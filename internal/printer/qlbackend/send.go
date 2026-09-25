package qlbackend

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"
)

// Outcome is how a job ended, as far as the printer told us.
type Outcome string

const (
	// OutcomePrinted: the printer reported every label as printed.
	OutcomePrinted Outcome = "printed"
	// OutcomeFailed: the printer reported a problem, or stopped answering
	// part-way through. The label may not have come out.
	OutcomeFailed Outcome = "failed"
	// OutcomeUnconfirmed: the printer answered nothing at all, before or
	// after. The bytes went out, but whether a label came out is unknown.
	OutcomeUnconfirmed Outcome = "unconfirmed"
	// OutcomeCanceled: CUPS canceled the job while it was being sent.
	OutcomeCanceled Outcome = "canceled"
)

// Result is what the backend reports for one job. raku-sika-hub reads it to
// tell the tablet what really happened.
type Result struct {
	JobID      string    `json:"jobId"`
	Outcome    Outcome   `json:"outcome"`
	Message    string    `json:"message"`
	Reasons    []string  `json:"reasons,omitempty"`
	Pages      int       `json:"pages"`
	Completed  int       `json:"completed"`
	Muted      bool      `json:"muted,omitempty"`
	FinishedAt time.Time `json:"finishedAt"`
}

// problemReasons are every printer-state-reason this backend sets, so that a
// successful job can clear whatever an earlier one left behind.
var problemReasons = []string{"media-empty-error", "media-jam-error", "media-needed-error", "cover-open-error", "other-error"}

// Sender sends one job and follows it to the end.
type Sender struct {
	// Log receives the lines CUPS reads from a backend's stderr
	// ("INFO: ...", "ERROR: ...", "STATE: ...").
	Log io.Writer
	// PreflightTimeout bounds the wait for the answer to a status request.
	PreflightTimeout time.Duration
	// BusyRetries is how often to ask again while the printer says it is busy.
	BusyRetries int
	// PerPageTimeout is how long each label may take to be reported printed.
	PerPageTimeout time.Duration
	// WriteTimeout bounds each write; a printer that takes no data at all
	// must not hang the queue.
	WriteTimeout time.Duration
	// SettleTimeout bounds the wait for the printer's last notification
	// after the final label.
	SettleTimeout time.Duration
	// Sleep is time.Sleep, replaceable in tests.
	Sleep func(time.Duration)
}

// NewSender returns a Sender with the timeouts used in production.
func NewSender(log io.Writer) *Sender {
	return &Sender{
		Log:              log,
		PreflightTimeout: 2 * time.Second,
		BusyRetries:      5,
		PerPageTimeout:   20 * time.Second,
		WriteTimeout:     15 * time.Second,
		SettleTimeout:    1500 * time.Millisecond,
		Sleep:            time.Sleep,
	}
}

var (
	cmdReset         = append(make([]byte, 200), 0x1B, 0x40) // invalidate + initialize
	cmdStatusRequest = []byte{0x1B, 0x69, 0x53}
)

type writeDeadliner interface {
	SetWriteDeadline(time.Time) error
}

// Send prints data on dev and closes dev when done. dev must return the
// printer's status frames from Read.
func (s *Sender) Send(ctx context.Context, dev io.ReadWriteCloser, job Job, data []byte) Result {
	frames := make(chan Status, 64)
	go readFrames(dev, frames)
	defer func() {
		dev.Close()
		for range frames { // let the reader finish
		}
	}()

	res := Result{Pages: job.Pages}

	// 1. Ask before sending.
	s.info("プリンタの状態を確認しています")
	reply, answered, err := s.preflight(dev, frames)
	if err != nil {
		return s.fail(res, "プリンタにデータを送れません。電源と USB ケーブルを確認してください。", "other-error", err)
	}
	if answered {
		if ps := reply.Problems(); len(ps) > 0 {
			return s.failProblems(res, ps)
		}
		if p, bad := reply.MediaMismatch(job.Media); bad {
			return s.failProblems(res, []Problem{p})
		}
	} else {
		// The printer takes data but answers nothing. It still prints in this
		// state (office, 2026-09-25), so send anyway; the result is then
		// reported as unconfirmed rather than printed.
		res.Muted = true
		s.info("プリンタが状態の問い合わせに応答しません。印刷は続けます")
	}

	// 2. Send, collecting notifications as they come.
	s.info("印刷データを送っています")
	if err := s.write(ctx, dev, data); err != nil {
		if ctx.Err() != nil {
			return s.cancel(dev, res)
		}
		return s.fail(res, "プリンタが印刷データを受け取りません。電源を入れ直してから再送信してください。", "other-error", err)
	}

	// 3. Wait for the printer to report every label.
	heard := false
	timeout := time.NewTimer(time.Duration(job.Pages) * s.PerPageTimeout)
	defer timeout.Stop()
	for res.Completed < job.Pages {
		select {
		case st, ok := <-frames:
			if !ok {
				return s.fail(res, "印刷中にプリンタとの接続が切れました。電源と USB ケーブルを確認し、ラベルを確かめてから再送信してください。", "other-error", nil)
			}
			heard = true
			switch st.Type {
			case TypePrintingCompleted:
				res.Completed++
			case TypeErrorOccurred:
				ps := st.Problems()
				if len(ps) == 0 {
					ps = []Problem{{Message: fmt.Sprintf("プリンタがエラーを報告しました（コード %02x%02x）。", st.Err1, st.Err2), Reason: "other-error"}}
				}
				return s.failProblems(res, ps)
			case TypeTurnedOff:
				return s.fail(res, "印刷中にプリンタの電源が切れました。ラベルを確かめてから再送信してください。", "other-error", nil)
			default:
				if ps := st.Problems(); len(ps) > 0 {
					return s.failProblems(res, ps)
				}
			}
		case <-timeout.C:
			if res.Muted && !heard {
				res.Outcome = OutcomeUnconfirmed
				res.Message = "印刷データは送りましたが、プリンタが応答しないため印刷できたか確認できません。ラベルが出たか確かめてください。プリンタの電源を入れ直すと応答が戻ります。"
				s.info(res.Message)
				return res
			}
			return s.fail(res, fmt.Sprintf("プリンタから印刷完了の知らせが届きませんでした（%d 枚中 %d 枚）。ラベルを確かめ、足りなければ再送信してください。", job.Pages, res.Completed), "other-error", nil)
		case <-ctx.Done():
			return s.cancel(dev, res)
		}
	}

	// Read what the printer still has to say — normally its return to
	// "receiving" — so that nothing is left unread. Unread notifications are
	// what put the printer into the state where it answers nothing.
	settle := time.NewTimer(s.SettleTimeout)
	defer settle.Stop()
drain:
	for {
		select {
		case st, ok := <-frames:
			if !ok || (st.Type == TypePhaseChange && st.Phase == 0x00) {
				break drain
			}
		case <-settle.C:
			break drain
		}
	}

	res.Outcome = OutcomePrinted
	res.Muted = false
	res.Message = "印刷しました。"
	fmt.Fprintf(s.Log, "STATE: -%s\n", strings.Join(problemReasons, ","))
	s.info(res.Message)
	return res
}

// preflight resets the printer's input and asks for its status. answered is
// false when the printer took the request but sent nothing back in time.
func (s *Sender) preflight(dev io.Writer, frames <-chan Status) (Status, bool, error) {
	if err := s.writeAll(dev, append(append([]byte{}, cmdReset...), cmdStatusRequest...)); err != nil {
		return Status{}, false, err
	}
	for attempt := 0; ; attempt++ {
		st, ok := s.awaitReply(frames)
		if !ok {
			return Status{}, false, nil
		}
		if !st.Busy() || attempt >= s.BusyRetries {
			return st, true, nil
		}
		s.Sleep(time.Second)
		if err := s.writeAll(dev, cmdStatusRequest); err != nil {
			return Status{}, false, err
		}
	}
}

func (s *Sender) awaitReply(frames <-chan Status) (Status, bool) {
	t := time.NewTimer(s.PreflightTimeout)
	defer t.Stop()
	for {
		select {
		case st, ok := <-frames:
			if !ok {
				return Status{}, false
			}
			if st.Type == TypeReply {
				return st, true
			}
		case <-t.C:
			return Status{}, false
		}
	}
}

// write sends data in chunks so that a cancel is noticed between them.
func (s *Sender) write(ctx context.Context, dev io.Writer, data []byte) error {
	const chunk = 16 << 10
	for off := 0; off < len(data); off += chunk {
		if err := ctx.Err(); err != nil {
			return err
		}
		end := min(off+chunk, len(data))
		if err := s.writeAll(dev, data[off:end]); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sender) writeAll(dev io.Writer, b []byte) error {
	if d, ok := dev.(writeDeadliner); ok && s.WriteTimeout > 0 {
		_ = d.SetWriteDeadline(time.Now().Add(s.WriteTimeout))
	}
	_, err := dev.Write(b)
	return err
}

func (s *Sender) cancel(dev io.Writer, res Result) Result {
	// Drop whatever part of the job the printer has buffered, so the next
	// job starts from a clean state.
	_ = s.writeAll(dev, cmdReset)
	res.Outcome = OutcomeCanceled
	res.Message = "印刷を取り消しました。"
	s.info(res.Message)
	return res
}

func (s *Sender) failProblems(res Result, ps []Problem) Result {
	res.Outcome = OutcomeFailed
	res.Message = joinMessages(ps)
	seen := map[string]bool{}
	for _, p := range ps {
		if !seen[p.Reason] {
			seen[p.Reason] = true
			res.Reasons = append(res.Reasons, p.Reason)
		}
	}
	fmt.Fprintf(s.Log, "STATE: +%s\n", strings.Join(res.Reasons, ","))
	fmt.Fprintf(s.Log, "ERROR: %s\n", res.Message)
	return res
}

func (s *Sender) fail(res Result, message, reason string, cause error) Result {
	if cause != nil {
		fmt.Fprintf(s.Log, "DEBUG: %v\n", cause)
	}
	return s.failProblems(res, []Problem{{Message: message, Reason: reason}})
}

func (s *Sender) info(msg string) {
	fmt.Fprintf(s.Log, "INFO: %s\n", msg)
}
