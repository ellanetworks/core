// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/interworking"
	"github.com/ellanetworks/core/internal/mme"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

type refusingFiveGSPeer struct {
	err error
}

func (*refusingFiveGSPeer) ForwardRelocation(context.Context, interworking.FiveGSRelocationRequest) (interworking.FiveGSRelocationResponse, error) {
	return interworking.FiveGSRelocationResponse{}, nil
}

func (*refusingFiveGSPeer) RelocationCancel(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (*refusingFiveGSPeer) RelocationComplete(context.Context, etsi.SUPI, interworking.RelocationID) error {
	return nil
}

func (p *refusingFiveGSPeer) EPSContext(context.Context, interworking.EPSContextRequest) (interworking.EPSContextResponse, error) {
	return interworking.EPSContextResponse{}, p.err
}

func (*refusingFiveGSPeer) EPSContextAck(context.Context, etsi.SUPI, []uint8) error { return nil }

// TS 24.301 §5.5.3.2.5
func TestInterSystemTAUWithNoRecoverableContextIsRejected(t *testing.T) {
	for _, tc := range []struct {
		name        string
		err         error
		noPeer      bool
		unprotected bool
	}{
		{name: "the 5GS peer holds no context", err: interworking.ErrUnknownUEContext},
		{name: "the 5GS peer could not verify the update", err: interworking.ErrIntegrityCheckFailed},
		{name: "the update carries no integrity protection", err: interworking.ErrIntegrityCheckFailed, unprotected: true},
		{name: "the operator configured no 5GS peer", noPeer: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newTestMME(t)

			if !tc.noPeer {
				m.FiveGS = &refusingFiveGSPeer{err: tc.err}
			}

			native := eps.GUTITypeNative

			tau, err := (&eps.TrackingAreaUpdateRequest{
				OldGUTI: eps.GUTIIdentity(eps.GUTI{
					PLMN: nas.PLMN{MCC: "001", MNC: "01"}, MMEGroupID: 0x8100, MMECode: 0x40,
					TMSI: [4]byte{0x00, 0x00, 0xde, 0xad},
				}),
				OldGUTIType: &native,
				UEStatus:    &eps.UEStatus{N1ModeReg: true},
			}).MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			wire := append([]byte{0x17, 0xde, 0xad, 0xbe, 0xef, 0x00}, tau...)
			if tc.unprotected {
				wire = tau
			}

			plmnID := s1ap.PLMNIdentity{0x00, 0xf1, 0x10}

			initialUE := &s1ap.InitialUEMessage{
				ENBUES1APID:           1001,
				NASPDU:                s1ap.NASPDU(wire),
				TAI:                   s1ap.TAI{PLMNIdentity: plmnID, TAC: 1},
				EUTRANCGI:             s1ap.Ptr(s1ap.EUTRANCGI{PLMNIdentity: plmnID, CellID: 1}),
				RRCEstablishmentCause: s1ap.Ptr(s1ap.RRCCauseMOSignalling),
			}

			im, err := initialUE.Marshal()
			if err != nil {
				t.Fatal(err)
			}

			conn := &captureConn{}
			HandleInitialUEMessage(m, context.Background(), mme.NewRadioForTest(conn), initiatingValue(t, im))

			if conn.count() != 2 {
				t.Fatalf("expected a TAU Reject and a UE Context Release Command, got %d messages", conn.count())
			}

			rej, err := eps.ParseTrackingAreaUpdateReject(decodeDownlinkNAS(t, conn.sent[0]))
			if err != nil {
				t.Fatalf("parse TAU Reject: %v", err)
			}

			if rej.Cause != eps.EMMCauseUEIdentityCannotBeDerived {
				t.Fatalf("TAU Reject cause = %d, want #%d", rej.Cause, eps.EMMCauseUEIdentityCannotBeDerived)
			}

			// TS 24.301 §5.3.1.2.1 d)
			if cmd := parseUEContextReleaseCommand(t, lastSent(t, conn)); cmd.UES1APIDs.ENBUES1APID != 1001 {
				t.Errorf("release command names eNB-UE-S1AP-ID %d, want the rejected connection's 1001", cmd.UES1APIDs.ENBUES1APID)
			}
		})
	}
}
