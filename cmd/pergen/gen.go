// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/types"
	"slices"
	"sort"

	"golang.org/x/tools/go/packages"
)

// generator drives the pergen code-generation pass.
type generator struct {
	cfg      Config
	buf      bytes.Buffer
	pkg      *packages.Package
	genTypes map[string]bool // types that will get generated methods (two-pass)
}

func newGenerator(cfg Config) *generator {
	return &generator{cfg: cfg}
}

// generate iterates loaded packages, finds types with per: tags, and emits
// MarshalPER/UnmarshalPER methods for them.
func (g *generator) generate(pkgs []*packages.Package) error {
	if len(pkgs) == 0 {
		return errors.New("no packages to generate")
	}

	for _, pkg := range pkgs {
		if len(pkg.Errors) > 0 {
			return fmt.Errorf("package %s has errors: %w", pkg.Name, pkg.Errors[0])
		}

		if err := g.generatePackage(pkg); err != nil {
			return err
		}
	}

	return nil
}

// generatePackage emits methods for every struct in pkg that has at least one
// field with a `per:` tag (or whose named type has a per: tag).
func (g *generator) generatePackage(pkg *packages.Package) error {
	g.pkg = pkg
	scope := pkg.Types.Scope()
	// Collect names so output is deterministic.
	names := make([]string, 0, len(scope.Names()))
	names = append(names, scope.Names()...)
	sort.Strings(names)

	// First pass: collect all types that will get generated methods. This lets
	// field classification know which named types will implement MarshalPER
	// even before their methods are emitted. We start with structs that have
	// per: tags, then add structs referenced as field types (transitive
	// closure) so delegate types get methods too.
	g.genTypes = make(map[string]bool)

	for _, name := range names {
		obj := scope.Lookup(name)
		if obj == nil {
			continue
		}

		nt, ok := obj.(*types.TypeName)
		if !ok {
			continue
		}

		named, ok := nt.Type().(*types.Named)
		if !ok {
			continue
		}

		underlying, ok := named.Underlying().(*types.Struct)
		if !ok {
			continue
		}

		if structIsPER(underlying, named) || structIsExtSeq(underlying) {
			g.genTypes[name] = true
		}
	}
	// Transitive closure: add structs referenced as field types.
	g.collectReferencedStructs(scope)

	// Second pass: parse and emit.
	found := false

	for _, name := range names {
		if !g.genTypes[name] {
			continue
		}

		if !g.typeEnabled(name) {
			continue
		}

		obj := scope.Lookup(name)
		nt := obj.(*types.TypeName)
		named := nt.Type().(*types.Named)
		underlying := named.Underlying().(*types.Struct)

		st, err := g.parseStruct(name, named, underlying)
		if err != nil {
			return err
		}

		found = true

		if isChoiceType(st) {
			if err := g.emitChoice(name, st); err != nil {
				return err
			}
		} else {
			if err := g.emitStruct(name, st); err != nil {
				return err
			}
		}
	}

	if !found {
		return fmt.Errorf("no types with `per:` tags found in package %s", pkg.Name)
	}

	return nil
}

// collectReferencedStructs adds same-package named struct types that are
// referenced as field types by types already in genTypes. Repeats until no new
// types are found (transitive closure).
func (g *generator) collectReferencedStructs(scope *types.Scope) {
	changed := true
	for changed {
		changed = false

		for name := range g.genTypes {
			obj := scope.Lookup(name)
			if obj == nil {
				continue
			}

			nt, ok := obj.(*types.TypeName)
			if !ok {
				continue
			}

			named, ok := nt.Type().(*types.Named)
			if !ok {
				continue
			}

			underlying, ok := named.Underlying().(*types.Struct)
			if !ok {
				continue
			}

			for i := 0; i < underlying.NumFields(); i++ {
				ft := underlying.Field(i).Type()
				// Dereference pointer/slice elements.
				if p, ok := ft.(*types.Pointer); ok {
					ft = p.Elem()
				}

				if s, ok := ft.(*types.Slice); ok {
					ft = s.Elem()
				}

				if n, ok := ft.(*types.Named); ok {
					if n.Obj().Pkg() != g.pkg.Types {
						continue
					}

					if _, isStruct := n.Underlying().(*types.Struct); isStruct {
						if !g.genTypes[n.Obj().Name()] {
							g.genTypes[n.Obj().Name()] = true
							changed = true
						}
					}
				}
			}
		}
	}
}

// structIsPER reports whether a struct type carries PER-relevant tags: at least
// one field has a `per:` tag (excluding `per:"-"` which means skip).
func structIsPER(s *types.Struct, _ *types.Named) bool {
	for i := 0; i < s.NumFields(); i++ {
		tag := structTagGet(s.Tag(i), "per")
		if tag == "" || tag == "-" {
			continue
		}

		return true
	}

	return false
}

// structIsExtSeq reports whether a struct has a field tagged `per:"extseq"`,
// marking the type as extensible even if no other per: tags exist.
func structIsExtSeq(s *types.Struct) bool {
	for i := 0; i < s.NumFields(); i++ {
		tag := structTagGet(s.Tag(i), "per")
		if tag == "extseq" {
			return true
		}
	}

	return false
}

// typeEnabled reports whether name should be generated, per the -type filter.
func (g *generator) typeEnabled(name string) bool {
	if len(g.cfg.types) == 0 {
		return true
	}

	return slices.Contains(g.cfg.types, name)
}
