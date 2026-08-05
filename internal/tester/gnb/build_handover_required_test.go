// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package gnb

import (
	"testing"

	"github.com/ellanetworks/core/ngap"
)

// The simulator's HANDOVER REQUIRED must decode with the same library the AMF
// parses it with.
func TestBuildHandoverRequiredRoundTrips(t *testing.T) {
	b, err := BuildHandoverRequired(&HandoverRequiredOpts{
		AMFUENGAPID: 1, RANUENGAPID: 2,
		HandoverType: ngap.HandoverTypeIntra5GS,
		TargetMcc:    "001", TargetMnc: "01", TargetGnbID: "000102", TargetTac: "000001",
		PDUSessions: []HandoverRequiredPDUSession{{PDUSessionID: 5}},
	})
	if err != nil {
		t.Fatal(err)
	}

	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		t.Fatal(err)
	}

	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcHandoverPreparation {
		t.Fatalf("decoded %T", pdu)
	}

	out, err := ngap.ParseHandoverRequired(im.Value)
	if err != nil {
		t.Fatal(err)
	}

	if out.AMFUENGAPID != 1 || out.RANUENGAPID != 2 ||
		len(out.PDUSessionResourceListHORqd) != 1 || out.PDUSessionResourceListHORqd[0].PDUSessionID != 5 ||
		out.TargetID.TargetRANNodeID.SelectedTAI.TAC != 1 {
		t.Fatalf("round trip %+v", out)
	}

	if _, err := ngap.ParseHandoverRequiredTransfer(out.PDUSessionResourceListHORqd[0].Transfer); err != nil {
		t.Fatalf("nested transfer: %v", err)
	}
}
