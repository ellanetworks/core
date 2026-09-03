// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package fgs

import (
	"testing"

	naslib "github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// The deregistration request pair shares a message type between its UE- and
// network-originated variants (TS 24.501 table 8.2.1), so each is decoded end to
// end rather than trusted to the dispatch table.
func TestDecodeGmmMisc(t *testing.T) {
	for _, c := range []struct {
		name string
		msg  interface{ MarshalBinary() ([]byte, error) }
		want func(*testing.T, *GmmMessage)
	}{
		{
			name: "ConfigurationUpdateCommand",
			msg: &fgs.ConfigurationUpdateCommand{
				ConfigurationUpdateIndication: &fgs.ConfigurationUpdateIndication{ACK: true},
				FullNameForNetwork:            &naslib.NetworkName{Name: "Ella Networks"},
			},
			want: func(t *testing.T, m *GmmMessage) {
				c := m.ConfigurationUpdateCommand
				if c == nil || c.FullNameForNetwork == nil || c.FullNameForNetwork.Name != "Ella Networks" {
					t.Fatalf("configuration update command = %+v", c)
				}

				if c.ConfigurationUpdateIndication == nil || !c.ConfigurationUpdateIndication.Acknowledgement {
					t.Errorf("configuration update indication = %+v", c.ConfigurationUpdateIndication)
				}
			},
		},
		{
			name: "DeregistrationRequestUETerminated",
			msg: &fgs.DeregistrationRequestUETerminated{
				AccessType: fgs.AccessType3GPP,
				Cause:      func() *fgs.GMMCause { c := fgs.GMMCauseIllegalUE; return &c }(),
			},
			want: func(t *testing.T, m *GmmMessage) {
				d := m.DeregistrationRequestUETerminated
				if d == nil || d.Cause == nil || d.Cause.Label == "" {
					t.Fatalf("deregistration request (network) = %+v", d)
				}
			},
		},
		{
			name: "GMMStatus",
			msg:  &fgs.GMMStatus{Cause: fgs.GMMCauseIllegalUE},
			want: func(t *testing.T, m *GmmMessage) {
				if m.GMMStatus == nil || m.GMMStatus.Cause.Label == "" {
					t.Errorf("gmm status = %+v", m.GMMStatus)
				}
			},
		},
		{
			name: "SecurityModeReject",
			msg:  &fgs.SecurityModeReject{Cause: fgs.GMMCauseUESecurityCapabilitiesMismatch},
			want: func(t *testing.T, m *GmmMessage) {
				if m.SecurityModeReject == nil {
					t.Errorf("security mode reject = %+v", m.SecurityModeReject)
				}
			},
		},
		{
			name: "ConfigurationUpdateComplete",
			msg:  &fgs.ConfigurationUpdateComplete{},
			want: func(t *testing.T, m *GmmMessage) {
				if m.ConfigurationUpdateComplete == nil {
					t.Errorf("configuration update complete = %+v", m.ConfigurationUpdateComplete)
				}
			},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			b, err := c.msg.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}

			msg := DecodeNASMessage(b)
			if msg.GmmMessage == nil {
				t.Fatalf("no 5GMM message decoded: %+v", msg)
			}

			c.want(t, msg.GmmMessage)
		})
	}
}
