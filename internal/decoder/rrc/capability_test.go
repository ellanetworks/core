// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrc_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"testing"

	"github.com/ellanetworks/core/internal/decoder/rrc"
)

// Regenerate with: go test ./internal/decoder/rrc/ -update
var updateGolden = flag.Bool("update", false, "regenerate UE radio capability golden fixtures")

func decodeCapture(t *testing.T, name, capture string) []byte {
	t.Helper()

	raw, err := hex.DecodeString(capture)
	if err != nil {
		t.Fatalf("%s: decode capture: %v", name, err)
	}

	return raw
}

type capabilityDigest struct {
	Summary  *rrc.Summary `json:"summary"`
	NRNodes  int          `json:"nr_nodes"`
	EUNodes  int          `json:"eutra_nodes"`
	TreeHash string       `json:"tree_sha256"`
}

func countNodes(v any) int {
	switch t := v.(type) {
	case map[string]any:
		n := 1
		for _, child := range t {
			n += countNodes(child)
		}

		return n
	case []any:
		n := 1
		for _, child := range t {
			n += countNodes(child)
		}

		return n
	default:
		return 1
	}
}

func digest(t *testing.T, c *rrc.Capability) capabilityDigest {
	t.Helper()

	tree, err := json.Marshal(map[string]any{"nr": c.NR, "eutra": c.EUTRA})
	if err != nil {
		t.Fatalf("marshal tree: %v", err)
	}

	sum := sha256.Sum256(tree)

	return capabilityDigest{
		Summary:  c.Summary,
		NRNodes:  countNodes(c.NR),
		EUNodes:  countNodes(c.EUTRA),
		TreeHash: hex.EncodeToString(sum[:]),
	}
}

func decodeCaptures(t *testing.T, captures map[string]string, parse func([]byte) (*rrc.Capability, error)) map[string]*rrc.Capability {
	t.Helper()

	if len(captures) == 0 {
		t.Fatal("no captures to decode")
	}

	got := make(map[string]*rrc.Capability, len(captures))

	for name, capture := range captures {
		capability, err := parse(decodeCapture(t, name, capture))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		got[name] = capability
	}

	return got
}

func checkGolden(t *testing.T, path string, got map[string]*rrc.Capability) {
	t.Helper()

	digests := make(map[string]capabilityDigest, len(got))
	for name, capability := range got {
		digests[name] = digest(t, capability)
	}

	encoded, err := json.MarshalIndent(digests, "", "  ")
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
		decodeCaptures(t, ngapCapabilityCaptures, rrc.ParseNGAPUERadioCapability))
}

func TestS1APCapabilityGolden(t *testing.T) {
	checkGolden(t, "testdata/s1ap_capability_golden.json",
		decodeCaptures(t, s1apCapabilityCaptures, rrc.ParseS1APUERadioCapability))
}

func TestCapabilityRejectsGarbage(t *testing.T) {
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
				t.Error("ParseNGAPUERadioCapability: expected an error, got nil")
			}

			if _, err := rrc.ParseS1APUERadioCapability(tc.in); err == nil {
				t.Error("ParseS1APUERadioCapability: expected an error, got nil")
			}
		})
	}
}

func TestNGAPCapabilityBandsAreSane(t *testing.T) {
	for name, capability := range decodeCaptures(t, ngapCapabilityCaptures, rrc.ParseNGAPUERadioCapability) {
		if capability.NR == nil || capability.Summary == nil || capability.Summary.NR == nil {
			t.Errorf("%s: no NR capability", name)
			continue
		}

		if capability.Summary.NR.AccessStratumRelease == "" {
			t.Errorf("%s: empty NR access stratum release", name)
		}

		for _, b := range capability.Summary.NR.Bands {
			if b.Band < 1 || b.Band > 1024 {
				t.Errorf("%s: NR band %d out of range", name, b.Band)
			}
		}

		if capability.Summary.EUTRA == nil {
			continue
		}

		for _, b := range capability.Summary.EUTRA.Bands {
			if b.Band < 1 || b.Band > 256 {
				t.Errorf("%s: E-UTRA band %d out of range", name, b.Band)
			}
		}
	}
}
