// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/util"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/ngap"
)

type SliceOpt struct {
	Sst int32
	Sd  string
}

type NGSetupRequestOpts struct {
	Name   string
	GnbID  string
	ID     int64
	Mcc    string
	Mnc    string
	Tac    string
	Sst    int32
	Sd     string
	Slices []SliceOpt
}

func buildNGSetupRequest(opts *NGSetupRequestOpts) (*ngap.NGSetupRequest, error) {
	if opts.Mcc == "" || opts.Mnc == "" {
		return nil, fmt.Errorf("MCC and MNC are required to build NGSetupRequest")
	}

	plmn, err := util.PLMNToNGAP(models.PlmnID{Mcc: opts.Mcc, Mnc: opts.Mnc})
	if err != nil {
		return nil, fmt.Errorf("could not encode PLMN: %w", err)
	}

	gnbID, err := strconv.ParseUint(opts.GnbID, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("could not parse gNB id %q: %w", opts.GnbID, err)
	}

	slices := opts.Slices
	if len(slices) == 0 {
		if opts.Sst == 0 {
			return nil, fmt.Errorf("SST is required to build NGSetupRequest")
		}

		slices = []SliceOpt{{Sst: opts.Sst, Sd: opts.Sd}}
	}

	req := &ngap.NGSetupRequest{
		GlobalRANNodeID: ngap.GlobalRANNodeID{
			Kind:         ngap.RANNodeIDGNB,
			PLMNIdentity: plmn,
			Value:        uint32(gnbID),
			Bits:         24,
		},
		RANNodeName:      ngap.Ptr(opts.Name),
		DefaultPagingDRX: ngap.Ptr(ngap.PagingDRXv128),
	}

	if opts.Tac == "" {
		return req, nil
	}

	tac, err := strconv.ParseUint(opts.Tac, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("could not parse TAC %q: %w", opts.Tac, err)
	}

	support := make(ngap.SliceSupportList, 0, len(slices))

	for _, sl := range slices {
		snssai, err := util.SNSSAIToNGAP(models.Snssai{Sst: sl.Sst, Sd: sl.Sd})
		if err != nil {
			return nil, fmt.Errorf("could not encode slice: %w", err)
		}

		support = append(support, ngap.SliceSupportItem{SNSSAI: snssai})
	}

	req.SupportedTAList = ngap.SupportedTAList{{
		TAC: ngap.TAC(tac),
		BroadcastPLMNList: ngap.BroadcastPLMNList{{
			PLMNIdentity:        plmn,
			TAISliceSupportList: support,
		}},
	}}

	return req, nil
}

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBDoesntSupportAnyTAC(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
		Sd:    "010203",
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}, nil, nil)

	msg.SupportedTAList = nil

	HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure to be sent, but got %d", len(sender.SentNGSetupFailures))
	}

	cause := sender.SentNGSetupFailures[0].Cause

	want := ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnspecified}
	if cause == nil || *cause != want {
		t.Errorf("cause = %v, want unspecified", cause)
	}

	if ran.RanID != nil {
		t.Error("RanID should remain nil after failed NG Setup")
	}
}

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBSupportsDifferentTAC(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
		Sd:    "010203",
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{
		Mcc: "001",
		Mnc: "01",
	}

	err = op.SetSupportedTacs([]string{"000065", "000066"})
	if err != nil {
		t.Fatalf("failed to set supported TACS: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
	}, nil, nil)

	HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure to be sent, but got %d", len(sender.SentNGSetupFailures))
	}

	cause := sender.SentNGSetupFailures[0].Cause

	want := ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnspecified}
	if cause == nil || *cause != want {
		t.Errorf("cause = %v, want unspecified", cause)
	}

	if ran.RanID != nil {
		t.Error("RanID should remain nil after failed NG Setup")
	}
}

