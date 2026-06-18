// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"reflect"
	"testing"
)

func TestPagingRoundtrip(t *testing.T) {
	in := &Paging{
		UEIdentityIndexValue: 0x2a9, // 10-bit value (IMSI mod 1024)
		STMSI:                STMSI{MMEC: 0x01, MTMSI: 0xdeadbeef},
		CNDomain:             CNDomainPS,
		TAIList: []TAI{
			{PLMNIdentity: PLMNIdentity{0x00, 0xf1, 0x10}, TAC: TAC(0x0001)},
		},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal pdu: %v", err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcPaging {
		t.Fatalf("expected Paging InitiatingMessage, got %T (proc %v)", pdu, im)
	}

	out, err := ParsePaging(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if out.UEIdentityIndexValue != in.UEIdentityIndexValue {
		t.Fatalf("UE identity index = %#x, want %#x", out.UEIdentityIndexValue, in.UEIdentityIndexValue)
	}

	if out.STMSI != in.STMSI {
		t.Fatalf("S-TMSI = %+v, want %+v", out.STMSI, in.STMSI)
	}

	if out.CNDomain != CNDomainPS {
		t.Fatalf("CN domain = %d, want PS", out.CNDomain)
	}

	if !reflect.DeepEqual(out.TAIList, in.TAIList) {
		t.Fatalf("TAI list = %+v, want %+v", out.TAIList, in.TAIList)
	}
}
