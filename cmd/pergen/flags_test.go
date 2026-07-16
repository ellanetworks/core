// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"testing"
)

func TestParseFlagsDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlags([]string{})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.dir != "." {
		t.Fatalf("dir = %q", cfg.dir)
	}

	if cfg.output != "per_gen.go" {
		t.Fatalf("output = %q", cfg.output)
	}

	if len(cfg.patterns) != 1 || cfg.patterns[0] != "." {
		t.Fatalf("patterns = %v", cfg.patterns)
	}

	if len(cfg.types) != 0 {
		t.Fatalf("types = %v", cfg.types)
	}
}

func TestParseFlagsAll(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlags([]string{"-d", "src", "-o", "gen.go", "-type", "A,B", "-suffix", "X", "./pkg"})
	if err != nil {
		t.Fatal(err)
	}

	if cfg.dir != "src" {
		t.Fatalf("dir = %q", cfg.dir)
	}

	if cfg.output != "gen.go" {
		t.Fatalf("output = %q", cfg.output)
	}

	if len(cfg.types) != 2 || cfg.types[0] != "A" || cfg.types[1] != "B" {
		t.Fatalf("types = %v", cfg.types)
	}

	if cfg.suffix != "X" {
		t.Fatalf("suffix = %q", cfg.suffix)
	}

	if len(cfg.patterns) != 1 || cfg.patterns[0] != "./pkg" {
		t.Fatalf("patterns = %v", cfg.patterns)
	}
}

func TestParseFlagsMultiplePatterns(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlags([]string{"./pkg1", "./pkg2"})
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.patterns) != 2 {
		t.Fatalf("patterns = %v", cfg.patterns)
	}
}

func TestParseFlagsTypeEmpty(t *testing.T) {
	t.Parallel()

	cfg, err := parseFlags([]string{"-type", ""})
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.types) != 0 {
		t.Fatalf("types = %v", cfg.types)
	}
}

func TestReceiverName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in, want string
	}{
		{"Message", "message"},
		{"Foo", "foo"},
		{"", "v"},
		{"ABC", "aBC"},
	}
	for _, c := range cases {
		if got := receiverName(c.in); got != c.want {
			t.Errorf("receiverName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStructTagGet(t *testing.T) {
	t.Parallel()

	cases := []struct {
		tag, key, want string
	}{
		{`per:"INTEGER,range:0..3" json:"version"`, "per", "INTEGER,range:0..3"},
		{`json:"version" per:"optional"`, "per", "optional"},
		{`json:"version"`, "per", ""},
		{``, "per", ""},
		{`per:""`, "per", ""},
	}
	for _, c := range cases {
		if got := structTagGet(c.tag, c.key); got != c.want {
			t.Errorf("structTagGet(%q, %q) = %q, want %q", c.tag, c.key, got, c.want)
		}
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
	// Set("") is a no-op, keeps existing values
	if len(vals) != 3 {
		t.Fatalf("vals after empty Set = %v", vals)
	}
}
