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

func TestDeregisterAndRemoveUeContext_KeepsTransferredUeConn(t *testing.T) {
	amfInstance := amf.New(nil, nil, nil)
	radio := &amf.Radio{Log: logger.AmfLog}
	radio.BindAMFForTest(amfInstance)

	ueConn := amf.NewUeConnForTest(radio, models.RanUeNgapIDUnspecified, 500, logger.AmfLog)

	old := addUE(t, amfInstance, "001010000000030", func(u *amf.UeContext) { ueConn.AMFForTest().AttachUeConn(u, ueConn) })

	fresh := amf.NewUeContext()
	ueConn.AMFForTest().AttachUeConn(fresh, ueConn)

	amfInstance.DeregisterAndRemoveUeContext(context.Background(), old)

	if got := amfInstance.FindUEByAmfUeNgapID(radio, 500); got != ueConn {
		t.Fatal("supersede tore down a UeConn already transferred to the fresh context")
	}
}

func TestNewUeContext_HasNoConn(t *testing.T) {
	ue := amf.NewUeContext()

	if ue.Conn() != nil {
		t.Fatal("fresh UeContext should have no connection until a UeConn attaches")
	}
}

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
