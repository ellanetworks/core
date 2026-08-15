// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"net/netip"
	"testing"

	"github.com/ellanetworks/core/ngap"
)

// The simulator's HANDOVER REQUEST ACKNOWLEDGE must decode with the same
// library the AMF parses it with.
func TestBuildHandoverRequestAcknowledgeRoundTrips(t *testing.T) {
	b, err := BuildHandoverRequestAcknowledge(&HandoverRequestAcknowledgeOpts{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		PDUSessions: []HandoverAdmittedPDUSession{{
			PDUSessionID: 5, DLTEID: 0x11223344, DLIP: netip.MustParseAddr("10.0.0.2"),
		}},
	})
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	so, ok := pdu.(*ngap.SuccessfulOutcome)
	if !ok || so.ProcedureCode != ngap.ProcHandoverResourceAllocation {
		t.Fatalf("decoded %T", pdu)
	}

	out, err := ngap.ParseHandoverRequestAcknowledge(so.Value)
	if err != nil {
		t.Fatal(err)
	}

	if len(out.PDUSessionResourceAdmittedList) != 1 || out.PDUSessionResourceAdmittedList[0].PDUSessionID != 5 {
		t.Fatalf("round trip %+v", out)
	}

	transfer, err := ngap.ParseHandoverRequestAcknowledgeTransfer(out.PDUSessionResourceAdmittedList[0].Transfer)
	if err != nil {
		t.Fatal(err)
	}

	if transfer.DLNGUUPTNLInformation.GTPTunnel.GTPTEID != 0x11223344 {
		t.Errorf("nested tunnel = %+v", transfer.DLNGUUPTNLInformation.GTPTunnel)
	}
}
