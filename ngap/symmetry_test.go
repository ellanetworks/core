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

const s1apDir = "../s1ap"

// The files the two specs share a design for. Their declarations are named for
// the machinery rather than either protocol, so a faithful transposition uses
// identical names. Message and IE files are absent: each is its own vocabulary.
var engineFiles = []string{
	"cause.go",
	"common.go",
	"criticality_diagnostics.go",
	"errors.go",
	"ie_table.go",
	"ies.go",
	"pdu.go",
}

// Declarations allowed on one side only. An entry is a claim that 3GPP requires
// the asymmetry; anything else is drift.
var mandatedDeviations = map[string]string{
	// TS 38.413 §9.3 closes its CHOICEs with a choice-Extensions alternative
	// carrying an open IE container, where TS 36.413 marks them extensible.
	"ngap only: func decodeChoiceExtension": "NGAP choice-Extensions (TS 38.413 §9.3)",

	// NGAP lists are plain SEQUENCE OF Item; S1AP wraps its E-RAB list items
	// in ProtocolIE-SingleContainer, which needs a list codec NGAP has no use
	// for (TS 36.413 §9.1.3 vs TS 38.413 §9.3.1).
	"s1ap only: func encodeSingleContainerList": "S1AP ProtocolIE-SingleContainer lists (TS 36.413 §9.1.3)",
	"s1ap only: func decodeItemList":            "S1AP ProtocolIE-SingleContainer lists (TS 36.413 §9.1.3)",

	// PATH SWITCH REQUEST FAILURE reports a mandatory released session list in
	// NGAP (TS 38.413 §9.2.3.10) and a mandatory Cause with no list in S1AP
	// (TS 36.413 §9.1.5.10), so only NGAP has to recover the sessions from a
	// rejected request to build the message at all.
	"ngap only: func (AbstractSyntaxError) PathSwitchSessions": "NGAP PathSwitchRequestFailure released list is mandatory (TS 38.413 §9.2.3.10)",
}

// ngapOnlyFiles' mirror, so a file added to one side and forgotten on the other
// is caught whichever side it landed on.
var s1apOnlyFiles = map[string]string{}

// S1AP files whose NGAP counterparts are not written yet — outstanding work,
// not a mandated asymmetry. Entries are deleted as counterparts appear, so the
// list empties itself. A file that exists on both sides under different names
// belongs in renamedFiles, which is consulted first and actually verifies the
// counterpart; an entry here would never be reached.
var pendingNGAPMigration = map[string]struct{}{}

// Same thing, different name per spec. Both sides must still exist.
var renamedFiles = map[string]string{
	// TS 38.413 names the procedure NG Reset where TS 36.413 names it Reset.
	"ng_reset.go":              "reset.go",
	"ng_reset_test.go":         "reset_test.go",
	"ng_setup.go":              "s1_setup.go",
	"ng_setup_resp.go":         "s1_setup_resp.go",
	"ng_setup_failure.go":      "s1_setup_failure.go",
	"ng_setup_test.go":         "s1_setup_test.go",
	"ng_setup_resp_test.go":    "s1_setup_resp_test.go",
	"ng_setup_failure_test.go": "s1_setup_failure_test.go",

	// TS 38.413 carries user plane in PDU sessions where TS 36.413 carries it in
	// E-RABs, so each session-oriented procedure is named after its own bearer.
	"pdu_session_resource_setup.go":                  "erab_setup.go",
	"pdu_session_resource_release.go":                "erab_release.go",
	"pdu_session_resource_modify.go":                 "erab_modify.go",
	"pdu_session_resource_modify_indication.go":      "erab_modification.go",
	"pdu_session_resource_modify_indication_test.go": "erab_modification_test.go",
	"pdu_session_resource_modify_test.go":            "erab_modify_test.go",
	"pdu_session_resource_release_test.go":           "erab_release_test.go",
	"pdu_session_resource_setup_test.go":             "erab_setup_test.go",

	// TS 38.413 names the procedure UE Radio Capability Info Indication where
	// TS 36.413 names it UE Capability Info Indication.
	"ue_radio_capability.go":      "ue_capability.go",
	"ue_radio_capability_test.go": "ue_capability_test.go",

	// TS 38.413 carries LMF signalling as NRPPa where TS 36.413 carries the
	// E-SMLC's as LPPa (TS 38.455 / TS 36.455).
	"nrppa_transport.go":      "lppa_transport.go",
	"nrppa_transport_test.go": "lppa_transport_test.go",

	// TS 38.413 names the NG-RAN node's update RAN Configuration Update where
	// TS 36.413 names the eNB's ENB Configuration Update.
	"ran_config_update.go":      "enb_config_update.go",
	"ran_config_update_test.go": "enb_config_update_test.go",

	// GUAMI replaces GUMMEI and is bit-string rather than octet shaped
	// (TS 38.413 §9.3.3.3).
	"ie_guami.go": "ie_gummei.go",

	// 3GPP scopes these QoS parameters to E-RABs in TS 36.413 and to QoS flows
	// in TS 38.413, so each library names the file after its own bearer.
	"ie_qos.go":      "ie_erab.go",
	"ie_qos_test.go": "ie_erab_test.go",
}

