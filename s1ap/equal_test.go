// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func equalByMethod(got, want any) bool {
	g, w := reflect.ValueOf(got), reflect.ValueOf(want)
	if g.Type() != w.Type() {
		return false
	}

	if g.Kind() == reflect.Pointer {
		if g.IsNil() || w.IsNil() {
			return g.IsNil() == w.IsNil()
		}

		g, w = g.Elem(), w.Elem()
	}

	eq := g.MethodByName("Equal")
	if !eq.IsValid() {
		panic(g.Type().String() + " has no Equal method")
	}

	return eq.Call([]reflect.Value{w})[0].Bool()
}

var typesWithoutEqual = []string{"AbstractSyntaxError", "Diagnostics", "RawPDU"}

func TestEveryModelTypeHasEqual(t *testing.T) {
	fset := token.NewFileSet()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}

	var files []*ast.File

	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}

		f, err := parser.ParseFile(fset, e.Name(), nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}

		files = append(files, f)
	}

	structs := map[string]bool{}
	lists := map[string]string{}
	equal := map[string]bool{}

	for _, f := range files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok || ts.Assign.IsValid() || !ts.Name.IsExported() {
						continue
					}

					switch typ := ts.Type.(type) {
					case *ast.StructType:
						structs[ts.Name.Name] = true
					case *ast.ArrayType:
						if elem, ok := typ.Elt.(*ast.Ident); ok && typ.Len == nil {
							lists[ts.Name.Name] = elem.Name
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil || d.Name.Name != "Equal" || len(d.Type.Params.List) != 1 || d.Type.Results == nil || len(d.Type.Results.List) != 1 {
					continue
				}

				recv := d.Recv.List[0].Type
				if star, ok := recv.(*ast.StarExpr); ok {
					recv = star.X
				}

				name, ok := recv.(*ast.Ident)
				param, pok := d.Type.Params.List[0].Type.(*ast.Ident)
				res, rok := d.Type.Results.List[0].Type.(*ast.Ident)

				if ok && pok && rok && param.Name == name.Name && res.Name == "bool" {
					equal[name.Name] = true
				}
			}
		}
	}

	for name := range structs {
		if !equal[name] && !slices.Contains(typesWithoutEqual, name) {
			t.Errorf("%s has no Equal(%s) bool: run go generate", name, name)
		}
	}

	for name, elem := range lists {
		if structs[elem] && !equal[name] {
			t.Errorf("SEQUENCE OF %s (%s) has no Equal(%s) bool: run go generate", elem, name, name)
		}
	}

	for _, name := range typesWithoutEqual {
		if !structs[name] {
			t.Errorf("%s is allowlisted but is not an exported struct type of this package", name)
		}
	}
}
