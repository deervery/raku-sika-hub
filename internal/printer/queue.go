package printer

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type QueueJob struct {
	ID          string `json:"id"`
	Destination string `json:"destination"`
	Owner       string `json:"owner"`
	SizeBytes   int    `json:"sizeBytes"`
	SubmittedAt string `json:"submittedAt"`
}

type QueueStatus struct {
	Printer      string     `json:"printer"`
	Source       string     `json:"source"`
	Address      string     `json:"address,omitempty"`
	Jobs         []QueueJob `json:"jobs"`
	JobCount     int        `json:"jobCount"`
	Clearable    bool       `json:"clearable"`
	DirectSocket bool       `json:"directSocket"`
	Message      string     `json:"message,omitempty"`
}

func queueStatusForDirect(printerName, source, address string) QueueStatus {
	return QueueStatus{
		Printer:      printerName,
		Source:       source,
		Address:      address,
		Jobs:         []QueueJob{},
		JobCount:     0,
		Clearable:    false,
		DirectSocket: true,
		Message:      "TCP 直送のため CUPS キューはありません。",
	}
}

func readCUPSQueue(selectedName, source string) (QueueStatus, error) {
	out, err := exec.Command("lpstat", "-o", selectedName).CombinedOutput()
	if err != nil {
		text := strings.TrimSpace(string(out))
		if text == "" || strings.Contains(text, "no entries") {
			return QueueStatus{
				Printer:   selectedName,
				Source:    source,
				Jobs:      []QueueJob{},
				JobCount:  0,
				Clearable: true,
			}, nil
		}
		return QueueStatus{}, fmt.Errorf("lpstat -o %s failed: %w: %s", selectedName, err, text)
	}

	jobs, err := parseLPStatJobs(string(out))
	if err != nil {
		return QueueStatus{}, err
	}
	return QueueStatus{
		Printer:   selectedName,
		Source:    source,
		Jobs:      jobs,
		JobCount:  len(jobs),
		Clearable: true,
	}, nil
}

func clearCUPSQueue(selectedName, source string) (QueueStatus, error) {
	out, err := exec.Command("cancel", "-a", selectedName).CombinedOutput()
	if err != nil {
		return QueueStatus{}, fmt.Errorf("cancel -a %s failed: %w: %s", selectedName, err, strings.TrimSpace(string(out)))
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		status, err := readCUPSQueue(selectedName, source)
		if err == nil && status.JobCount == 0 {
			status.Message = "キューを全削除しました。"
			return status, nil
		}
		if time.Now().After(deadline) {
			if err != nil {
				return QueueStatus{}, err
			}
			status.Message = "削除を要求しましたが、まだキューにジョブが残っています。"
			return status, nil
		}
		time.Sleep(200 * time.Millisecond)
	}
}

func waitForCUPSJobToLeaveQueue(selectedName, jobID string, timeout time.Duration) error {
	if jobID == "" {
		return nil
	}

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status, err := readCUPSQueue(selectedName, "ptouch-template-cups")
		if err != nil {
			return fmt.Errorf("PRINTER_ERROR: CUPS キュー確認に失敗しました: %s", err)
		}
		if !queueContainsJob(status.Jobs, jobID) {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	status, err := readCUPSQueue(selectedName, "ptouch-template-cups")
	if err != nil {
		return fmt.Errorf("PRINTER_ERROR: 印刷ジョブ %s の停滞確認に失敗しました: %s", jobID, err)
	}
	return fmt.Errorf(
		"PRINTER_ERROR: 印刷ジョブ %s が CUPS キューから流れません。jobCount=%d printer=%q /printer/queue で確認し、不要ジョブは削除してください",
		jobID,
		status.JobCount,
		selectedName,
	)
}

func parseLPRequestID(output string) string {
	text := strings.TrimSpace(output)
	if text == "" {
		return ""
	}
	fields := strings.Fields(text)
	for i, field := range fields {
		if field == "is" && i+1 < len(fields) {
			return strings.TrimSpace(fields[i+1])
		}
	}
	return ""
}

func parseLPStatJobs(output string) ([]QueueJob, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	jobs := make([]QueueJob, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			return nil, fmt.Errorf("unexpected lpstat line: %s", line)
		}
		sizeBytes, err := strconv.Atoi(fields[2])
		if err != nil {
			return nil, fmt.Errorf("parse lpstat size %q: %w", fields[2], err)
		}
		jobs = append(jobs, QueueJob{
			ID:          fields[0],
			Destination: destinationFromJobID(fields[0]),
			Owner:       fields[1],
			SizeBytes:   sizeBytes,
			SubmittedAt: strings.Join(fields[3:], " "),
		})
	}
	return jobs, nil
}

func destinationFromJobID(jobID string) string {
	if idx := strings.LastIndex(jobID, "-"); idx > 0 {
		return jobID[:idx]
	}
	return jobID
}

func queueContainsJob(jobs []QueueJob, jobID string) bool {
	for _, job := range jobs {
		if job.ID == jobID {
			return true
		}
	}
	return false
}
