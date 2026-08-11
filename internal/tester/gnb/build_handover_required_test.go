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

func TestBuildHandoverRequiredToENB(t *testing.T) {
	const enbID = uint32(0x00abc)

	target := enbID

	raw, err := BuildHandoverRequired(&HandoverRequiredOpts{
		AMFUENGAPID:  1,
		RANUENGAPID:  1,
		HandoverType: ngap.HandoverTypeFiveGSToEPS,
		TargetMcc:    "001",
		TargetMnc:    "01",
		TargetTac:    "000001",
		TargetENBID:  &target,
		PDUSessions:  []HandoverRequiredPDUSession{{PDUSessionID: 1}},
	})
	if err != nil {
		t.Fatalf("BuildHandoverRequired: %v", err)
	}

	pdu, err := ngap.Unmarshal(raw)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcHandoverPreparation {
		t.Fatalf("decoded %T", pdu)
	}

	msg, err := ngap.ParseHandoverRequired(im.Value)
	if err != nil {
		t.Fatalf("parse Handover Required: %v", err)
	}

	if msg.HandoverType != ngap.HandoverTypeFiveGSToEPS {
		t.Errorf("handover type = %d, want fivegs-to-eps", msg.HandoverType)
	}

	if msg.TargetID.TargeteNBID == nil {
		t.Fatal("Target ID does not name an eNB")
	}

	got := msg.TargetID.TargeteNBID.GlobalENBID.NgENBID
	if got.Kind != ngap.NgENBIDMacro || got.Value != enbID {
		t.Errorf("ng-eNB ID = kind %d value %#x, want a macro %#x", got.Kind, got.Value, enbID)
	}

	if msg.TargetID.TargetRANNodeID != nil {
		t.Error("Target ID names an NG-RAN node as well as an eNB")
	}
}

func TestBuildHandoverRequiredRejectsAnUnnamedTarget(t *testing.T) {
	if _, err := BuildHandoverRequired(&HandoverRequiredOpts{
		AMFUENGAPID: 1, RANUENGAPID: 1,
		TargetMcc: "001", TargetMnc: "01", TargetTac: "000001",
	}); err == nil {
		t.Fatal("a Handover Required naming neither a gNB nor an eNB was built")
	}
}
