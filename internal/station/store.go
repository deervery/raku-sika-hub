//go:build linux

package station

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type Store struct {
	dir  string
	lock *os.File
}

func openStore(dir string) (*Store, Database, error) {
	db := Database{Schema: 1, Sessions: map[string]State{}, Operations: map[string]Operation{}, Jobs: map[string]AnalysisJob{}}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, db, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, db, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, db, fmt.Errorf("station store already in use: %w", err)
	}
	s := &Store{dir: dir, lock: f}
	b, err := os.ReadFile(filepath.Join(dir, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, db, nil
	}
	if err == nil {
		err = json.Unmarshal(b, &db)
	}
	if db.Jobs == nil {
		db.Jobs = map[string]AnalysisJob{}
	}
	if err == nil && (db.Schema != 1 || db.Sessions == nil || db.Operations == nil) {
		err = errors.New("unsupported or invalid station store")
	}
	if err != nil {
		s.close()
		return nil, db, err
	}
	return s, db, nil
}

// The checkpoint is fsynced before rename, followed by directory fsync. Failure
// poisons the service: callers must restart/reconcile instead of dispatching.
func (s *Store) save(db Database) error {
	b, err := json.Marshal(db)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(s.dir, ".state-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(f.Name(), filepath.Join(s.dir, "state.json")); err != nil {
		return err
	}
	d, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func (s *Store) close() error { return s.lock.Close() }

func clone[T any](v T) T {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var out T
	if err := json.Unmarshal(b, &out); err != nil {
		panic(err)
	}
	return out
}
