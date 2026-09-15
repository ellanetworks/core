// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"testing"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
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
	ueConn := &UeConn{AmfUeNgapID: 7}
	ueConn.setRanUeNgapID(models.RanUeNgapIDUnspecified)
	ueConn.bindLogFields([]zap.Field{logger.RanAddr("10.0.0.1")})

	prepared := ueConn.LogFields()

	if f, ok := fieldByKey(prepared, "amf_ue_ngap_id"); !ok || f.Integer != 7 {
		t.Errorf("expected amf_ue_ngap_id 7, got %v", prepared)
	}

	if _, ok := fieldByKey(prepared, "ran_ue_ngap_id"); ok {
		t.Errorf("expected ran_ue_ngap_id to be omitted while unspecified, got %v", prepared)
	}

	if f, ok := fieldByKey(prepared, "ran_addr"); !ok || f.String != "10.0.0.1" {
		t.Errorf("expected the gNB fields to be carried through, got %v", prepared)
	}

	ueConn.setRanUeNgapID(42)
	ueConn.refreshLog()

	assigned := ueConn.LogFields()

	if f, ok := fieldByKey(assigned, "amf_ue_ngap_id"); !ok || f.Integer != 7 {
		t.Errorf("expected amf_ue_ngap_id 7, got %v", assigned)
	}

	if f, ok := fieldByKey(assigned, "ran_ue_ngap_id"); !ok || f.Integer != 42 {
		t.Errorf("expected ran_ue_ngap_id 42, got %v", assigned)
	}
}

func TestUeConnWithoutABoundRadioCarriesNoFields(t *testing.T) {
	ueConn := &UeConn{AmfUeNgapID: 1}
	ueConn.setRanUeNgapID(2)
	ueConn.refreshLog()

	if got := ueConn.LogFields(); len(got) != 0 {
		t.Errorf("expected no fields before the radio is bound, got %v", got)
	}
}
