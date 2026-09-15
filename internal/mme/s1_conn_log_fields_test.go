// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package mme

import (
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/per"
	"go.uber.org/zap"
)

func fieldByKey(fields []zap.Field, key string) (zap.Field, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f, true
		}
	}

	return zap.Field{}, false
}

func TestUeConnLogFieldsCarryUeAssociationIDs(t *testing.T) {
	ueConn := &UeConn{MMEUES1APID: 7}
	ueConn.setENBUES1APID(enbUES1APIDUnspecified)
	ueConn.bindLogFields([]zap.Field{logger.RanAddr("10.0.0.1")})

	prepared := ueConn.LogFields()

	if f, ok := fieldByKey(prepared, "mme_ue_s1ap_id"); !ok || f.Integer != 7 {
		t.Errorf("expected mme_ue_s1ap_id 7, got %v", prepared)
	}

	if _, ok := fieldByKey(prepared, "enb_ue_s1ap_id"); ok {
		t.Errorf("expected enb_ue_s1ap_id to be omitted while unspecified, got %v", prepared)
	}

	if f, ok := fieldByKey(prepared, "ran_addr"); !ok || f.String != "10.0.0.1" {
		t.Errorf("expected the eNB fields to be carried through, got %v", prepared)
	}

	ueConn.setENBUES1APID(42)
	ueConn.refreshLog()

	assigned := ueConn.LogFields()

	if f, ok := fieldByKey(assigned, "mme_ue_s1ap_id"); !ok || f.Integer != 7 {
		t.Errorf("expected mme_ue_s1ap_id 7, got %v", assigned)
	}

	if f, ok := fieldByKey(assigned, "enb_ue_s1ap_id"); !ok || f.Integer != 42 {
		t.Errorf("expected enb_ue_s1ap_id 42, got %v", assigned)
	}
}

func TestUeConnWithoutABoundRadioCarriesNoFields(t *testing.T) {
	ueConn := &UeConn{MMEUES1APID: 1}
	ueConn.setENBUES1APID(2)
	ueConn.refreshLog()

	if got := ueConn.LogFields(); len(got) != 0 {
		t.Errorf("expected no fields before the radio is bound, got %v", got)
	}
}

func TestEnbUES1APIDUnspecifiedIsOutOfSpecRange(t *testing.T) {
	if err := enbUES1APIDUnspecified.MarshalPER(per.NewWriter(), per.Aligned); err == nil {
		t.Fatal("expected the unspecified marker to be unencodable as an ENB-UE-S1AP-ID")
	}
}
