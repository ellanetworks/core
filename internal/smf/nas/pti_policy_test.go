// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nas_test

import (
	"testing"

	smfNas "github.com/ellanetworks/core/internal/smf/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

func TestPolicePTI(t *testing.T) {
	never := func(uint8) bool { return false }
	always := func(uint8) bool { return true }

	tests := []struct {
		name        string
		msgType     fgs.GSMMessageType
		pti         uint8
		ptiInUse    func(uint8) bool
		wantVerdict smfNas.PTIVerdict
		wantCause   uint8
	}{
		{
			name:        "reserved PTI ignored (§7.3.1 d)",
			msgType:     fgs.MsgPDUSessionEstablishmentRequest,
			pti:         0xff,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIIgnore,
		},
		{
			name:        "reserved PTI on a complete is ignored (§7.3.1 d)",
			msgType:     fgs.MsgPDUSessionReleaseComplete,
			pti:         0xff,
			ptiInUse:    always,
			wantVerdict: smfNas.PTIIgnore,
		},
		{
			name:        "establishment request with unassigned PTI (§7.3.1 c)",
			msgType:     fgs.MsgPDUSessionEstablishmentRequest,
			pti:         0x00,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   fgs.GSMCauseInvalidPTIValue,
		},
		{
			name:        "modification request with unassigned PTI (§7.3.1 c)",
			msgType:     fgs.MsgPDUSessionModificationRequest,
			pti:         0x00,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   fgs.GSMCauseInvalidPTIValue,
		},
		{
			name:        "release request with unassigned PTI (§7.3.1 c)",
			msgType:     fgs.MsgPDUSessionReleaseRequest,
			pti:         0x00,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   fgs.GSMCauseInvalidPTIValue,
		},
		{
			name:        "establishment request with assigned PTI is processed",
			msgType:     fgs.MsgPDUSessionEstablishmentRequest,
			pti:         0x01,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIProcess,
		},
		{
			name:        "release complete with PTI not in use (§7.3.1 a)",
			msgType:     fgs.MsgPDUSessionReleaseComplete,
			pti:         0x05,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   fgs.GSMCausePTIMismatch,
		},
		{
			name:        "modification complete with PTI not in use (§7.3.1 a)",
			msgType:     fgs.MsgPDUSessionModificationComplete,
			pti:         0x00,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   fgs.GSMCausePTIMismatch,
		},
		{
			name:        "modification command reject with PTI not in use (§7.3.1 a)",
			msgType:     fgs.MsgPDUSessionModificationCmdReject,
			pti:         0x07,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   fgs.GSMCausePTIMismatch,
		},
		{
			name:        "modification complete with PTI in use is processed (§7.3.1 a)",
			msgType:     fgs.MsgPDUSessionModificationComplete,
			pti:         0x00,
			ptiInUse:    always,
			wantVerdict: smfNas.PTIProcess,
		},
		{
			name:        "authentication complete with assigned PTI (§7.3.1 b)",
			msgType:     fgs.MsgPDUSessionAuthenticationComplete,
			pti:         0x03,
			ptiInUse:    never,
			wantVerdict: smfNas.PTIRespondStatus,
			wantCause:   fgs.GSMCauseInvalidPTIValue,
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
