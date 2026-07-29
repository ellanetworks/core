// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"testing"

	"github.com/ellanetworks/core/nas"
)

// TestESMHeaderAccessors checks that every ESM message reports the header
// TS 24.007 §11.2.3.1.1 gives it, so a receiver reads the EPS bearer and the
// transaction without a type switch and without indexing octets.
func TestESMHeaderAccessors(t *testing.T) {
	const (
		bearer = EPSBearerIdentity(5)
		pti    = nas.ProcedureTransactionIdentity(9)
	)

	msgs := []ESMMessage{
		&ActivateDefaultEPSBearerContextRequest{EPSBearerIdentity: bearer, PTI: pti},
		&ActivateDefaultEPSBearerContextAccept{EPSBearerIdentity: bearer, PTI: pti},
		&ActivateDefaultEPSBearerContextReject{EPSBearerIdentity: bearer, PTI: pti},
		&BearerResourceAllocationRequest{EPSBearerIdentity: bearer, PTI: pti},
		&BearerResourceAllocationReject{EPSBearerIdentity: bearer, PTI: pti},
		&BearerResourceModificationRequest{EPSBearerIdentity: bearer, PTI: pti},
		&BearerResourceModificationReject{EPSBearerIdentity: bearer, PTI: pti},
		&DeactivateEPSBearerContextRequest{EPSBearerIdentity: bearer, PTI: pti},
		&DeactivateEPSBearerContextAccept{EPSBearerIdentity: bearer, PTI: pti},
		&ESMInformationRequest{EPSBearerIdentity: bearer, PTI: pti},
		&ESMInformationResponse{EPSBearerIdentity: bearer, PTI: pti},
		&ModifyEPSBearerContextRequest{EPSBearerIdentity: bearer, PTI: pti},
		&ModifyEPSBearerContextAccept{EPSBearerIdentity: bearer, PTI: pti},
		&ModifyEPSBearerContextReject{EPSBearerIdentity: bearer, PTI: pti},
		&PDNConnectivityRequest{EPSBearerIdentity: bearer, PTI: pti},
		&PDNConnectivityReject{EPSBearerIdentity: bearer, PTI: pti},
		&PDNDisconnectRequest{EPSBearerIdentity: bearer, PTI: pti},
		&PDNDisconnectReject{EPSBearerIdentity: bearer, PTI: pti},
		&ESMStatus{EPSBearerIdentity: bearer, PTI: pti},
	}

	if len(msgs) != len(esmParsers) {
		t.Errorf("%d messages listed, %d in the dispatch table: a new ESM message needs a case here", len(msgs), len(esmParsers))
	}

	for _, m := range msgs {
		if m.BearerIdentity() != bearer {
			t.Errorf("%s: BearerIdentity = %d, want %d", m.MessageType(), m.BearerIdentity(), bearer)
		}

		if m.TransactionIdentity() != pti {
			t.Errorf("%s: TransactionIdentity = %d, want %d", m.MessageType(), m.TransactionIdentity(), pti)
		}
	}
}

// TestESMHeaderSurvivesDecode checks the accessors read what the wire carried.
func TestESMHeaderSurvivesDecode(t *testing.T) {
	in := &ESMStatus{EPSBearerIdentity: 7, PTI: 3, Cause: ESMCauseInvalidPTIValue}

	b, err := in.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}

	msg, err := ParseMessage(b, nas.DirectionUplink)
	if err != nil {
		t.Fatal(err)
	}

	esm, ok := msg.(ESMMessage)
	if !ok {
		t.Fatalf("%T is not an ESMMessage", msg)
	}

	if esm.BearerIdentity() != 7 || esm.TransactionIdentity() != 3 {
		t.Errorf("header = bearer %d, transaction %d, want 7 / 3", esm.BearerIdentity(), esm.TransactionIdentity())
	}
}
