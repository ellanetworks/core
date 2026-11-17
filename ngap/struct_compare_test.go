// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func holdsReference(t types.Type, seen map[types.Type]bool) bool {
	if seen[t] {
		return false
	}

	seen[t] = true

	switch u := t.Underlying().(type) {
	case *types.Pointer, *types.Slice, *types.Map, *types.Signature, *types.Interface, *types.Chan:
		return true
	case *types.Array:
		return holdsReference(u.Elem(), seen)
	case *types.Struct:
		for i := range u.NumFields() {
			if holdsReference(u.Field(i).Type(), seen) {
				return true
			}
		}
	}

	return false
}

func comparesByReference(t types.Type) bool {
	n, ok := t.(*types.Named)
	if !ok || n.Obj().Pkg() == nil {
		return false
	}

	if _, ok := t.Underlying().(*types.Struct); !ok {
		return false
	}

	return strings.HasPrefix(n.Obj().Pkg().Path(), "github.com/ellanetworks/core/") && holdsReference(t, map[types.Type]bool{})
}

func TestNoEqualityOperatorOnReferenceHoldingStructs(t *testing.T) {
	if testing.Short() {
		t.Skip("type-checks the package from source")
	}

	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	fset := token.NewFileSet()
	pkgName := reflect.TypeFor[ProtocolIEID]().PkgPath()

	var files []*ast.File

	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}

		if f.Name.Name == filepath.Base(pkgName) {
			files = append(files, f)
		}
	}

	info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}

	if _, err := conf.Check(pkgName, fset, files, info); err != nil {
		t.Fatal(err)
	}

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BinaryExpr:
				if x.Op != token.EQL && x.Op != token.NEQ {
					return true
				}

				if ty := info.TypeOf(x.X); ty != nil && comparesByReference(ty) {
					t.Errorf("%s: %s on %s compares pointers, not values: use Equal", fset.Position(x.OpPos), x.Op, ty)
				}
			case *ast.MapType:
				if ty := info.TypeOf(x.Key); ty != nil && comparesByReference(ty) {
					t.Errorf("%s: map keyed by %s compares pointers, not values", fset.Position(x.Pos()), ty)
				}
			}

			return true
		})
	}
}
