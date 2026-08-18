// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import "testing"

// An unset class must be treated as session management, so a caller that omits it keeps the
// PDU session delivery path rather than silently falling through to standalone delivery.
func TestN1N2MessageTransferRequest_Standalone(t *testing.T) {
	for _, tc := range []struct {
		name string
		req  N1N2MessageTransferRequest
		want bool
	}{
		{"zero value", N1N2MessageTransferRequest{}, false},
		{"explicit SM N1", N1N2MessageTransferRequest{N1Class: N1ClassSM}, false},
		{"explicit SM N2", N1N2MessageTransferRequest{N2Class: N2ClassSM}, false},
		{"explicit SM both", N1N2MessageTransferRequest{N1Class: N1ClassSM, N2Class: N2ClassSM}, false},
		{"LPP", N1N2MessageTransferRequest{N1Class: N1ClassLPP}, true},
		{"NRPPa", N1N2MessageTransferRequest{N2Class: N2ClassNRPPa}, true},
		{"unknown N1 class", N1N2MessageTransferRequest{N1Class: "SMS"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.req.Standalone(); got != tc.want {
				t.Errorf("Standalone() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestN1N2MessageTransferRequest_HasClass(t *testing.T) {
	lpp := N1N2MessageTransferRequest{N1Class: N1ClassLPP}
	nrppa := N1N2MessageTransferRequest{N2Class: N2ClassNRPPa}
	sm := N1N2MessageTransferRequest{N1Class: N1ClassSM, N2Class: N2ClassSM}

	if !lpp.HasClass(N1ClassLPP, N2ClassNRPPa) {
		t.Error("an LPP request must match a positioning cancel")
	}

	if !nrppa.HasClass(N1ClassLPP, N2ClassNRPPa) {
		t.Error("an NRPPa request must match a positioning cancel")
	}

	if sm.HasClass(N1ClassLPP, N2ClassNRPPa) {
		t.Error("an SM request must not match a positioning cancel")
	}

	// An empty class must never match, otherwise a cancel naming only one side would also
	// discard requests that leave the other side unset.
	if sm.HasClass("", "") {
		t.Error("empty classes must not match")
	}

	if lpp.HasClass("", N2ClassNRPPa) {
		t.Error("an LPP request must not match an NRPPa-only cancel")
	}
}
