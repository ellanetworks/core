// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"reflect"
	"strings"
	"testing"
	"unicode"
)

type elementaryProcedure struct {
	name        string
	code        ProcedureCode
	criticality Criticality
	messages    [3]string
}

func asn1Values(t *testing.T, typeName string) map[int64]string {
	t.Helper()

	s := vendoredSpec(t)
	out := map[int64]string{}

	for _, m := range s.Modules {
		for _, va := range m.Values {
			if va.Type.Name != typeName {
				continue
			}

			n, err := s.Int(va.Value)
			if err != nil {
				t.Fatal(err)
			}

			out[n] = va.Name
		}
	}

	return out
}

func elementaryProcedures(t *testing.T) []elementaryProcedure {
	t.Helper()

	s := vendoredSpec(t)

	class := s.Class("S1AP-ELEMENTARY-PROCEDURE")
	if class == nil {
		t.Fatal("S1AP-ELEMENTARY-PROCEDURE not found")
	}

	def := class.Field("&criticality")
	if def == nil || def.Default == nil || def.Default.Value == nil {
		t.Fatal("S1AP-ELEMENTARY-PROCEDURE has no default criticality")
	}

	set := s.ObjectSet("S1AP-ELEMENTARY-PROCEDURES")
	if set == nil {
		t.Fatal("S1AP-ELEMENTARY-PROCEDURES not found")
	}

	objs, err := s.Objects(set.Set)
	if err != nil {
		t.Fatal(err)
	}

	crits := map[string]Criticality{"reject": CriticalityReject, "ignore": CriticalityIgnore, "notify": CriticalityNotify}

	out := make([]elementaryProcedure, 0, len(objs))

	for _, o := range objs {
		cv, _ := o.Value("&procedureCode")

		code, err := s.Int(cv)
		if err != nil {
			t.Fatal(err)
		}

		crit := def.Default.Value.Ref
		if v, ok := o.Value("&criticality"); ok {
			crit = v.Ref
		}

		ep := elementaryProcedure{name: cv.Ref, code: ProcedureCode(code), criticality: crits[crit]}

		for i, f := range []string{"&InitiatingMessage", "&SuccessfulOutcome", "&UnsuccessfulOutcome"} {
			if ty := o.Type(f); ty != nil {
				ep.messages[i] = ty.Name
			}
		}

		out = append(out, ep)
	}

	return out
}

func goIdentifier(asn1Name string) string {
	s := strings.ReplaceAll(strings.TrimPrefix(asn1Name, "id-"), "-", "")
	r := []rune(s)
	r[0] = unicode.ToUpper(r[0])

	return string(r)
}

func TestProtocolIEIDsMatchASN1(t *testing.T) {
	spec := asn1Values(t, "ProtocolIE-ID")

	for id, name := range spec {
		if got, ok := protocolIENames[ProtocolIEID(id)]; !ok || "id-"+got != name {
			t.Errorf("%s = %d: protocolIENames has %q", name, id, got)
		}
	}

	for id, name := range protocolIENames {
		if _, ok := spec[int64(id)]; !ok {
			t.Errorf("protocolIENames[%d] = %q is not a TS 36.413 ProtocolIE-ID", uint16(id), name)
		}
	}
}

func TestProcedureCodesMatchASN1(t *testing.T) {
	spec := asn1Values(t, "ProcedureCode")

	for code, name := range spec {
		if got := procedureNames[ProcedureCode(code)]; got != goIdentifier(name) {
			t.Errorf("%s = %d: procedureNames has %q", name, code, got)
		}
	}

	for code, name := range procedureNames {
		if _, ok := spec[int64(code)]; !ok {
			t.Errorf("procedureNames[%d] = %q is not a TS 36.413 ProcedureCode", uint8(code), name)
		}
	}
}

func TestProcedureInfoMatchesASN1(t *testing.T) {
	eps := elementaryProcedures(t)

	if len(eps) != len(procedureInfos) {
		t.Errorf("S1AP-ELEMENTARY-PROCEDURES has %d procedures, procedureInfos %d", len(eps), len(procedureInfos))
	}

	for _, ep := range eps {
		info, ok := procedureInfos[ep.code]
		if !ok {
			t.Errorf("%s (%d) is missing from procedureInfos", ep.name, ep.code)

			continue
		}

		if info.crit != ep.criticality {
			t.Errorf("%s: criticality %s, spec says %s", ep.name, info.crit, ep.criticality)
		}

		if info.outcome != (ep.messages[2] != "") {
			t.Errorf("%s: unsuccessful outcome %v, spec defines %q", ep.name, info.outcome, ep.messages[2])
		}
	}
}

func TestMessagesMatchElementaryProcedures(t *testing.T) {
	type slot struct {
		ep     elementaryProcedure
		choice int
	}

	bySpecName := map[string]slot{}

	for _, ep := range elementaryProcedures(t) {
		for i, m := range ep.messages {
			if m != "" {
				bySpecName[m] = slot{ep, i}
			}
		}
	}

	for _, c := range messageCases() {
		t.Run(c.asn1, func(t *testing.T) {
			want, ok := bySpecName[c.asn1]
			if !ok {
				t.Fatalf("%s is not a message of any S1AP-ELEMENTARY-PROCEDURE", c.asn1)
			}

			b, err := sampleMessage(t, c.typ).Interface().(interface{ Marshal() ([]byte, error) }).Marshal()
			if err != nil {
				t.Fatal(err)
			}

			pdu, err := Unmarshal(b)
			if err != nil {
				t.Fatal(err)
			}

			if pdu.choiceIndex() != want.choice || pdu.procedureCode() != want.ep.code || pdu.criticality() != want.ep.criticality {
				t.Errorf("sent as choice %d, %s, %s; spec says choice %d, %s (%d), %s",
					pdu.choiceIndex(), pdu.procedureCode(), pdu.criticality(), want.choice, want.ep.name, want.ep.code, want.ep.criticality)
			}
		})
	}
}

func TestTypedCodecsCoverEveryMessage(t *testing.T) {
	keys := map[string]typedKey{}

	for _, ep := range elementaryProcedures(t) {
		for i, m := range ep.messages {
			if m != "" {
				keys[m] = typedKey{TriggeringMessage(i), ep.code}
			}
		}
	}

	covered := map[typedKey]bool{}

	for _, c := range messageCases() {
		key, ok := keys[c.asn1]
		if !ok {
			t.Errorf("%s is not a message of any elementary procedure", c.asn1)

			continue
		}

		covered[key] = true

		codec, ok := typedCodecs[key]
		if !ok {
			t.Errorf("%s: typedCodecs has no entry for %v", c.asn1, key)

			continue
		}

		b, err := sampleMessage(t, c.typ).Interface().(typedMessage).Marshal()
		if err != nil {
			t.Fatal(err)
		}

		pdu, err := Unmarshal(b)
		if err != nil {
			t.Fatal(err)
		}

		msg, err := codec(pdu.value())
		if err != nil {
			t.Errorf("%s: %v", c.asn1, err)

			continue
		}

		if got := reflect.TypeOf(msg); got != reflect.PointerTo(c.typ) {
			t.Errorf("%s: typedCodecs[%v] returns %v", c.asn1, key, got)
		}
	}

	for key := range typedCodecs {
		if !covered[key] {
			t.Errorf("typedCodecs[%v] has no message case", key)
		}
	}
}
