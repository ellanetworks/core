// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per/ap/apimpl"
)

type totalCase struct {
	name   string
	value  []byte
	decode func([]byte) (interface{ Marshal() ([]byte, error) }, error)
	parse  func([]byte) error
	cause  *CauseProtocol
	ies    []ProtocolIEID
}

func checkTotal(t *testing.T, tc totalCase) {
	t.Helper()

	m, err := tc.decode(tc.value)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}

	b, err := m.Marshal()
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(pdu.value(), tc.value) {
		t.Fatalf("Marshal(Decode(b))\n got  %x\n want %x", pdu.value(), tc.value)
	}

	err = tc.parse(tc.value)
	if tc.cause == nil {
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}

		return
	}

	var ase *AbstractSyntaxError
	if !errors.As(err, &ase) {
		t.Fatalf("Parse = %v, want *AbstractSyntaxError", err)
	}

	if !ase.Cause.Equal(tc.cause.Cause()) {
		t.Fatalf("cause = %v, want %v", ase.Cause, *tc.cause)
	}

	if len(ase.IEs) != len(tc.ies) {
		t.Fatalf("IEs = %+v, want %v", ase.IEs, tc.ies)
	}

	for i, id := range tc.ies {
		if ase.IEs[i].IEID != id {
			t.Fatalf("IEs = %+v, want %v", ase.IEs, tc.ies)
		}
	}
}

func ngSetupCase(name string, fields []ieField, t *testing.T, cause *CauseProtocol, ies ...ProtocolIEID) totalCase {
	return totalCase{
		name:  name,
		value: container(t, fields...),
		decode: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) {
			return DecodeNGSetupRequest(b)
		},
		parse: func(b []byte) error {
			_, err := ParseNGSetupRequest(b)
			return err
		},
		cause: cause,
		ies:   ies,
	}
}

func TestDecodeIsTotalAndLossless(t *testing.T) {
	reject := CauseProtocolAbstractSyntaxErrorReject
	falsely := CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage
	f := ngSetupFields()

	unknownIgnore := ieField{ID: 40000, Criticality: CriticalityIgnore, Value: apimpl.RawValue([]byte{0x01})}
	unknownReject := ieField{ID: 40001, Criticality: CriticalityReject, Value: apimpl.RawValue([]byte{0x02})}

	item := goldAMFConfigurationUpdateRelay().PLMNSupportList[1]
	item.iEExtensions.Append(rawIE{ID: 9999, Criticality: CriticalityReject, Value: []byte{0x5a}})

	nested, err := Unmarshal(marshalT(t, &AMFConfigurationUpdate{PLMNSupportList: PLMNSupportList{item}}))
	if err != nil {
		t.Fatal(err)
	}

	cases := []totalCase{
		ngSetupCase("unknown ignore IE", []ieField{f[0], unknownIgnore, f[1], f[2], f[3]}, t, nil),
		ngSetupCase("unknown reject IE", []ieField{f[0], f[1], f[2], unknownReject, f[3]}, t, &reject, 40001),
		ngSetupCase("missing mandatory IE", []ieField{f[0], f[1], f[3]}, t, &reject, IDSupportedTAList),
		ngSetupCase("duplicate IE", []ieField{f[0], f[0], f[1], f[2], f[3]}, t, &falsely),
		ngSetupCase("wrong order", []ieField{f[2], f[0], f[1], f[3]}, t, &falsely),
		{
			name:  "unknown reject extension nested in an item",
			value: nested.value(),
			decode: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) {
				return DecodeAMFConfigurationUpdate(b)
			},
			parse: func(b []byte) error {
				_, err := ParseAMFConfigurationUpdate(b)
				return err
			},
			cause: &reject,
			ies:   []ProtocolIEID{9999},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) { checkTotal(t, tc) })
	}
}

func TestRawPDUUEIDAccessorRefusesDuplicate(t *testing.T) {
	id := ieField{ID: IDAMFUENGAPID, Criticality: CriticalityReject, Value: apimpl.RawValue(ieRaw(t, Ptr(AMFUENGAPID(1))))}

	b, err := Marshal(&InitiatingMessage{ProcedureCode: ProcUplinkNASTransport, Criticality: CriticalityIgnore, Value: container(t, id, id)})
	if err != nil {
		t.Fatal(err)
	}

	p, err := ParseRawPDU(b)
	if err != nil {
		t.Fatal(err)
	}

	if _, _, err := p.AMFUENGAPID(); !errors.Is(err, ErrDuplicateIE) {
		t.Fatalf("AMFUENGAPID: %v", err)
	}

	if err := p.SetAMFUENGAPID(2); !errors.Is(err, ErrDuplicateIE) {
		t.Fatalf("SetAMFUENGAPID: %v", err)
	}

	n := 0
	for range p.All(IDAMFUENGAPID) {
		n++
	}

	if n != 2 {
		t.Fatalf("All yielded %d occurrences, want 2", n)
	}
}
