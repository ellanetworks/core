// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
	"github.com/ellanetworks/core/nas/nastest"
)

// TS 24.501 §7
func TestDecodeCompliance_Section7(t *testing.T) {
	smContainer := []byte{0x2e, 0x01, 0x00, 0xc1, 0xff, 0x00}

	tests := []struct {
		name   string
		build  func() []byte
		action nasreply.Action
		domain nasreply.Domain
		cause  uint8
	}{
		{
			name:   "unknown message type -> 5GMM STATUS #97 (7.4)",
			build:  func() []byte { return nastest.BuildGMMRaw(uint8(fgs.EPD5GMM), uint8(fgs.SHTPlain), 0xff).Bytes() },
			action: nasreply.ActionStatus, domain: nasreply.DomainMM, cause: nasreply.CauseMessageTypeNotImplemented,
		},
		{
			name: "defined downlink type on uplink, plain -> silent (4.4.4.3)",
			build: func() []byte {
				return nastest.BuildGMMRaw(uint8(fgs.EPD5GMM), uint8(fgs.SHTPlain), uint8(fgs.MsgRegistrationAccept)).Bytes()
			},
			action: nasreply.ActionSilent,
		},
		{
			name:   "truncated REGISTRATION REQUEST -> 5GMM STATUS #96 (7.5.1)",
			build:  func() []byte { return nastest.BuildGMM(fgs.MsgRegistrationRequest).Bytes() },
			action: nasreply.ActionStatus, domain: nasreply.DomainMM, cause: nasreply.CauseInvalidMandatoryInfo,
		},
		{
			name:   "well-formed but non-whitelisted plain (UL NAS TRANSPORT) -> silent (4.4.4.3)",
			build:  func() []byte { return nastest.BuildGMM(fgs.MsgULNASTransport).U8(0x01).LVE(smContainer).Bytes() },
			action: nasreply.ActionSilent,
		},
		{
			name: "too short to carry a message type -> silent (7.2.1)",
			build: func() []byte {
				return nastest.BuildGMMRaw(uint8(fgs.EPD5GMM), uint8(fgs.SHTPlain), 0).Truncate(2).Bytes()
			},
			action: nasreply.ActionSilent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ue := newDecoderTestUE(t)
			ue.secured = false

			_, err := DecodeNASMessage(ue, tt.build())
			if err == nil {
				t.Fatalf("expected the decoder to reject this message")
			}

			d := DispositionForDecodeError(err)
			if d.Action != tt.action {
				t.Fatalf("Action = %v, want %v (disposition %+v)", d.Action, tt.action, d)
			}

			if tt.action == nasreply.ActionStatus && (d.Domain != tt.domain || d.Cause != tt.cause) {
				t.Fatalf("STATUS domain/cause = %v/#%d, want %v/#%d", d.Domain, d.Cause, tt.domain, tt.cause)
			}
		})
	}
}
