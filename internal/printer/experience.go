package printer

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"regexp"
	"time"

	"github.com/deervery/raku-sika-hub/internal/logging"
)

var experienceUUID = regexp.MustCompile(`^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$`)

// Diagnostic identifiers never authorize, deduplicate or retry a print job.
// Reject arbitrary caller text; logs contain no product, URL or queue name.
func experienceID(value string) string {
	if experienceUUID.MatchString(value) {
		return value
	}
	var data [16]byte
	if _, err := rand.Read(data[:]); err != nil {
		return ""
	}
	text := hex.EncodeToString(data[:])
	return text[:8] + "-" + text[8:12] + "-" + text[12:16] + "-" + text[16:20] + "-" + text[20:]
}

type printTiming struct {
	id              string
	start, previous time.Time
	logger          *logging.Logger
}

func newPrintTiming(id string, logger *logging.Logger) *printTiming {
	now := time.Now()
	return &printTiming{id: experienceID(id), start: now, previous: now, logger: logger}
}

func (t *printTiming) mark(stage, outcome string) {
	if t == nil || t.logger == nil || t.id == "" {
		return
	}
	now := time.Now()
	// All stage/outcome callers are fixed literals/internal result enums.
	event := struct {
		Event      string  `json:"event"`
		TraceID    string  `json:"traceId"`
		Stage      string  `json:"stage"`
		Outcome    string  `json:"outcome"`
		DurationMs float64 `json:"durationMs"`
		StageMs    float64 `json:"stageMs"`
	}{"experience_timing_v1", t.id, stage, outcome,
		float64(now.Sub(t.start).Microseconds()) / 1000,
		float64(now.Sub(t.previous).Microseconds()) / 1000}
	t.previous = now
	if data, err := json.Marshal(event); err == nil {
		t.logger.Info("%s", data)
	}
}

func firstTiming(timings []*printTiming) *printTiming {
	if len(timings) == 0 {
		return nil
	}
	return timings[0]
}
