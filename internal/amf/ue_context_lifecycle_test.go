// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf_test

import (
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/logger"
)

// A freshly-constructed UeContext has a live per-registration context and an
// active NAS connection.
func TestNewUeContext_HasLiveCtxAndConn(t *testing.T) {
	ue := amf.NewUeContext()

	if err := ue.Ctx().Err(); err != nil {
		t.Fatalf("fresh UeContext ctx already cancelled: %v", err)
	}

	if ue.NasConn() == nil {
		t.Fatal("fresh UeContext has no NAS connection")
	}
}

// AttachNasConnection installs a fresh connection whose ctx is a child of the
// per-registration context, parented to the UeContext.
func TestUeContext_AttachNasConnection_ChildCtx(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewUeContext()
	conn := ue.AttachNasConnection(ranUe)

	if conn == nil {
		t.Fatal("AttachNasConnection returned nil")
	}

	if conn.Parent() != ue {
		t.Errorf("conn.Parent() = %p, want %p", conn.Parent(), ue)
	}

	if conn.RanUe() != ranUe {
		t.Errorf("conn.RanUe() = %p, want %p", conn.RanUe(), ranUe)
	}

	if ue.NasConn() != conn {
		t.Errorf("NasConn() = %p, want %p", ue.NasConn(), conn)
	}

	if err := conn.Ctx().Err(); err != nil {
		t.Fatalf("fresh nas connection ctx already cancelled: %v", err)
	}
}

// Release tears down the connection but leaves the per-registration context
// intact, so the UeContext remains usable.
func TestActiveNasConnection_Release(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewUeContext()
	conn := ue.AttachNasConnection(ranUe)

	conn.Release()

	if ue.NasConn() != nil {
		t.Error("NasConn() still set after Release")
	}

	if conn.Ctx().Err() == nil {
		t.Error("connection ctx not cancelled after Release")
	}

	if ue.Ctx().Err() != nil {
		t.Errorf("per-registration ctx cancelled by connection Release: %v", ue.Ctx().Err())
	}
}

// After ActiveNasConnection.Release, a subsequent AttachRanUe must restore
// the NAS connection so handlers don't dereference a nil NasConn.
func TestUeContext_AttachRanUe_RestoresNasConnAfterRelease(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe1 := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewUeContext()
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

// Reattaching cancels the old connection ctx but leaves the per-registration
// context alive.
func TestUeContext_AttachNasConnection_ReplacesOld(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe1 := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)
	ranUe2 := amf.NewRanUeForTest(radio, 2, 20, logger.AmfLog)

	ue := amf.NewUeContext()
	conn1 := ue.AttachNasConnection(ranUe1)
	conn2 := ue.AttachNasConnection(ranUe2)

	if conn1.Ctx().Err() == nil {
		t.Error("first connection ctx not cancelled after second attach")
	}

	if ue.NasConn() != conn2 {
		t.Errorf("NasConn() = %p, want %p", ue.NasConn(), conn2)
	}

	if ue.Ctx().Err() != nil {
		t.Error("per-registration ctx cancelled by reattach")
	}
}

// RotateContext cancels the prior per-registration context (and its NAS
// connection), installs a fresh live context, and resets the security state.
func TestUeContext_RotateContext(t *testing.T) {
	radio := newTestRadioForRanUe()
	ranUe := amf.NewRanUeForTest(radio, 1, 10, logger.AmfLog)

	ue := amf.NewUeContext()
	conn := ue.AttachNasConnection(ranUe)
	ue.SecurityContextAvailable = true

	oldCtx := ue.Ctx()

	ue.RotateContext()

	if oldCtx.Err() == nil {
		t.Error("prior per-registration ctx not cancelled after RotateContext")
	}

	if conn.Ctx().Err() == nil {
		t.Error("NAS connection ctx not cancelled after RotateContext")
	}

	if ue.Ctx().Err() != nil {
		t.Errorf("fresh per-registration ctx cancelled after RotateContext: %v", ue.Ctx().Err())
	}

	if ue.NasConn() != nil {
		t.Error("NAS connection still set after RotateContext")
	}

	if ue.SecurityContextAvailable {
		t.Error("security context not reset after RotateContext")
	}
}
