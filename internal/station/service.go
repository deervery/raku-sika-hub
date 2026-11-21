//go:build linux

package station

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

var ErrStale = errors.New("stale revision or media epoch; fetch the current state")
var ErrBusy = errors.New("operation is executing or unconfirmed; reconcile its result first")

type Service struct {
	mu       sync.Mutex
	store    *Store
	db       Database
	executor Executor
	poison   error
	workers  sync.WaitGroup
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b[:])
}

func Open(dir string, executor Executor) (*Service, error) {
	if executor == nil {
		return nil, errors.New("executor required")
	}
	store, db, err := openStore(dir)
	if err != nil {
		return nil, err
	}
	s := &Service{store: store, db: db, executor: executor}
	for id, job := range s.db.Jobs {
		if job.Status == "queued" || job.Status == "running" {
			job.Status = "failed"
			job.Error = "再起動で中断しました。照会条件を保持しています。"
			s.db.Jobs[id] = job
		}
	}
	// A crash can occur after dispatch but before the response is persisted.
	// The operation stays reserved forever; restart never calls Execute again.
	for id, op := range s.db.Operations {
		if op.Status == "executing" {
			op.Status, op.Result = "unknown", "再起動のため結果未確認"
			s.db.Operations[id] = op
			state := s.db.Sessions[op.SessionID]
			if state.OperationID == id {
				state.Status = "unknown"
				state.Reply = Patterns()[state.Kind].Unknown
				state.Revision++
				state.MediaEpoch++
				s.db.Sessions[op.SessionID] = state
			}
		}
	}
	if err := store.save(s.db); err != nil {
		store.close()
		return nil, err
	}
	return s, nil
}

// Close only after the HTTP server and in-flight calls have drained.
func (s *Service) Close() error {
	s.workers.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.store.close()
}

func (s *Service) snapshot(state State) Snapshot {
	result := Snapshot{State: clone(state), Mock: true, Jobs: []AnalysisJob{}}
	for _, job := range s.db.Jobs {
		if job.SessionID == state.SessionID {
			result.Jobs = append(result.Jobs, clone(job))
		}
	}
	if op, ok := s.db.Operations[state.OperationID]; ok {
		copy := clone(op)
		result.Operation = &copy
	}
	return result
}

func (s *Service) save(db Database) error {
	if s.poison != nil {
		return fmt.Errorf("store unavailable; restart and reconcile: %w", s.poison)
	}
	if err := s.store.save(db); err != nil {
		s.poison = err
		return err
	}
	s.db = db
	return nil
}

func (s *Service) Get(id string) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poison != nil {
		return Snapshot{}, s.poison
	}
	v, ok := s.db.Sessions[id]
	if !ok {
		return Snapshot{}, errors.New("session not found")
	}
	return s.snapshot(v), nil
}

func (s *Service) Create() (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state := State{SessionID: newID(), TaskID: newID(), Revision: 1, MediaEpoch: 1, Kind: "print", Route: "work", Status: "drafting", Fields: Fields{}}
	prepare(&state, nil)
	db := clone(s.db)
	db.Sessions[state.SessionID] = state
	db.History = append(db.History, Event{state.SessionID, state.Revision, "create", time.Now().UTC().Format(time.RFC3339Nano)})
	if err := s.save(db); err != nil {
		return Snapshot{}, err
	}
	return s.snapshot(state), nil
}

