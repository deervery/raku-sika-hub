// Package qlbackend is the CUPS backend that sends qlraster jobs to a Brother
// QL-800 series printer over USB and waits for the printer to confirm them.
//
// Why it exists: CUPS's own usb backend writes the job and stops reading as
// soon as the last byte is out. The printer then has "printing completed"
// notifications nobody collects, and from that moment it answers no status
// request until it is power-cycled — it also refuses to switch off, showing an
// orange lamp. Reproduced on office on 2026-09-25: after one job through the
// usb backend, 0 of 43 status requests were answered; after the same label
// sent while reading every notification, 5 of 5 were.
//
// Reading the printer's notifications is also what lets a failed print be
// reported as failed: the usb backend reports success once the bytes are
// written, even when the printer silently discarded them.
package qlbackend

import (
	"fmt"
	"strings"
)

// StatusSize is the length of every status frame the printer sends.
const StatusSize = 32

// Status types (byte 18).
const (
	TypeReply             byte = 0x00
	TypePrintingCompleted byte = 0x01
	TypeErrorOccurred     byte = 0x02
	TypeTurnedOff         byte = 0x04
	TypeNotification      byte = 0x05
	TypePhaseChange       byte = 0x06
)

// Status is one decoded status frame.
type Status struct {
	Err1        byte
	Err2        byte
	MediaWidth  byte // mm
	MediaType   byte
	MediaLength byte // mm; 0 for continuous tape
	Type        byte
	Phase       byte
}

// ParseStatus decodes a status frame. It reports false for anything that is
// not one: a frame always starts with 0x80 0x20 ('print head mark', size).
func ParseStatus(b []byte) (Status, bool) {
	if len(b) < StatusSize || b[0] != 0x80 || b[1] != 0x20 {
		return Status{}, false
	}
	return Status{
		Err1:        b[8],
		Err2:        b[9],
		MediaWidth:  b[10],
		MediaType:   b[11],
		MediaLength: b[17],
		Type:        b[18],
		Phase:       b[19],
	}, true
}

// Problem is something that keeps the printer from printing, worded for the
// person standing at it.
type Problem struct {
	// Message is shown on the tablet.
	Message string
	// Reason is the CUPS printer-state-reason reported for it.
	Reason string
}

type flag struct {
	bit     byte
	message string
	reason  string
}

// The printer reports these in error information 1 and 2. "Printer in use"
// (err1 0x10) is not a problem, and the high-resolution and fan bits never
// stop a job on the QL-800 series, so they are left out.
var err1Flags = []flag{
	{0x01, "ロールが入っていません。ロールを入れてください。", "media-empty-error"},
	{0x02, "ロールがなくなりました。新しいロールに交換してください。", "media-empty-error"},
	{0x04, "カッターが詰まっています。詰まったラベルを取り除いてください。", "media-jam-error"},
}

var err2Flags = []flag{
	{0x01, "装着されているロールと印刷データが一致しません。ロールを確認してください。", "media-needed-error"},
	{0x04, "プリンタの受信バッファがいっぱいです。", "other-error"},
	{0x08, "プリンタとの通信でエラーが起きました。", "other-error"},
	{0x10, "プリンタの受信バッファがいっぱいです。", "other-error"},
	{0x20, "カバーが開いています。カバーを閉じてください。", "cover-open-error"},
	{0x80, "ラベルを送れません。ロールの詰まりを確認してください。", "media-jam-error"},
}

// Problems lists what the error bits say is wrong.
func (s Status) Problems() []Problem {
	var out []Problem
	seen := map[string]bool{}
	add := func(f flag) {
		if seen[f.message] {
			return
		}
		seen[f.message] = true
		out = append(out, Problem{Message: f.message, Reason: f.reason})
	}
	for _, f := range err1Flags {
		if s.Err1&f.bit != 0 {
			add(f)
		}
	}
	for _, f := range err2Flags {
		if s.Err2&f.bit != 0 {
			add(f)
		}
	}
	return out
}

// Busy reports the printer saying it is still busy with something else.
func (s Status) Busy() bool {
	return s.Err1&0x10 != 0
}

// Media kinds as the printer reports them (byte 11) and as a job declares
// them (ESC i z, n2). The printer uses 0x4A/0x4B for some rolls.
func isContinuous(t byte) bool { return t == 0x0A || t == 0x4A }
func isDieCut(t byte) bool     { return t == 0x0B || t == 0x4B }

func mediaName(t byte, width byte) string {
	switch {
	case isContinuous(t):
		return fmt.Sprintf("%dmm 長尺テープ", width)
	case isDieCut(t):
		return fmt.Sprintf("%dmm カット済みラベル", width)
	case t == 0x00 || width == 0:
		return "ロールなし"
	default:
		return fmt.Sprintf("%dmm（種類 0x%02x）", width, t)
	}
}

// MediaMismatch compares the loaded roll with what the job was encoded for.
// It returns a problem only when both are known and they differ: an unknown
// job media is not a reason to refuse printing.
func (s Status) MediaMismatch(want Media) (Problem, bool) {
	if !want.Known || s.MediaWidth == 0 {
		return Problem{}, false
	}
	sameKind := (isContinuous(want.Type) && isContinuous(s.MediaType)) ||
		(isDieCut(want.Type) && isDieCut(s.MediaType))
	if s.MediaWidth == want.WidthMM && sameKind {
		return Problem{}, false
	}
	return Problem{
		Message: fmt.Sprintf("装着されているロール（%s）が印刷データ（%s）と一致しません。%s のロールを入れてください。",
			mediaName(s.MediaType, s.MediaWidth), mediaName(want.Type, want.WidthMM), mediaName(want.Type, want.WidthMM)),
		Reason: "media-needed-error",
	}, true
}

func joinMessages(ps []Problem) string {
	msgs := make([]string, 0, len(ps))
	for _, p := range ps {
		msgs = append(msgs, p.Message)
	}
	return strings.Join(msgs, " ")
}
