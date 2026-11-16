// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package asn1spec_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per/asn1spec"
)

func module(body string) []byte {
	return []byte("M { itu-t (0) x (1) 2 } DEFINITIONS AUTOMATIC TAGS ::=\nBEGIN\n" + body + "\nEND\n")
}

func mustParse(t *testing.T, body string) *asn1spec.Module {
	t.Helper()

	m, err := asn1spec.Parse("m.asn", module(body))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	return m
}

func TestParseHeader(t *testing.T) {
	m := mustParse(t, "")

	if m.Name != "M" || m.TagDefault != "AUTOMATIC" {
		t.Fatalf("got name %q tags %q", m.Name, m.TagDefault)
	}

	want := []asn1spec.OIDComponent{{Name: "itu-t", HasNumber: true}, {Name: "x", Number: 1, HasNumber: true}, {Number: 2, HasNumber: true}}
	if len(m.OID) != len(want) {
		t.Fatalf("OID = %+v", m.OID)
	}

	for i := range want {
		if m.OID[i] != want[i] {
			t.Errorf("OID[%d] = %+v, want %+v", i, m.OID[i], want[i])
		}
	}
}

func TestParseImports(t *testing.T) {
	m := mustParse(t, "IMPORTS\n\tA,\n\tb-c,\n\tP{}\nFROM X\n\tD\nFROM Y;")

	if len(m.Imports) != 2 {
		t.Fatalf("imports = %+v", m.Imports)
	}

	if got := strings.Join(m.Imports[0].Symbols, " "); got != "A b-c P" || m.Imports[0].From != "X" {
		t.Errorf("first import = %+v", m.Imports[0])
	}

	if got := strings.Join(m.Imports[1].Symbols, " "); got != "D" || m.Imports[1].From != "Y" {
		t.Errorf("second import = %+v", m.Imports[1])
	}
}

func TestParseValueAssignments(t *testing.T) {
	tests := []struct {
		src     string
		name    string
		typ     string
		value   string
		integer int64
	}{
		{"id-MME-UE-S1AP-ID ProtocolIE-ID ::= 0", "id-MME-UE-S1AP-ID", "ProtocolIE-ID", "0", 0},
		{"maxnoofE-RABs INTEGER ::= 256", "maxnoofE-RABs", "INTEGER", "256", 256},
		{"id-HandoverPreparation ProcedureCode ::= 0", "id-HandoverPreparation", "ProcedureCode", "0", 0},
		{"neg INTEGER ::= -12", "neg", "INTEGER", "-12", -12},
		{"big INTEGER ::= 18446744073709551615", "big", "INTEGER", "18446744073709551615", -1},
		{"alias INTEGER ::= -- comment -- 7", "alias", "INTEGER", "7", 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := mustParse(t, tt.src)

			va := m.Value(tt.name)
			if va == nil {
				t.Fatalf("value %s not found", tt.name)
			}

			if va.Type.String() != tt.typ || va.Value.String() != tt.value {
				t.Errorf("got %s ::= %s", va.Type, va.Value)
			}

			n, err := va.Value.Int64()
			if tt.integer < 0 && tt.value != "-12" {
				if err == nil {
					t.Errorf("Int64 accepted out-of-range %s", tt.value)
				}

				return
			}

			if err != nil || n != tt.integer {
				t.Errorf("Int64 = %d, %v", n, err)
			}
		})
	}
}

