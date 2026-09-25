package qlbackend

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"
)

// ResultDir is where the backend leaves one file per job for raku-sika-hub.
// /run is cleared on reboot, which is fine: a result only matters for the few
// seconds hub waits on a job.
const ResultDir = "/run/raku-sika/print-results"

// keepResults bounds how many result files stay around.
const keepResults = 200

// WriteResult stores res as <dir>/<job id>.json, atomically so that hub never
// reads half a file.
func WriteResult(dir string, res Result) error {
	if !validJobID(res.JobID) {
		return fmt.Errorf("qlbackend: invalid job id %q", res.JobID)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(res)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".result-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, res.JobID+".json")); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	pruneResults(dir, keepResults)
	return nil
}

// ReadResult returns the result the backend wrote for a CUPS job id (the
// number, not "Queue-123"). ok is false when there is none (yet).
func ReadResult(dir, jobID string) (Result, bool, error) {
	if !validJobID(jobID) {
		return Result{}, false, fmt.Errorf("qlbackend: invalid job id %q", jobID)
	}
	b, err := os.ReadFile(filepath.Join(dir, jobID+".json"))
	if errors.Is(err, os.ErrNotExist) {
		return Result{}, false, nil
	}
	if err != nil {
		return Result{}, false, err
	}
	var res Result
	if err := json.Unmarshal(b, &res); err != nil {
		return Result{}, false, err
	}
	return res, true, nil
}

func validJobID(id string) bool {
	n, err := strconv.Atoi(id)
	return err == nil && n > 0
}

func pruneResults(dir string, keep int) {
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) <= keep {
		return
	}
	type entry struct {
		path string
		mod  time.Time
	}
	entries := make([]entry, 0, len(files))
	for _, f := range files {
		if info, err := os.Stat(f); err == nil {
			entries = append(entries, entry{f, info.ModTime()})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].mod.After(entries[j].mod) })
	for _, e := range entries[min(keep, len(entries)):] {
		os.Remove(e.path)
	}
}
