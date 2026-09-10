// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/per"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestUeConnLogCarriesUeAssociationIDs(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	ueConn := &UeConn{MMEUES1APID: 7, ENBUES1APID: enbUES1APIDUnspecified}
	ueConn.bindLog(zap.New(core))
	ueConn.Log().Info("handover target prepared")

	ueConn.ENBUES1APID = 42
	ueConn.refreshLog()
	ueConn.Log().Info("handover target assigned")

	entries := logs.All()
	if len(entries) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(entries))
	}

	prepared := entries[0].ContextMap()
	if prepared["mme_ue_s1ap_id"] != uint32(7) {
		t.Errorf("expected mme_ue_s1ap_id 7, got %v", prepared["mme_ue_s1ap_id"])
	}

	if _, ok := prepared["enb_ue_s1ap_id"]; ok {
		t.Errorf("expected enb_ue_s1ap_id to be omitted while unspecified, got %v", prepared["enb_ue_s1ap_id"])
	}

	assigned := entries[1].ContextMap()
	if assigned["mme_ue_s1ap_id"] != uint32(7) {
		t.Errorf("expected mme_ue_s1ap_id 7, got %v", assigned["mme_ue_s1ap_id"])
	}

	if assigned["enb_ue_s1ap_id"] != uint32(42) {
		t.Errorf("expected enb_ue_s1ap_id 42, got %v", assigned["enb_ue_s1ap_id"])
	}
}

func TestUeConnLogWithoutBaseKeepsExplicitLogger(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	ueConn := &UeConn{MMEUES1APID: 1, ENBUES1APID: 2}
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

func TestEnbUES1APIDUnspecifiedIsOutOfSpecRange(t *testing.T) {
	if err := enbUES1APIDUnspecified.MarshalPER(per.NewWriter(), per.Aligned); err == nil {
		t.Fatal("expected the unspecified marker to be unencodable as an ENB-UE-S1AP-ID")
	}
}
