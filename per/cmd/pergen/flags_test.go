// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"slices"
	"testing"
)

func TestParseFlagsDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlags([]string{})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Output != "per_gen.go" || cfg.Dir != "." || len(cfg.Types) != 0 || cfg.Profile != "" || cfg.IEID != "" {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestParseFlagsAll(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlags([]string{"-o", "gen.go", "-type", "A,B", "-profile", "profile", "-ieid", "ProtocolIEID", "./pkg"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Output != "gen.go" || cfg.Dir != "./pkg" || !slices.Equal(cfg.Types, []string{"A", "B"}) || cfg.Profile != "profile" || cfg.IEID != "ProtocolIEID" {
		t.Fatalf("parsed = %+v", cfg)
	}
}

func TestParseFlagsOneDirectory(t *testing.T) {
	t.Parallel()

	if _, err := parseFlags([]string{"./pkg1", "./pkg2"}); err == nil {
		t.Fatal("two package directories accepted")
	}
}

func TestParseFlagsTypeEmpty(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlags([]string{"-type", ""})
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Types) != 0 {
		t.Fatalf("types = %v", cfg.Types)
	}
}

func TestCommaSliceFlag(t *testing.T) {
	t.Parallel()

	var vals []string

	f := commaSliceFlag{&vals}
	if f.String() != "" {
		t.Fatalf("String() = %q", f.String())
	}

	if err := f.Set("A,B,C"); err != nil {
		t.Fatal(err)
	}

	if len(vals) != 3 || vals[0] != "A" || vals[2] != "C" {
		t.Fatalf("vals = %v", vals)
	}

	if f.String() != "A,B,C" {
		t.Fatalf("String() = %q", f.String())
	}

	if err := f.Set(""); err != nil {
		t.Fatal(err)
	}

	if len(vals) != 3 {
		t.Fatalf("vals after empty Set = %v", vals)
	}
}
