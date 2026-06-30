// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
	"github.com/ellanetworks/core/s1ap"
)

func TestIsInitialAttach(t *testing.T) {
	attach := plainAttachNAS(t)

	tests := []struct {
		name string
		nas  []byte
		want bool
	}{
		{"plain attach", attach, true},
		{"integrity-only attach", append([]byte{0x17, 0x00, 0x00, 0x00, 0x00, 0x00}, attach...), true},
		{"ciphered (unpeekable)", append([]byte{0x27, 0x00, 0x00, 0x00, 0x00, 0x00}, attach...), false},
		{"plain EMM STATUS", []byte{0x07, 0x60, 0x00}, false},
		{"non-EMM PD", []byte{0x02, 0x41}, false},
		{"empty", nil, false},
		{"short protected", []byte{0x17, 0x00}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isInitialAttach(tt.nas); got != tt.want {
				t.Fatalf("isInitialAttach = %v, want %v", got, tt.want)
			}
		})
	}
}

func plainAttachNAS(t *testing.T) []byte {
	t.Helper()

	esm, err := (&eps.PDNConnectivityRequest{ProcedureTransactionIdentity: 1, RequestType: 1, PDNType: eps.PDNTypeIPv4}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	nas, err := (&eps.AttachRequest{
		EPSAttachType:       eps.AttachTypeEPS,
		NASKeySetIdentifier: 7,
		EPSMobileIdentity:   eps.EPSMobileIdentity{Type: eps.IdentityIMSI, Digits: testSubscriber.IMSI},
		UENetworkCapability: eps.UENetworkCapability{EEA: 0xf0, EIA: 0x70}.Marshal(),
		ESMMessageContainer: esm,
	}).Marshal()
	if err != nil {
		t.Fatal(err)
	}

	return nas
}

// TestNonAttachInitialUEMessageCreatesNoContext checks that an Initial UE Message
// whose NAS is not an Attach Request binds no UE context and leaves no connection
// behind (its bare connection is released), so an unauthenticated peer cannot
// exhaust contexts (TS 24.301).
// TestHandleParseError_EmitsErrorIndication asserts that an undecodable
// UE-associated S1AP message draws an ERROR INDICATION carrying Criticality
// Diagnostics that name the procedure (TS 36.413 §10.4) instead of being silently
// dropped, mirroring the 5G AMF's fatal-decode response.
func TestHandleParseError_EmitsErrorIndication(t *testing.T) {
	m := newTestMME(t)
	cc := &captureConn{}

	handleUEContextReleaseRequest(m, context.Background(), cc, []byte{0xff, 0xff, 0xff})

	if cc.count() != 1 {
		t.Fatalf("expected 1 Error Indication, got %d", cc.count())
	}

	ind := parseOutboundErrorIndication(t, cc.sent[0])

	if ind.Cause == nil || ind.Cause.Group != s1ap.CauseGroupProtocol {
		t.Errorf("cause = %+v, want protocol group", ind.Cause)
	}

	cd := ind.CriticalityDiagnostics
	if cd == nil {
		t.Fatal("Error Indication carried no Criticality Diagnostics")
	}

	if cd.ProcedureCode == nil || *cd.ProcedureCode != s1ap.ProcUEContextReleaseRequest {
		t.Errorf("diagnostics procedure-code = %v, want %d", cd.ProcedureCode, s1ap.ProcUEContextReleaseRequest)
	}

	if cd.TriggeringMessage == nil || *cd.TriggeringMessage != s1ap.TriggeringInitiatingMessage {
		t.Errorf("diagnostics triggering-message = %v, want initiating", cd.TriggeringMessage)
	}
}

func TestNonAttachInitialUEMessageCreatesNoContext(t *testing.T) {
	m := newTestMME(t)

	// A plain EMM STATUS — a valid EMM message that is not an Attach Request.
	emmStatus := []byte{0x07, 0x60, 0x00}
	for i := 0; i < 100; i++ {
		HandleInitialUEMessage(m, context.Background(), nil, initiatingValue(t, initialUEMessagePDU(t, s1ap.ENBUES1APID(1000+i), emmStatus)))
	}

	if got := m.ConnCountForTest(); got != 0 {
		t.Fatalf("non-Attach Initial UE Messages left %d connections, want 0", got)
	}
}
