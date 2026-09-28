package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// ModelInfo is one Model an Agent says it accepts.
type ModelInfo struct {
	// Name is what configuration stores and what the Agent is invoked with:
	// the pinned Model, never a moving alias.
	Name  Model
	Label string
}

// ModelLister is an Agent that can say which Models it accepts. The list
// comes from the Agent itself, so it is as current as the installed CLI.
type ModelLister interface {
	ListModels(context.Context) ([]ModelInfo, error)
}

var _ ModelLister = (*Claude)(nil)

// listModelsTimeout bounds a CLI that starts but never answers. Listing sends
// no prompt, so a healthy CLI replies in well under a second.
const listModelsTimeout = 20 * time.Second

// ListModels asks the claude CLI for its Models through the initialize
// control request of its stream-json protocol — the one the Agent SDK uses
// for the same question. No prompt is sent, so it costs nothing.
func (c *Claude) ListModels(ctx context.Context) ([]ModelInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, listModelsTimeout)
	defer cancel()

	command := c.Command
	if command == "" {
		command = ClaudeCommand
	}
	cmd := exec.CommandContext(ctx, command,
		"--print",
		"--input-format", "stream-json",
		"--output-format", "stream-json",
		"--verbose",
		"--tools", "",
		"--disallowed-tools", "mcp__*",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: list claude Models: %v", ErrCall, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%w: list claude Models: %v", ErrCall, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%w: list claude Models: %v", ErrCall, err)
	}
	// The process has no further use once it has answered, and it must not
	// outlive the request whichever way this returns.
	defer func() {
		_ = stdin.Close()
		cancel()
		_ = cmd.Wait()
	}()

	const requestID = "scriptorium-list-models"
	request, err := json.Marshal(map[string]any{
		"type":       "control_request",
		"request_id": requestID,
		"request":    map[string]string{"subtype": "initialize"},
	})
	if err != nil {
		return nil, fmt.Errorf("%w: encode initialize request: %v", ErrCall, err)
	}
	if _, err := stdin.Write(append(request, '\n')); err != nil {
		return nil, fmt.Errorf("%w: list claude Models: %v", ErrCall, err)
	}

	models, err := readModels(bufio.NewReader(stdout), requestID)
	if err != nil {
		if message := strings.TrimSpace(stderr.String()); message != "" {
			return nil, fmt.Errorf("%w: list claude Models: %v: %s", ErrCall, err, message)
		}
		return nil, fmt.Errorf("%w: list claude Models: %v", ErrCall, err)
	}
	return models, nil
}

// readModels reads stream-json lines until the answer to requestID, skipping
// the other events the CLI emits around it.
func readModels(r *bufio.Reader, requestID string) ([]ModelInfo, error) {
	for {
		line, err := r.ReadBytes('\n')
		if len(bytes.TrimSpace(line)) > 0 {
			var message struct {
				Type     string `json:"type"`
				Response struct {
					Subtype   string `json:"subtype"`
					RequestID string `json:"request_id"`
					Error     string `json:"error"`
					Response  struct {
						Models []struct {
							Value         string `json:"value"`
							ResolvedModel string `json:"resolvedModel"`
							DisplayName   string `json:"displayName"`
						} `json:"models"`
					} `json:"response"`
				} `json:"response"`
			}
			if json.Unmarshal(line, &message) == nil && message.Type == "control_response" && message.Response.RequestID == requestID {
				if message.Response.Subtype != "success" {
					return nil, fmt.Errorf("claude refused the request: %s", message.Response.Error)
				}
				var models []ModelInfo
				seen := map[string]bool{}
				for _, model := range message.Response.Response.Models {
					// "default" is whatever the user's own configuration picks,
					// which is exactly what Scriptorium pins against.
					if model.Value == "default" {
						continue
					}
					name := model.ResolvedModel
					if name == "" {
						name = model.Value
					}
					if name == "" || seen[name] {
						continue
					}
					seen[name] = true
					models = append(models, ModelInfo{Name: name, Label: model.DisplayName})
				}
				return models, nil
			}
		}
		if errors.Is(err, io.EOF) {
			return nil, errors.New("claude exited without listing its Models")
		}
		if err != nil {
			return nil, err
		}
	}
}
