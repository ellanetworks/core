// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package amf

import (
	"bytes"
	"testing"

	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

func pagingTestGuami() *models.Guami {
	return &models.Guami{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, AmfID: "cafe00"}
}

func pagingTestUE(t *testing.T) *UeContext {
	t.Helper()

	tmsi, err := etsi.NewTMSI(0x0000dead)
	if err != nil {
		t.Fatal(err)
	}

	ue := NewUeContext()
	ue.RegistrationArea = []models.Tai{
		{PlmnID: &models.PlmnID{Mcc: "001", Mnc: "01"}, Tac: "000001"},
	}
	ue.SetTmsiForTest(tmsi)
	ue.SetOldTmsiForTest(etsi.InvalidTMSI)

	return ue
}

func TestBuildPaging(t *testing.T) {
	amfInstance := New(nil, nil, nil)
	ue := pagingTestUE(t)
	ue.RadioCapabilityForPaging = &models.UERadioCapabilityForPaging{NR: []byte{0xaa, 0xbb, 0xcc}}

	paging, err := amfInstance.buildPaging(pagingTestGuami(), ue)
	if err != nil {
		t.Fatalf("BuildPaging: %v", err)
	}

	if paging.FiveGSTMSI == nil {
		t.Fatal("paging carries no 5G-S-TMSI")
	}

	if got, want := paging.FiveGSTMSI.AMFSetID, ngap.AMFSetID(uint16(0xfe)<<2|uint16(0x00)>>6); got != want {
		t.Errorf("AMFSetID = %d, want %d", got, want)
	}

	if got := paging.FiveGSTMSI.FiveGTMSI; got != ngap.FiveGTMSI(0x0000dead) {
		t.Errorf("5G-TMSI = %#x, want %#x", got, 0x0000dead)
	}

	if len(paging.TAIListForPaging) != 1 {
		t.Fatalf("TAI list length = %d, want 1", len(paging.TAIListForPaging))
	}

	if paging.TAIListForPaging[0].TAC != 1 {
		t.Errorf("paging TAI TAC = %d, want 1 (served)", paging.TAIListForPaging[0].TAC)
	}

	if paging.UERadioCapabilityForPaging == nil || paging.UERadioCapabilityForPaging.NR == nil {
		t.Fatal("paging dropped the reported NR paging capability")
	}

	if got := *paging.UERadioCapabilityForPaging.NR; !bytes.Equal(got, []byte{0xaa, 0xbb, 0xcc}) {
		t.Errorf("NR paging capability = %x, want aabbcc", got)
	}

	b, err := paging.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	pdu, err := ngap.Unmarshal(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if im, ok := pdu.(*ngap.InitiatingMessage); !ok || im.ProcedureCode != ngap.ProcPaging {
		t.Fatalf("expected Paging InitiatingMessage, got %T", pdu)
	}
}

// TS 38.413 §9.2.4.1
func TestBuildPagingRejectsEmptyRegistrationArea(t *testing.T) {
	amfInstance := New(nil, nil, nil)
	ue := pagingTestUE(t)
	ue.RegistrationArea = nil

	if _, err := amfInstance.buildPaging(pagingTestGuami(), ue); err == nil {
		t.Fatal("buildPaging() = nil error, want a failure for an empty registration area")
	}
}

func TestBuildPagingOmitsAbsentRadioCapability(t *testing.T) {
	amfInstance := New(nil, nil, nil)
	ue := pagingTestUE(t)
	ue.RadioCapabilityForPaging = nil

	paging, err := amfInstance.buildPaging(pagingTestGuami(), ue)
	if err != nil {
		t.Fatalf("BuildPaging: %v", err)
	}

	if paging.UERadioCapabilityForPaging != nil {
		t.Errorf("UERadioCapabilityForPaging = %+v, want nil", paging.UERadioCapabilityForPaging)
	}
}
