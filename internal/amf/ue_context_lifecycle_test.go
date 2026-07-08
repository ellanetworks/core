// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"context"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
)

// TestDeregisterAndRemoveUeContext_KeepsTransferredUeConn verifies that superseding a
// context leaves intact a UeConn that a restart-on-fresh re-registration has since
// transferred to a new context (handleRegistrationRequest reuses the same radio). Only
// a UeConn the superseded context still owns is torn down.
func TestDeregisterAndRemoveUeContext_KeepsTransferredUeConn(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	radio := &amf.Radio{Log: logger.AmfLog}
	radio.BindAMFForTest(amfInstance)

	ueConn := amf.NewUeConnForTest(radio, models.RanUeNgapIDUnspecified, 500, logger.AmfLog)

	// The old context owns the UeConn and is in the pool.
	old := addUE(t, amfInstance, "001010000000030", func(u *amf.UeContext) { ueConn.AMFForTest().AttachUeConn(u, ueConn) })

	// A restart-on-fresh re-registration transfers the shared UeConn to a new context.
	fresh := amf.NewUeContext()
	ueConn.AMFForTest().AttachUeConn(fresh, ueConn)

	amfInstance.DeregisterAndRemoveUeContext(context.Background(), old)

	if got := amfInstance.FindUEByAmfUeNgapID(radio, 500); got != ueConn {
		t.Fatal("supersede tore down a UeConn already transferred to the fresh context")
	}
}

// A freshly-constructed UeContext has NO connection yet — one is bound when a UeConn
// attaches (2-level model).
func TestNewUeContext_HasNoConn(t *testing.T) {
	ue := amf.NewUeContext()

	if ue.Conn() != nil {
		t.Fatal("fresh UeContext should have no connection until a UeConn attaches")
	}
}

// AttachUeConn binds the single connection object, parented to the UeContext; NasConn
// and UeConn return the same object.
func TestUeContext_AttachUeConn_BindsConn(t *testing.T) {
	radio := newTestRadioForUeConn()
	ueConn := amf.NewUeConnForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewUeContext()
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	if ue.Conn() != ueConn {
		t.Errorf("NasConn() = %p, want %p", ue.Conn(), ueConn)
	}

	if ueConn.Parent() != ue {
		t.Errorf("Parent() = %p, want %p", ueConn.Parent(), ue)
	}
}

// Release tears down the connection, clearing it from the UeContext.
func TestUeConn_Release(t *testing.T) {
	radio := newTestRadioForUeConn()
	ueConn := amf.NewUeConnForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewUeContext()
	ueConn.AMFForTest().AttachUeConn(ue, ueConn)

	ueConn.Release()

	if ue.Conn() != nil {
		t.Error("NasConn() still set after Release")
	}
}

// After Release, a subsequent AttachUeConn must restore the connection so handlers
// don't dereference a nil NasConn.
func TestUeContext_AttachUeConn_RestoresNasConnAfterRelease(t *testing.T) {
	radio := newTestRadioForUeConn()
	ranUe1 := amf.NewUeConnForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewUeContext()
	ranUe1.AMFForTest().AttachUeConn(ue, ranUe1)

	conn := ue.Conn()
	if conn == nil {
		t.Fatal("initial NasConn is nil")
	}

	conn.Release()

	if ue.Conn() != nil {
		t.Fatal("NasConn should be nil right after Release")
	}

	ranUe2 := amf.NewUeConnForTest(radio, 2, 20, logger.AmfLog)
	ranUe2.AMFForTest().AttachUeConn(ue, ranUe2)

	if ue.Conn() == nil {
		t.Error("NasConn still nil after re-AttachUeConn")
	}
}

// Reattaching a new UeConn replaces the connection and detaches the old, leaving the
// per-registration context alive.
func TestUeContext_AttachUeConn_ReplacesOld(t *testing.T) {
	radio := newTestRadioForUeConn()
	ranUe1 := amf.NewUeConnForTest(radio, 1, 10, logger.AmfLog)
	ranUe2 := amf.NewUeConnForTest(radio, 2, 20, logger.AmfLog)

	ue := amf.NewUeContext()
	ranUe1.AMFForTest().AttachUeConn(ue, ranUe1)
	ranUe2.AMFForTest().AttachUeConn(ue, ranUe2)

	if ue.Conn() != ranUe2 {
		t.Errorf("NasConn() = %p, want %p", ue.Conn(), ranUe2)
	}

	if ranUe1.Parent() == ue {
		t.Error("old UeConn still parented to ue after replacement")
	}
}
