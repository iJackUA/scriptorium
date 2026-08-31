package agent

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestClaudeUsesPlainModelInvocationAndReadsJSONResponse(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.json")
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_ARGS", argsPath)
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_OUTPUT", `{"result":"translated","is_error":false,"total_cost_usd":0.42}`)
	command := helperCommand(t)

	request := Request{Prompt: "translate this", Model: "claude-opus"}
	response, err := (&Claude{Command: command}).Call(context.Background(), request)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if response != (Response{Result: "translated", Cost: 0.42}) {
		t.Errorf("response = %+v", response)
	}

	args := readArgs(t, argsPath)
	if !slices.Contains(args, "--print") {
		t.Error("invocation did not use print mode")
	}
	if slices.Contains(args, "--append-system-prompt") {
		t.Error("invocation appended the system prompt")
	}
	assertArgValue(t, args, "--system-prompt", "You are a plain language model. Follow the user's request and return only the requested result. Do not use tools.")
	assertArgValue(t, args, "--tools", "")
	assertArgValue(t, args, "--disallowed-tools", "mcp__*")
	assertArgValue(t, args, "--output-format", "json")
	assertArgValue(t, args, "--model", request.Model)
	assertArgValue(t, args, "--effort", "medium")

	settings := argValue(t, args, "--settings")
	var settingsJSON map[string]string
	if err := json.Unmarshal([]byte(settings), &settingsJSON); err != nil {
		t.Fatalf("settings override is not JSON: %v", err)
	}
	if settingsJSON["model"] != request.Model {
		t.Errorf("settings model = %q, want %q", settingsJSON["model"], request.Model)
	}
	if args[len(args)-1] != request.Prompt {
		t.Errorf("prompt argument = %q, want %q", args[len(args)-1], request.Prompt)
	}
}

func TestClaudeTurnsAnErrorFlagIntoAnAgentError(t *testing.T) {
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_OUTPUT", `{"result":"diagnostic","is_error":true,"total_cost_usd":0.17}`)
	command := helperCommand(t)

	response, err := (&Claude{Command: command}).Call(context.Background(), Request{Prompt: "prompt", Model: "model"})
	if err == nil {
		t.Fatal("Call succeeded for an error response")
	}
	if !errors.Is(err, ErrCall) {
		t.Errorf("error = %v, want ErrCall", err)
	}
	if response.Result != "diagnostic" || response.Cost != 0.17 {
		t.Errorf("error response = %+v, want parsed response preserved", response)
	}
}

func TestClaudeTurnsANonZeroExitIntoAnAgentError(t *testing.T) {
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_EXIT", "23")
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_STDERR", "the CLI failed")
	command := helperCommand(t)

	_, err := (&Claude{Command: command}).Call(context.Background(), Request{Prompt: "prompt", Model: "model"})
	if err == nil || !errors.Is(err, ErrCall) {
		t.Fatalf("Call error = %v, want an ErrCall", err)
	}
	if !strings.Contains(err.Error(), "the CLI failed") {
		t.Errorf("error = %v, want stderr", err)
	}
}

func TestClaudeRecordsItsFullInvocationAndReplyTranscript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "agent-transcript.jsonl")
	logger, err := NewFileLogger(path)
	if err != nil {
		t.Fatalf("NewFileLogger: %v", err)
	}
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_OUTPUT", `{"result":"reply prose","is_error":false,"total_cost_usd":0.42}`)
	if _, err := (&Claude{Command: helperCommand(t), Logger: logger}).Call(context.Background(), Request{Prompt: "request prose", Model: "cheap"}); err != nil {
		t.Fatalf("Call: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read transcript: %v", err)
	}
	var events []Event
	for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n") {
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode transcript: %v", err)
		}
		events = append(events, event)
	}
	if len(events) != 2 || events[0].Phase != "invoked" || events[1].Phase != "returned" {
		t.Fatalf("events = %#v, want invocation and return", events)
	}
	if events[0].Prompt != "request prose" || events[1].Reply != "reply prose" || !strings.Contains(events[1].RawReply, "reply prose") {
		t.Errorf("transcript = %#v, want full request and reply", events)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat transcript: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("transcript mode = %o, want 600", info.Mode().Perm())
	}
}

