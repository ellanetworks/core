// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps_test

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/nas/eps"
)

func TestDeactivateEPSBearerContextRequestRoundTrip(t *testing.T) {
	req := &eps.DeactivateEPSBearerContextRequest{
		EPSBearerIdentity:            5,
		ProcedureTransactionIdentity: 0,
		ESMCause:                     eps.ESMCauseReactivationRequested,
	}

	b, err := req.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	// ESM header (EBI|PD, PTI, type) + ESM cause.
	want := []byte{5<<4 | 0x02, 0x00, byte(eps.MsgDeactivateEPSBearerContextRequest), eps.ESMCauseReactivationRequested}
	if !bytes.Equal(b, want) {
		t.Fatalf("encoded = %x, want %x", b, want)
	}

	got, err := eps.ParseDeactivateEPSBearerContextRequest(b)
	if err != nil {
		t.Fatal(err)
	}

	if got.EPSBearerIdentity != 5 || got.ESMCause != eps.ESMCauseReactivationRequested {
		t.Fatalf("decoded = %+v, want EBI 5 cause %d", got, eps.ESMCauseReactivationRequested)
	}
}

func TestDeactivateEPSBearerContextAcceptRoundTrip(t *testing.T) {
	acc := &eps.DeactivateEPSBearerContextAccept{EPSBearerIdentity: 5}

	b, err := acc.Marshal()
	if err != nil {
		t.Fatal(err)
	}

	got, err := eps.ParseDeactivateEPSBearerContextAccept(b)
	if err != nil {
		t.Fatal(err)
	}

	if got.EPSBearerIdentity != 5 {
		t.Fatalf("decoded EBI = %d, want 5", got.EPSBearerIdentity)
	}
}
