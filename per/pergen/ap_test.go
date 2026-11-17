// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package pergen

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

func apConfig(dir string) Config {
	return Config{Dir: dir, Output: filepath.Join(dir, "per_gen.go"), Profile: "profile", IEID: "ID"}
}

func TestGenerateApplicationProtocolFixture(t *testing.T) {
	out, err := Generate(t.Context(), apConfig("testdata/ap"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := parser.ParseFile(token.NewFileSet(), "per_gen.go", out, 0); err != nil {
		t.Fatalf("generated code does not parse: %v", err)
	}

	src := string(out)

	for _, want := range []string{
		"func (kind Kind) MarshalPER(w *per.Writer, enc per.Encoding) error {\n\treturn enumKind.Encode(w, enc, kind)",
		"func (kind Kind) Name() string {\n\treturn enumKind.Name(kind)",
		"func (kind Kind) Diagnose(*apimpl.Diagnosis[ID]) bool {\n\treturn !enumKind.Known(kind)",
		"var extIEsInner = []apimpl.IESpec[ID, Inner]{",
		"ID:          IDThing,",
		"ext := apimpl.ExtensionFields(extIEsInner, inner, inner.ext)",
		"apimpl.DecodeExtensionContainer(profile, r, enc, extIEsInner, inner, &inner.ext)",
		"apimpl.DiagnoseExtensions(d, extIEsInner, inner, &inner.ext)",
		"*inner = Inner{}",
		"inner.add.UnmarshalPER(r, enc)",
		"choice.Ext, err = per.DecodeExtensionAlternative(r, enc, extIdx)",
		"f, err := apimpl.DecodeSingleContainerValue[ID](profile, r, enc)",
		"choice.crit = apimpl.Wire(f.Criticality, apimpl.CriticalityReject)",
		"if err := apimpl.DecodeValue([]byte(f.Value), &v); err != nil {",
		"Clear: func(x *Inner) {",
		"choice.Raw = &f",
		"return apimpl.EncodeField(profile, w, enc, apimpl.FieldValue[ID]{ID: IDThing, Criticality: choice.crit.Of(apimpl.CriticalityReject), Value: choice.Typed})",
		"return apimpl.DiagnoseUnknownChoiceExtension(d, choice.Raw)",
		"return apimpl.DiagnoseChoiceExtension(d, IDThing, choice.crit.Of(apimpl.CriticalityReject), &choice.Typed, &choice.Raw)",
		"return per.ErrChoice",
		"func (list List) Diagnose(d *apimpl.Diagnosis[ID]) bool {",
		"func (inner Inner) Equal(y Inner) bool {",
		"(inner.Thing == nil) == (y.Thing == nil) && (inner.Thing == nil || (*inner.Thing) == *y.Thing)",
		"inner.ext == y.ext",
		"inner.add == y.add",
		"(choice.In == nil) == (y.In == nil) && (choice.In == nil || (*choice.In).Equal(*y.In))",
		"(choice.Raw == nil) == (y.Raw == nil) && (choice.Raw == nil || (*choice.Raw) == *y.Raw)",
		"choice.crit == y.crit",
		"func (list List) Equal(y List) bool {",
		"if !(list[i].Equal(y[i])) {",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("generated code lacks %q", want)
		}
	}

	if !strings.Contains(src, "func (choice *Choice) MarshalPER(") || !strings.Contains(src, "\t\treturn per.ErrEmpty\n\t}\n\treturn nil\n}\n\nfunc (choice *Choice) UnmarshalPER(") {
		t.Error("a CHOICE with a root alternative that falls through lost its final return")
	}

	if !strings.Contains(src, "\t\treturn per.ErrEmpty\n\t}\n}\n\nfunc (container *Container) UnmarshalPER(") {
		t.Error("a CHOICE whose every alternative returns ends in unreachable code")
	}

	for _, banned := range []string{"SkipOpenType", "RootEnumerated"} {
		if strings.Contains(src, banned) {
			t.Errorf("generated code drops data through %s", banned)
		}
	}
}

func TestGenerateApplicationProtocolRejectsLossyModels(t *testing.T) {
	for dir, want := range map[string]string{
		"extseq":     "extseq drops extension additions",
		"inlineenum": "needs its own per.Enum type",
		"handtable":  "names a hand-written table",
		"choiceraw":  "needs a bare extie field",
		"equalref":   "Hand: Equal: field byName: map[string]uint8 has no Equal method",
	} {
		t.Run(dir, func(t *testing.T) {
			_, err := Generate(t.Context(), apConfig(filepath.Join("testdata/apbad", dir)))
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("err = %v, want %q", err, want)
			}
		})
	}
}

func TestParseTagRejectsSkip(t *testing.T) {
	if _, err := ParseTag(",skip"); err == nil {
		t.Fatal("skip accepted")
	}
}

func TestParseTagApplicationProtocolOptions(t *testing.T) {
	tag, err := ParseTag(",extie:IDX,crit:reject,presence:conditional,cond:needsX")
	if err != nil {
		t.Fatal(err)
	}

	if !tag.ExtIE || tag.ExtIEID != "IDX" || tag.Criticality != "reject" || tag.Presence != "conditional" || tag.Condition != "needsX" {
		t.Fatalf("parsed %+v", tag)
	}

	for _, bad := range []string{",crit:sometimes", ",presence:maybe", ",extie:", ",cond:"} {
		if _, err := ParseTag(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}
