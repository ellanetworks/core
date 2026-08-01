// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"
)

// s1apDir is where the S1AP library sits relative to this module.
const s1apDir = "../s1ap"

// engineFiles are the files TS 38.413 and TS 36.413 share a design for: the
// abstract syntax error model, the IE-container engine and the PDU envelope.
// Their declarations are named for the protocol machinery rather than for
// either protocol, so a faithful transposition uses identical names and this
// test needs no renaming table.
//
// Message and IE files are deliberately absent: what they declare is each
// protocol's own vocabulary.
var engineFiles = []string{
	"cause.go",
	"common.go",
	"criticality_diagnostics.go",
	"errors.go",
	"ie_table.go",
	"ies.go",
	"pdu.go",
}

// mandatedDeviations lists every declaration allowed to exist in one library
// and not the other, with the TS clause that forces it. Anything else is
// drift: either a change made to one library and not the other, or a
// divergence nobody decided on.
//
// Adding an entry here is a claim that 3GPP requires the asymmetry. It should
// be as hard to justify as it looks.
var mandatedDeviations = map[string]string{
	// TS 38.413 §9.3 closes its CHOICEs with a choice-Extensions alternative
	// carrying an open IE container, where TS 36.413 marks them extensible.
	"ngap only: func decodeChoiceExtension": "NGAP choice-Extensions (TS 38.413 §9.3)",

	// NGAP lists are plain SEQUENCE OF Item; S1AP wraps its E-RAB list items
	// in ProtocolIE-SingleContainer, which needs a list codec NGAP has no use
	// for (TS 36.413 §9.1.3 vs TS 38.413 §9.3.1).
	"s1ap only: func encodeSingleContainerList": "S1AP ProtocolIE-SingleContainer lists (TS 36.413 §9.1.3)",
	"s1ap only: func decodeItemList":            "S1AP ProtocolIE-SingleContainer lists (TS 36.413 §9.1.3)",
}

// ngapOnlyFiles are files this library has and s1ap does not. NGAP-only IE
// vocabulary belongs here; a message file does not, because every message this
// library grows should sit in a file named for the same procedure on both
// sides.
// renamedFiles maps an ngap file to the s1ap file holding the same thing,
// where 3GPP gives the two protocols different names for it. The pair still
// has to exist on both sides; only the spelling differs.
var renamedFiles = map[string]string{
	"ng_setup.go":              "s1_setup.go",
	"ng_setup_resp.go":         "s1_setup_resp.go",
	"ng_setup_failure.go":      "s1_setup_failure.go",
	"ng_setup_test.go":         "s1_setup_test.go",
	"ng_setup_resp_test.go":    "s1_setup_resp_test.go",
	"ng_setup_failure_test.go": "s1_setup_failure_test.go",
}

var ngapOnlyFiles = map[string]string{
	"ie_slice.go":      "S-NSSAI and the slice support lists have no S1AP counterpart (TS 38.413 §9.3.1.24)",
	"ie_guami.go":      "GUAMI replaces GUMMEI and is bit-string rather than octet shaped (TS 38.413 §9.3.3.3)",
	"symmetry_test.go": "this file: the checker lives on one side",
}

// TestFileNamesAreSymmetric holds the two libraries to the same file layout:
// a type that exists on both sides must live in the same-named file, so a
// reviewer can read them side by side. Files for messages ngap has not modeled
// yet are ignored — absence is backlog, not drift.
func TestFileNamesAreSymmetric(t *testing.T) {
	if _, err := os.Stat(s1apDir); err != nil {
		t.Skipf("s1ap module not present at %s: %v", s1apDir, err)
	}

	theirs := map[string]bool{}
	for _, n := range goFiles(t, s1apDir) {
		theirs[n] = true
	}

	for _, name := range goFiles(t, ".") {
		if theirs[name] {
			continue
		}

		if counterpart, ok := renamedFiles[name]; ok {
			if !theirs[counterpart] {
				t.Errorf("ngap/%s is mapped to s1ap/%s, which does not exist", name, counterpart)
			}

			continue
		}

		if reason, ok := ngapOnlyFiles[name]; ok {
			t.Logf("permitted ngap-only file — %s: %s", name, reason)

			continue
		}

		t.Errorf("ngap/%s has no s1ap counterpart.\n"+
			"\tEither name it after the file s1ap keeps the same types in, or add it to "+
			"ngapOnlyFiles with the reason it is NGAP-only.", name)
	}

	for name, reason := range ngapOnlyFiles {
		if theirs[name] {
			t.Errorf("ngapOnlyFiles claims %s is NGAP-only (%q), but s1ap has it too", name, reason)
		}
	}
}

