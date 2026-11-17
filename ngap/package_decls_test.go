// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

type packageDecls struct {
	enums     map[string]bool
	choices   map[string]bool
	ieTables  map[string]bool
	extTables map[string]bool
}

var (
	declsOnce sync.Once
	decls     packageDecls
	declsErr  error
)

func typeArgName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}

	return ""
}

func isSelector(e ast.Expr, pkg, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	id, ok := sel.X.(*ast.Ident)

	return ok && id.Name == pkg && sel.Sel.Name == name
}

func ieTableElem(e ast.Expr) string {
	at, ok := e.(*ast.ArrayType)
	if !ok || at.Len != nil {
		return ""
	}

	switch x := at.Elt.(type) {
	case *ast.IndexExpr:
		if id, ok := x.X.(*ast.Ident); ok && id.Name == "ieSpec" {
			return typeArgName(x.Index)
		}
	case *ast.IndexListExpr:
		if isSelector(x.X, "apimpl", "IESpec") && len(x.Indices) == 2 {
			return typeArgName(x.Indices[1])
		}
	}

	return ""
}

func scanPackage() (packageDecls, error) {
	d := packageDecls{enums: map[string]bool{}, choices: map[string]bool{}, ieTables: map[string]bool{}, extTables: map[string]bool{}}

	names, err := filepath.Glob("*.go")
	if err != nil {
		return d, err
	}

	fset := token.NewFileSet()

	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			return d, err
		}

		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok {
				continue
			}

			for _, spec := range gd.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					st, ok := s.Type.(*ast.StructType)
					if !ok {
						continue
					}

					for _, fld := range st.Fields.List {
						if fld.Tag == nil {
							continue
						}

						tag, err := strconv.Unquote(fld.Tag.Value)
						if err != nil {
							return d, err
						}

						if strings.Contains(reflect.StructTag(tag).Get("per"), "choice:") {
							d.choices[s.Name.Name] = true
						}
					}
				case *ast.ValueSpec:
					if gd.Tok != token.VAR {
						continue
					}

					for i, n := range s.Names {
						if name, ok := strings.CutSuffix(n.Name, "IEs"); ok && !strings.HasPrefix(n.Name, "extIEs") {
							m := strings.ToUpper(name[:1]) + name[1:]

							if i < len(s.Values) {
								if cl, ok := s.Values[i].(*ast.CompositeLit); ok {
									if m = ieTableElem(cl.Type); m == "" {
										return d, fmt.Errorf("%s: %s is not a []ieSpec", fset.Position(n.Pos()), n.Name)
									}
								}
							}

							d.ieTables[m] = true

							continue
						}

						typ := s.Type

						if typ == nil && i < len(s.Values) {
							if cl, ok := s.Values[i].(*ast.CompositeLit); ok {
								typ = cl.Type
							}
						}

						if typ == nil {
							continue
						}

						if ix, ok := typ.(*ast.IndexExpr); ok && isSelector(ix.X, "per", "Enum") {
							d.enums[typeArgName(ix.Index)] = true

							continue
						}

						if m := ieTableElem(typ); m != "" && strings.HasPrefix(n.Name, "extIEs") {
							d.extTables[m] = true
						}
					}
				}
			}
		}
	}

	return d, nil
}

func scannedDecls(t *testing.T) packageDecls {
	t.Helper()

	declsOnce.Do(func() { decls, declsErr = scanPackage() })

	if declsErr != nil {
		t.Fatal(declsErr)
	}

	return decls
}

func checkComplete(t *testing.T, what string, declared map[string]bool, covered []reflect.Type) {
	t.Helper()

	if len(declared) == 0 {
		t.Fatalf("no %s found in the package source", what)
	}

	have := map[string]bool{}

	for _, ty := range covered {
		if ty.PkgPath() != reflect.TypeFor[ProtocolIEID]().PkgPath() {
			t.Errorf("%s case %v is not a type of this package", what, ty)
		}

		if have[ty.Name()] {
			t.Errorf("%s %s is listed twice", what, ty.Name())
		}

		have[ty.Name()] = true
	}

	var missing, extra []string

	for n := range declared {
		if !have[n] {
			missing = append(missing, n)
		}
	}

	for n := range have {
		if !declared[n] {
			extra = append(extra, n)
		}
	}

	slices.Sort(missing)
	slices.Sort(extra)

	for _, n := range missing {
		t.Errorf("%s %s is declared in the package but has no case", what, n)
	}

	for _, n := range extra {
		t.Errorf("%s case %s does not match a declaration in the package", what, n)
	}
}
