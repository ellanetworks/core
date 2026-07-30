// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/types"
	"path/filepath"
	"slices"
	"sort"

	"golang.org/x/tools/go/packages"
)

type generator struct {
	cfg      Config
	buf      bytes.Buffer
	pkg      *packages.Package
	genTypes map[string]bool // types this run will emit methods for
}

func newGenerator(cfg Config) *generator {
	return &generator{cfg: cfg}
}

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

func (g *generator) generatePackage(pkg *packages.Package) error {
	g.pkg = pkg
	scope := pkg.Types.Scope()
	// Sorted so the output is deterministic.
	names := make([]string, 0, len(scope.Names()))
	names = append(names, scope.Names()...)
	sort.Strings(names)

	// First pass, so field classification can see which named types will
	// implement MarshalPER before any method is emitted.
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

	g.collectReferencedStructs(scope)

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

// collectReferencedStructs takes the transitive closure of genTypes over
// same-package struct types used as field types.
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

					// A hand-written MarshalPER is kept, so the closure does not
					// emit a conflicting method.
					if g.hasSourceMethod(n, "MarshalPER") {
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

// hasSourceMethod ignores methods declared in the output file, since this run
// replaces it.
func (g *generator) hasSourceMethod(named *types.Named, name string) bool {
	for obj := range named.Methods() {
		obj := obj
		if obj.Name() != name {
			continue
		}

		pos := g.pkg.Fset.Position(obj.Pos())
		if filepath.Base(pos.Filename) != filepath.Base(g.cfg.output) {
			return true
		}
	}

	return false
}

// structIsPER reports whether any field carries a `per:` tag other than "-".
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

// structIsExtSeq admits a type whose only per: tag is `extseq`.
func structIsExtSeq(s *types.Struct) bool {
	for i := 0; i < s.NumFields(); i++ {
		tag := structTagGet(s.Tag(i), "per")
		if tag == "extseq" {
			return true
		}
	}

	return false
}

// typeEnabled applies the -type filter; an empty filter enables every type.
func (g *generator) typeEnabled(name string) bool {
	if len(g.cfg.types) == 0 {
		return true
	}

	return slices.Contains(g.cfg.types, name)
}
