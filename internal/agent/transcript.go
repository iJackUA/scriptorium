package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Event records one side of an Agent call. It intentionally preserves prose:
// the transcript is a local debugging artifact, not telemetry.
type Event struct {
	At         time.Time `json:"at"`
	Phase      string    `json:"phase"`
	Command    string    `json:"command"`
	Arguments  []string  `json:"arguments,omitempty"`
	Model      Model     `json:"model"`
	Prompt     string    `json:"prompt"`
	RawReply   string    `json:"raw_reply,omitempty"`
	Reply      string    `json:"reply,omitempty"`
	Error      string    `json:"error,omitempty"`
	DurationMS int64     `json:"duration_ms,omitempty"`
	Cost       float64   `json:"cost_usd,omitempty"`
}

// Logger receives the full request and reply transcript of an Agent call.
type Logger interface {
	Record(Event) error
}

// FileLogger appends JSON Lines to a private local file. A separate record is
// written when the Agent is invoked and when it returns, so a missing return
// record identifies a process that is still blocked.
type FileLogger struct {
	path string
	mu   sync.Mutex
}

// NewFileLogger creates a private transcript under a workspace-local path.
func NewFileLogger(path string) (*FileLogger, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create Agent transcript folder: %w", err)
	}
	return &FileLogger{path: path}, nil
}

// Record appends one complete JSON record. The transcript retains full Book
// prose and Agent output, so it is created with owner-only permissions.
func (l *FileLogger) Record(event Event) error {
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("encode Agent transcript: %w", err)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	file, err := os.OpenFile(l.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open Agent transcript: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(append(body, '\n')); err != nil {
		return fmt.Errorf("write Agent transcript: %w", err)
	}
	return nil
}