func TestHandleNGSetupRequest_NGSetupResponse(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
		Sd:    "010203",
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{
		Mcc: "001",
		Mnc: "01",
	}

	err = op.SetSupportedTacs([]string{"000064", "000065"})
	if err != nil {
		t.Fatalf("failed to set supported TACS: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
	}, nil, nil)
	amfInstance.Name = "ella-core"

	HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse to be sent, but got %d", len(sender.SentNGSetupResponses))
	}

	response := sender.SentNGSetupResponses[0]

	if len(response.ServedGUAMIList) != 1 {
		t.Fatalf("expected 1 served GUAMI, got %d", len(response.ServedGUAMIList))
	}

	if got := response.ServedGUAMIList[0].GUAMI.PLMNIdentity; got != operatorPLMN {
		t.Errorf("served GUAMI PLMN = %x, want %x (001/01)", got, operatorPLMN)
	}

	if len(response.PLMNSupportList) != 1 {
		t.Fatalf("expected 1 supported PLMN, got %d", len(response.PLMNSupportList))
	}

	slices := response.PLMNSupportList[0].SliceSupportList
	if len(slices) != 1 {
		t.Fatalf("expected 1 supported slice, got %d", len(slices))
	}

	if slices[0].SNSSAI.SST != 1 {
		t.Errorf("expected SST 1, got %d", slices[0].SNSSAI.SST)
	}

	if response.AMFName != "ella-core" {
		t.Errorf("expected AmfName to be 'ella-core', but got '%s'", response.AMFName)
	}

	if response.RelativeAMFCapacity == nil || *response.RelativeAMFCapacity != 0xff {
		t.Errorf("expected RelativeAMFCapacity 0xff, got %v", response.RelativeAMFCapacity)
	}

	if ran.RanID == nil {
		t.Fatal("RanID should be set after successful NG Setup")
	}

	tais := amfInstance.RadioSupportedTAIsForTest(ran)
	if len(tais) != 1 {
		t.Fatalf("expected 1 SupportedTAI, got %d", len(tais))
	}

	if tais[0].Tai.Tac != "000064" {
		t.Errorf("expected TAC '000064', got %q", tais[0].Tai.Tac)
	}

	if len(tais[0].SNssaiList) != 1 {
		t.Fatalf("expected 1 SNssai in SupportedTAI, got %d", len(tais[0].SNssaiList))
	}

	if tais[0].SNssaiList[0].Sst != 1 {
		t.Errorf("expected SNssai SST 1, got %d", tais[0].SNssaiList[0].Sst)
	}

	if tais[0].SNssaiList[0].Sd != "010203" {
		t.Errorf("expected SNssai SD '010203', got %q", tais[0].SNssaiList[0].Sd)
	}
}

func TestHandleNGSetupRequest_MultipleSlicesInRequest(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Slices: []SliceOpt{
			{Sst: 1, Sd: "010203"},
			{Sst: 2, Sd: "aabbcc"},
			{Sst: 3, Sd: ""},
		},
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{
		Mcc: "001",
		Mnc: "01",
	}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
	}, nil, nil)
	amfInstance.Name = "ella-core"

	HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse, got %d", len(sender.SentNGSetupResponses))
	}

	tais := amfInstance.RadioSupportedTAIsForTest(ran)
	if len(tais) != 1 {
		t.Fatalf("expected 1 SupportedTAI, got %d", len(tais))
	}

	snssaiList := tais[0].SNssaiList
	if len(snssaiList) != 3 {
		t.Fatalf("expected 3 SNssai items in SupportedTAI, got %d", len(snssaiList))
	}

	expectedSlices := []struct {
		sst int32
		sd  string
	}{
		{1, "010203"},
		{2, "aabbcc"},
		{3, ""},
	}

	for i, expected := range expectedSlices {
		if snssaiList[i].Sst != expected.sst {
			t.Errorf("SNssai[%d]: expected SST %d, got %d", i, expected.sst, snssaiList[i].Sst)
		}

		if snssaiList[i].Sd != expected.sd {
			t.Errorf("SNssai[%d]: expected SD %q, got %q", i, expected.sd, snssaiList[i].Sd)
		}
	}
}

