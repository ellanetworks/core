// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestSyncPDUSessionStatus_ReleasesOnTheAMFSideToo(t *testing.T) {
	ue, _, smf, amfInstance := buildMobilityRegUeAndAMF(t)

	snssai := models.Snssai{Sst: 1, Sd: "010203"}
	if err := ue.CreateSmContext(3, "ref-3", &snssai, "internet"); err != nil {
		t.Fatalf("create sm context: %v", err)
	}

	ue.SetEPSBearerIdentity(3, 6)

	var reported [16]bool

	held, err := syncPDUSessionStatus(t.Context(), amfInstance, ue, &fgs.RegistrationRequest{
		PDUSessionStatus: &fgs.PSIBitmap{PSI: reported},
	})
	if err != nil {
		t.Fatalf("syncPDUSessionStatus: %v", err)
	}

	if held[3] {
		t.Error("the registration accept must report PDU session 3 inactive")
	}

	if len(smf.ReleasedSmContext) != 1 || smf.ReleasedSmContext[0] != "ref-3" {
		t.Fatalf("released %v in the SMF, want [ref-3]", smf.ReleasedSmContext)
	}

	if _, ok := ue.SmContextFindByPDUSessionID(3); ok {
		t.Error("TS 24.501 §5.5.1.3.4: the AMF must perform a local release on the AMF side, not only ask the SMF")
	}

	if _, ok := ue.EPSBearerIdentities()[3]; ok {
		t.Error("the released session must not keep its EPS bearer identity reserved")
	}

	if got := ue.AllTransferableEPSSessions(); len(got) != 0 {
		t.Errorf("AllTransferableEPSSessions = %+v, want none: a released session must not move to EPS", got)
	}
}
