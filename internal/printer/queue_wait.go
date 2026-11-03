package printer

import "time"

// Only the job list is polled at this cadence. Printer/device metadata is
// queried once on return, not three subprocesses on every poll. The printer
// mutex and rakuql's separate completion verdict remain unchanged.
const submittedJobPollInterval = 250 * time.Millisecond

// waitForQueueJob returns the last successfully read queue, including at the
// deadline. An unreadable queue is never evidence that a job has completed.
// Clock injection lets tests replay real completion times without printing.
func waitForQueueJob(jobID string, timeout time.Duration, read func() ([]QueueJobStatus, error), now func() time.Time, sleep func(time.Duration)) ([]QueueJobStatus, error) {
	deadline := now().Add(timeout)
	for {
		jobs, err := read()
		if err == nil {
			if _, found := (QueueSnapshot{Jobs: jobs}).findJob(jobID); !found {
				return jobs, nil
			}
		}
		remaining := deadline.Sub(now())
		if remaining <= 0 {
			return jobs, err
		}
		interval := submittedJobPollInterval
		if err != nil {
			// Keep the existing error backoff; faster successful reads should
			// not hammer an unavailable CUPS service.
			interval = 2 * time.Second
		}
		sleep(min(interval, remaining))
	}
}
