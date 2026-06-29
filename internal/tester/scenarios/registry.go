// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package scenarios is the registry and runtime for core-tester scenarios.
// Each scenario registers itself with Register at init time.
package scenarios

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/spf13/pflag"
)

type Scenario struct {
	// Name is the value passed to --scenario.
	Name string

	// BindFlags attaches scenario-specific flags to fs; its return value is
	// handed to Run as-is.
	BindFlags func(fs *pflag.FlagSet) any

	Run func(ctx context.Context, env Env, params any) error

	// Fixture returns the Ella Core fixture to provision before the scenario
	// runs. When nil, only the baseline fixture applies.
	Fixture func(env Env) FixtureSpec
}

var (
	mu       sync.Mutex
	registry = map[string]Scenario{}
)

// Register adds s to the registry, panicking on a duplicate name.
func Register(s Scenario) {
	mu.Lock()
	defer mu.Unlock()

	if _, dup := registry[s.Name]; dup {
		panic(fmt.Sprintf("scenario %q already registered", s.Name))
	}

	registry[s.Name] = s
}

func Get(name string) (Scenario, bool) {
	mu.Lock()
	defer mu.Unlock()

	s, ok := registry[name]

	return s, ok
}

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
