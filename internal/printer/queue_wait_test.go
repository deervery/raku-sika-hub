package printer

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

func TestQueueWaitCompletionDetection(t *testing.T) {
	// Replay completion throughout a two-second phase, including the observed
	// 3.160s/4.869s backend durations. This isolates detection, not physical speed.
	for _, completed := range []time.Duration{0, 1, 249, 250, 251, 3160, 4001, 4869, 5999} {
		completed *= time.Millisecond
		at := time.Unix(0, 0)
		start := at
		polls := 0
		jobs, err := waitForQueueJob("label-7", 12*time.Second, func() ([]QueueJobStatus, error) {
			polls++
			if at.Sub(start) < completed {
				return []QueueJobStatus{{ID: "label-7", State: "queued"}}, nil
			}
			// Another queued job must not prolong this request's wait.
			return []QueueJobStatus{{ID: "label-8", State: "queued"}}, nil
		}, func() time.Time { return at }, func(d time.Duration) { at = at.Add(d) })
		lag := at.Sub(start) - completed
		if err != nil || lag < 0 || lag >= 250*time.Millisecond || len(jobs) != 1 || jobs[0].ID != "label-8" {
			t.Fatalf("completion=%v lag=%v jobs=%+v err=%v", completed, lag, jobs, err)
		}
		if polls > 1+int((at.Sub(start)+249*time.Millisecond)/(250*time.Millisecond)) {
			t.Fatalf("busy polling: %d reads", polls)
		}
		t.Logf("completion=%v detected=%v polls=%d", completed, at.Sub(start), polls)
	}
}

func TestQueueWaitDeadlineAndErrors(t *testing.T) {
	for _, kind := range []string{"pending", "clears at deadline", "always unavailable", "recovers"} {
		t.Run(kind, func(t *testing.T) {
			at := time.Unix(0, 0)
			start := at
			calls := 0
			unavailable := errors.New("CUPS unavailable")
			jobs, err := waitForQueueJob("label-7", 2100*time.Millisecond, func() ([]QueueJobStatus, error) {
				calls++
				if kind == "always unavailable" || (kind == "recovers" && calls == 1) {
					return nil, unavailable
				}
				if kind == "recovers" || (kind == "clears at deadline" && at.Sub(start) == 2100*time.Millisecond) {
					return nil, nil
				}
				return []QueueJobStatus{{ID: "label-7", State: "queued"}}, nil
			}, func() time.Time { return at }, func(d time.Duration) { at = at.Add(d) })
			if at.Sub(start) > 2100*time.Millisecond {
				t.Fatalf("slept past deadline: %v", at.Sub(start))
			}
			switch kind {
			case "always unavailable":
				if !errors.Is(err, unavailable) || calls != 3 {
					t.Fatalf("unreadable queue treated as clear or no backoff: %v, %d", err, calls)
				}
			case "pending":
				if err != nil || len(jobs) != 1 || at.Sub(start) != 2100*time.Millisecond {
					t.Fatalf("lost pending job: %+v %v", jobs, err)
				}
			default:
				if err != nil || len(jobs) != 0 {
					t.Fatalf("clear queue: %+v %v", jobs, err)
				}
			}
		})
	}
}

func TestSubmittedJobUsesOnlyQueuePollsUntilReturn(t *testing.T) {
	dir := t.TempDir()
	callsPath := dir + "/calls"
	writeStub(t, dir, "lpstat", `#!/bin/sh
echo "$*" >> "`+callsPath+`"
case "$*" in
  *not-completed*)
    if [ ! -e "`+dir+`/seen" ]; then
      touch "`+dir+`/seen"
      echo 'label-7 user 123 Tue 03 Nov 2026'
    fi ;;
  *-l*) echo 'printer label is idle.' ;;
  *) exit 99 ;;
esac
`)
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	logger, err := logging.New(t.TempDir(), logging.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	b := &Brother{logger: logger}
	result, err := b.verifySubmittedJob("label", "label-7", time.Second)
	if err != nil || result.State != "done" || result.JobState != "cleared" || result.PrinterState != "idle" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	got, err := os.ReadFile(callsPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "-W not-completed -o label\n-W not-completed -o label\n-p label -l\n" {
		t.Fatalf("unexpected metadata reads during wait: %s", got)
	}
}

func TestSubmittedJobDoesNotCompleteOnSilentCommandFailure(t *testing.T) {
	dir := t.TempDir()
	writeStub(t, dir, "lpstat", "#!/bin/sh\nexit 1\n")
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	logger, err := logging.New(t.TempDir(), logging.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	b := &Brother{logger: logger}
	result, err := b.verifySubmittedJob("label", "label-7", 0)
	if err == nil || result.State == "done" {
		t.Fatalf("silent CUPS failure claimed completion: %+v %v", result, err)
	}
}
