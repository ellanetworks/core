// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

var testName = regexp.MustCompile(`^Test[A-Za-z0-9_]*$`)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dir, err := os.MkdirTemp("", "suites")
	if err != nil {
		return fmt.Errorf("temp dir: %w", err)
	}

	defer func() { _ = os.RemoveAll(dir) }()

	decls, err := declare(ctx, filepath.Join(dir, "declarations.json"))
	if err != nil {
		return err
	}

	defined, err := definedTests(ctx)
	if err != nil {
		return err
	}

	if missing := undeclared(defined, decls); len(missing) > 0 {
		return fmt.Errorf("these tests in integration/ call no suites.Require, so no CI job would run them:\n  %s\n"+
			"declare a suite for each, or add it to suites.Exempt with a reason",
			strings.Join(missing, "\n  "))
	}

	prefixes, err := scenarioPrefixes()
	if err != nil {
		return err
	}

	legs, err := suites.BuildLegs(decls, prefixes)
	if err != nil {
		return err
	}

	out, err := json.Marshal(legs)
	if err != nil {
		return fmt.Errorf("marshal legs: %w", err)
	}

	fmt.Println(string(out))

	return nil
}

func declare(ctx context.Context, path string) ([]suites.Declaration, error) {
	cmd := exec.CommandContext(ctx, "go", "test", "./integration/", "-run", ".", "-count=1")

	cmd.Env = append(os.Environ(), suites.DumpEnv+"="+path)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("listing run: %w", err)
	}

	// #nosec G304 -- path was created by this program inside its own temp dir.
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read declarations: %w", err)
	}

	var decls []suites.Declaration
	if err := json.Unmarshal(b, &decls); err != nil {
		return nil, fmt.Errorf("decode declarations: %w", err)
	}

	if len(decls) == 0 {
		return nil, fmt.Errorf("no declarations found; the listing run is broken")
	}

	return decls, nil
}

func definedTests(ctx context.Context) ([]string, error) {
	out, err := exec.CommandContext(ctx, "go", "test", "./integration/", "-list", ".*").Output()
	if err != nil {
		return nil, fmt.Errorf("list tests: %w", err)
	}

	var names []string

	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if testName.MatchString(line) {
			names = append(names, line)
		}
	}

	if len(names) == 0 {
		return nil, fmt.Errorf("go test -list found no integration tests; discovery is broken")
	}

	return names, nil
}

func undeclared(defined []string, decls []suites.Declaration) []string {
	seen := make(map[string]bool, len(decls))
	for _, d := range decls {
		seen[d.Test] = true
	}

	var missing []string

	for _, n := range defined {
		if seen[n] {
			continue
		}

		if _, exempt := suites.Exempt[n]; exempt {
			continue
		}

		missing = append(missing, n)
	}

	sort.Strings(missing)

	return missing
}

func scenarioPrefixes() ([]string, error) {
	seen := map[string]bool{}

	var out, bare []string

	for _, n := range scenarios.List() {
		p, _, found := strings.Cut(n, "/")
		if !found {
			bare = append(bare, n)
			continue
		}

		if seen[p] {
			continue
		}

		seen[p] = true

		out = append(out, p)
	}

	if len(bare) > 0 {
		sort.Strings(bare)

		return nil, fmt.Errorf("these core-tester scenarios have no \"<ran>/\" prefix, so they would run in both the 4G and 5G suites:\n  %s",
			strings.Join(bare, "\n  "))
	}

	sort.Strings(out)

	return out, nil
}