func (s *Service) Apply(ctx context.Context, c Command) (Snapshot, error) {
	s.mu.Lock()
	if s.poison != nil {
		s.mu.Unlock()
		return Snapshot{}, s.poison
	}
	state, ok := s.db.Sessions[c.SessionID]
	if !ok {
		s.mu.Unlock()
		return Snapshot{}, errors.New("session not found")
	}
	// Exact confirmation replay is idempotent even after a crash or a new task.
	if c.Action == "confirm" && c.ConfirmationID != "" {
		for _, op := range s.db.Operations {
			if op.SessionID == c.SessionID && op.ConfirmationID == c.ConfirmationID {
				result := s.snapshot(state)
				copy := clone(op)
				result.Operation = &copy
				s.mu.Unlock()
				return result, nil
			}
		}
	}
	if state.Revision != c.Revision || state.MediaEpoch != c.MediaEpoch {
		s.mu.Unlock()
		return Snapshot{}, ErrStale
	}
	question := ""
	if c.Action == "utterance" {
		c, question = interpret(state, c)
	}
	if (state.Status == "executing" || state.Status == "unknown") && c.Action != "reconcile" && c.Action != "interrupt" && c.Action != "chat" && c.Action != "analysis" && c.Action != "question" && c.Action != "repeat" {
		s.mu.Unlock()
		return Snapshot{}, ErrBusy
	}
	db := clone(s.db)
	state = clone(state)
	var dispatch *Operation
	lookup := false
	switch c.Action {
	case "begin":
		if _, ok := Patterns()[c.Kind]; !ok {
			s.mu.Unlock()
			return Snapshot{}, errors.New("unsupported operation kind")
		}
		state.TaskID, state.Kind, state.Fields = newID(), c.Kind, Fields{}
		state.Route = "work"
		state.Revision++
		state.MediaEpoch++
		prepare(&state, nil)
	case "patch":
		if state.Status == "done" || state.Status == "cancelled" || state.Status == "failed" {
			s.mu.Unlock()
			return Snapshot{}, errors.New("begin a new task first")
		}
		if c.Kind != "" {
			if _, ok := Patterns()[c.Kind]; !ok {
				s.mu.Unlock()
				return Snapshot{}, errors.New("unsupported operation kind")
			}
			state.Kind = c.Kind
		}
		changed := []string{}
		for k, v := range c.Fields {
			if state.Fields[k] != v {
				changed = append(changed, k)
			}
			if v == "" {
				delete(state.Fields, k)
			} else {
				state.Fields[k] = v
			}
		}
		if err := validate(state.Fields); err != nil {
			s.mu.Unlock()
			return Snapshot{}, err
		}
		state.Route = "work"
		state.Revision++
		state.MediaEpoch++
		prepare(&state, changed)
	case "confirm":
		if state.Status != "awaiting_confirmation" || state.Confirmation == nil || state.Confirmation.ID != c.ConfirmationID || state.Confirmation.Revision != c.Revision {
			s.mu.Unlock()
			return Snapshot{}, errors.New("latest confirmation required")
		}
		op := Operation{ID: newID(), SessionID: state.SessionID, TaskID: state.TaskID, ConfirmationID: state.Confirmation.ID, Revision: state.Revision, Kind: state.Kind, Fields: clone(state.Fields), Status: "executing", Mock: true}
		db.Operations[op.ID] = op
		dispatch = &op
		state.OperationID, state.Status, state.Reply = op.ID, "executing", "検証用の模擬操作を実行中です。"
		state.Confirmation = nil
		state.Revision++
		state.MediaEpoch++
	case "reconcile":
		op, exists := db.Operations[state.OperationID]
		if !exists || op.Status != "unknown" {
			s.mu.Unlock()
			return Snapshot{}, errors.New("no unknown operation to reconcile")
		}
		dispatch, lookup = &op, true
	case "cancel":
		if state.Status == "done" || state.Status == "failed" {
			s.mu.Unlock()
			return Snapshot{}, errors.New("completed operations cannot be cancelled")
		}
		state.Status, state.Reply = "cancelled", Patterns()[state.Kind].Cancel
		state.Confirmation = nil
		state.Revision++
		state.MediaEpoch++
		state.Route = "work"
	case "interrupt":
		// Retain business revision/confirmation, revoke ASR/TTS/playback epoch.
		state.MediaEpoch++
	case "repeat":
		if state.Confirmation != nil {
			state.Reply = state.Confirmation.Text
			state.Route = "work"
		}
		state.MediaEpoch++
	case "chat", "analysis", "question":
		// Work fields and the view are retained; asynchronous analysis must use
		// a separate notification surface. No Gemini completion is fabricated.
		state.Route = c.Action
		state.MediaEpoch++
		if question != "" {
			state.Reply = question
		} else if c.Action == "chat" {
			state.Reply = "雑談モデルはまだ接続していません。作業内容は保持しています。"
		} else {
			state.Reply = "分析する期間・施設・指標・比較条件を指定してください。"
		}
	default:
		s.mu.Unlock()
		return Snapshot{}, errors.New("unsupported action")
	}
	db.Sessions[state.SessionID] = state
	db.History = append(db.History, Event{state.SessionID, state.Revision, c.Action, time.Now().UTC().Format(time.RFC3339Nano)})
	if err := s.save(db); err != nil {
		s.mu.Unlock()
		return Snapshot{}, err
	}
	result := s.snapshot(state)
	if dispatch != nil {
		s.workers.Add(1)
	}
	s.mu.Unlock()
	if dispatch == nil {
		return result, nil
	}
	defer s.workers.Done()
	var status, message string
	var err error
	if lookup {
		status, message, err = s.executor.Lookup(ctx, *dispatch)
	} else {
		status, message, err = s.executor.Execute(ctx, *dispatch)
	}
	if err != nil || (status != "done" && status != "failed") {
		status, message = "unknown", Patterns()[dispatch.Kind].Unknown
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	db = clone(s.db)
	op := db.Operations[dispatch.ID]
	// Concurrent result inquiries cannot regress a confirmed terminal result.
	if op.Status == "done" || op.Status == "failed" {
		return s.snapshot(db.Sessions[op.SessionID]), nil
	}
	op.Status, op.Result = status, message
	db.Operations[op.ID] = op
	state = db.Sessions[op.SessionID]
	state.Status, state.Reply = status, message
	state.Revision++
	state.MediaEpoch++
	db.Sessions[state.SessionID] = state
	db.History = append(db.History, Event{state.SessionID, state.Revision, "result:" + status, time.Now().UTC().Format(time.RFC3339Nano)})
	if err := s.save(db); err != nil {
		return Snapshot{}, err
	}
	return s.snapshot(state), nil
}

type MockExecutor struct{}

func (MockExecutor) Execute(ctx context.Context, op Operation) (string, string, error) {
	if err := ctx.Err(); err != nil {
		return "unknown", "", err
	}
	return "done", "検証用の模擬操作が完了しました。実機・業務データは変更していません。", nil
}
func (MockExecutor) Lookup(context.Context, Operation) (string, string, error) {
	return "unknown", "模擬操作の外部完了証跡がありません。", nil
}
