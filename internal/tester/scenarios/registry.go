// Package scenarios is the registry and runtime for core-tester scenarios.
//
// Each scenario lives in its own file and registers itself with Register at
// init time. core-tester list enumerates names; core-tester run --scenario
// <name> dispatches to the registered Runner.
package scenarios

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/pflag"
)

// Scenario describes a single RAN/UE scenario.
type Scenario struct {
	// Name is the value passed to --scenario.
	Name string

	// BindFlags attaches scenario-specific flags to fs. It is called once
	// per invocation. The returned value is handed to Run as-is.
	BindFlags func(fs *pflag.FlagSet) any

	// Run executes the scenario. params is whatever BindFlags returned;
	// env carries the common flag values (core N2 addresses, gNB specs).
	Run func(ctx context.Context, env Env, params any) error
}

var (
	mu       sync.Mutex
	registry = map[string]Scenario{}
)

// Register adds s to the registry. Panics on duplicate name.
func Register(s Scenario) {
	mu.Lock()
	defer mu.Unlock()

	if _, dup := registry[s.Name]; dup {
		panic(fmt.Sprintf("scenario %q already registered", s.Name))
	}

	registry[s.Name] = s
}

// Get returns the named scenario, or false if not found.
func Get(name string) (Scenario, bool) {
	mu.Lock()
	defer mu.Unlock()

	s, ok := registry[name]

	return s, ok
}

// List returns all registered scenario names, sorted.
func List() []string {
	mu.Lock()
	defer mu.Unlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}
