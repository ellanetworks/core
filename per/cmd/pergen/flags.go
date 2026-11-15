// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"errors"
	"flag"
	"strings"

	"github.com/ellanetworks/core/per/pergen"
)

func parseFlags(args []string) (pergen.Config, error) {
	var cfg pergen.Config

	fs := flag.NewFlagSet("pergen", flag.ContinueOnError)
	fs.StringVar(&cfg.Output, "o", "per_gen.go", "output file path")
	fs.Var(commaSliceFlag{&cfg.Types}, "type", "comma-separated list of types")
	fs.StringVar(&cfg.Profile, "profile", "", "package variable holding the *ap.Profile (application protocol mode)")
	fs.StringVar(&cfg.IEID, "ieid", "", "ProtocolIE-ID type (application protocol mode)")

	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	switch rest := fs.Args(); len(rest) {
	case 0:
		cfg.Dir = "."
	case 1:
		cfg.Dir = rest[0]
	default:
		return cfg, errors.New("pergen generates one package directory")
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