func TestParseSequence(t *testing.T) {
	m := mustParse(t, `
Seq ::= SEQUENCE {
	a		INTEGER (0..4095, ...),
	b		OCTET STRING								OPTIONAL,
	iE-Extensions	ProtocolExtensionContainer { {Seq-ExtIEs} }	OPTIONAL,
	...,
	c		BOOLEAN,
	[[ d	NULL,
	   e	BIT STRING (SIZE(1..160, ...))		OPTIONAL ]],
	...,
	f		Other DEFAULT x
}`)

	typ := m.Type("Seq").Type
	if typ.Kind != asn1spec.Sequence || !typ.Extensible {
		t.Fatalf("kind %s extensible %v", typ.Kind, typ.Extensible)
	}

	type comp struct {
		name     string
		typ      string
		optional bool
		ext      bool
		group    int
	}

	want := []comp{
		{"a", "INTEGER (0..4095, ...)", false, false, 0},
		{"b", "OCTET STRING", true, false, 0},
		{"iE-Extensions", "ProtocolExtensionContainer {{Seq-ExtIEs}}", true, false, 0},
		{"c", "BOOLEAN", false, true, 0},
		{"d", "NULL", false, true, 1},
		{"e", "BIT STRING (SIZE(1..160, ...))", true, true, 1},
		{"f", "Other", false, false, 0},
	}

	if len(typ.Components) != len(want) {
		t.Fatalf("got %d components", len(typ.Components))
	}

	for i, w := range want {
		c := typ.Components[i]

		got := comp{c.Name, c.Type.String(), c.Optional, c.Extension, c.Group}
		if got != w {
			t.Errorf("component %d = %+v, want %+v", i, got, w)
		}
	}

	if d := typ.Component("f").Default; d == nil || d.Ref != "x" {
		t.Errorf("DEFAULT = %v", d)
	}

	ext := typ.Component("iE-Extensions").Type
	if ext.Kind != asn1spec.Reference || len(ext.Args) != 1 || ext.Args[0].Set == nil || ext.Args[0].Set.Root[0].Ref != "Seq-ExtIEs" {
		t.Errorf("iE-Extensions = %+v", ext)
	}
}

func TestParseTypes(t *testing.T) {
	tests := []struct {
		src  string
		kind asn1spec.Kind
		str  string
	}{
		{"T ::= SEQUENCE (SIZE(1.. maxnoofE-RABs)) OF ProtocolIE-SingleContainer { { E-RABItemIEs} }", asn1spec.SequenceOf, "SEQUENCE (SIZE(1..maxnoofE-RABs)) OF ProtocolIE-SingleContainer {{E-RABItemIEs}}"},
		{"T ::= SEQUENCE SIZE (0..8) OF Item", asn1spec.SequenceOf, "SEQUENCE (SIZE(0..8)) OF Item"},
		{"T ::= INTEGER (1..30|40|50, ...)", asn1spec.Integer, "INTEGER (1..30|40|50, ...)"},
		{"T ::= INTEGER (0..4095, ..., 4096..65535)", asn1spec.Integer, "INTEGER (0..4095, ..., 4096..65535)"},
		{"T ::= INTEGER (-127..128)", asn1spec.Integer, "INTEGER (-127..128)"},
		{"T ::= BIT STRING (SIZE(128, ...))", asn1spec.BitString, "BIT STRING (SIZE(128, ...))"},
		{"T ::= OCTET STRING (SIZE (3))", asn1spec.OctetString, "OCTET STRING (SIZE(3))"},
		{"T ::= OCTET STRING (CONTAINING Transfer)", asn1spec.OctetString, "OCTET STRING (CONTAINING Transfer)"},
		{"T ::= PrintableString (SIZE (1..150,...))", asn1spec.CharacterString, "PrintableString (SIZE(1..150, ...))"},
		{"T ::= VisibleString", asn1spec.CharacterString, "VisibleString"},
		{"T ::= OBJECT IDENTIFIER", asn1spec.ObjectIdentifier, "OBJECT IDENTIFIER"},
		{"T ::= Other", asn1spec.Reference, "Other"},
		{"T ::= C.&id ({Set}{@id})", asn1spec.ObjectClassField, "C.&id"},
		{"T ::= List { 1, maxnoofX, {Items} }", asn1spec.Reference, "List {1, maxnoofX, {Items}}"},
	}

	for _, tt := range tests {
		t.Run(tt.str, func(t *testing.T) {
			typ := mustParse(t, tt.src).Type("T").Type
			if typ.Kind != tt.kind || typ.String() != tt.str {
				t.Errorf("got %s %q", typ.Kind, typ.String())
			}
		})
	}
}

func TestParseTableConstraint(t *testing.T) {
	typ := mustParse(t, "T ::= SEQUENCE { id C.&id ({S}), value C.&Value ({S}{@id}) }").Type("T").Type

	if tc := typ.Component("id").Type.Table; tc == nil || tc.Set != "S" || tc.At != "" {
		t.Errorf("id table = %+v", tc)
	}

	if tc := typ.Component("value").Type.Table; tc == nil || tc.Set != "S" || tc.At != "id" {
		t.Errorf("value table = %+v", tc)
	}
}

