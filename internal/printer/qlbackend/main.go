package qlbackend

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

// CUPS backend exit codes (cups/backend.h).
const (
	exitOK     = 0
	exitFailed = 1
)

// openWait is how long a missing printer is waited for before the job fails.
const defaultOpenWait = 5 * time.Second

var openWait = defaultOpenWait

// maxJobBytes guards against reading an unbounded stream into memory. A
// 30-copy 62x100mm job is under 1.5 MB.
const maxJobBytes = 64 << 20

// Main runs the backend the way CUPS calls it:
//
//	backend                                         list devices (none; queues are set up by raku-sika-ops)
//	backend job user title copies options [file]    print file, or stdin
//
// with DEVICE_URI in the environment. Messages for CUPS go to stderr.
func Main(args []string) int {
	return run(args, os.Stdin, os.Stderr, os.Getenv("DEVICE_URI"), "/", ResultDir)
}

func run(args []string, stdin io.Reader, stderr io.Writer, uri, sysRoot, resultDir string) int {
	if len(args) == 0 {
		return exitOK
	}
	if len(args) != 5 && len(args) != 6 {
		fmt.Fprintln(stderr, "Usage: rakuql job-id user title copies options [file]")
		return exitFailed
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer stop()

	res := Result{JobID: args[0]}
	sender := NewSender(stderr)
	finish := func(r Result) int {
		r.JobID = args[0]
		r.FinishedAt = time.Now()
		if err := WriteResult(resultDir, r); err != nil {
			fmt.Fprintf(stderr, "DEBUG: 結果を書けません: %v\n", err)
		}
		if r.Outcome == OutcomeFailed {
			return exitFailed
		}
		return exitOK
	}

	in := stdin
	if len(args) == 6 {
		f, err := os.Open(args[5])
		if err != nil {
			return finish(sender.fail(res, "印刷データを読めません。", "other-error", err))
		}
		defer f.Close()
		in = f
	}
	data, err := io.ReadAll(io.LimitReader(in, maxJobBytes+1))
	if err != nil || len(data) > maxJobBytes {
		return finish(sender.fail(res, "印刷データを読めません。", "other-error", err))
	}

	job, err := ParseJob(data)
	if err != nil {
		return finish(sender.fail(res, "印刷データが Brother QL のラスタ形式ではありません。hub の版を確認してください。", "other-error", err))
	}
	// With a file argument CUPS leaves copies to the backend.
	if copies, _ := strconv.Atoi(args[3]); len(args) == 6 && copies > 1 {
		one := append([]byte(nil), data...)
		for i := 1; i < copies; i++ {
			data = append(data, one...)
		}
		job.Pages *= copies
	}
	res.Pages = job.Pages

	dev, err := openWithRetry(uri, sysRoot, openWait)
	if err != nil {
		return finish(sender.fail(res, err.Error(), "other-error", nil))
	}
	return finish(sender.Send(ctx, dev, job, data))
}

// openWithRetry tolerates the printer re-enumerating on USB for a moment.
func openWithRetry(uri, sysRoot string, wait time.Duration) (*os.File, error) {
	deadline := time.Now().Add(wait)
	for {
		path, err := ResolveDevice(uri, sysRoot)
		if err == nil {
			var f *os.File
			if f, err = OpenDevice(path); err == nil {
				return f, nil
			}
			err = fmt.Errorf("プリンタを開けません（%v）。電源と USB ケーブルを確認してください。", err)
		}
		if time.Now().After(deadline) {
			return nil, err
		}
		time.Sleep(500 * time.Millisecond)
	}
}
