// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import "testing"

// Golden PDU SESSION RESOURCE NOTIFY PDU. free5gc/ngap v1.1.3 and pycrate's
// NGAP module, two independent implementations, encode this identically.
const goldenPDUSessionResourceNotify = "001e4021000004000a000200010055000200020042000500000501aa0043400500000901bb"

func goldPDUSessionResourceNotify() *PDUSessionResourceNotify {
	return &PDUSessionResourceNotify{
		AMFUENGAPID:                1,
		RANUENGAPID:                2,
		PDUSessionResourceNotify:   PDUSessionResourceNotifyList{{PDUSessionID: 5, Transfer: TransferContainer{0xaa}}},
		PDUSessionResourceReleased: PDUSessionResourceReleasedListNot{{PDUSessionID: 9, Transfer: TransferContainer{0xbb}}},
	}
}

func TestPDUSessionResourceNotifyGoldenEncoding(t *testing.T) {
	if got := mustMarshalHex(t, goldPDUSessionResourceNotify()); got != goldenPDUSessionResourceNotify {
		t.Errorf("encoded\n got %s\nwant %s", got, goldenPDUSessionResourceNotify)
	}
}

func TestPDUSessionResourceNotifyRoundTrip(t *testing.T) {
	plmn := PLMNIdentity{0x00, 0xf1, 0x10}
	in := goldPDUSessionResourceNotify()
	in.UserLocationInformation = &UserLocationInformation{
		Kind: UserLocationNR, PLMNIdentity: plmn, CellIdentity: 0x123456789,
		TAI: TAI{PLMNIdentity: plmn, TAC: 1},
	}

	b, err := in.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*InitiatingMessage)
	if !ok || im.ProcedureCode != ProcPDUSessionResourceNotify {
		t.Fatalf("got %T procedureCode %d", pdu, pdu.procedureCode())
	}

	out, err := ParsePDUSessionResourceNotify(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.PDUSessionResourceNotify) != 1 || out.PDUSessionResourceNotify[0].PDUSessionID != 5 {
		t.Fatalf("notify list = %+v", out.PDUSessionResourceNotify)
	}

	if len(out.PDUSessionResourceReleased) != 1 || out.PDUSessionResourceReleased[0].PDUSessionID != 9 {
		t.Fatalf("released list = %+v", out.PDUSessionResourceReleased)
	}

	if out.UserLocationInformation == nil || out.UserLocationInformation.CellIdentity != 0x123456789 {
		t.Fatalf("ULI = %+v", out.UserLocationInformation)
	}
}

// Both lists are optional: the NG-RAN node may report only a location change.
func TestPDUSessionResourceNotifyWithoutLists(t *testing.T) {
	in := &PDUSessionResourceNotify{AMFUENGAPID: 1, RANUENGAPID: 2}

	b, err := in.Marshal()
	if err != nil {
		t.Fatalf("a notify carrying neither list is valid: %v", err)
	}

	pdu, _ := Unmarshal(b)

	out, err := ParsePDUSessionResourceNotify(pdu.(*InitiatingMessage).Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.PDUSessionResourceNotify != nil || out.PDUSessionResourceReleased != nil {
		t.Fatalf("lists = %+v / %+v, want nil", out.PDUSessionResourceNotify, out.PDUSessionResourceReleased)
	}
}
