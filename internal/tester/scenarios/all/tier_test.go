// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package all

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ellanetworks/core/internal/tester/scenarios"
)

const integrationDir = "../../../../integration"

func integrationSources(t *testing.T) string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(integrationDir, "*_test.go"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no integration test sources found in %s: %v", integrationDir, err)
	}

	var b strings.Builder

	for _, f := range files {
		src, err := os.ReadFile(f) // #nosec G304 -- path comes from a fixed glob
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}

		b.Write(src)
	}

	return b.String()
}

func TestDrivenByNamesAnExistingTest(t *testing.T) {
	src := integrationSources(t)

	for _, name := range scenarios.List() {
		s, _ := scenarios.Get(name)
		if s.DrivenBy == "" {
			continue
		}

		if !strings.Contains(src, "func "+s.DrivenBy+"(t *testing.T)") {
			t.Errorf("%s is driven by %s, which is not a test in integration/", name, s.DrivenBy)
		}
	}
}

func TestComposeRunsNoInProcessScenario(t *testing.T) {
	src := integrationSources(t)

	for _, name := range scenarios.ListTier(scenarios.TierInProcess) {
		if regexp.MustCompile(`"` + regexp.QuoteMeta(name) + `"`).MatchString(src) {
			t.Errorf("%s runs in-process but is also named in integration/; each scenario runs in exactly one tier", name)
		}
	}
}

func TestEveryTierIsPopulated(t *testing.T) {
	for _, tier := range []scenarios.Tier{scenarios.TierInProcess, scenarios.TierCompose, scenarios.TierDedicated} {
		if len(scenarios.ListTier(tier)) == 0 {
			t.Errorf("no scenario in tier %s", tier)
		}
	}
}
