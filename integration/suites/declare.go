// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package suites

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"sync"
	"testing"
)

const DumpEnv = "ELLA_SUITES_DUMP"

type Declaration struct {
	Test           string `json:"test"`
	Suite          string `json:"suite"`
	SubtestPrefix  string `json:"subtest_prefix,omitempty"`
	ExcludeSubtest string `json:"exclude_subtest_prefix,omitempty"`
}

var (
	mu           sync.Mutex
	declarations []Declaration
)

func listing() bool {
	return os.Getenv(DumpEnv) != ""
}

func Declare(t *testing.T, suite Name) {
	t.Helper()
	record(t, Declaration{Test: t.Name(), Suite: string(suite)})
}

func DeclareSplit(t *testing.T, prefix string, matching, rest Name) {
	t.Helper()
	record(t,
		Declaration{Test: t.Name(), Suite: string(matching), SubtestPrefix: prefix},
		Declaration{Test: t.Name(), Suite: string(rest), ExcludeSubtest: prefix},
	)
}

func record(t *testing.T, ds ...Declaration) {
	t.Helper()

	for _, d := range ds {
		if _, ok := Definitions[Name(d.Suite)]; !ok {
			t.Fatalf("%s declares unknown suite %q", d.Test, d.Suite)
		}
	}

	mu.Lock()
	declarations = append(declarations, ds...)
	mu.Unlock()

	if listing() {
		t.Skip("listing suite declarations")
	}
}

func WriteDump() error {
	path := os.Getenv(DumpEnv)
	if path == "" {
		return nil
	}

	mu.Lock()
	out := append([]Declaration(nil), declarations...)
	mu.Unlock()

	sort.Slice(out, func(i, j int) bool {
		if out[i].Test != out[j].Test {
			return out[i].Test < out[j].Test
		}

		return out[i].Suite < out[j].Suite
	})

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal declarations: %w", err)
	}

	return os.WriteFile(path, append(b, '\n'), 0o600)
}

func Require(t *testing.T, suite Name) {
	t.Helper()
	Declare(t, suite)

	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}
}

func RequireSplit(t *testing.T, prefix string, matching, rest Name) {
	t.Helper()
	DeclareSplit(t, prefix, matching, rest)

	if os.Getenv("INTEGRATION") == "" {
		t.Skip("skipping integration tests, set environment variable INTEGRATION")
	}
}
