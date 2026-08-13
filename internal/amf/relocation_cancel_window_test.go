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

// TS 36.413 §8.4.5.2
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

// The relocation registry admits one handover from EPS per subscriber, so an entry
// that outlives the context naming it locks that subscriber out for good. The MME
// clears the mirror registry from removeContextLocked; the AMF must clear this one
// from its own removal path, which is reached even when the connection has already
// passed to a fresh re-registration.
func TestRemovingAUeContextEndsItsRelocationFromEPS(t *testing.T) {
	a, ue, _, supi := cancelWindowUE(t)

	if !a.beginRelocationFromEPS(supi, 7, ue) {
		t.Fatal("beginRelocationFromEPS refused a fresh relocation")
	}

	a.DeregisterAndRemoveUeContext(context.Background(), ue)

	if !a.beginRelocationFromEPS(supi, 8, NewUeContext()) {
		t.Fatal("the subscriber is still marked as relocating from EPS, so every later handover for it is refused")
	}
}

// A removal must not clear an entry another context holds: a superseded husk is torn
// down after a fresh context has taken over the subscriber.
func TestRemovingASupersededContextKeepsTheLiveRelocation(t *testing.T) {
	a, husk, _, supi := cancelWindowUE(t)

	live := NewUeContext()
	live.SetSupi(supi)

	if !a.beginRelocationFromEPS(supi, 7, live) {
		t.Fatal("beginRelocationFromEPS refused a fresh relocation")
	}

	a.DeregisterAndRemoveUeContext(context.Background(), husk)

	if a.beginRelocationFromEPS(supi, 8, NewUeContext()) {
		t.Fatal("tearing down a superseded context dropped the live relocation another context holds")
	}
}
