// Copyright 2026 Ella Networks
// SPDX-License-Identifier: Apache-2.0

package nas_test

import (
	"testing"

	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/free5gc/nas"
	"github.com/free5gc/nas/nasMessage"
)

func TestPolicePTI(t *testing.T) {
	never := func(uint8) bool { return false }
	always := func(uint8) bool { return true }

	tests := []struct {
		name        string
		msgType     uint8
		pti         uint8
		ptiInUse    func(uint8) bool
		wantVerdict smfNas.PTIVerdict
		wantCause   uint8
	}{
		{
			name:        "reserved PTI ignored (§7.3.1 d)",
			msgType:     nas.MsgTypePDUSessionEstablishmentRequest,
			pti:         0xff,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIIgnore,
		},
		{
			name:        "reserved PTI on a complete is ignored (§7.3.1 d)",
			msgType:     nas.MsgTypePDUSessionReleaseComplete,
			pti:         0xff,
			ptiInUse:    always,
			wantVerdict: smfNas.PTIIgnore,
		},
		{
			name:        "establishment request with unassigned PTI (§7.3.1 c)",
			msgType:     nas.MsgTypePDUSessionEstablishmentRequest,
			pti:         0x00,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   nasMessage.Cause5GSMInvalidPTIValue,
		},
		{
			name:        "modification request with unassigned PTI (§7.3.1 c)",
			msgType:     nas.MsgTypePDUSessionModificationRequest,
			pti:         0x00,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   nasMessage.Cause5GSMInvalidPTIValue,
		},
		{
			name:        "release request with unassigned PTI (§7.3.1 c)",
			msgType:     nas.MsgTypePDUSessionReleaseRequest,
			pti:         0x00,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   nasMessage.Cause5GSMInvalidPTIValue,
		},
		{
			name:        "establishment request with assigned PTI is processed",
			msgType:     nas.MsgTypePDUSessionEstablishmentRequest,
			pti:         0x01,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIProcess,
		},
		{
			name:        "release complete with PTI not in use (§7.3.1 a)",
			msgType:     nas.MsgTypePDUSessionReleaseComplete,
			pti:         0x05,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   nasMessage.Cause5GSMPTIMismatch,
		},
		{
			name:        "modification complete with PTI not in use (§7.3.1 a)",
			msgType:     nas.MsgTypePDUSessionModificationComplete,
			pti:         0x00,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   nasMessage.Cause5GSMPTIMismatch,
		},
		{
			name:        "modification command reject with PTI not in use (§7.3.1 a)",
			msgType:     nas.MsgTypePDUSessionModificationCommandReject,
			pti:         0x07,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   nasMessage.Cause5GSMPTIMismatch,
		},
		{
			name:        "modification complete with PTI in use is processed (§7.3.1 a)",
			msgType:     nas.MsgTypePDUSessionModificationComplete,
			pti:         0x00,
			ptiInUse:    always,
			wantVerdict: smfNas.PTIProcess,
		},
		{
			name:        "authentication complete with assigned PTI (§7.3.1 b)",
			msgType:     nas.MsgTypePDUSessionAuthenticationComplete,
			pti:         0x03,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   nasMessage.Cause5GSMInvalidPTIValue,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotVerdict, gotCause := smfNas.PolicePTI(tt.msgType, tt.pti, tt.ptiInUse)
			if gotVerdict != tt.wantVerdict {
				t.Errorf("verdict = %d, want %d", gotVerdict, tt.wantVerdict)
			}

			if tt.wantVerdict == smfNas.PTIRespondStatus && gotCause != tt.wantCause {
				t.Errorf("cause = %d, want %d", gotCause, tt.wantCause)
			}
		})
	}
}
