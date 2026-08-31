// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

func supersedeOntoNewConnection(t *testing.T) (amfInstance *amf.AMF, ran *amf.Radio, sender *fakeNGAPSender, oldAmfID, oldRanID int64) {
	t.Helper()

	amfInstance = newTestAMF()
	ran = newTestRadio(amfInstance)
	amfInstance.SetRadioForTest(new(sctp.SCTPConn), ran)
	sender = ran.Conn.(*fakeNGAPSender)

	amfUe := amf.NewUeContext()

	oldConn, err := amfInstance.NewUeConn(ran, 1)
	if err != nil {
		t.Fatal(err)
	}

	amfInstance.AttachUeConn(amfUe, oldConn)
	oldAmfID = int64(oldConn.AmfUeNgapID)
	oldRanID = int64(oldConn.RanUeNgapID)

	newConn, err := amfInstance.NewUeConn(ran, 2)
	if err != nil {
		t.Fatal(err)
	}

	amfInstance.AttachUeConn(amfUe, newConn)

	return amfInstance, ran, sender, oldAmfID, oldRanID
}

// TS 23.502 §4.2.3.2
func TestSupersededConnectionIsReleasedTowardGNB(t *testing.T) {
	_, _, sender, oldAmfID, oldRanID := supersedeOntoNewConnection(t)

	var released bool

	for _, cmd := range sender.SentUEContextReleaseCommands {
		if cmd.UENGAPIDs.AMFUENGAPID == ngap.AMFUENGAPID(oldAmfID) && cmd.UENGAPIDs.RANUENGAPID == ngap.RANUENGAPID(oldRanID) {
			released = true
		}
	}

	if !released {
		t.Fatalf("no UE CONTEXT RELEASE COMMAND sent for the superseded context (AMF-UE-NGAP-ID %d)", oldAmfID)
	}
}

// TS 38.413 §8.3.2.2
func TestSupersededConnectionReleaseRequestNoErrorIndication(t *testing.T) {
	amfInstance, ran, sender, oldAmfID, oldRanID := supersedeOntoNewConnection(t)

	before := len(sender.SentErrorIndications)

	HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, &ngap.UEContextReleaseRequest{
		AMFUENGAPID: ngap.AMFUENGAPID(oldAmfID),
		RANUENGAPID: ngap.RANUENGAPID(oldRanID),
	})

	if len(sender.SentErrorIndications) != before {
		t.Fatalf("release request for the superseded context (AMF-UE-NGAP-ID %d) answered with "+
			"ERROR INDICATION; want UE CONTEXT RELEASE COMMAND (TS 38.413 §8.3.2.2)", oldAmfID)
	}
}
