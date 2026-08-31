// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func TestBuildAMFStatusIndication(t *testing.T) {
	guami := &models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "cafe00"}

	b, err := BuildAMFStatusIndication(guami)
	if err != nil {
		t.Fatalf("BuildAMFStatusIndication: %v", err)
	}

	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	im, ok := pdu.(*ngap.InitiatingMessage)
	if !ok || im.ProcedureCode != ngap.ProcAMFStatusIndication {
		t.Fatalf("got %T procedureCode %d", pdu, im.ProcedureCode)
	}

	msg, err := ngap.ParseAMFStatusIndication(im.Value)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	if len(msg.UnavailableGUAMIList) != 1 {
		t.Fatalf("UnavailableGUAMIList = %+v, want one item", msg.UnavailableGUAMIList)
	}

	got := msg.UnavailableGUAMIList[0].GUAMI
	if got.AMFRegionID != 0xca || got.AMFSetID != 0x3f8 || got.AMFPointer != 0 {
		t.Errorf("GUAMI = %+v, want region ca / set 3f8 / pointer 0", got)
	}

	if got.PLMNIdentity != (ngap.PLMNIdentity{0x00, 0xf1, 0x10}) {
		t.Errorf("PLMNIdentity = %v", got.PLMNIdentity)
	}
}

func TestBuildAMFStatusIndicationRejectsUnusableGUAMI(t *testing.T) {
	for _, tc := range []struct {
		name  string
		guami *models.Guami
	}{
		{"nil", nil},
		{"no PLMN", &models.Guami{AmfID: "cafe00"}},
		{"malformed AMF id", &models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "zz"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := BuildAMFStatusIndication(tc.guami); err == nil {
				t.Fatal("BuildAMFStatusIndication() = nil error, want a failure")
			}
		})
	}
}
