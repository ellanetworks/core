// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ellanetworks/core/per/ap"
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

func uplinkNASCase(name string, fields []ieField, t *testing.T, cause *CauseProtocol, ies ...ProtocolIEID) totalCase {
	return totalCase{
		name:  name,
		value: container(t, fields...),
		decode: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) {
			return DecodeUplinkNASTransport(b)
		},
		parse: func(b []byte) error {
			_, err := ParseUplinkNASTransport(b)
			return err
		},
		cause: cause,
		ies:   ies,
	}
}

func TestDecodeIsTotalAndLossless(t *testing.T) {
	reject := CauseProtocolAbstractSyntaxErrorReject
	falsely := CauseProtocolAbstractSyntaxErrorFalselyConstructedMessage
	f := uplinkNASFields()

	unknownIgnore := ieField{ID: 40000, Criticality: CriticalityIgnore, Value: ap.RawValue([]byte{0x01})}
	unknownReject := ieField{ID: 40001, Criticality: CriticalityReject, Value: ap.RawValue([]byte{0x02})}

	item := ERABToBeSetupItemBearerSUReq{ERABID: 1, QoS: goldQoS(), TransportLayerAddress: goldTLA(), GTPTEID: 1, NASPDU: NASPDU{0x01}}
	item.iEExtensions.Append(rawIE{ID: 9999, Criticality: CriticalityReject, Value: []byte{0x5a}})

	nested, err := Unmarshal(marshalT(t, &ERABSetupRequest{MMEUES1APID: 1, ENBUES1APID: 2, ERABToBeSetup: []ERABToBeSetupItemBearerSUReq{item}}))
	if err != nil {
		t.Fatal(err)
	}

	cases := []totalCase{
		uplinkNASCase("unknown ignore IE", []ieField{f[0], unknownIgnore, f[1], f[2], f[3], f[4]}, t, nil),
		uplinkNASCase("unknown reject IE", []ieField{f[0], f[1], f[2], unknownReject, f[3], f[4]}, t, &reject, 40001),
		uplinkNASCase("missing mandatory IE", []ieField{f[0], f[1], f[3], f[4]}, t, &reject, IDNASPDU),
		uplinkNASCase("duplicate IE", []ieField{f[0], f[0], f[1], f[2], f[3], f[4]}, t, &falsely),
		uplinkNASCase("wrong order", []ieField{f[1], f[0], f[2], f[3], f[4]}, t, &falsely),
		{
			name:  "unknown reject extension nested in an item",
			value: nested.value(),
			decode: func(b []byte) (interface{ Marshal() ([]byte, error) }, error) {
				return DecodeERABSetupRequest(b)
			},
			parse: func(b []byte) error {
				_, err := ParseERABSetupRequest(b)
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

func marshalT(t *testing.T, m interface{ Marshal() ([]byte, error) }) []byte {
	t.Helper()

	b, err := m.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return b
}
