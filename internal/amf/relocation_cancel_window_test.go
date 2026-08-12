// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/sctp"
	"go.uber.org/zap"
)

func cancelWindowUE(t *testing.T) (*AMF, *UeContext, *Radio, etsi.SUPI) {
	t.Helper()

	supi, err := etsi.NewSUPIFromIMSI("001010000000001")
	if err != nil {
		t.Fatalf("NewSUPIFromIMSI: %v", err)
	}

	ue := NewUeContext()
	ue.SetSupi(supi)

	a := New(nil, nil, nil)
	radio := &Radio{Conn: &sctp.SCTPConn{}, Log: zap.NewNop()}
	radio.BindAMFForTest(a)

	return a, ue, radio, supi
}

// TS 36.413 §8.4.5.2: the EPC shall terminate the ongoing handover preparation
// and release its resources on a HANDOVER CANCEL. Answering "too late" before
// the preparation even exists leaves the AMF building a context the source eNB
// has already been told is gone.
func TestRelocationCancelBeforePreparationIsAccepted(t *testing.T) {
	a, ue, radio, supi := cancelWindowUE(t)

	if !a.beginRelocationFromEPS(supi, 7, ue) {
		t.Fatal("beginRelocationFromEPS refused a fresh relocation")
	}

	if err := a.RelocationCancel(context.Background(), supi, 7); err != nil {
		t.Fatalf("a cancel in the preparation window was refused: %v", err)
	}

	if _, _, ok := a.prepareRelocationFromEPS(context.Background(), ue, radio, nil); ok {
		t.Fatal("the handover was prepared after the cancel was acknowledged: the AMF would send a Handover Request for a cancelled handover")
	}
}

// Only a relocation the AMF actually holds may be cancelled, and only the
// relocation the source names.
func TestRelocationCancelRejectsAnUnknownRelocation(t *testing.T) {
	a, ue, _, supi := cancelWindowUE(t)

	if err := a.RelocationCancel(context.Background(), supi, 7); err == nil {
		t.Fatal("a cancel for a relocation the AMF never held was accepted")
	}

	if !a.beginRelocationFromEPS(supi, 7, ue) {
		t.Fatal("beginRelocationFromEPS refused a fresh relocation")
	}

	if err := a.RelocationCancel(context.Background(), supi, 8); err == nil {
		t.Fatal("a cancel naming another relocation was accepted")
	}
}
