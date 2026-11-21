//go:build linux

package station

import (
	"errors"
	"time"
)

type AnalysisRequest struct {
	SessionID    string `json:"session_id"`
	Facility     string `json:"facility"`
	Start        string `json:"start"`
	End          string `json:"end"`
	CompareStart string `json:"compare_start"`
	CompareEnd   string `json:"compare_end"`
	Metric       string `json:"metric"`
}

type AnalysisJob struct {
	ID         string          `json:"id"`
	SessionID  string          `json:"session_id"`
	TaskID     string          `json:"task_id"`
	Status     string          `json:"status"`
	Request    AnalysisRequest `json:"request"`
	Current    int64           `json:"current"`
	Comparison int64           `json:"comparison"`
	Difference int64           `json:"difference"`
	Unit       string          `json:"unit"`
	Source     string          `json:"source"`
	Error      string          `json:"error,omitempty"`
	CreatedAt  string          `json:"created_at"`
	Mock       bool            `json:"mock"`
}

func (r AnalysisRequest) validate() error {
	if r.Facility != "fixture" {
		return errors.New("検証用施設fixtureのみ指定できます")
	}
	if r.Metric != "sales_yen" && r.Metric != "weight_g" {
		return errors.New("指標はsales_yenまたはweight_gです")
	}
	for _, d := range []string{r.Start, r.End, r.CompareStart, r.CompareEnd} {
		if _, err := time.Parse("2006-01-02", d); err != nil {
			return errors.New("対象期間・比較期間をYYYY-MM-DDで指定してください")
		}
	}
	for _, d := range []string{r.Start, r.End, r.CompareStart, r.CompareEnd} {
		if d < "2026-10-01" || d > "2026-11-30" {
			return errors.New("検証データの対象期間は2026年10〜11月です")
		}
	}
	if r.Start > r.End || r.CompareStart > r.CompareEnd {
		return errors.New("期間の開始日が終了日より後です")
	}
	return nil
}

// StartAnalysis persists the job separately from the work state. Completion
// cannot advance the task revision, replace the form, or authorize a change.
func (s *Service) StartAnalysis(r AnalysisRequest) (AnalysisJob, error) {
	if err := r.validate(); err != nil {
		return AnalysisJob{}, err
	}
	s.mu.Lock()
	state, ok := s.db.Sessions[r.SessionID]
	if !ok {
		s.mu.Unlock()
		return AnalysisJob{}, errors.New("session not found")
	}
	job := AnalysisJob{ID: newID(), SessionID: r.SessionID, TaskID: state.TaskID, Status: "queued", Request: r, Source: "station-fixture-v1 (synthetic; not facility data)", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Mock: true}
	db := clone(s.db)
	db.Jobs[job.ID] = job
	if err := s.save(db); err != nil {
		s.mu.Unlock()
		return AnalysisJob{}, err
	}
	s.workers.Add(1)
	s.mu.Unlock()
	go func() { defer s.workers.Done(); s.finishAnalysis(job) }()
	return job, nil
}

func (s *Service) finishAnalysis(job AnalysisJob) {
	// Integer aggregation is independent of the language model. This fixture
	// must be replaced by a scoped read adapter before facility analysis use.
	rows := []struct {
		date       string
		yen, grams int64
	}{
		{"2026-10-01", 10000, 2000}, {"2026-10-15", 15000, 3000},
		{"2026-11-01", 12000, 2500}, {"2026-11-15", 18000, 3500},
	}
	for _, row := range rows {
		value := row.yen
		job.Unit = "JPY"
		if job.Request.Metric == "weight_g" {
			value = row.grams
			job.Unit = "g"
		}
		if row.date >= job.Request.Start && row.date <= job.Request.End {
			job.Current += value
		}
		if row.date >= job.Request.CompareStart && row.date <= job.Request.CompareEnd {
			job.Comparison += value
		}
	}
	job.Difference = job.Current - job.Comparison
	job.Status = "done"
	s.mu.Lock()
	defer s.mu.Unlock()
	db := clone(s.db)
	db.Jobs[job.ID] = job
	// Failure poisons the service instead of reporting unpersisted completion.
	_ = s.save(db)
}