// NGAP-only IE vocabulary, and message files for procedures 3GPP defines only
// for NGAP. Nothing else.
var ngapOnlyFiles = map[string]string{
	"ie_slice.go":                    "S-NSSAI and the slice support lists have no S1AP counterpart (TS 38.413 §9.3.1.24)",
	"pdu_session_resource_notify.go": "TS 36.413 defines no E-RAB Notify: an eNB reports a changed bearer through E-RAB Modification Indication and a self-initiated release through E-RAB Release Indication (TS 38.413 §8.2.6)",

	"pdu_session_resource_notify_test.go": "tests for pdu_session_resource_notify.go",
	"handover_transfer.go":                "the §9.3.4 transfers have no S1AP counterpart: TS 36.413 defines no OCTET STRING (CONTAINING ...) at all and carries the forwarding tunnels as plain IEs in the message body (TS 38.413 §9.3.4)",
	"handover_transfer_test.go":           "tests for handover_transfer.go",
	"ie_tnl.go":                           "NG-RAN TNL association removal has no S1AP counterpart: ENB CONFIGURATION UPDATE cannot remove SCTP endpoints (TS 38.413 §9.3.2.6, §9.2.6.4)",
	"ie_tnl_test.go":                      "tests for ie_tnl.go",
	"ie_transport_test.go":                "UPTransportLayerInformation is a CHOICE closed by choice-Extensions; S1AP carries the user-plane endpoint as a bare address and GTP-TEID pair inside each E-RAB item (TS 38.413 §9.3.2.1)",
	"amf_status_indication.go":            "TS 36.413 defines no procedure signalling that core-network identities are unavailable: mMEStatusTransfer is UE-associated handover status and Overload Start/Stop is overload control (TS 38.413 §8.7.6)",
	"amf_status_indication_test.go":       "tests for amf_status_indication.go",
	"not_comprehended_test.go":            "§10.3.1 case 6 reached through choice-Extensions, which only NGAP has (TS 38.413 §9.3)",
	"symmetry_test.go":                    "this file: the checker lives on one side",
}

// A type on both sides must live in the same-named file, so the two can be read
// side by side.
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

	// The same check the other way.
	ours := map[string]bool{}
	for _, n := range goFiles(t, ".") {
		ours[n] = true
	}

	renamedToNGAP := map[string]string{}
	for ngapName, s1apName := range renamedFiles {
		renamedToNGAP[s1apName] = ngapName
	}

	for _, name := range goFiles(t, s1apDir) {
		if ours[name] {
			continue
		}

		if counterpart, ok := renamedToNGAP[name]; ok {
			if !ours[counterpart] {
				t.Errorf("s1ap/%s is mapped to ngap/%s, which does not exist", name, counterpart)
			}

			continue
		}

		if reason, ok := s1apOnlyFiles[name]; ok {
			t.Logf("permitted s1ap-only file — %s: %s", name, reason)

			continue
		}

		if _, ok := pendingNGAPMigration[name]; ok {
			continue
		}

		t.Errorf("s1ap/%s has no ngap counterpart.\n"+
			"\tEither name it after the file ngap keeps the same types in, or add it to "+
			"s1apOnlyFiles with the reason it is S1AP-only.", name)
	}

	for name, reason := range s1apOnlyFiles {
		if ours[name] {
			t.Errorf("s1apOnlyFiles claims %s is S1AP-only (%q), but ngap has it too", name, reason)
		}
	}

	for name := range pendingNGAPMigration {
		if ours[name] {
			t.Errorf("ngap/%s now exists; drop %s from pendingNGAPMigration", name, name)
		}
	}
}

// per_gen.go is excluded: it tracks whatever pergen was pointed at.
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

// The machine-checked half of the rule that ngap and s1ap are one design over
// two specifications.
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

// So the allowlist cannot rot into a blanket excuse.
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

// Type methods in these files are each protocol's own vocabulary; their free
// functions are shared plumbing, and one library growing a helper the other
// open-codes is drift.
var helperFiles = []string{
	"ie_common.go",
	"per_leaf.go",
}

func TestHelperFuncsAreSymmetric(t *testing.T) {
	if _, err := os.Stat(s1apDir); err != nil {
		t.Skipf("s1ap module not present at %s: %v", s1apDir, err)
	}

	ours := helperFuncs(t, ".")
	theirs := helperFuncs(t, s1apDir)

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

		t.Errorf("%s\n\tShared helpers are meant to exist in both libraries. Either add it "+
			"to the other side, or add the declaration to mandatedDeviations with the TS "+
			"clause that forces it.", u)
	}
}

// Methods are skipped: their receiver is a protocol-specific IE type.
func helperFuncs(t *testing.T, dir string) []string {
	t.Helper()

	var out []string

	fset := token.NewFileSet()

	for _, name := range helperFiles {
		path := filepath.Join(dir, name)

		src, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("read %s: %v", path, err)

			continue
		}

		f, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}

		for _, decl := range f.Decls {
			d, ok := decl.(*ast.FuncDecl)
			if !ok || d.Recv != nil {
				continue
			}

			out = append(out, funcDecl(d))
		}
	}

	sort.Strings(out)

	return out
}

// Types and functions only: constants and variables hold each protocol's own
// procedure codes, IE ids and cause values, which the ground-truth tests pin.
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

// The receiver's pointer-ness is dropped: a Go choice, not a design one.
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
