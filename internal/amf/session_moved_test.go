// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/sctp"
	"github.com/ellanetworks/core/ngap"
)

// The AMF carries the SMF's release transfer through as an opaque container, so
// its content is immaterial here.
var n2Release = []byte{0x00}

// releaseCountingWriter counts the PDU Session Resource Release Commands written
// to the radio and runs onRelease, if set, as each one is written.
type releaseCountingWriter struct {
	releases  int
	onRelease func()
}

func (w *releaseCountingWriter) WriteMsg(b []byte, _ *sctp.SndRcvInfo) (int, error) {
	if msg, err := ngap.Unmarshal(b); err == nil {
		if im, ok := msg.(*ngap.InitiatingMessage); ok && im.ProcedureCode == ngap.ProcPDUSessionResourceRelease {
			w.releases++

			if w.onRelease != nil {
				w.onRelease()
			}
		}
	}

	return len(b), nil
}

// movedSessionUE returns a registered UE holding one SM context on a radio whose
// NGAP output is counted.
func movedSessionUE(t *testing.T, ref string, upActive bool) (*AMF, *UeContext, etsi.SUPI, *releaseCountingWriter) {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatal(err)
	}

	writer := &releaseCountingWriter{}
	amfInstance := New(nil, nil, nil)
	radio := &Radio{Log: logger.AmfLog, Conn: writer}
	radio.BindAMFForTest(amfInstance)

	ue := NewUeContext()
	ue.SetSupiForTest(supi)
	ue.smf = &deregisterTestSmf{}
	ue.SmContextList[1] = &SmContext{Ref: ref, PduSessionInactive: !upActive}

	if err := amfInstance.CommitUEIdentity(context.Background(), ue, MintAuthProofForRegistrationCommit()); err != nil {
		t.Fatal(err)
	}

	ueConn := NewUeConnForTest(radio, 1, 10, logger.AmfLog)
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)
	ue.ForceStateForTest(Registered)

	return amfInstance, ue, supi, writer
}

// A UE that moved the session back to 5GS holds a fresh context under the same
// PDU session identity. Ref is invariant across a transfer, so only the ref the
// notification names distinguishes the context the move left behind.
func TestSessionTransferredIgnoresAStaleRef(t *testing.T) {
	amfInstance, ue, supi, writer := movedSessionUE(t, "back-on-5gs", true)

	amfInstance.SessionTransferred(context.Background(), supi, 1, "moved-to-eps", n2Release)

	if _, exists := ue.SmContextFindByPDUSessionID(1); !exists {
		t.Error("the notification for the earlier session dropped the current routing context")
	}

	if writer.releases != 0 {
		t.Errorf("PDU Session Resource Release Commands = %d, want 0", writer.releases)
	}
}

func TestSessionReleasedIgnoresAStaleRef(t *testing.T) {
	amfInstance, ue, supi, _ := movedSessionUE(t, "re-established", true)

	amfInstance.SessionReleased(context.Background(), supi, 1, "released-by-the-anchor")

	if _, exists := ue.SmContextFindByPDUSessionID(1); !exists {
		t.Error("the notification for the earlier session dropped the current routing context")
	}
}

// TS 23.502 §4.11.2.2 step 14: the N2 release is skipped when the PDU session's
// user plane is not active. The routing context goes either way.
func TestSessionTransferredReleasesN2ResourcesOnlyWhileTheUserPlaneIsActive(t *testing.T) {
	for _, tc := range []struct {
		name         string
		upActive     bool
		wantReleases int
	}{
		{"user plane active", true, 1},
		{"user plane inactive", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			amfInstance, ue, supi, writer := movedSessionUE(t, "moved-to-eps", tc.upActive)

			amfInstance.SessionTransferred(context.Background(), supi, 1, "moved-to-eps", n2Release)

			if _, exists := ue.SmContextFindByPDUSessionID(1); exists {
				t.Error("routing context for the moved PDU session is still present")
			}

			if writer.releases != tc.wantReleases {
				t.Errorf("PDU Session Resource Release Commands = %d, want %d", writer.releases, tc.wantReleases)
			}
		})
	}
}

// The RAN's release response resolves the session by the AMF's routing context, so
// the context is gone before the release command is sent: a context still present
// there tears down the session the move handed to EPS.
func TestSessionTransferredDropsTheRoutingContextBeforeReleasingNGRANResources(t *testing.T) {
	amfInstance, ue, supi, writer := movedSessionUE(t, "moved-to-eps", true)

	heldAtRelease := true

	writer.onRelease = func() {
		_, heldAtRelease = ue.SmContextFindByPDUSessionID(1)
	}

	amfInstance.SessionTransferred(context.Background(), supi, 1, "moved-to-eps", n2Release)

	if writer.releases != 1 {
		t.Fatalf("PDU Session Resource Release Commands = %d, want 1", writer.releases)
	}

	if heldAtRelease {
		t.Error("routing context still present when the release command reached the radio, want dropped")
	}
}
