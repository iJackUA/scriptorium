package ui

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ijackua/scriptorium/internal/agent"
)

// How long a Model list is trusted. The list only changes when the Agent's
// CLI is updated, while the pickers render on every screen, so a fresh answer
// is not worth a process per render. A failure is retried sooner: the usual
// cause is a CLI the user is about to install or log in to.
const (
	catalogTTL        = 10 * time.Minute
	catalogFailureTTL = time.Minute
)

// modelCatalogs asks each Agent which Models it accepts and remembers the
// answer for a while.
type modelCatalogs struct {
	agents func(string, agent.Logger) (agent.Agent, error)
	now    func() time.Time

	mu      sync.Mutex
	entries map[string]catalogEntry
}

type catalogEntry struct {
	models []agent.ModelInfo
	err    error
	until  time.Time
}

func newModelCatalogs(agents func(string, agent.Logger) (agent.Agent, error)) *modelCatalogs {
	return &modelCatalogs{agents: agents, now: time.Now, entries: map[string]catalogEntry{}}
}

// models is the Model list of the named Agent. An Agent that cannot list its
// Models is reported rather than hidden, so the picker can say why it offers
// nothing but the Models already configured.
func (c *modelCatalogs) models(name string) ([]agent.ModelInfo, error) {
	if c == nil {
		// Screens assembled without Agents, as some tests do, offer only
		// what is configured.
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if entry, ok := c.entries[name]; ok && c.now().Before(entry.until) {
		return entry.models, entry.err
	}
	models, err := c.fetch(name)
	ttl := catalogTTL
	if err != nil {
		ttl = catalogFailureTTL
	}
	c.entries[name] = catalogEntry{models: models, err: err, until: c.now().Add(ttl)}
	return models, err
}

func (c *modelCatalogs) fetch(name string) ([]agent.ModelInfo, error) {
	client, err := c.agents(name, nil)
	if err != nil {
		return nil, err
	}
	lister, ok := client.(agent.ModelLister)
	if !ok {
		return nil, errors.New("this Agent cannot list its Models")
	}
	return lister.ListModels(context.Background())
}
