// SPDX-FileCopyrightText: Ella Networks Inc.
//
// SPDX-License-Identifier: BUSL-1.1

package ngap_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/sctp"
)

// supersedeOntoNewConnection registers a UE on a first NG connection, then has it
// re-establish on a second one, superseding the first. It returns the AMF and the old
// connection's AMF/RAN NGAP IDs.
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

	amfInstance.AttachUeConn(amfUe, newConn) // supersede

	return amfInstance, ran, sender, oldAmfID, oldRanID
}

// TestSupersededConnectionIsReleasedTowardGNB is the 5G/NGAP mirror of the S1AP fix for
// issue #1482: when a UE re-establishes on a new NG connection, the AMF must release the
// old (superseded) connection toward the gNB with a UE CONTEXT RELEASE COMMAND
// (TS 23.502 §4.2.3.2, §4.2.6) — not drop it silently, which leaves the gNB's stale
// context dangling and its later release request unanswerable.
func TestSupersededConnectionIsReleasedTowardGNB(t *testing.T) {
	_, _, sender, oldAmfID, oldRanID := supersedeOntoNewConnection(t)

	var released bool

	for _, cmd := range sender.SentUEContextReleaseCommands {
		if cmd.AmfUeNgapID == oldAmfID && cmd.RanUeNgapID == oldRanID {
			released = true
		}
	}

	if !released {
		t.Fatalf("issue #1482: AMF did not send a UE CONTEXT RELEASE COMMAND for the superseded "+
			"context (AMF-UE-NGAP-ID %d); it dropped the old NG connection silently", oldAmfID)
	}
}

// TestSupersededConnectionReleaseRequestNoErrorIndication verifies that a gNB release
// request for the superseded context — one that crossed the AMF's own release command —
// is not answered with an ERROR INDICATION (TS 38.413 §8.3.2.2 requires the release
// procedure, i.e. a UE CONTEXT RELEASE COMMAND).
func TestSupersededConnectionReleaseRequestNoErrorIndication(t *testing.T) {
	amfInstance, ran, sender, oldAmfID, oldRanID := supersedeOntoNewConnection(t)

	before := len(sender.SentErrorIndications)

	ngap.HandleUEContextReleaseRequest(context.Background(), amfInstance, ran, decode.UEContextReleaseRequest{
		AMFUENGAPID: oldAmfID,
		RANUENGAPID: oldRanID,
	})

	if len(sender.SentErrorIndications) != before {
		t.Fatalf("issue #1482: AMF answered the gNB's release request for the superseded context "+
			"(AMF-UE-NGAP-ID %d) with ERROR INDICATION; TS 38.413 §8.3.2.2 requires a UE CONTEXT "+
			"RELEASE COMMAND", oldAmfID)
	}
}
