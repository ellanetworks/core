// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrc_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/rrc"
)

// Regenerate with: go test ./internal/decoder/rrc/ -run TestNGAPCapabilityGolden -update
var updateGolden = flag.Bool("update", false, "regenerate UE radio capability golden fixtures")

func fixtures(t *testing.T, pattern string) []string {
	t.Helper()

	paths, err := filepath.Glob(pattern)
	if err != nil {
		t.Fatalf("glob fixtures: %v", err)
	}

	if len(paths) == 0 {
		t.Fatalf("no capability fixtures matched %s", pattern)
	}

	sort.Strings(paths)

	return paths
}

func decodeFixtures(t *testing.T, pattern string, parse func([]byte) (*rrc.Capability, error)) map[string]*rrc.Capability {
	t.Helper()

	got := map[string]*rrc.Capability{}

	for _, p := range fixtures(t, pattern) {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}

		capability, err := parse(raw)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(p), err)
		}

		got[filepath.Base(p)] = capability
	}

	return got
}

func checkGolden(t *testing.T, path string, got map[string]*rrc.Capability) {
	t.Helper()

	encoded, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	encoded = append(encoded, '\n')

	if *updateGolden {
		if err := os.WriteFile(path, encoded, 0o600); err != nil {
			t.Fatalf("write golden: %v", err)
		}

		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}

	if string(encoded) != string(want) {
		t.Errorf("decoded capability differs from %s; rerun with -update to accept", path)
	}
}

func TestNGAPCapabilityGolden(t *testing.T) {
	checkGolden(t, "testdata/ngap_capability_golden.json",
		decodeFixtures(t, "testdata/uecap_ngap_*.bin", rrc.ParseNGAPUERadioCapability))
}

func TestS1APCapabilityGolden(t *testing.T) {
	checkGolden(t, "testdata/s1ap_capability_golden.json",
		decodeFixtures(t, "testdata/uecap_s1ap_*.bin", rrc.ParseS1APUERadioCapability))
}

func TestNGAPCapabilityRejectsGarbage(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []byte
	}{
		{"empty", nil},
		{"truncated", []byte{0x04, 0x54}},
		{"random", []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := rrc.ParseNGAPUERadioCapability(tc.in); err == nil {
				t.Fatal("expected an error, got nil")
			}
		})
	}
}

func TestNGAPCapabilityBandsAreSane(t *testing.T) {
	for _, p := range fixtures(t, "testdata/uecap_ngap_*.bin") {
		raw, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}

		capability, err := rrc.ParseNGAPUERadioCapability(raw)
		if err != nil {
			t.Fatalf("%s: %v", filepath.Base(p), err)
		}

		if capability.NR == nil {
			t.Fatalf("%s: no NR capability", filepath.Base(p))
		}

		if capability.NR.AccessStratumRelease == "" {
			t.Errorf("%s: empty NR access stratum release", filepath.Base(p))
		}

		for _, b := range capability.NR.Bands {
			if b.Band < 1 || b.Band > 1024 {
				t.Errorf("%s: NR band %d out of range", filepath.Base(p), b.Band)
			}
		}

		if capability.EUTRA == nil {
			continue
		}

		for _, b := range capability.EUTRA.Bands {
			if b.Band < 1 || b.Band > 256 {
				t.Errorf("%s: E-UTRA band %d out of range", filepath.Base(p), b.Band)
			}
		}
	}
}
