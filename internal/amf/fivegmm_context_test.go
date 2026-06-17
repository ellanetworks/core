// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
)

// NewAmfUe allocates an initial FivegmmContext.
func TestAmfUe_Current_NewUeHasContext(t *testing.T) {
	ue := amf.NewAmfUe()

	if ue.Current() == nil {
		t.Fatal("Current() returned nil for a freshly-constructed AmfUe")
	}
}

// FivegmmContext.Parent returns the owning AmfUe.
func TestFivegmmContext_Parent(t *testing.T) {
	ue := amf.NewAmfUe()
	fc := ue.Current()

	if fc.Parent() != ue {
		t.Errorf("Parent() = %p, want %p", fc.Parent(), ue)
	}
}

// SwapContext replaces the current context and cancels the old one's ctx.
func TestAmfUe_SwapContext_CancelsOldCtx(t *testing.T) {
	ue := amf.NewAmfUe()
	old := ue.Current()

	if err := old.Ctx().Err(); err != nil {
		t.Fatalf("fresh context already cancelled: %v", err)
	}

	fresh := amf.NewFivegmmContextForTest(ue)
	ue.SwapContext(fresh)

	if ue.Current() != fresh {
		t.Errorf("Current() = %p, want %p", ue.Current(), fresh)
	}

	if old.Ctx().Err() == nil {
		t.Error("old context ctx still alive after SwapContext")
	}

	if fresh.Ctx().Err() != nil {
		t.Errorf("fresh context cancelled after install: %v", fresh.Ctx().Err())
	}
}

// SwapContext to nil clears the active context.
func TestAmfUe_SwapContext_Nil(t *testing.T) {
	ue := amf.NewAmfUe()
	old := ue.Current()

	ue.SwapContext(nil)

	if ue.Current() != nil {
		t.Errorf("Current() = %p, want nil", ue.Current())
	}

	if old.Ctx().Err() == nil {
		t.Error("old context ctx still alive after SwapContext(nil)")
	}
}

// AttachNasConnection installs a fresh connection whose ctx is a child
// of the 5GMM context.
func TestAmfUe_AttachNasConnection_ChildCtx(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewAmfUe()
	conn := ue.AttachNasConnection(ranUe)

	if conn == nil {
		t.Fatal("AttachNasConnection returned nil")
	}

	if conn.Parent() != ue.Current() {
		t.Errorf("conn.Parent() = %p, want %p", conn.Parent(), ue.Current())
	}

	if conn.RanUe() != ranUe {
		t.Errorf("conn.RanUe() = %p, want %p", conn.RanUe(), ranUe)
	}

	if ue.Current().ActiveConnection() != conn {
		t.Errorf("ActiveConnection() = %p, want %p", ue.Current().ActiveConnection(), conn)
	}
}

// Cancelling the FivegmmContext ctx (via SwapContext) cancels the
// child ActiveNasConnection ctx.
func TestAttachNasConnection_CtxPropagation(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewAmfUe()
	conn := ue.AttachNasConnection(ranUe)

	if err := conn.Ctx().Err(); err != nil {
		t.Fatalf("fresh nas connection ctx already cancelled: %v", err)
	}

	ue.SwapContext(nil)

	if conn.Ctx().Err() == nil {
		t.Error("nas connection ctx not cancelled after 5GMM context swap")
	}
}

// Release tears down the connection but leaves the 5GMM context intact.
func TestActiveNasConnection_Release(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewAmfUe()
	fc := ue.Current()
	conn := ue.AttachNasConnection(ranUe)

	conn.Release()

	if fc.ActiveConnection() != nil {
		t.Error("ActiveConnection() still set after Release")
	}

	if conn.Ctx().Err() == nil {
		t.Error("connection ctx not cancelled after Release")
	}

	if fc.Ctx().Err() != nil {
		t.Errorf("5GMM context ctx cancelled by connection Release: %v", fc.Ctx().Err())
	}

	if ue.Current() != fc {
		t.Error("5GMM context replaced by connection Release")
	}
}

// After ActiveNasConnection.Release, a subsequent AttachRanUe must restore
// the NAS connection so handlers don't dereference a nil NasConn.
func TestAmfUe_AttachRanUe_RestoresNasConnAfterRelease(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe1 := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewAmfUe()
	ue.AttachRanUe(ranUe1)

	conn := ue.NasConn()
	if conn == nil {
		t.Fatal("initial NasConn is nil")
	}

	conn.Release()

	if ue.NasConn() != nil {
		t.Fatal("NasConn should be nil right after Release")
	}

	ranUe2 := amf.NewRanUeForTest(radio, 2, 20, logger.AmfLog)
	ue.AttachRanUe(ranUe2)

	if ue.NasConn() == nil {
		t.Error("NasConn still nil after re-AttachRanUe")
	}
}

// Reattaching cancels the old connection but does not affect the 5GMM context.
func TestAmfUe_AttachNasConnection_ReplacesOld(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe1 := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)
	ranUe2 := amf.NewRanUeForTest(radio, 2, 20, logger.AmfLog)

	ue := amf.NewAmfUe()
	fc := ue.Current()
	conn1 := ue.AttachNasConnection(ranUe1)
	conn2 := ue.AttachNasConnection(ranUe2)

	if conn1.Ctx().Err() == nil {
		t.Error("first connection ctx not cancelled after second attach")
	}

	if fc.ActiveConnection() != conn2 {
		t.Errorf("ActiveConnection() = %p, want %p", fc.ActiveConnection(), conn2)
	}

	if fc.Ctx().Err() != nil {
		t.Error("5GMM context cancelled by reattach")
	}
}
