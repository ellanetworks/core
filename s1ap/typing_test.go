// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A field may be a value type only when its absence stops the message from
// reaching the application, which TS 36.413 §10.3.5 makes true exactly for
// required IEs of reject criticality. Every other IE must be nil-able, so a
// receiver can tell an absent IE from a zero one; a slice type already is.
func TestNilableUnlessRequiredAndReject(t *testing.T) {
	rows := parseIETables(t)
	fields, sliceTypes := parseMessageFields(t)

	for _, r := range rows {
		field, ok := fields[r.message][r.field]
		if !ok {
			continue
		}

		pointer := strings.HasPrefix(field, "*")
		nilable := pointer || strings.HasPrefix(field, "[]") || sliceTypes[field]
		mustHold := r.presence == "PresenceMandatory" && r.criticality == "CriticalityReject"

		if mustHold && pointer {
			t.Errorf("%s.%s is %q for %s/%s: want a value type",
				r.message, r.field, field, r.presence, r.criticality)
		}

		if !mustHold && !nilable {
			t.Errorf("%s.%s is %q for %s/%s: want nilable", r.message, r.field, field, r.presence, r.criticality)
		}
	}
}

// A conditional row must state its condition, or its presence is unenforceable.
func TestConditionalRowsDeclareACondition(t *testing.T) {
	for _, r := range parseIETables(t) {
		if r.presence == "PresenceConditional" && !r.hasCondition {
			t.Errorf("%s IE %s is conditional but declares no condition", r.message, r.id)
		}
	}
}

type ieRow struct {
	message      string
	id           string
	presence     string
	criticality  string
	field        string
	hasCondition bool
}

var (
	reTable = regexp.MustCompile(`(?s)var \w+IEs = \[\]ieSpec\[(\w+)\]\{(.*?)\n\}\n`)
	reRow   = regexp.MustCompile(`(?s)\{\n\t\tid: (id\w+), presence: (Presence\w+), crit: (Criticality\w+),\n(.*?)\n\t\},`)
	reField = regexp.MustCompile(`m\.([A-Z]\w*)`)
)

func parseIETables(t *testing.T) []ieRow {
	t.Helper()

	var rows []ieRow

	for _, src := range packageSources(t) {
		for _, tbl := range reTable.FindAllStringSubmatch(src, -1) {
			for _, m := range reRow.FindAllStringSubmatch(tbl[2], -1) {
				r := ieRow{
					message:      tbl[1],
					id:           m[1],
					presence:     m[2],
					criticality:  m[3],
					hasCondition: strings.Contains(m[4], "condition:"),
				}

				if f := reField.FindStringSubmatch(m[4]); f != nil {
					r.field = f[1]
				}

				rows = append(rows, r)
			}
		}
	}

	if len(rows) == 0 {
		t.Fatal("no IE table rows found")
	}

	return rows
}

// parseMessageFields returns each struct's field types, and the set of package
// type names whose underlying type is a slice.
func parseMessageFields(t *testing.T) (map[string]map[string]string, map[string]bool) {
	t.Helper()

	out := map[string]map[string]string{}
	slices := map[string]bool{}
	fset := token.NewFileSet()

	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok {
				return true
			}

			if at, ok := ts.Type.(*ast.ArrayType); ok && at.Len == nil {
				slices[ts.Name.Name] = true

				return true
			}

			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}

			fields := map[string]string{}

			for _, fld := range st.Fields.List {
				for _, id := range fld.Names {
					fields[id.Name] = typeString(fld.Type)
				}
			}

			out[ts.Name.Name] = fields

			return true
		})
	}

	return out, slices
}

func typeString(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	default:
		return "?"
	}
}

func packageSources(t *testing.T) []string {
	t.Helper()

	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}

	var out []string

	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}

		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		out = append(out, string(b))
	}

	return out
}
