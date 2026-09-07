// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ci_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/ellanetworks/core/integration/suites"
	"github.com/ellanetworks/core/internal/tester/scenarios"
	_ "github.com/ellanetworks/core/internal/tester/scenarios/all"
)

var integrationTestRe = regexp.MustCompile(`(?m)^func ((?:TestIntegration|TestAPIMatrix)[A-Za-z0-9_]*)\(t \*testing\.T\) \{`)

func TestEveryIntegrationTestBelongsToExactlyOneSuite(t *testing.T) {
	declared := suites.SplitTests()
	found := integrationTests(t, repoRoot(t))

	for _, name := range found {
		owners := declared[name]

		if prefix, split := suites.SubtestPartitioned[name]; split {
			if len(owners) != 2 {
				t.Errorf("%s is declared subtest-partitioned on %q but is claimed by %d suites (%v); it must be exactly 2",
					name, prefix, len(owners), owners)
			}

			continue
		}

		switch len(owners) {
		case 1:
		case 0:
			t.Errorf("%s is in integration/ but no suite claims it, so no CI job runs it", name)
		default:
			t.Errorf("%s is claimed by %d suites (%v), so CI runs it more than once", name, len(owners), owners)
		}
	}

	for name := range declared {
		if !slices.Contains(found, name) {
			t.Errorf("suite(s) %v list %s, which does not exist in integration/", declared[name], name)
		}
	}
}

func TestSubtestPartitionedTestsAreSplitByTheDeclaredPrefix(t *testing.T) {
	for name, prefix := range suites.SubtestPartitioned {
		var selects, excludes []string

		for _, s := range suites.All {
			if !slices.Contains(s.Tests, name) {
				continue
			}

			switch {
			case strings.Contains(s.Run, name+"/"+prefix):
				selects = append(selects, s.Name)
			case strings.Contains(s.Skip, name+"/"+prefix):
				excludes = append(excludes, s.Name)
			default:
				t.Errorf("suite %s claims subtest-partitioned %s but neither selects nor excludes %s/%s",
					s.Name, name, name, prefix)
			}
		}

		if len(selects) != 1 || len(excludes) != 1 {
			t.Errorf("%s: expected exactly one suite selecting %s/%s and one excluding it, got selects=%v excludes=%v",
				name, name, prefix, selects, excludes)
		}
	}
}

func TestSuiteProfilesComeFromTheClosedVocabulary(t *testing.T) {
	names := make(map[string]suites.Profile, len(suites.Profiles))

	for _, p := range suites.Profiles {
		names[p.Name] = p
	}

	for _, s := range suites.All {
		known, ok := names[s.Profile.Name]
		if !ok {
			t.Errorf("suite %s uses profile %q, which is not in suites.Profiles", s.Name, s.Profile.Name)
			continue
		}

		if known.String() != s.Profile.String() {
			t.Errorf("suite %s profile %q does not match the canonical definition:\n  got  %s\n  want %s",
				s.Name, s.Profile.Name, s.Profile, known)
		}
	}
}

func TestSuiteNamesAndCellsAreUnique(t *testing.T) {
	seen := map[string]bool{}

	for _, s := range suites.All {
		if seen[s.Name] {
			t.Errorf("duplicate suite name %q", s.Name)
		}

		seen[s.Name] = true
	}

	cells := map[string][]string{}

	for _, leg := range suites.Legs() {
		key := fmt.Sprintf("%s/%s/%s", leg.Arch, leg.IPFamily, leg.AttachMode)
		for _, test := range mustLookup(t, leg.Suite).Tests {
			cells[test+" "+key] = append(cells[test+" "+key], leg.Suite)
		}
	}

	for k, v := range cells {
		if len(v) > 1 {
			parts := strings.SplitN(k, " ", 2)
			if _, split := suites.SubtestPartitioned[parts[0]]; split {
				continue
			}

			t.Errorf("%s runs on %s in more than one suite: %v", parts[0], parts[1], v)
		}
	}
}

func TestSuiteRunPatternsSelectExactlyTheirDeclaredTests(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: builds and runs a stub test binary for every suite")
	}

	dir := stubPackage(t, integrationTests(t, repoRoot(t)))

	for _, s := range suites.All {
		t.Run(s.Name, func(t *testing.T) {
			got := stubSelect(t, dir, s.Run, s.Skip)
			want := append([]string(nil), s.Tests...)
			sort.Strings(want)

			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("-run %q -skip %q selects\n  %v\nbut the suite declares\n  %v", s.Run, s.Skip, got, want)
			}
		})
	}
}

