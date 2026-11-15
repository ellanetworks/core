// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pergen

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"sort"
	"strings"
)

type Config struct {
	Dir     string
	Output  string
	Types   []string
	Profile string
	IEID    string
}

func Generate(ctx context.Context, cfg Config) ([]byte, error) {
	g, err := run(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return g.output()
}

func Undiagnosed(ctx context.Context, cfg Config) ([]string, error) {
	if cfg.Profile == "" {
		return nil, errors.New("undiagnosed types are an application protocol mode check")
	}

	g, err := run(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return g.undiagnosed(), nil
}

func run(ctx context.Context, cfg Config) (*generator, error) {
	if cfg.Dir == "" {
		cfg.Dir = "."
	}

	if cfg.Output == "" {
		cfg.Output = "per_gen.go"
	}

	if (cfg.Profile == "") != (cfg.IEID == "") {
		return nil, errors.New("application protocol mode needs both a profile and a ProtocolIE-ID type")
	}

	dir, err := filepath.Abs(cfg.Dir)
	if err != nil {
		return nil, err
	}

	out, err := filepath.Abs(cfg.Output)
	if err != nil {
		return nil, err
	}

	exclude := ""
	if filepath.Dir(out) == dir {
		exclude = filepath.Base(out)
	}

	pkg, err := load(ctx, dir, exclude)
	if err != nil {
		return nil, err
	}

	g := &generator{cfg: cfg, pkg: pkg.types, fset: pkg.fset, files: pkg.files}
	if err := g.generatePackage(); err != nil {
		return nil, err
	}

	return g, nil
}

func (g *generator) undiagnosed() []string {
	var out []string

	for _, name := range g.pkg.Scope().Names() {
		nt, ok := g.pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok || nt.IsAlias() || g.diagTypes[name] {
			continue
		}

		named, ok := nt.Type().(*types.Named)
		if !ok || hasDeclaredMethod(named, "Diagnose") || g.isMessage(named) {
			continue
		}

		var holds []types.Type

		switch u := named.Underlying().(type) {
		case *types.Struct:
			for i := 0; i < u.NumFields(); i++ {
				holds = append(holds, u.Field(i).Type())
			}
		case *types.Slice:
			holds = append(holds, u.Elem())
		}

		for _, t := range holds {
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}

			if sl, ok := t.(*types.Slice); ok {
				t = sl.Elem()
			}

			if g.typeDiagnosable(g.diagTypes, t) {
				out = append(out, name)

				break
			}
		}
	}

	sort.Strings(out)

	return out
}

type generator struct {
	cfg   Config
	buf   bytes.Buffer
	pkg   *types.Package
	fset  *token.FileSet
	files []*ast.File

	genTypes  map[string]bool
	enums     map[string]enumInfo
	diagTypes map[string]bool
	eqTypes   map[string]bool
	imports   []string
	usesAP    bool
	cur       *structType
}

func (g *generator) apMode() bool { return g.cfg.Profile != "" }

func (g *generator) generatePackage() error {
	scope := g.pkg.Scope()
	names := slices.Clone(scope.Names())
	sort.Strings(names)

	g.genTypes = make(map[string]bool)

	if err := g.collectEnums(names); err != nil {
		return err
	}

	for _, name := range names {
		underlying, ok := g.localStruct(name)
		if !ok {
			continue
		}

		if structIsPER(underlying) || structIsExtSeq(underlying) {
			g.genTypes[name] = true
		}
	}

	g.collectReferencedStructs(scope)

	structs := make(map[string]*structType)

	for _, name := range names {
		if !g.genTypes[name] || g.enums[name].varName != "" || !g.typeEnabled(name) {
			continue
		}

		underlying, _ := g.localStruct(name)

		st, err := g.parseStruct(name, scope.Lookup(name).Type().(*types.Named), underlying)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		structs[name] = st
	}

	var lists []string

	if g.apMode() {
		lists = g.sliceTypes(names)
		g.diagTypes = g.diagnosable(structs, lists)
	}

	found := false

	for _, name := range names {
		if e, ok := g.enums[name]; ok && g.typeEnabled(name) {
			found = true

			g.emitEnum(name, e)

			continue
		}

		st, ok := structs[name]
		if !ok {
			continue
		}

		found = true

		var err error

		if isChoiceType(st) {
			err = g.emitChoice(name, st)
		} else {
			err = g.emitStruct(name, st)
		}

		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}

		if g.diagTypes[name] {
			g.emitDiagnose(name, st)
		}
	}

	for _, name := range lists {
		if g.diagTypes[name] && g.typeEnabled(name) {
			g.emitListDiagnose(name)
		}
	}

	if g.apMode() {
		for _, name := range g.equalTypes(structs, names) {
			if !g.typeEnabled(name) {
				continue
			}

			if err := g.emitEqual(name); err != nil {
				return fmt.Errorf("%s: Equal: %w", name, err)
			}
		}
	}

	if !found {
		return fmt.Errorf("no types with `per:` tags found in package %s", g.pkg.Name())
	}

	return nil
}

