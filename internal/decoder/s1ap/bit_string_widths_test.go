// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestBitStringWidthsMatchCodec(t *testing.T) {
	codec := codecBitStringWidths(t, filepath.Join("..", "..", "..", "s1ap"))
	if len(codec) == 0 {
		t.Fatal("found no fixed-size BIT STRING types in the codec")
	}

	rendered := map[string]int{}
	for typ, bits := range bitStringWidths {
		rendered[typ.Name()] = bits
	}

	for name, bits := range codec {
		got, ok := rendered[name]
		if !ok {
			t.Errorf("%s is a %d-bit BIT STRING the decoder would render as an integer", name, bits)

			continue
		}

		if got != bits {
			t.Errorf("%s: decoder renders %d bits, codec encodes %d", name, got, bits)
		}
	}
}

func codecBitStringWidths(t *testing.T, dir string) map[string]int {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}

	consts := map[string]int{}

	var files []*ast.File

	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}

		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}

		files = append(files, f)

		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}

			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}

					if lit, ok := vs.Values[i].(*ast.BasicLit); ok && lit.Kind == token.INT {
						n, err := strconv.Atoi(lit.Value)
						if err == nil {
							consts[name.Name] = n
						}
					}
				}
			}
		}
	}

	out := map[string]int{}

	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv == nil || fn.Name.Name != "MarshalPER" || fn.Body == nil {
				continue
			}

			recv, ok := fn.Recv.List[0].Type.(*ast.Ident)
			if !ok {
				continue
			}

			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || len(call.Args) != 4 {
					return true
				}

				if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "encodeBitStringUint" {
					return true
				}

				switch w := call.Args[3].(type) {
				case *ast.BasicLit:
					n, err := strconv.Atoi(w.Value)
					if err != nil {
						t.Fatalf("%s: width %q: %v", recv.Name, w.Value, err)
					}

					out[recv.Name] = n
				case *ast.Ident:
					n, ok := consts[w.Name]
					if !ok {
						t.Fatalf("%s: width constant %s not found", recv.Name, w.Name)
					}

					out[recv.Name] = n
				default:
					t.Fatalf("%s: width is not a constant", recv.Name)
				}

				return false
			})
		}
	}

	return out
}
