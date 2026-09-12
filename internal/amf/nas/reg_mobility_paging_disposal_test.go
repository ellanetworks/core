// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/models"
)

func TestMobilityReg_PagedRequestDoesNotOutliveTheUEContext(t *testing.T) {
	ue, _, _, amfInstance := buildMobilityRegUeAndAMF(t)
	setTestUESecurityCapability(ue)

	snssai := &models.Snssai{Sst: 1}
	_ = ue.CreateSmContext(5, "ref-5", snssai, "internet")

	ue.Conn().RegistrationRequest.AllowedPDUSessionStatus = nil
	ue.SetPagedRequestForTest(&models.N1N2MessageTransferRequest{
		PduSessionID:            5,
		SNssai:                  snssai,
		BinaryDataN2Information: []byte{0x01},
	})
	ue.PagingAnswered()

	HandleMobilityAndPeriodicRegistrationUpdating(context.TODO(), amfInstance, ue)

	conn := ue.Conn()
	if conn == nil {
		t.Fatal("no connection after the registration")
	}

	conn.Release()

	if state := ue.PagingState(); state != amf.PagingIdle {
		t.Fatalf("paging state = %s after the connection carrying the undelivered request was released, want Idle", state)
	}
}