func integrationTests(t *testing.T, root string) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(root, "integration", "*_test.go"))
	if err != nil {
		t.Fatalf("glob integration tests: %v", err)
	}

	if len(paths) == 0 {
		t.Fatal("no integration test files found; the discovery glob is broken")
	}

	var names []string

	for _, p := range paths {
		b, err := os.ReadFile(p) //nolint:gosec // test-only read of a repo-relative path
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}

		for _, m := range integrationTestRe.FindAllStringSubmatch(string(b), -1) {
			names = append(names, m[1])
		}
	}

	sort.Strings(names)

	return names
}

func stubPackage(t *testing.T, names []string) string {
	t.Helper()

	dir := t.TempDir()

	var b strings.Builder

	b.WriteString("package stub\n\nimport \"testing\"\n\n")

	for _, n := range names {
		if _, split := suites.SubtestPartitioned[n]; split {
			b.WriteString("func " + n + "(t *testing.T) {\n")

			for _, sc := range scenarios.List() {
				b.WriteString("\tt.Run(" + fmt.Sprintf("%q", sc) + ", func(t *testing.T) {})\n")
			}

			b.WriteString("}\n\n")

			continue
		}

		b.WriteString("func " + n + "(t *testing.T) {}\n")
	}

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	write("go.mod", "module stub\n\ngo 1.24\n")
	write("stub_test.go", b.String())

	return dir
}

var stubRunRe = regexp.MustCompile(`(?m)^=== RUN\s+([A-Za-z0-9_]+)$`)

func stubSelect(t *testing.T, dir, run, skip string) []string {
	t.Helper()

	args := []string{"test", "-v", "-count=1"}
	if run != "" {
		args = append(args, "-run", run)
	}

	if skip != "" {
		args = append(args, "-skip", skip)
	}

	cmd := exec.CommandContext(t.Context(), "go", append(args, "./...")...)
	cmd.Dir = dir

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run stub selection: %v\n%s", err, out)
	}

	seen := map[string]bool{}

	var got []string

	for _, m := range stubRunRe.FindAllStringSubmatch(string(out), -1) {
		if !seen[m[1]] {
			seen[m[1]] = true

			got = append(got, m[1])
		}
	}

	sort.Strings(got)

	return got
}

func mustLookup(t *testing.T, name string) suites.Suite {
	t.Helper()

	s, ok := suites.Lookup(name)
	if !ok {
		t.Fatalf("suite %q not found", name)
	}

	return s
}

func TestIntegrationWorkflowConsumesTheSuiteRegistry(t *testing.T) {
	root := repoRoot(t)

	wf, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "integration.yaml"))
	if err != nil {
		t.Fatalf("read integration workflow: %v", err)
	}

	for _, want := range []string{
		"fromJSON(needs.discover.outputs.legs)",
		".github/scripts/list-integration-legs.sh",
		"timeout-minutes: ${{ matrix.timeout_minutes }}",
	} {
		if !strings.Contains(string(wf), want) {
			t.Errorf("integration workflow no longer contains %q, so the registry may not drive the matrix", want)
		}
	}

	stale, err := filepath.Glob(filepath.Join(root, ".github", "workflows", "integration-tests-*.yaml"))
	if err != nil {
		t.Fatalf("glob stale workflows: %v", err)
	}

	if len(stale) > 0 {
		t.Errorf("per-suite workflows still exist alongside the registry-driven one: %v", stale)
	}
}

func TestIntegrationLegScriptMatchesTheRegistry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping: shells out to the discovery script")
	}

	root := repoRoot(t)

	cmd := exec.CommandContext(t.Context(), "bash", filepath.Join(root, ".github", "scripts", "list-integration-legs.sh"))
	cmd.Dir = root

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("run discovery script: %v", err)
	}

	var got []suites.Leg
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("decode discovery output: %v (output=%q)", err, out)
	}

	want := suites.Legs()
	if len(got) != len(want) {
		t.Fatalf("discovery emitted %d legs, registry defines %d", len(got), len(want))
	}

	for i := range want {
		if got[i] != want[i] {
			t.Errorf("leg %d differs:\n  script   %+v\n  registry %+v", i, got[i], want[i])
		}
	}
}
