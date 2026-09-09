// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/internal/models"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestUeConnLogCarriesUeAssociationIDs(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	ueConn := &UeConn{AmfUeNgapID: 7, RanUeNgapID: models.RanUeNgapIDUnspecified}
	ueConn.bindLog(zap.New(core))
	ueConn.Log().Info("handover target prepared")

	ueConn.RanUeNgapID = 42
	ueConn.refreshLog()
	ueConn.Log().Info("handover target assigned")

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}

	prepared := entries[0].ContextMap()
	if prepared["amf_ue_ngap_id"] != int64(7) {
		t.Errorf("expected amf_ue_ngap_id 7, got %v", prepared["amf_ue_ngap_id"])
	}

	if _, ok := prepared["ran_ue_ngap_id"]; ok {
		t.Errorf("expected ran_ue_ngap_id to be omitted while unspecified, got %v", prepared["ran_ue_ngap_id"])
	}

	assigned := entries[1].ContextMap()
	if assigned["amf_ue_ngap_id"] != int64(7) {
		t.Errorf("expected amf_ue_ngap_id 7, got %v", assigned["amf_ue_ngap_id"])
	}

	if assigned["ran_ue_ngap_id"] != int64(42) {
		t.Errorf("expected ran_ue_ngap_id 42, got %v", assigned["ran_ue_ngap_id"])
	}
}

func TestUeConnLogWithoutBaseKeepsExplicitLogger(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	ueConn := &UeConn{AmfUeNgapID: 1, RanUeNgapID: 2}
	ueConn.setLog(zap.New(core))
	ueConn.refreshLog()
	ueConn.Log().Info("still mine")

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}

	if got := logs.All()[0].ContextMap(); len(got) != 0 {
		t.Errorf("expected no injected fields, got %v", got)
	}
}