func TestFakeReplaysResponsesAndRecordsARequestTranscript(t *testing.T) {
	fake := NewFake(
		Response{Result: "one", Cost: 0.1},
		Response{Result: "two", Cost: 0.2},
	)
	first := Request{Prompt: "dictionary Term: Holmes", Model: "cheap"}
	second := Request{Prompt: "Continuity Window\nnode 4", Model: "strong"}

	gotFirst, err := fake.Call(context.Background(), first)
	if err != nil {
		t.Fatalf("first Call: %v", err)
	}
	gotSecond, err := fake.Call(context.Background(), second)
	if err != nil {
		t.Fatalf("second Call: %v", err)
	}
	if gotFirst.Result != "one" || gotSecond.Result != "two" {
		t.Errorf("responses = %+v, %+v", gotFirst, gotSecond)
	}
	if got := fake.RecordedRequests(); !slices.Equal(got, []Request{first, second}) {
		t.Errorf("recorded requests = %+v", got)
	}
}

func TestFakeReportsAnExhaustedScript(t *testing.T) {
	fake := NewFake(Response{Result: "only"})
	_, _ = fake.Call(context.Background(), Request{Prompt: "first", Model: "model"})
	_, err := fake.Call(context.Background(), Request{Prompt: "second", Model: "model"})
	if err == nil || !errors.Is(err, ErrCall) {
		t.Fatalf("second Call error = %v, want ErrCall", err)
	}
	if got := len(fake.RecordedRequests()); got != 2 {
		t.Errorf("recorded request count = %d, want 2", got)
	}
}

func TestUnknownAgentNamesAreRejected(t *testing.T) {
	if err := ValidateName("codex"); err == nil || !errors.Is(err, ErrUnknown) {
		t.Fatalf("ValidateName(codex) = %v, want ErrUnknown", err)
	}
	if _, err := New("not-an-agent"); err == nil || !errors.Is(err, ErrUnknown) {
		t.Fatalf("New(not-an-agent) = %v, want ErrUnknown", err)
	}
}

func readArgs(t *testing.T, path string) []string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read helper args: %v", err)
	}
	text := strings.TrimSuffix(string(body), "\n")
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func assertArgValue(t *testing.T, args []string, flag, want string) string {
	t.Helper()
	got := argValue(t, args, flag)
	if got != want {
		t.Errorf("%s value = %q, want %q", flag, got, want)
	}
	return got
}

func argValue(t *testing.T, args []string, flag string) string {
	t.Helper()
	for i, arg := range args {
		if i+1 >= len(args) {
			break
		}
		if arg == flag {
			return args[i+1]
		}
	}
	t.Errorf("missing %s in %q", flag, args)
	return ""
}

func helperCommand(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "claude-helper")
	script := `#!/bin/sh
if [ -n "$SCRIPTORIUM_AGENT_HELPER_ARGS" ]; then
  printf '%s\n' "$@" > "$SCRIPTORIUM_AGENT_HELPER_ARGS"
fi
if [ -n "$SCRIPTORIUM_AGENT_HELPER_STDERR" ]; then
  printf '%s' "$SCRIPTORIUM_AGENT_HELPER_STDERR" >&2
fi
if [ -n "$SCRIPTORIUM_AGENT_HELPER_OUTPUT" ]; then
  printf '%s' "$SCRIPTORIUM_AGENT_HELPER_OUTPUT"
fi
exit "${SCRIPTORIUM_AGENT_HELPER_EXIT:-0}"
`
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write helper: %v", err)
	}
	return path
}
