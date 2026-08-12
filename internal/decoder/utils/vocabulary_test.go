// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// The protocol vocabulary lives in the ngap, s1ap and nas modules. A map to
// string in one of these packages is a second copy of it, free to disagree
// with the spec.
func TestDecodersHoldNoNameTables(t *testing.T) {
	for _, pkg := range []string{"../ngap", "../s1ap", "../nas", "../eps"} {
		files, err := filepath.Glob(filepath.Join(pkg, "*.go"))
		if err != nil {
			t.Fatal(err)
		}

		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}

			f, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
			if err != nil {
				t.Fatal(err)
			}

			ast.Inspect(f, func(n ast.Node) bool {
				m, ok := n.(*ast.MapType)
				if !ok {
					return true
				}

				if v, ok := m.Value.(*ast.Ident); ok && v.Name == "string" {
					t.Errorf("%s declares a map to string; take the names from the codec", file)
				}

				return true
			})
		}
	}
}
