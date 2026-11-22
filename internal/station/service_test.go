//go:build linux

package station

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

type counter struct {
	calls   atomic.Int32
	unknown bool
	entered chan struct{}
	release chan struct{}
}

func (e *counter) Execute(ctx context.Context, op Operation) (string, string, error) {
	e.calls.Add(1)
	if e.entered != nil {
		close(e.entered)
		<-e.release
	}
	if e.unknown {
		return "", "", context.DeadlineExceeded
	}
	return "done", "mock receipt", nil
}
func (e *counter) Lookup(context.Context, Operation) (string, string, error) {
	return "done", "verified mock receipt", nil
}

func fixture(t *testing.T, e Executor) (*Service, Snapshot, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := Open(dir, e)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	v, err := s.Create()
	if err != nil {
		t.Fatal(err)
	}
	return s, v, dir
}
func cmd(v Snapshot, action string) Command {
	return Command{SessionID: v.State.SessionID, Revision: v.State.Revision, MediaEpoch: v.State.MediaEpoch, Action: action}
}
func apply(t *testing.T, s *Service, c Command) Snapshot {
	t.Helper()
	v, err := s.Apply(context.Background(), c)
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func ready(t *testing.T, s *Service, v Snapshot) Snapshot {
	c := cmd(v, "patch")
	c.Fields = Fields{"product": "鹿肉", "cut": "ロース", "lot": "00123", "copies": "3"}
	return apply(t, s, c)
}
func confirm(v Snapshot) Command {
	c := cmd(v, "confirm")
	c.ConfirmationID = v.State.Confirmation.ID
	return c
}

func TestCorrectionRevokesConfirmationAndLateASR(t *testing.T) {
	e := &counter{}
	s, v, _ := fixture(t, e)
	v = ready(t, s, v)
	old := confirm(v)
	c := cmd(v, "patch")
	c.Fields = Fields{"copies": "2"}
	v = apply(t, s, c)
	if _, err := s.Apply(context.Background(), old); !errors.Is(err, ErrStale) {
		t.Fatalf("old confirmation accepted: %v", err)
	}
	old.Action = "utterance"
	old.Text = "5枚に訂正"
	if _, err := s.Apply(context.Background(), old); !errors.Is(err, ErrStale) {
		t.Fatal("late ASR accepted")
	}
	v = apply(t, s, confirm(v))
	if v.Operation.Fields["copies"] != "2" || v.Operation.Fields["lot"] != "00123" || e.calls.Load() != 1 {
		t.Fatal(v)
	}
}

func TestConfirmationSurvivesRestartWithoutRedispatch(t *testing.T) {
	e := &counter{}
	s, v, dir := fixture(t, e)
	v = ready(t, s, v)
	c := confirm(v)
	first := apply(t, s, c)
	_ = s.Close()
	s2, err := Open(dir, e)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	v = apply(t, s2, c)
	if e.calls.Load() != 1 || v.Operation.ID != first.Operation.ID {
		t.Fatal("duplicate execution")
	}
}

func TestConcurrentConfirmationsExecuteOnce(t *testing.T) {
	e := &counter{}
	s, v, _ := fixture(t, e)
	v = ready(t, s, v)
	c := confirm(v)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Apply(context.Background(), c); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if e.calls.Load() != 1 {
		t.Fatalf("executed %d times", e.calls.Load())
	}
}

func TestTimeoutQueriesResultWithoutRetry(t *testing.T) {
	e := &counter{unknown: true}
	s, v, _ := fixture(t, e)
	v = ready(t, s, v)
	c := confirm(v)
	v = apply(t, s, c)
	if v.State.Status != "unknown" {
		t.Fatal(v)
	}
	v = apply(t, s, c)
	if e.calls.Load() != 1 {
		t.Fatal("timeout retried")
	}
	change := cmd(v, "patch")
	change.Fields = Fields{"copies": "2"}
	if _, err := s.Apply(context.Background(), change); !errors.Is(err, ErrBusy) {
		t.Fatal("unknown operation editable")
	}
	v = apply(t, s, cmd(v, "reconcile"))
	if v.State.Status != "done" || e.calls.Load() != 1 {
		t.Fatal(v)
	}
}

func TestCrashCheckpointBecomesUnknown(t *testing.T) {
	e := &counter{entered: make(chan struct{}), release: make(chan struct{})}
	s, v, dir := fixture(t, e)
	v = ready(t, s, v)
	done := make(chan struct{})
	go func() { defer close(done); _, _ = s.Apply(context.Background(), confirm(v)) }()
	<-e.entered
	// Snapshot the exact durable pre-dispatch checkpoint, as if power failed.
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	recovery := t.TempDir()
	if err = os.WriteFile(filepath.Join(recovery, "state.json"), b, 0600); err != nil {
		t.Fatal(err)
	}
	other := &counter{}
	s2, err := Open(recovery, other)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	restored, err := s2.Get(v.State.SessionID)
	if err != nil || restored.State.Status != "unknown" || other.calls.Load() != 0 {
		t.Fatalf("unsafe recovery: %+v %v", restored, err)
	}
	close(e.release)
	<-done
}

func TestStoreExclusiveAndCorruptionFailClosed(t *testing.T) {
	s, _, dir := fixture(t, &counter{})
	if other, err := Open(dir, &counter{}); err == nil {
		other.Close()
		t.Fatal("multiple writers")
	}
	s.Close()
	os.WriteFile(filepath.Join(dir, "state.json"), []byte("{"), 0600)
	if other, err := Open(dir, &counter{}); err == nil {
		other.Close()
		t.Fatal("corruption silently reset ledger")
	}
}

func TestChatYesDoesNotApproveAndKeepsWork(t *testing.T) {
	e := &counter{}
	s, v, _ := fixture(t, e)
	v = ready(t, s, v)
	revision := v.State.Revision
	c := cmd(v, "utterance")
	c.Text = "今日はいい天気"
	v = apply(t, s, c)
	c = cmd(v, "utterance")
	c.Text = "はい"
	c.ConfirmationID = v.State.Confirmation.ID
	v = apply(t, s, c)
	if e.calls.Load() != 0 || v.State.Fields["copies"] != "3" || v.State.Revision != revision {
		t.Fatal(v)
	}
}

func TestAmbiguousEndAndCancellation(t *testing.T) {
	e := &counter{}
	s, v, _ := fixture(t, e)
	v = ready(t, s, v)
	old := confirm(v)
	c := cmd(v, "utterance")
	c.Text = "もういい"
	v = apply(t, s, c)
	if v.State.Status != "awaiting_confirmation" || !strings.Contains(v.State.Reply, "取り消しますか") {
		t.Fatal(v)
	}
	v = apply(t, s, cmd(v, "cancel"))
	if v.State.Status != "cancelled" || v.State.Confirmation != nil {
		t.Fatal(v)
	}
	if _, err := s.Apply(context.Background(), old); err == nil {
		t.Fatal("cancelled confirmation accepted")
	}
}

func TestMissingUnitAndInvalidValues(t *testing.T) {
	s, v, _ := fixture(t, &counter{})
	c := cmd(v, "begin")
	c.Kind = "inventory"
	v = apply(t, s, c)
	c = cmd(v, "patch")
	c.Fields = Fields{"product": "鹿肉", "lot": "123", "weight": "0.07"}
	v = apply(t, s, c)
	if v.State.Status != "drafting" || !contains(v.State.Missing, "unit") {
		t.Fatal("unit inferred")
	}
	for _, fields := range []Fields{{"weight": "-2"}, {"weight": "0"}, {"copies": "1.5"}, {"copies": "31"}, {"unit": "lb"}, {"unexpected": "x"}} {
		c = cmd(v, "patch")
		c.Fields = fields
		if _, err := s.Apply(context.Background(), c); err == nil {
			t.Fatalf("accepted %v", fields)
		}
	}
}

func TestInterruptRevokesMediaWithoutChangingFields(t *testing.T) {
	s, v, _ := fixture(t, &counter{})
	v = ready(t, s, v)
	old := cmd(v, "utterance")
	old.Text = "2枚"
	next := apply(t, s, cmd(v, "interrupt"))
	if next.State.Revision != v.State.Revision || next.State.MediaEpoch <= v.State.MediaEpoch {
		t.Fatal(next)
	}
	if _, err := s.Apply(context.Background(), old); !errors.Is(err, ErrStale) {
		t.Fatal("stale media accepted")
	}
}

func TestJapaneseReadbackCorrectionAndReadOnly(t *testing.T) {
	e := &counter{}
	s, v, _ := fixture(t, e)
	c := cmd(v, "utterance")
	c.Text = "鹿肉、ロース、ロット００１２３、ラベル３枚"
	v = apply(t, s, c)
	if v.State.Confirmation == nil {
		t.Fatal(v)
	}
	c = cmd(v, "utterance")
	c.Text = "2枚に訂正"
	v = apply(t, s, c)
	c = cmd(v, "utterance")
	c.Text = "はい"
	c.ConfirmationID = v.State.Confirmation.ID
	v = apply(t, s, c)
	if v.Operation.Fields["copies"] != "2" {
		t.Fatal(v)
	}
	c = cmd(v, "begin")
	c.Kind = "stock"
	v = apply(t, s, c)
	c = cmd(v, "patch")
	c.Fields = Fields{"product": "鹿肉"}
	v = apply(t, s, c)
	if v.State.Status != "done" || e.calls.Load() != 1 || !strings.Contains(v.State.Reply, "検証用") {
		t.Fatal(v)
	}
}

func TestHTTPRejectsCrossOriginAndUsesSameValidation(t *testing.T) {
	s, v, _ := fixture(t, &counter{})
	handler := s.Handler()
	for _, origin := range []string{"https://evil.example", "null"} {
		req := httptest.NewRequest("POST", "http://localhost:3110/api/sessions", strings.NewReader("{}"))
		req.Header.Set("Origin", origin)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != 403 {
			t.Fatal(w.Code)
		}
	}
	c := cmd(v, "patch")
	c.Fields = Fields{"copies": "-1"}
	b, _ := json.Marshal(c)
	req := httptest.NewRequest("POST", "http://localhost:3110/api/commands", strings.NewReader(string(b)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatal(w.Code, w.Body.String())
	}
}

func TestAnalysisDoesNotReplaceInProgressWork(t *testing.T) {
	s, v, dir := fixture(t, &counter{})
	v = ready(t, s, v)
	r := AnalysisRequest{SessionID: v.State.SessionID, Facility: "fixture", Start: "2026-11-01", End: "2026-11-30", CompareStart: "2026-10-01", CompareEnd: "2026-10-31", Metric: "sales_yen"}
	job, err := s.StartAnalysis(r)
	if err != nil {
		t.Fatal(err)
	}
	s.workers.Wait()
	next, err := s.Get(v.State.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(next.State, v.State) {
		t.Fatal("analysis changed working state")
	}
	if len(next.Jobs) != 1 || next.Jobs[0].ID != job.ID || next.Jobs[0].Current != 30000 || next.Jobs[0].Comparison != 25000 || next.Jobs[0].Difference != 5000 {
		t.Fatal(next.Jobs)
	}
	s.Close()
	s2, err := Open(dir, &counter{})
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	restored, _ := s2.Get(v.State.SessionID)
	if restored.Jobs[0].Status != "done" {
		t.Fatal(restored)
	}
}

func TestAmbiguousNumbersDoNotChangeFields(t *testing.T) {
	for _, text := range []string{"-2枚に訂正", ".07kg", "1,500円", "1e3kg", "3枚か2枚"} {
		s, v, _ := fixture(t, &counter{})
		v = ready(t, s, v)
		before := clone(v.State.Fields)
		c := cmd(v, "utterance")
		c.Text = text
		next := apply(t, s, c)
		if !reflect.DeepEqual(before, next.State.Fields) {
			t.Fatalf("%s changed fields: %+v", text, next.State)
		}
	}
}

func TestFailedDurableReservationNeverDispatches(t *testing.T) {
	e := &counter{}
	s, v, _ := fixture(t, e)
	v = ready(t, s, v)
	before := s.store.dir
	s.store.dir = filepath.Join(before, "missing-parent", "store")
	if _, err := s.Apply(context.Background(), confirm(v)); err == nil {
		t.Fatal("reservation failure ignored")
	}
	s.store.dir = before
	if _, err := s.Apply(context.Background(), confirm(v)); err == nil {
		t.Fatal("poisoned store kept accepting operations")
	}
	if e.calls.Load() != 0 {
		t.Fatal("dispatched without durable reservation")
	}
}
