// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"flag"
	"strings"
)

// Config holds parsed command-line flags for pergen.
type Config struct {
	// patterns are the Go package patterns to load (defaults to ".").
	patterns []string
	// output is the path to the generated file (defaults to "per_gen.go").
	output string
	// types restricts generation to the named types (comma-separated). Empty = all.
	types []string
	// suffix is appended to the generated method names (default "").
	suffix string
}

// parseFlags parses the pergen command-line. Supported flags:
//
//	-o <file>           output file path (default "per_gen.go")
//	-type <a,b>         restrict to types
//	-suffix <s>         method suffix (e.g. "Unaligned")
//	patterns            package patterns (default ".")
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

// commaSliceFlag implements flag.Value for a comma-separated list.
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
