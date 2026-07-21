// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Command pergen generates reflection-free PER marshal/unmarshal methods for
// types declared with `per:` struct tags. It is invoked via go:generate:
//
//	//go:generate go run github.com/ellanetworks/core/cmd/pergen
//
// pergen parses Go source via go/ast and go/types (no reflect at build time)
// and emits code that uses no reflect at run time.
package main
