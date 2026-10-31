package printer

import (
	"encoding/json"
	"github.com/deervery/raku-sika-hub/internal/logging"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExperienceIDNeverLogsCallerText(t *testing.T) {
	for _, input := range []string{"", "private product\nforged log", strings.Repeat("a", 10000)} {
		id := experienceID(input)
		if !experienceUUID.MatchString(id) || id == input {
			t.Fatalf("unsafe identifier")
		}
	}
	id := "12345678-1234-1234-1234-123456789abc"
	if experienceID(id) != id {
		t.Fatal("valid correlation ID not preserved")
	}
}

func TestExperienceTimingsAreMonotonicAndStructural(t *testing.T) {
	dir := t.TempDir()
	logger, err := logging.New(dir, logging.LevelInfo)
	if err != nil {
		t.Fatal(err)
	}
	span := newPrintTiming("private product", logger)
	span.mark("request", "started")
	span.mark("return", "failed")
	logger.Close()
	paths, _ := filepath.Glob(filepath.Join(dir, "*.log"))
	if len(paths) != 1 {
		t.Fatal(paths)
	}
	raw, _ := os.ReadFile(paths[0])
	if strings.Contains(string(raw), "private product") {
		t.Fatal("caller text leaked")
	}
	previous, count := 0.0, 0
	for _, line := range strings.Split(string(raw), "\n") {
		index := strings.Index(line, "{\"event\":")
		if index < 0 {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(line[index:]), &event); err != nil {
			t.Fatal(err)
		}
		if len(event) != 6 {
			t.Fatal(event)
		}
		elapsed := event["durationMs"].(float64)
		if elapsed < previous || event["stageMs"].(float64) < 0 {
			t.Fatal(event)
		}
		previous = elapsed
		count++
	}
	if count != 2 {
		t.Fatalf("got %d events", count)
	}
	var absent *printTiming
	absent.mark("request", "started")
}