func TestHandleNGSetupRequest_ResponseContainsAllConfiguredSlices(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
		Sd:    "010203",
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{
		Mcc: "001",
		Mnc: "01",
	}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	sd1 := "010203"
	sd2 := "aabbcc"

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
		Slices: []db.NetworkSlice{
			{ID: "slice-1", Name: "eMBB", Sst: 1, Sd: &sd1},
			{ID: "slice-2", Name: "URLLC", Sst: 2, Sd: &sd2},
			{ID: "slice-3", Name: "mMTC", Sst: 3, Sd: nil},
		},
	}, nil, nil)
	amfInstance.Name = "ella-core"

	HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse, got %d", len(sender.SentNGSetupResponses))
	}

	response := sender.SentNGSetupResponses[0]

	if len(response.PLMNSupportList) != 1 {
		t.Fatalf("expected 1 supported PLMN, got %d", len(response.PLMNSupportList))
	}

	slices := response.PLMNSupportList[0].SliceSupportList
	if len(slices) != 3 {
		t.Fatalf("expected 3 supported slices, got %d", len(slices))
	}

	expectedSlices := []struct {
		sst ngap.SST
		sd  *ngap.SD
	}{
		{1, &ngap.SD{0x01, 0x02, 0x03}},
		{2, &ngap.SD{0xaa, 0xbb, 0xcc}},
		{3, nil},
	}

	for i, expected := range expectedSlices {
		got := slices[i].SNSSAI
		if got.SST != expected.sst {
			t.Errorf("slice %d: expected SST %d, got %d", i, expected.sst, got.SST)
		}

		switch {
		case expected.sd == nil && got.SD != nil:
			t.Errorf("slice %d: expected no SD, got %x", i, *got.SD)
		case expected.sd != nil && (got.SD == nil || *got.SD != *expected.sd):
			t.Errorf("slice %d: expected SD %x, got %v", i, *expected.sd, got.SD)
		}
	}
}

func TestHandleNGSetupRequest_NGSetupFailure_PLMNMismatch(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "310",
		Mnc:   "410",
		Tac:   "000064",
		Sst:   1,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{Mcc: "001", Mnc: "01"}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{Operator: op}, nil, nil)

	HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure, got %d", len(sender.SentNGSetupFailures))
	}

	cause := sender.SentNGSetupFailures[0].Cause

	want := ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnknownPLMNOrSNPN}
	if cause == nil || *cause != want {
		t.Errorf("cause = %v, want unknown-PLMN-or-SNPN", cause)
	}
}

func TestHandleNGSetupRequest_DBFailure_SendsNGSetupFailure(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		OperatorErr: fmt.Errorf("database unavailable"),
	}, nil, nil)

	HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected NGSetupFailure on DB error, got %d", len(sender.SentNGSetupFailures))
	}

	cause := sender.SentNGSetupFailures[0].Cause

	want := ngap.Cause{Group: ngap.CauseGroupMisc, Value: ngap.CauseMiscUnspecified}
	if cause == nil || *cause != want {
		t.Errorf("cause = %v, want unspecified", cause)
	}
}

func TestHandleNGSetupRequest_SliceDBFailure_SendsNGSetupFailure(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   1,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{Mcc: "001", Mnc: "01"}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator:  op,
		SlicesErr: fmt.Errorf("slice query failed"),
	}, nil, nil)

	HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected NGSetupFailure on slice DB error, got %d", len(sender.SentNGSetupFailures))
	}
}

func TestHandleNGSetupRequest_NoSliceOverlap_SucceedsWithWarning(t *testing.T) {
	sender := &fakeNGAPSender{}

	ran := &amf.Radio{
		Log:  logger.AmfLog,
		Conn: sender,
	}
	ran.BindAMFForTest(amf.New(nil, nil, nil))

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Tac:   "000064",
		Sst:   2,
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	op := &db.Operator{Mcc: "001", Mnc: "01"}

	err = op.SetSupportedTacs([]string{"000064"})
	if err != nil {
		t.Fatalf("failed to set supported TACs: %v", err)
	}

	amfInstance := amf.New(&fakeDBInstance{
		Operator: op,
		Slices:   []db.NetworkSlice{{ID: "s1", Name: "eMBB", Sst: 1}},
	}, nil, nil)
	amfInstance.Name = "ella-core"

	HandleNGSetupRequest(context.Background(), amfInstance, ran, msg)

	if len(sender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected NGSetupResponse even with no slice overlap, got %d responses", len(sender.SentNGSetupResponses))
	}

	if len(sender.SentNGSetupFailures) != 0 {
		t.Errorf("expected no NGSetupFailure for slice mismatch, got %d", len(sender.SentNGSetupFailures))
	}
}
