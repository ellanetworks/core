// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"os"
	"slices"
	"testing"

	"github.com/ellanetworks/core/per/pergen"
)

var pergenConfig = pergen.Config{Dir: ".", Output: "per_gen.go", Profile: "profile", IEID: "ProtocolIEID"}

func TestGeneratedCodecsAreUpToDate(t *testing.T) {
	want, err := pergen.Generate(t.Context(), pergenConfig)
	if err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile("per_gen.go")
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(got, want) {
		t.Fatal("per_gen.go is stale: run go generate")
	}
}

var undiagnosedHandWrittenTypes = []string{"AbstractSyntaxError"}

func TestEveryTypeHoldingDiagnosableStateIsDiagnosed(t *testing.T) {
	got, err := pergen.Undiagnosed(t.Context(), pergenConfig)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(got, undiagnosedHandWrittenTypes) {
		t.Fatalf("types that hold diagnosable state without a Diagnose method:\n got %q\nwant %q", got, undiagnosedHandWrittenTypes)
	}
}
