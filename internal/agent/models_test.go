package agent

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const initializeReply = `{"type":"system","subtype":"hook_started"}
{"type":"control_response","response":{"subtype":"success","request_id":"scriptorium-list-models","response":{"models":[` +
	`{"value":"default","resolvedModel":"claude-opus-5-5","displayName":"Default (recommended)"},` +
	`{"value":"opus","resolvedModel":"claude-opus-5-5","displayName":"Opus 5.5"},` +
	`{"value":"haiku","resolvedModel":"claude-haiku-4-5-20251001","displayName":"Haiku 4.5"},` +
	`{"value":"claude-opus-5","resolvedModel":"claude-opus-5","displayName":"Opus 5"},` +
	`{"value":"claude-opus-5","resolvedModel":"claude-opus-5","displayName":"Opus 5"}]}}}
`

func TestClaudeListsThePinnedModelsItsCLIReports(t *testing.T) {
	argsPath := filepath.Join(t.TempDir(), "args.json")
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_ARGS", argsPath)
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_OUTPUT", initializeReply)

	models, err := (&Claude{Command: helperCommand(t)}).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	want := []ModelInfo{
		{Name: "claude-opus-5-5", Label: "Opus 5.5"},
		{Name: "claude-haiku-4-5-20251001", Label: "Haiku 4.5"},
		{Name: "claude-opus-5", Label: "Opus 5"},
	}
	if !slices.Equal(models, want) {
		t.Errorf("models = %+v, want the resolved Models without the user's default or duplicates: %+v", models, want)
	}

	args := readArgs(t, argsPath)
	assertArgValue(t, args, "--input-format", "stream-json")
	assertArgValue(t, args, "--output-format", "stream-json")
	assertArgValue(t, args, "--tools", "")
	if slices.Contains(args, "--model") {
		t.Error("listing Models named a Model")
	}
}

func TestClaudeReportsARefusedListing(t *testing.T) {
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_OUTPUT", `{"type":"control_response","response":{"subtype":"error","request_id":"scriptorium-list-models","error":"not logged in"}}`+"\n")

	_, err := (&Claude{Command: helperCommand(t)}).ListModels(context.Background())
	if err == nil || !errors.Is(err, ErrCall) || !strings.Contains(err.Error(), "not logged in") {
		t.Fatalf("ListModels error = %v, want an ErrCall naming the reason", err)
	}
}

func TestClaudeReportsACLIThatNeverAnswers(t *testing.T) {
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_STDERR", "claude: command failed")
	t.Setenv("SCRIPTORIUM_AGENT_HELPER_EXIT", "1")

	_, err := (&Claude{Command: helperCommand(t)}).ListModels(context.Background())
	if err == nil || !errors.Is(err, ErrCall) || !strings.Contains(err.Error(), "command failed") {
		t.Fatalf("ListModels error = %v, want an ErrCall carrying the CLI's stderr", err)
	}
}

func TestTheFakeListsItsScriptedModelsWithoutRecordingARequest(t *testing.T) {
	fake := NewFake()
	fake.Models = []ModelInfo{{Name: "cheap", Label: "Cheap"}}

	models, err := fake.ListModels(context.Background())
	if err != nil || !slices.Equal(models, fake.Models) {
		t.Fatalf("ListModels = %+v, %v", models, err)
	}
	if got := len(fake.RecordedRequests()); got != 0 {
		t.Errorf("requests = %d, want listing not recorded as a Request", got)
	}
}