func TestParseIntegerNamedNumbers(t *testing.T) {
	typ := mustParse(t, "PriorityLevel ::= INTEGER { spare (0), highest (1), lowest (14), no-priority (15) } (0..15)").Type("PriorityLevel").Type

	if len(typ.NamedNumbers) != 4 || typ.NamedNumbers[3].Name != "no-priority" || typ.NamedNumbers[3].Value.Number != "15" {
		t.Errorf("named numbers = %+v", typ.NamedNumbers)
	}

	if typ.Range.String() != "0..15" {
		t.Errorf("range = %s", typ.Range)
	}
}

func TestParseEnumerated(t *testing.T) {
	tests := []struct {
		src        string
		root, ext  string
		extensible bool
	}{
		{"T ::= ENUMERATED { reject, ignore, notify }", "reject ignore notify", "", false},
		{"T ::= ENUMERATED { a, b, ... }", "a b", "", true},
		{"T ::= ENUMERATED {n2, n3, ..., n0}", "n2 n3", "n0", true},
		{"T ::= ENUMERATED { x(1), y(0), ..., z(2) }", "x y", "z", true},
	}

	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			typ := mustParse(t, tt.src).Type("T").Type

			var root, ext []string

			for _, e := range typ.Enum {
				if e.Extension {
					ext = append(ext, e.Name)
				} else {
					root = append(root, e.Name)
				}
			}

			if strings.Join(root, " ") != tt.root || strings.Join(ext, " ") != tt.ext || typ.Extensible != tt.extensible {
				t.Errorf("root %v ext %v extensible %v", root, ext, typ.Extensible)
			}
		})
	}
}

func TestParseChoice(t *testing.T) {
	typ := mustParse(t, `T ::= CHOICE {
	gTPTunnel			GTPTunnel,
	choice-Extensions	ProtocolIE-SingleContainer { {T-ExtIEs} },
	...,
	late				NULL
}`).Type("T").Type

	if typ.Kind != asn1spec.Choice || !typ.Extensible || len(typ.Components) != 3 {
		t.Fatalf("got %+v", typ)
	}

	if typ.Components[1].Extension || !typ.Components[2].Extension {
		t.Errorf("extension flags wrong")
	}
}

const classes = `
Criticality ::= ENUMERATED { reject, ignore, notify }
Presence ::= ENUMERATED { optional, conditional, mandatory }
ProtocolIE-ID ::= INTEGER (0..65535)
id-A ProtocolIE-ID ::= 1
id-B ProtocolIE-ID ::= 2
ProcedureCode ::= INTEGER (0..255)
id-P ProcedureCode ::= 9
X-PROTOCOL-IES ::= CLASS {
	&id				ProtocolIE-ID			UNIQUE,
	&criticality	Criticality,
	&Value,
	&presence		Presence
}
WITH SYNTAX {
	ID				&id
	CRITICALITY		&criticality
	TYPE			&Value
	PRESENCE		&presence
}
X-PROTOCOL-IES-PAIR ::= CLASS {
	&id					ProtocolIE-ID		UNIQUE,
	&firstCriticality	Criticality,
	&FirstValue,
	&secondCriticality	Criticality,
	&SecondValue,
	&presence			Presence
}
WITH SYNTAX {
	ID					&id
	FIRST CRITICALITY	&firstCriticality
	FIRST TYPE			&FirstValue
	SECOND CRITICALITY	&secondCriticality
	SECOND TYPE			&SecondValue
	PRESENCE			&presence
}
X-ELEMENTARY-PROCEDURE ::= CLASS {
	&InitiatingMessage				,
	&SuccessfulOutcome							OPTIONAL,
	&UnsuccessfulOutcome						OPTIONAL,
	&procedureCode				ProcedureCode	UNIQUE,
	&criticality				Criticality		DEFAULT ignore
}
WITH SYNTAX {
	INITIATING MESSAGE			&InitiatingMessage
	[SUCCESSFUL OUTCOME			&SuccessfulOutcome]
	[UNSUCCESSFUL OUTCOME		&UnsuccessfulOutcome]
	PROCEDURE CODE				&procedureCode
	[CRITICALITY				&criticality]
}
ProtocolIE-Container {X-PROTOCOL-IES : IEsSetParam} ::=
	SEQUENCE (SIZE (0..65535)) OF
	ProtocolIE-Field {{IEsSetParam}}
ProtocolIE-Field {X-PROTOCOL-IES : IEsSetParam} ::= SEQUENCE {
	id				X-PROTOCOL-IES.&id				({IEsSetParam}),
	criticality		X-PROTOCOL-IES.&criticality		({IEsSetParam}{@id}),
	value			X-PROTOCOL-IES.&Value			({IEsSetParam}{@id})
}
`

