// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package utils_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A field the codec parses and the decoder never reads cannot reach the API or
// the UI: the element arrives, is decoded, and is dropped without trace. The
// check is reachability rather than name matching, because a decoder is free to
// render a field under another name or at another level — the EPS bearer
// identity, for one, is rendered on the ESM header rather than per message.
func TestDecodersReadEveryCodecField(t *testing.T) {
	for _, d := range []struct {
		decoder string
		codec   string
	}{
		{"../ngap", "../../../ngap"},
		{"../s1ap", "../../../s1ap"},
		{"../nas", "../../../nas/fgs"},
		{"../eps", "../../../nas/eps"},
	} {
		t.Run(strings.TrimPrefix(d.decoder, "../"), func(t *testing.T) {
			decoder := parseGoFiles(t, d.decoder)
			codec := parseGoFiles(t, d.codec)

			read := readsPerMessage(decoder)

			var unread []string

			for msg, fields := range read {
				if !isMessage(codec, msg) {
					continue
				}

				for _, field := range exportedFields(codec, msg) {
					if field == "Unrecognized" || fields[field] {
						continue
					}

					unread = append(unread, msg+"."+field)
				}
			}

			sort.Strings(unread)

			if len(unread) > 0 {
				t.Errorf("the codec parses %d field(s) the decoder never reads, so they cannot be rendered:\n  %s",
					len(unread), strings.Join(unread, "\n  "))
			}
		})
	}
}

func parseGoFiles(t *testing.T, dir string) []*ast.File {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}

	var out []*ast.File

	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}

		f, err := parser.ParseFile(token.NewFileSet(), p, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}

		out = append(out, f)
	}

	if len(out) == 0 {
		t.Fatalf("no Go files under %s", dir)
	}

	return out
}

// readsPerMessage maps each message a decoder handles to the codec field names
// it reads for that message. The scope is per message rather than per package:
// a field read while building one message says nothing about another.
//
// A build<Message> function is scoped to its own body. A type switch arm is
// scoped to the arm plus the statements before the switch, which is where a
// decoder reads a header shared by every arm.
func readsPerMessage(files []*ast.File) map[string]map[string]bool {
	out := map[string]map[string]bool{}

	add := func(msg string, names map[string]bool) {
		if out[msg] == nil {
			out[msg] = map[string]bool{}
		}

		for n := range names {
			out[msg][n] = true
		}
	}

	// a decoder may dispatch to a builder whose name differs from the codec
	// type's, so the arm's reads include the reads of what it delegates to
	builders := map[string]map[string]bool{}

	for _, f := range files {
		for _, decl := range f.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Body != nil {
				builders[fn.Name.Name] = selectorNames(fn.Body)
			}
		}
	}

	for _, f := range files {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Body == nil {
				continue
			}

			if strings.HasPrefix(fn.Name.Name, "build") {
				add(strings.TrimPrefix(fn.Name.Name, "build"), selectorNames(fn.Body))
			}

			for _, sw := range typeSwitches(fn.Body) {
				shared := selectorNames(fn.Body)
				for _, inner := range typeSwitches(fn.Body) {
					for name := range selectorNames(inner) {
						delete(shared, name)
					}
				}

				for _, stmt := range sw.Body.List {
					clause, ok := stmt.(*ast.CaseClause)
					if !ok {
						continue
					}

					names := selectorNames(clause)
					for n := range shared {
						names[n] = true
					}

					for _, called := range calledFuncs(clause) {
						for n := range builders[called] {
							names[n] = true
						}
					}

					for _, e := range clause.List {
						star, ok := e.(*ast.StarExpr)
						if !ok {
							continue
						}

						if sel, ok := star.X.(*ast.SelectorExpr); ok {
							add(sel.Sel.Name, names)
						}
					}
				}
			}
		}
	}

	return out
}

// calledFuncs names the plain functions a node calls, so a dispatch arm that
// delegates is credited with what the builder it calls reads.
func calledFuncs(n ast.Node) []string {
	var out []string

	ast.Inspect(n, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		if id, ok := call.Fun.(*ast.Ident); ok {
			out = append(out, id.Name)
		}

		return true
	})

	return out
}

func typeSwitches(n ast.Node) []*ast.TypeSwitchStmt {
	var out []*ast.TypeSwitchStmt

	ast.Inspect(n, func(n ast.Node) bool {
		if sw, ok := n.(*ast.TypeSwitchStmt); ok {
			out = append(out, sw)
		}

		return true
	})

	return out
}

// isMessage reports whether a codec type is a message rather than a nested
// information element. Only messages are checked: an element is rendered by its
// own builder, often through accessor methods rather than by reading its fields,
// which this check cannot see. A message carries the elements a version does not
// model, so it is the one that can silently drop them.
func isMessage(files []*ast.File, typeName string) bool {
	found := false

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || ts.Name.Name != typeName {
				return true
			}

			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}

			for _, field := range st.Fields.List {
				// NGAP and S1AP messages embed messageMeta; NAS messages carry the
				// elements they do not model in Unrecognized.
				if len(field.Names) == 0 {
					if id, ok := field.Type.(*ast.Ident); ok && id.Name == "messageMeta" {
						found = true
					}

					continue
				}

				if field.Names[0].Name == "Unrecognized" {
					found = true
				}
			}

			return false
		})
	}

	return found
}

// accumulators are the decoder's own values. A selector on one of these says the
// decoder has a field of that name, not that it read the codec's, so counting it
// would let a same-named output field hide a dropped element.
var accumulators = map[string]bool{"out": true, "estAcc": true, "a": true}

// selectorNames collects the field names the decoder reads off a codec value.
// The receiver must be a plain identifier: a chained selector such as
// m.AttachRequest.AdditionalGUTI is the decoder writing its own shape.
func selectorNames(n ast.Node) map[string]bool {
	out := map[string]bool{}

	ast.Inspect(n, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		id, ok := sel.X.(*ast.Ident)
		if !ok || accumulators[id.Name] {
			return true
		}

		out[sel.Sel.Name] = true

		return true
	})

	return out
}

func exportedFields(files []*ast.File, typeName string) []string {
	var out []string

	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			ts, ok := n.(*ast.TypeSpec)
			if !ok || ts.Name.Name != typeName {
				return true
			}

			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				return true
			}

			for _, field := range st.Fields.List {
				for _, name := range field.Names {
					if name.IsExported() {
						out = append(out, name.Name)
					}
				}
			}

			return false
		})
	}

	return out
}
