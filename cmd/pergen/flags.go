// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"flag"
	"strings"
)

type Config struct {
	patterns []string
	output   string
	// types restricts generation to the named types; empty means all.
	types  []string
	suffix string
}

func parseFlags(args []string) (Config, error) {
	cfg := Config{output: "per_gen.go"}
	fs := flag.NewFlagSet("pergen", flag.ContinueOnError)
	fs.StringVar(&cfg.output, "o", "per_gen.go", "output file path")
	fs.Var(commaSliceFlag{&cfg.types}, "type", "comma-separated list of types")
	fs.StringVar(&cfg.suffix, "suffix", "", "method name suffix")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	rest := fs.Args()
	if len(rest) == 0 {
		cfg.patterns = []string{"."}
	} else {
		cfg.patterns = rest
	}

	return cfg, nil
}

type commaSliceFlag struct {
	p *[]string
}

func (f commaSliceFlag) String() string {
	if f.p == nil || *f.p == nil {
		return ""
	}

	return strings.Join(*f.p, ",")
}

func (f commaSliceFlag) Set(s string) error {
	if s == "" {
		return nil
	}

	*f.p = strings.Split(s, ",")

	return nil
}