func TestObjectSets(t *testing.T) {
	m := mustParse(t, classes+`
Msg ::= SEQUENCE {
	protocolIEs		ProtocolIE-Container		{ {MsgIEs} },
	...
}
MsgIEs X-PROTOCOL-IES ::= {
	{ ID id-A	CRITICALITY reject	TYPE INTEGER (0..7)	PRESENCE mandatory}|
	{ ID id-B	CRITICALITY ignore	TYPE OCTET STRING (CONTAINING Msg)	PRESENCE optional},
	...
}
PairIEs X-PROTOCOL-IES-PAIR ::= {
	{ ID id-A FIRST CRITICALITY reject FIRST TYPE Msg SECOND CRITICALITY ignore SECOND TYPE NULL PRESENCE mandatory },
	...
}
proc X-ELEMENTARY-PROCEDURE ::= {
	INITIATING MESSAGE		Msg
	UNSUCCESSFUL OUTCOME	Msg
	PROCEDURE CODE			id-P
}
Procs X-ELEMENTARY-PROCEDURE ::= { proc, ... }
`)

	set := m.ObjectSet("MsgIEs")
	if set == nil || set.Class != "X-PROTOCOL-IES" || !set.Set.Extensible || len(set.Set.Root) != 2 {
		t.Fatalf("MsgIEs = %+v", set)
	}

	a := set.Set.Root[0].Object
	if v, _ := a.Value("&id"); v.Ref != "id-A" {
		t.Errorf("id = %v", v)
	}

	if v, _ := a.Value("&criticality"); v.Ref != "reject" {
		t.Errorf("criticality = %v", v)
	}

	if v, _ := a.Value("&presence"); v.Ref != "mandatory" {
		t.Errorf("presence = %v", v)
	}

	if typ := a.Type("&Value"); typ.String() != "INTEGER (0..7)" {
		t.Errorf("type = %s", typ)
	}

	if typ := set.Set.Root[1].Object.Type("&Value"); typ.Containing == nil || typ.Containing.Name != "Msg" {
		t.Errorf("containing = %v", typ)
	}

	pair := m.ObjectSet("PairIEs").Set.Root[0].Object
	if pair.Type("&FirstValue").Name != "Msg" || pair.Type("&SecondValue").Kind != asn1spec.Null {
		t.Errorf("pair = %+v", pair.Settings)
	}

	if v, _ := pair.Value("&secondCriticality"); v.Ref != "ignore" {
		t.Errorf("second criticality = %v", v)
	}

	proc := m.Object("proc").Object
	if proc.Type("&SuccessfulOutcome") != nil || proc.Type("&UnsuccessfulOutcome").Name != "Msg" {
		t.Errorf("proc = %+v", proc.Settings)
	}

	if _, ok := proc.Value("&criticality"); ok {
		t.Errorf("absent optional criticality reported present")
	}

	s, err := asn1spec.NewSpec(m)
	if err != nil {
		t.Fatal(err)
	}

	objs, err := s.Objects(s.ObjectSet("Procs").Set)
	if err != nil || len(objs) != 1 || objs[0] != proc {
		t.Errorf("Objects = %v, %v", objs, err)
	}

	if n, err := s.Int(asn1spec.Value{Ref: "id-B"}); err != nil || n != 2 {
		t.Errorf("Int(id-B) = %d, %v", n, err)
	}

	u, err := s.Underlying(&asn1spec.Type{Kind: asn1spec.Reference, Name: "Presence"})
	if err != nil || u.Kind != asn1spec.Enumerated {
		t.Errorf("Underlying = %v, %v", u, err)
	}
}

