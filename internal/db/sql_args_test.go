// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package db

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var numberedPlaceholder = regexp.MustCompile(`\?(\d+)`)

var queryArgIndex = map[string]int{
	"ExecContext":             1,
	"QueryContext":            1,
	"QueryRowContext":         1,
	"queryAll":                2,
	"queryPageWithTotal":      2,
	"queryCount":              2,
	"execCounting":            1,
	"countVisitors":           1,
	"updateOne":               1,
	"insertRoamingPartnerRow": 1,
}

var scanArgFuncs = map[string]bool{
	"queryAll":           true,
	"queryPageWithTotal": true,
}

func evalStringConst(consts map[string]ast.Expr, e ast.Expr, depth int) (string, bool) {
	if depth > 32 {
		return "", false
	}

	switch v := e.(type) {
	case *ast.BasicLit:
		if v.Kind != token.STRING {
			return "", false
		}

		s, err := strconv.Unquote(v.Value)

		return s, err == nil
	case *ast.Ident:
		x, ok := consts[v.Name]
		if !ok {
			return "", false
		}

		return evalStringConst(consts, x, depth+1)
	case *ast.BinaryExpr:
		if v.Op != token.ADD {
			return "", false
		}

		a, okA := evalStringConst(consts, v.X, depth+1)
		b, okB := evalStringConst(consts, v.Y, depth+1)

		return a + b, okA && okB
	case *ast.ParenExpr:
		return evalStringConst(consts, v.X, depth+1)
	}

	return "", false
}

func placeholderCount(query string) (int, error) {
	var b strings.Builder

	inString := false

	for _, c := range query {
		if c == '\'' {
			inString = !inString
			continue
		}

		if !inString {
			b.WriteRune(c)
		}
	}

	stripped := b.String()
	numbered := numberedPlaceholder.FindAllStringSubmatch(stripped, -1)
	plain := strings.Count(stripped, "?") - len(numbered)

	if len(numbered) == 0 {
		return plain, nil
	}

	if plain > 0 {
		return 0, strconv.ErrSyntax
	}

	highest := 0

	for _, m := range numbered {
		n, err := strconv.Atoi(m[1])
		if err != nil {
			return 0, err
		}

		highest = max(highest, n)
	}

	return highest, nil
}

func callName(fun ast.Expr) string {
	switch fn := fun.(type) {
	case *ast.SelectorExpr:
		return fn.Sel.Name
	case *ast.Ident:
		return fn.Name
	case *ast.IndexExpr:
		if id, ok := fn.X.(*ast.Ident); ok {
			return id.Name
		}
	}

	return ""
}

func driverArgCount(arg ast.Expr, fallback int) int {
	if id, ok := arg.(*ast.Ident); ok && id.Name == "nil" {
		return 0
	}

	lit, ok := arg.(*ast.CompositeLit)
	if !ok {
		return fallback
	}

	arr, ok := lit.Type.(*ast.ArrayType)
	if !ok {
		return fallback
	}

	sel, ok := arr.Elt.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "NamedValue" {
		return fallback
	}

	return len(lit.Elts)
}

func TestQueryArgumentCountsMatchPlaceholders(t *testing.T) {
	fset := token.NewFileSet()

	files := map[string]*ast.File{}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		files[name] = f
	}

	consts := map[string]ast.Expr{}

	for _, f := range files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}

			for _, s := range gd.Specs {
				vs, ok := s.(*ast.ValueSpec)
				if !ok {
					continue
				}

				for i, n := range vs.Names {
					if i < len(vs.Values) {
						consts[n.Name] = vs.Values[i]
					}
				}
			}
		}
	}

	checked := 0

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}

			name := callName(call.Fun)

			qi, ok := queryArgIndex[name]
			if !ok || len(call.Args) <= qi || call.Ellipsis.IsValid() {
				return true
			}

			query, ok := evalStringConst(consts, call.Args[qi], 0)
			if !ok {
				return true
			}

			want, err := placeholderCount(query)
			if err != nil {
				t.Errorf("%s: %s mixes numbered and positional placeholders", fset.Position(call.Pos()), name)
				return true
			}

			argStart := qi + 1
			if scanArgFuncs[name] {
				argStart++
			}

			got := len(call.Args) - argStart
			if got == 1 {
				got = driverArgCount(call.Args[argStart], got)
			}

			if got != want {
				t.Errorf("%s: %s binds %d arguments, query expects %d", fset.Position(call.Pos()), name, got, want)
			}

			checked++

			return true
		})
	}

	if checked < 200 {
		t.Fatalf("checked only %d call sites; the call-site matcher no longer finds the queries", checked)
	}
}
