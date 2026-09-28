// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package main

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

func TestEmitFieldUnmarshalTemporariesAreUnique(t *testing.T) {
	t.Parallel()

	kinds := []fieldInfo{
		{kind: kindBool},
		{kind: kindConstrainedInt, boundsExpr: "per.Bounds{}", typeStr: "int64"},
		{kind: kindEnum, enumRoot: 2, typeStr: "int64"},
		{kind: kindEnum, enumRoot: 2, enumExt: true, enumTotal: 3, typeStr: "int64"},
		{kind: kindOctetString},
		{kind: kindBitString},
		{kind: kindString, charTypeExpr: "per.CharVisibleString"},
		{kind: kindString, hasSizeLB: true, hasSizeUB: true, sizeLB: 1, sizeUB: 8},
		{kind: kindString},
		{kind: kindREAL},
	}

	for _, k := range kinds {
		var (
			g   generator
			buf bytes.Buffer
		)

		for idx := range 2 {
			fi := k
			fi.fieldIdx = idx
			g.emitFieldUnmarshal(&buf, "x.F"+string(rune('0'+idx)), fi, 0)
		}

		src := "package p\nfunc f() error {\n" + buf.String() + "return nil\n}\n"

		file, err := parser.ParseFile(token.NewFileSet(), "", src, 0)
		if err != nil {
			t.Fatalf("kind %d: parse generated code: %v\n%s", k.kind, err, src)
		}

		declared := map[string]bool{}

		ast.Inspect(file, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || as.Tok != token.DEFINE {
				return true
			}

			for _, lhs := range as.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name == "err" || id.Name == "_" {
					continue
				}

				if declared[id.Name] {
					t.Errorf("kind %d: %s declared twice in one scope\n%s", k.kind, id.Name, src)
				}

				declared[id.Name] = true
			}

			return true
		})
	}
}

func TestEmitBoundedEnumerated(t *testing.T) {
	t.Parallel()

	var (
		g   generator
		buf bytes.Buffer
	)

	fi := fieldInfo{kind: kindEnum, enumRoot: 5, enumExt: true, enumTotal: 7, typeStr: "int64"}

	g.emitFieldMarshal(&buf, "x", fi, "x.V", 0)
	g.emitFieldUnmarshal(&buf, "x.V", fi, 0)

	out := buf.String()

	for _, want := range []string{
		"v < 0 || v >= 7",
		"per.EncodeEnumerated(w, enc, 5, true, int64(x.V))",
		"per.DecodeEnumerated(r, enc, 5, true)",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("generated code lacks %q:\n%s", want, out)
		}
	}

	if strings.Contains(out, "RootEnumerated") {
		t.Errorf("bounded ENUMERATED must not use the root-only helpers:\n%s", out)
	}
}