func TestCrossModuleObjects(t *testing.T) {
	c, err := asn1spec.Parse("c.asn", []byte("C DEFINITIONS AUTOMATIC TAGS ::= BEGIN\n"+classes+"\nEND"))
	if err != nil {
		t.Fatal(err)
	}

	u, err := asn1spec.Parse("u.asn", []byte(`U DEFINITIONS AUTOMATIC TAGS ::= BEGIN
IMPORTS id-A, X-PROTOCOL-IES FROM C;
S X-PROTOCOL-IES ::= { { ID id-A CRITICALITY notify TYPE NULL PRESENCE conditional } }
END`))
	if err != nil {
		t.Fatal(err)
	}

	if u.ObjectSet("S").Set.Root[0].Object.Settings != nil {
		t.Fatalf("object resolved without its class")
	}

	if _, err := asn1spec.NewSpec(c, u); err != nil {
		t.Fatal(err)
	}

	if v, _ := u.ObjectSet("S").Set.Root[0].Object.Value("&criticality"); v.Ref != "notify" {
		t.Errorf("criticality = %v", v)
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"set", "T ::= SET { a INTEGER }", "unsupported type"},
		{"components of", "T ::= SEQUENCE { COMPONENTS OF X }", "unsupported component"},
		{"redefined", "T ::= NULL\nT ::= NULL", "T redefined"},
		{"stray", "T ::= NULL )", "expected"},
		{"bad char", "T ::= NULL #", "unexpected character"},
		{"double ellipsis enum", "T ::= ENUMERATED { a, ..., b, ... }", "duplicate extension marker"},
		{"group outside extension", "T ::= SEQUENCE { [[ a NULL ]] }", "extension addition group outside extension"},
		{"bad object", "X ::= CLASS { &id INTEGER } WITH SYNTAX { ID &id }\nS X ::= { { ID 1 EXTRA } }", "unexpected"},
		{"unterminated", "T ::= SEQUENCE {", `unsupported component "END"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := asn1spec.Parse("m.asn", module(tt.src))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}

	if _, err := asn1spec.Parse("m.asn", []byte("M DEFINITIONS ::= BEGIN T ::= NULL")); err == nil || !strings.Contains(err.Error(), "missing END") {
		t.Errorf("missing END: %v", err)
	}
}

func TestSpecErrors(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{"undefined type", "T ::= SEQUENCE { a Missing }", "undefined type Missing"},
		{"undefined value", "T ::= INTEGER (0..maxMissing)", "undefined value maxMissing"},
		{"parameter count", "P {X : S} ::= NULL\nT ::= P", "takes 1 parameters, got 0"},
		{"unknown import", "IMPORTS A FROM Nowhere;", "unknown module Nowhere"},
		{"enum literal", classes + "S X-PROTOCOL-IES ::= { { ID id-A CRITICALITY maybe TYPE NULL PRESENCE optional } }", "maybe is not a literal of Criticality"},
		{"missing mandatory field", classes + "p X-ELEMENTARY-PROCEDURE ::= { INITIATING MESSAGE NULL PROCEDURE CODE 1 }\nq X-PROTOCOL-IES-PAIR ::= { ID id-A FIRST CRITICALITY reject FIRST TYPE NULL SECOND CRITICALITY reject SECOND TYPE NULL PRESENCE optional }", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := asn1spec.Parse("m.asn", module(tt.src))
			if err != nil {
				t.Fatal(err)
			}

			_, err = asn1spec.NewSpec(m)
			if tt.want == "" {
				if err != nil {
					t.Errorf("unexpected error %v", err)
				}

				return
			}

			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestScannerComments(t *testing.T) {
	m := mustParse(t, "/* block /* nested */ */ A ::= -- inline -- INTEGER -- trailing\n-- full line\nB ::=\tNULL")

	if m.Type("A") == nil || m.Type("B") == nil {
		t.Fatalf("types = %+v", m.Types)
	}
}

func TestErrorPosition(t *testing.T) {
	_, err := asn1spec.Parse("m.asn", []byte("M DEFINITIONS ::= BEGIN\n  T ::= SET { }\nEND"))

	var perr *asn1spec.Error
	if !errors.As(err, &perr) || perr.Pos.Line != 2 || perr.Pos.Column != 9 || perr.Pos.Filename != "m.asn" {
		t.Fatalf("err = %#v", err)
	}
}