func (g *generator) localStruct(name string) (*types.Struct, bool) {
	nt, ok := g.pkg.Scope().Lookup(name).(*types.TypeName)
	if !ok || nt.IsAlias() {
		return nil, false
	}

	named, ok := nt.Type().(*types.Named)
	if !ok {
		return nil, false
	}

	s, ok := named.Underlying().(*types.Struct)

	return s, ok
}

func (g *generator) collectReferencedStructs(scope *types.Scope) {
	changed := true
	for changed {
		changed = false

		for name := range g.genTypes {
			underlying, ok := g.localStruct(name)
			if !ok {
				continue
			}

			for i := 0; i < underlying.NumFields(); i++ {
				if tag := structTagGet(underlying.Tag(i), "per"); tag == "-" || strings.Contains(tag, "exttable") || strings.Contains(tag, "extalt") {
					continue
				}

				ft := underlying.Field(i).Type()
				if p, ok := ft.(*types.Pointer); ok {
					ft = p.Elem()
				}

				if s, ok := ft.(*types.Slice); ok {
					ft = s.Elem()
				}

				n, ok := ft.(*types.Named)
				if !ok || n.Obj().Pkg() != g.pkg || hasDeclaredMethod(n, "MarshalPER") {
					continue
				}

				if _, isStruct := n.Underlying().(*types.Struct); isStruct && !g.genTypes[n.Obj().Name()] {
					g.genTypes[n.Obj().Name()] = true
					changed = true
				}
			}
		}
	}

	_ = scope
}

func structIsPER(s *types.Struct) bool {
	for i := 0; i < s.NumFields(); i++ {
		tag := structTagGet(s.Tag(i), "per")
		if tag == "" || tag == "-" {
			continue
		}

		return true
	}

	return false
}

func structIsExtSeq(s *types.Struct) bool {
	for i := 0; i < s.NumFields(); i++ {
		if structTagGet(s.Tag(i), "per") == "extseq" {
			return true
		}
	}

	return false
}

func (g *generator) typeEnabled(name string) bool {
	if len(g.cfg.Types) == 0 {
		return true
	}

	return slices.Contains(g.cfg.Types, name)
}

func (g *generator) output() ([]byte, error) {
	if g.buf.Len() == 0 {
		return nil, errors.New("no generated code")
	}

	var head bytes.Buffer

	head.WriteString("// Code generated by pergen. DO NOT EDIT.\n\n")
	head.WriteString("//go:build !ignore\n\n")
	head.WriteString("package " + g.pkg.Name() + "\n\n")

	imports := append(slices.Clone(g.imports), perImportPath)
	if g.usesAP {
		imports = append(imports, apImportPath)
	}

	if len(imports) == 1 {
		head.WriteString("import \"" + imports[0] + "\"\n\n")
	} else {
		head.WriteString("import (\n")

		for _, imp := range imports {
			head.WriteString("\t\"" + imp + "\"\n")
		}

		head.WriteString(")\n\n")
	}

	head.Write(g.buf.Bytes())

	formatted, err := format.Source(head.Bytes())
	if err != nil {
		return head.Bytes(), fmt.Errorf("format: %w", err)
	}

	return formatted, nil
}

func (g *generator) isMessage(named *types.Named) bool {
	obj, _, _ := types.LookupFieldOrMethod(named, true, g.pkg, "Message")

	v, ok := obj.(*types.Var)
	if !ok || !v.Embedded() {
		return false
	}

	n, ok := types.Unalias(v.Type()).(*types.Named)

	return ok && n.Origin().Obj().Pkg() != nil && n.Origin().Obj().Pkg().Path() == apImportPath && n.Origin().Obj().Name() == "Message"
}
