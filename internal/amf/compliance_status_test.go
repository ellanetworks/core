// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/internal/nasreply"
	"github.com/ellanetworks/core/nas/fgs"
)

// TestDecodeCompliance_Section7 feeds adversarial plain 5GMM messages — built with
// the fgs adversarial builder, no free5gc — through the decoder and asserts the
// disposition matches the latitude TS 24.501 §7 grants the network: 5GMM STATUS #97
// for an unknown/unimplemented message type (§7.4), STATUS #96 for a malformed
// mandatory part of a defined type (§7.5.1), and an audited silence for a too-short
// PDU (§7.2.1) or a well-formed message not permitted plain before secure exchange
// (§4.4.4.3). It also proves the decoder never panics on these inputs.
func TestDecodeCompliance_Section7(t *testing.T) {
	// A valid-enough LV-E payload container for a well-formed UL NAS TRANSPORT.
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
			build:  func() []byte { return fgs.BuildRaw(fgs.EPD5GMM, uint8(fgs.SHTPlain), 0xff).Bytes() },
			action: nasreply.ActionStatus, domain: nasreply.DomainMM, cause: nasreply.CauseMessageTypeNotImplemented,
		},
		{
			// A defined (assigned) downlink type decodes but is not on the plain-allowed
			// whitelist, so it is silently discarded before §7.4 applies — matching the
			// prior free5gc behaviour. (An *unassigned* type takes the STATUS #97 path above.)
			name: "defined downlink type on uplink, plain -> silent (4.4.4.3)",
			build: func() []byte {
				return fgs.BuildRaw(fgs.EPD5GMM, uint8(fgs.SHTPlain), uint8(fgs.MsgRegistrationAccept)).Bytes()
			},
			action: nasreply.ActionSilent,
		},
		{
			name:   "truncated REGISTRATION REQUEST -> 5GMM STATUS #96 (7.5.1)",
			build:  func() []byte { return fgs.Build(fgs.MsgRegistrationRequest).Bytes() },
			action: nasreply.ActionStatus, domain: nasreply.DomainMM, cause: nasreply.CauseInvalidMandatoryInfo,
		},
		{
			name:   "well-formed but non-whitelisted plain (UL NAS TRANSPORT) -> silent (4.4.4.3)",
			build:  func() []byte { return fgs.Build(fgs.MsgULNASTransport).U8(0x01).LVE(smContainer).Bytes() },
			action: nasreply.ActionSilent,
		},
		{
			name:   "too short to carry a message type -> silent (7.2.1)",
			build:  func() []byte { return fgs.BuildRaw(fgs.EPD5GMM, uint8(fgs.SHTPlain), 0).Truncate(2).Bytes() },
			action: nasreply.ActionSilent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ue := newDecoderTestUE(t)
			ue.secured = false // fresh UE: the plain path is taken

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
