package agent

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

// Fake is the test and headless substitute for an Agent. It replays
// Responses in order and records every Request it receives.
type Fake struct {
	mu sync.Mutex

	// Responses and Errors are scripts indexed by request order. Errors is
	// optional; an IsError response also follows the real adapter's error
	// semantics.
	Responses []Response
	Errors    []error

	// Requests is the recorded transcript. Prefer RecordedRequests when a
	// caller may inspect it while another goroutine is making calls.
	Requests []Request
}

var _ Agent = (*Fake)(nil)

// NewFake returns a Fake with the supplied response script.
func NewFake(responses ...Response) *Fake {
	return &Fake{Responses: slices.Clone(responses)}
}

// Call records request and returns the next scripted response.
func (f *Fake) Call(ctx context.Context, request Request) (Response, error) {
	if err := ctx.Err(); err != nil {
		return Response{}, errors.Join(ErrCall, err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	index := len(f.Requests)
	f.Requests = append(f.Requests, request)
	if index >= len(f.Responses) {
		return Response{}, fmt.Errorf("%w: fake has no response for request %d", ErrCall, index)
	}

	response := f.Responses[index]
	if index < len(f.Errors) && f.Errors[index] != nil {
		return response, errors.Join(ErrCall, f.Errors[index])
	}
	if response.IsError {
		return response, fmt.Errorf("%w: fake response %d is marked as an error", ErrCall, index)
	}
	return response, nil
}

// RecordedRequests returns a snapshot of the transcript.
func (f *Fake) RecordedRequests() []Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.Requests)
}
