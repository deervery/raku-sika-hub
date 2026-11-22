//go:build linux

// Package station is an isolated, mock-only reference implementation of the
// station protocol. Production device routes do not import this package.
package station

import "context"

type Fields map[string]string

type Confirmation struct {
	ID       string `json:"id"`
	Revision uint64 `json:"revision"`
	Text     string `json:"text"`
}

type View struct {
	Component string   `json:"component"`
	Fields    []string `json:"fields"`
	Changed   []string `json:"changed"`
	Mode      string   `json:"mode"`
}

type State struct {
	SessionID    string        `json:"session_id"`
	TaskID       string        `json:"task_id"`
	Revision     uint64        `json:"revision"`
	MediaEpoch   uint64        `json:"media_epoch"`
	Kind         string        `json:"kind"`
	Route        string        `json:"route"`
	Status       string        `json:"status"`
	Fields       Fields        `json:"fields"`
	Missing      []string      `json:"missing"`
	Confirmation *Confirmation `json:"confirmation,omitempty"`
	OperationID  string        `json:"operation_id,omitempty"`
	Reply        string        `json:"reply"`
	View         View          `json:"view"`
}

// Commands from ASR carry the revision/epoch captured at speech start, not at
// inference completion. UI edits use the same compare-and-swap contract.
type Command struct {
	SessionID      string `json:"session_id"`
	Revision       uint64 `json:"revision"`
	MediaEpoch     uint64 `json:"media_epoch"`
	Action         string `json:"action"`
	Kind           string `json:"kind,omitempty"`
	Fields         Fields `json:"fields,omitempty"`
	Text           string `json:"text,omitempty"`
	ConfirmationID string `json:"confirmation_id,omitempty"`
}

type Operation struct {
	ID             string `json:"id"`
	SessionID      string `json:"session_id"`
	TaskID         string `json:"task_id"`
	ConfirmationID string `json:"confirmation_id"`
	Revision       uint64 `json:"revision"`
	Kind           string `json:"kind"`
	Fields         Fields `json:"fields"`
	Status         string `json:"status"`
	Result         string `json:"result,omitempty"`
	Mock           bool   `json:"mock"`
}

type Snapshot struct {
	Jobs      []AnalysisJob `json:"jobs"`
	State     State         `json:"state"`
	Operation *Operation    `json:"operation,omitempty"`
	Mock      bool          `json:"mock"`
}

type Event struct {
	SessionID string `json:"session_id"`
	Revision  uint64 `json:"revision"`
	Action    string `json:"action"`
	At        string `json:"at"`
}

type Database struct {
	Jobs       map[string]AnalysisJob `json:"jobs"`
	Schema     int                    `json:"schema"`
	Sessions   map[string]State       `json:"sessions"`
	Operations map[string]Operation   `json:"operations"`
	History    []Event                `json:"history"`
}

// Executor is deliberately not connected to a real printer or facility DB.
// Execute may have taken effect even when it returns an error. Lookup must be
// read-only; absence of evidence must return unknown, never authorize a retry.
type Executor interface {
	Execute(context.Context, Operation) (status, result string, err error)
	Lookup(context.Context, Operation) (status, result string, err error)
}
