// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"testing"

	"github.com/ellanetworks/core/nas/fgs"
)

func TestDecodeGsmSession(t *testing.T) {
	for _, c := range []struct {
		name string
		msg  interface{ MarshalBinary() ([]byte, error) }
		want func(*testing.T, *GsmMessage)
	}{
		{
			name: "PDUSessionEstablishmentReject",
			msg:  &fgs.PDUSessionEstablishmentReject{PDUSessionID: 1, PTI: 1, Cause: fgs.GSMCauseInsufficientResources},
			want: func(t *testing.T, m *GsmMessage) {
				if m.PDUSessionEstablishmentReject == nil || m.PDUSessionEstablishmentReject.Cause5GSM.Label == "" {
					t.Errorf("establishment reject = %+v", m.PDUSessionEstablishmentReject)
				}
			},
		},
		{
			name: "PDUSessionReleaseCommand",
			msg:  &fgs.PDUSessionReleaseCommand{PDUSessionID: 1, PTI: 1, Cause: fgs.GSMCauseRegularDeactivation},
			want: func(t *testing.T, m *GsmMessage) {
				if m.PDUSessionReleaseCommand == nil {
					t.Errorf("release command = %+v", m.PDUSessionReleaseCommand)
				}
			},
		},
		{
			name: "PDUSessionReleaseComplete",
			msg:  &fgs.PDUSessionReleaseComplete{PDUSessionID: 1, PTI: 1},
			want: func(t *testing.T, m *GsmMessage) {
				if m.PDUSessionReleaseComplete == nil {
					t.Fatalf("release complete not decoded")
				}

				if m.PDUSessionReleaseComplete.Cause5GSM != nil {
					t.Errorf("cause rendered for a message that carried none")
				}
			},
		},
		{
			name: "GSMStatus",
			msg:  &fgs.GSMStatus{PDUSessionID: 1, PTI: 1, Cause: fgs.GSMCauseInvalidPDUSessionIdentity},
			want: func(t *testing.T, m *GsmMessage) {
				if m.GSMStatus == nil {
					t.Errorf("gsm status = %+v", m.GSMStatus)
				}
			},
		},
		{
			name: "PDUSessionModificationRequest",
			msg: &fgs.PDUSessionModificationRequest{
				PDUSessionID: 1, PTI: 1,
				GSMCapability: &fgs.GSMCapability{RqoS: true},
			},
			want: func(t *testing.T, m *GsmMessage) {
				r := m.PDUSessionModificationRequest
				if r == nil || r.Capability5GSM == nil || r.Capability5GSM.RqoS != 1 {
					t.Errorf("modification request = %+v", r)
				}
			},
		},
		{
			name: "PDUSessionModificationComplete",
			msg:  &fgs.PDUSessionModificationComplete{PDUSessionID: 1, PTI: 1},
			want: func(t *testing.T, m *GsmMessage) {
				if m.PDUSessionModificationComplete == nil {
					t.Errorf("modification complete = %+v", m.PDUSessionModificationComplete)
				}
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.msg.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			// A 5GSM message reaches the decoder inside a payload container, the
			// way UL/DL NAS TRANSPORT carries it (TS 24.501 §8.2.10).
			gsm := buildGsmMessage(b)
			if gsm == nil {
				t.Fatalf("no 5GSM message decoded")
			}

			if gsm.GsmHeader.PDUSessionID != 1 {
				t.Errorf("PDU session id = %d, want 1", gsm.GsmHeader.PDUSessionID)
			}

			c.want(t, gsm)
		})
	}
}
