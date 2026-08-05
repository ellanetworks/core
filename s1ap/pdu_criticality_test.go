// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"
)

// TS 36.413 §9.3.2 constrains InitiatingMessage/SuccessfulOutcome/
// UnsuccessfulOutcome criticality to the value the elementary procedure object
// declares: S1AP-ELEMENTARY-PROCEDURE.&criticality({S1AP-ELEMENTARY-PROCEDURES}
// {@procedureCode}). The decoder does not check it, and a body-only golden does
// not reach it, so the literals in every Marshal are checked against
// procedureInfos here.
func TestPDUCriticalityMatchesProcedure(t *testing.T) {
	procByConst := make(map[string]ProcedureCode, len(procedureNames))
	for code, name := range procedureNames {
		procByConst["Proc"+name] = code
	}

	critByConst := map[string]Criticality{
		"CriticalityReject": CriticalityReject,
		"CriticalityIgnore": CriticalityIgnore,
		"CriticalityNotify": CriticalityNotify,
	}

	fset := token.NewFileSet()

	pkgs, err := parser.ParseDir(fset, ".", func(fi fs.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatalf("parse package: %v", err)
	}

	checked := 0

	for _, pkg := range pkgs {
		for _, file := range pkg.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}

				name, ok := lit.Type.(*ast.Ident)
				if !ok {
					return true
				}

				switch name.Name {
				case "InitiatingMessage", "SuccessfulOutcome", "UnsuccessfulOutcome":
				default:
					return true
				}

				procIdent, critIdent := pduLiteralFields(lit)

				// The decode path rebuilds a PDU from the peer's values, which
				// are variables and carry whatever arrived.
				if !strings.HasPrefix(procIdent, "Proc") || !strings.HasPrefix(critIdent, "Criticality") {
					return true
				}

				code, ok := procByConst[procIdent]
				if !ok {
					t.Errorf("%s: unknown ProcedureCode constant %s", fset.Position(lit.Pos()), procIdent)

					return true
				}

				crit, ok := critByConst[critIdent]
				if !ok {
					t.Errorf("%s: unknown Criticality constant %s", fset.Position(lit.Pos()), critIdent)

					return true
				}

				checked++

				if want := ProcedureCriticality(code); crit != want {
					t.Errorf("%s: %s encodes criticality %s, TS 36.413 §9.3.2 fixes it at %s",
						fset.Position(lit.Pos()), procIdent, crit, want)
				}

				return true
			})
		}
	}

	// Guards against the walk silently matching nothing.
	if checked < 40 {
		t.Fatalf("checked %d PDU literals, expected at least 40", checked)
	}
}

// pduLiteralFields returns the ProcedureCode and Criticality field names of a
// PDU composite literal, empty where either is not a plain identifier.
func pduLiteralFields(lit *ast.CompositeLit) (proc, crit string) {
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}

		key, ok := kv.Key.(*ast.Ident)
		if !ok {
			continue
		}

		value, ok := kv.Value.(*ast.Ident)
		if !ok {
			continue
		}

		switch key.Name {
		case "ProcedureCode":
			proc = value.Name
		case "Criticality":
			crit = value.Name
		}
	}

	return proc, crit
}