// goFiles lists the Go file names directly in dir, generated output aside:
// per_gen.go tracks whatever pergen was pointed at, not a design choice.
func goFiles(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}

	var out []string

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || filepath.Ext(name) != ".go" || name == "per_gen.go" {
			continue
		}

		out = append(out, name)
	}

	sort.Strings(out)

	return out
}

// TestEngineIsSymmetricWithS1AP compares what the shared engine files declare
// in each library. It is the machine-checked half of the rule that ngap and
// s1ap are one design over two specifications.
func TestEngineIsSymmetricWithS1AP(t *testing.T) {
	if _, err := os.Stat(s1apDir); err != nil {
		t.Skipf("s1ap module not present at %s: %v", s1apDir, err)
	}

	ours := engineDecls(t, ".")
	theirs := engineDecls(t, s1apDir)

	var unexplained []string

	for _, d := range ours {
		if !slices.Contains(theirs, d) {
			unexplained = append(unexplained, check(t, "ngap only: "+d))
		}
	}

	for _, d := range theirs {
		if !slices.Contains(ours, d) {
			unexplained = append(unexplained, check(t, "s1ap only: "+d))
		}
	}

	for _, u := range unexplained {
		if u == "" {
			continue
		}

		t.Errorf("%s\n\tThe engine files are meant to be the same design in both libraries. "+
			"Either make the change in both, or add the declaration to mandatedDeviations "+
			"with the TS clause that forces it.", u)
	}
}

// check reports the deviation unless it is one 3GPP mandates.
func check(t *testing.T, key string) string {
	t.Helper()

	if reason, ok := mandatedDeviations[key]; ok {
		t.Logf("permitted asymmetry — %s: %s", key, reason)

		return ""
	}

	return key
}

// TestMandatedDeviationsAreAllReal fails when an allowlist entry stops
// describing something that is actually there, so the list cannot rot into a
// blanket excuse.
func TestMandatedDeviationsAreAllReal(t *testing.T) {
	if _, err := os.Stat(s1apDir); err != nil {
		t.Skipf("s1ap module not present at %s: %v", s1apDir, err)
	}

	ours := engineDecls(t, ".")
	theirs := engineDecls(t, s1apDir)

	for key := range mandatedDeviations {
		side, decl, ok := strings.Cut(key, " only: ")
		if !ok {
			t.Errorf("malformed allowlist key %q, want \"<lib> only: <decl>\"", key)

			continue
		}

		var have, lack []string

		switch side {
		case "ngap":
			have, lack = ours, theirs
		case "s1ap":
			have, lack = theirs, ours
		default:
			t.Errorf("allowlist key %q names unknown library %q", key, side)

			continue
		}

		if !slices.Contains(have, decl) {
			t.Errorf("allowlist claims %s declares %q, but it does not — remove the entry", side, decl)
		}

		if slices.Contains(lack, decl) {
			t.Errorf("allowlist claims only %s declares %q, but both do — remove the entry", side, decl)
		}
	}
}

// engineDecls returns the type and function declarations of the engine files
// in dir, as sorted "kind name" strings. Constants and variables are left out:
// they hold each protocol's own procedure codes, IE ids and cause values,
// which the ground-truth tests pin instead.
func engineDecls(t *testing.T, dir string) []string {
	t.Helper()

	var out []string

	fset := token.NewFileSet()

	for _, name := range engineFiles {
		path := filepath.Join(dir, name)

		src, err := os.ReadFile(path)
		if err != nil {
			// A file one library has and the other does not is itself drift.
			t.Errorf("read %s: %v", path, err)

			continue
		}

		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				out = append(out, funcDecl(d))
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}

				for _, spec := range d.Specs {
					if ts, ok := spec.(*ast.TypeSpec); ok {
						out = append(out, "type "+ts.Name.Name)
					}
				}
			}
		}
	}

	sort.Strings(out)

	return out
}

// funcDecl renders a function or method as "func Name" or "func (T) Name".
// The receiver's pointer-ness is dropped: it is a Go choice, not a design one.
func funcDecl(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return "func " + d.Name.Name
	}

	recv := d.Recv.List[0].Type
	if star, ok := recv.(*ast.StarExpr); ok {
		recv = star.X
	}

	name := "?"
	if id, ok := recv.(*ast.Ident); ok {
		name = id.Name
	}

	return "func (" + name + ") " + d.Name.Name
}
