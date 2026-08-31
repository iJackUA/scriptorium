// Package agent contains the narrow boundary between Scriptorium and an
// external command-line Agent.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Model is the name of a model understood by an Agent.
//
// It is an alias rather than a separate string type because model names come
// from hand-edited workspace configuration and should not need conversions at
// every call site.
type Model = string

// Effort controls how much reasoning an Agent spends on one request.
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
)

// Request is one plain language-model request. The adapter owns how its CLI
// represents the system prompt and other invocation options; callers only
// provide the prompt and the model to use.
type Request struct {
	Prompt string
	Model  Model
	Effort Effort
}

// Response is the useful part of a CLI response.
type Response struct {
	Result  string
	IsError bool
	Cost    float64
}

// Agent is the sole substitution seam for slow, costly and nondeterministic
// external AI work.
type Agent interface {
	Call(context.Context, Request) (Response, error)
}

var (
	// ErrCall reports that an Agent could not produce a usable response.
	ErrCall = errors.New("agent call failed")
	// ErrUnknown reports a configured Agent for which Scriptorium has no
	// adapter.
	ErrUnknown = errors.New("unknown Agent")
)

const (
	// ClaudeName is the configured name of the Agent adapter available in v1.
	ClaudeName = "claude"

	// ClaudeCommand is the executable used by NewClaude when no override is
	// supplied. Keeping the name here makes the production default explicit
	// while letting tests use a helper executable.
	ClaudeCommand = ClaudeName

	// defaultSystemPrompt replaces Claude Code's coding-agent persona. The
	// request itself remains the user prompt, while this fixed adapter prompt
	// makes the CLI behave as a plain text model.
	defaultSystemPrompt = "You are a plain language model. Follow the user's request and return only the requested result. Do not use tools."
)

// ValidateName checks the Agent names accepted by the current build.
func ValidateName(name string) error {
	if name == ClaudeName {
		return nil
	}
	return fmt.Errorf("%w %q", ErrUnknown, name)
}

// New constructs the adapter named in configuration.
func New(name string) (Agent, error) {
	return NewWithLogger(name, nil)
}

// NewWithLogger constructs an adapter which records every Agent invocation
// when logger is non-nil.
func NewWithLogger(name string, logger Logger) (Agent, error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}
	return NewClaudeWithLogger(logger), nil
}

// Claude drives the claude CLI as a single non-interactive plain-model call.
// Command is overridable for tests; its zero value is the real claude
// executable. SystemPrompt is likewise exposed so a test can prove the
// replacement flag is used without depending on Claude Code's own prompt.
type Claude struct {
	Command      string
	SystemPrompt string
	Logger       Logger
}

var _ Agent = (*Claude)(nil)

// NewClaude returns the production Claude adapter.
func NewClaude() *Claude { return &Claude{} }

// NewClaudeWithLogger returns the production adapter with an optional full
// request/reply transcript.
func NewClaudeWithLogger(logger Logger) *Claude { return &Claude{Logger: logger} }

// Call invokes claude without a tool loop or conversational session and
// decodes its JSON result.
func (c *Claude) Call(ctx context.Context, request Request) (Response, error) {
	if strings.TrimSpace(request.Model) == "" {
		return Response{}, fmt.Errorf("%w: Model is required", ErrCall)
	}

	settingsBytes, err := json.Marshal(struct {
		Model string `json:"model"`
	}{Model: request.Model})
	if err != nil {
		return Response{}, fmt.Errorf("%w: encode Model settings: %v", ErrCall, err)
	}
	settings := string(settingsBytes)
	command := c.Command
	if command == "" {
		command = ClaudeCommand
	}
	systemPrompt := c.SystemPrompt
	if systemPrompt == "" {
		systemPrompt = defaultSystemPrompt
	}

	args := []string{
		"--print",
		"--system-prompt", systemPrompt,
		"--tools", "",
		"--disallowed-tools", "mcp__*",
		"--output-format", "json",
		"--model", request.Model,
		"--effort", string(effortFor(request.Effort)),
		// Claude's user/project configuration can silently override --model.
		// An inline settings object is therefore part of every invocation.
		"--settings", settings,
		request.Prompt,
	}
	started := time.Now()
	if err := c.record(Event{At: started, Phase: "invoked", Command: command, Arguments: args, Model: request.Model, Prompt: request.Prompt}); err != nil {
		return Response{}, err
	}
	cmd := exec.CommandContext(ctx, command, args...)
	output, err := cmd.Output()
	if err != nil {
		callErr := fmt.Errorf("%w: run claude: %v", ErrCall, err)
		if exitErr, ok := err.(*exec.ExitError); ok {
			message := strings.TrimSpace(string(exitErr.Stderr))
			if message != "" {
				callErr = fmt.Errorf("%w: claude exited unsuccessfully: %s", ErrCall, message)
			}
		}
		if recordErr := c.record(Event{At: time.Now(), Phase: "returned", Command: command, Model: request.Model, Prompt: request.Prompt, Error: callErr.Error(), DurationMS: time.Since(started).Milliseconds()}); recordErr != nil {
			return Response{}, recordErr
		}
		return Response{}, callErr
	}

	var response Response
	if err := unmarshalResponse(output, &response); err != nil {
		callErr := fmt.Errorf("%w: decode claude response: %v", ErrCall, err)
		if recordErr := c.record(Event{At: time.Now(), Phase: "returned", Command: command, Model: request.Model, Prompt: request.Prompt, RawReply: string(output), Error: callErr.Error(), DurationMS: time.Since(started).Milliseconds()}); recordErr != nil {
			return Response{}, recordErr
		}
		return Response{}, callErr
	}
	if response.IsError {
		callErr := fmt.Errorf("%w: claude reported an error", ErrCall)
		if recordErr := c.record(Event{At: time.Now(), Phase: "returned", Command: command, Model: request.Model, Prompt: request.Prompt, RawReply: string(output), Reply: response.Result, Error: callErr.Error(), DurationMS: time.Since(started).Milliseconds(), Cost: response.Cost}); recordErr != nil {
			return Response{}, recordErr
		}
		return response, callErr
	}
	if err := c.record(Event{At: time.Now(), Phase: "returned", Command: command, Model: request.Model, Prompt: request.Prompt, RawReply: string(output), Reply: response.Result, DurationMS: time.Since(started).Milliseconds(), Cost: response.Cost}); err != nil {
		return Response{}, err
	}
	return response, nil
}

func effortFor(effort Effort) Effort {
	if effort == "" {
		return EffortMedium
	}
	return effort
}

func (c *Claude) record(event Event) error {
	if c.Logger == nil {
		return nil
	}
	if err := c.Logger.Record(event); err != nil {
		return fmt.Errorf("%w: record Agent transcript: %v", ErrCall, err)
	}
	return nil
}
