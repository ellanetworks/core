// Copyright 2025 Ella Networks

package ngap_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/ellanetworks/core/internal/amf"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/amf/ngap/decode"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

func decodeNGSetupRequestOrFatal(t *testing.T, pdu *ngapType.NGAPPDU) decode.NGSetupRequest {
	t.Helper()

	decoded, report := decode.DecodeNGSetupRequest(pdu.InitiatingMessage.Value.NGSetupRequest)
	if report != nil {
		t.Fatalf("decoder produced report: %+v", report)
	}

	return decoded
}

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
	Slices []SliceOpt // if set, overrides Sst/Sd
}

func buildNGSetupRequest(opts *NGSetupRequestOpts) (*ngapType.NGAPPDU, error) {
	if opts.Mcc == "" {
		return nil, fmt.Errorf("MCC is required to build NGSetupRequest")
	}

	if opts.Mnc == "" {
		return nil, fmt.Errorf("MNC is required to build NGSetupRequest")
	}

	plmnID, err := getMccAndMncInOctets(opts.Mcc, opts.Mnc)
	if err != nil {
		return nil, fmt.Errorf("could not get plmnID in octets: %v", err)
	}

	slices := opts.Slices
	if len(slices) == 0 {
		if opts.Sst == 0 {
			return nil, fmt.Errorf("SST is required to build NGSetupRequest")
		}

		slices = []SliceOpt{{Sst: opts.Sst, Sd: opts.Sd}}
	}

	pdu := ngapType.NGAPPDU{}
	pdu.Present = ngapType.NGAPPDUPresentInitiatingMessage
	pdu.InitiatingMessage = new(ngapType.InitiatingMessage)

	initiatingMessage := pdu.InitiatingMessage
	initiatingMessage.ProcedureCode.Value = ngapType.ProcedureCodeNGSetup
	initiatingMessage.Criticality.Value = ngapType.CriticalityPresentReject

	initiatingMessage.Value.Present = ngapType.InitiatingMessagePresentNGSetupRequest
	initiatingMessage.Value.NGSetupRequest = new(ngapType.NGSetupRequest)

	nGSetupRequest := initiatingMessage.Value.NGSetupRequest
	nGSetupRequestIEs := &nGSetupRequest.ProtocolIEs

	ie := ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDGlobalRANNodeID
	ie.Criticality.Value = ngapType.CriticalityPresentReject
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentGlobalRANNodeID
	ie.Value.GlobalRANNodeID = new(ngapType.GlobalRANNodeID)

	globalRANNodeID := ie.Value.GlobalRANNodeID
	globalRANNodeID.Present = ngapType.GlobalRANNodeIDPresentGlobalGNBID
	globalRANNodeID.GlobalGNBID = new(ngapType.GlobalGNBID)

	globalGNBID := globalRANNodeID.GlobalGNBID
	globalGNBID.PLMNIdentity.Value = plmnID
	globalGNBID.GNBID.Present = ngapType.GNBIDPresentGNBID
	globalGNBID.GNBID.GNBID = new(aper.BitString)

	gNBID := globalGNBID.GNBID.GNBID

	*gNBID = ngapConvert.HexToBitString(opts.GnbID, 24)

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	// RANNodeName
	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANNodeName
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentRANNodeName
	ie.Value.RANNodeName = new(ngapType.RANNodeName)

	rANNodeName := ie.Value.RANNodeName
	rANNodeName.Value = opts.Name

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	if opts.Tac != "" {
		tac, err := hex.DecodeString(opts.Tac)
		if err != nil {
			return nil, fmt.Errorf("could not get tac in bytes: %v", err)
		}

		ie = ngapType.NGSetupRequestIEs{}
		ie.Id.Value = ngapType.ProtocolIEIDSupportedTAList
		ie.Criticality.Value = ngapType.CriticalityPresentReject
		ie.Value.Present = ngapType.NGSetupRequestIEsPresentSupportedTAList
		ie.Value.SupportedTAList = new(ngapType.SupportedTAList)

		supportedTAList := ie.Value.SupportedTAList

		supportedTAItem := ngapType.SupportedTAItem{}
		supportedTAItem.TAC.Value = tac

		broadcastPLMNList := &supportedTAItem.BroadcastPLMNList
		broadcastPLMNItem := ngapType.BroadcastPLMNItem{}
		broadcastPLMNItem.PLMNIdentity.Value = plmnID
		sliceSupportList := &broadcastPLMNItem.TAISliceSupportList

		for _, s := range slices {
			sst, sd, err := getSliceInBytes(s.Sst, s.Sd)
			if err != nil {
				return nil, fmt.Errorf("could not get slice info in bytes: %v", err)
			}

			sliceSupportItem := ngapType.SliceSupportItem{}
			sliceSupportItem.SNSSAI.SST.Value = sst

			if sd != nil {
				sliceSupportItem.SNSSAI.SD = new(ngapType.SD)
				sliceSupportItem.SNSSAI.SD.Value = sd
			}

			sliceSupportList.List = append(sliceSupportList.List, sliceSupportItem)
		}

		broadcastPLMNList.List = append(broadcastPLMNList.List, broadcastPLMNItem)

		supportedTAList.List = append(supportedTAList.List, supportedTAItem)

		nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)
	}

	// PagingDRX
	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDDefaultPagingDRX
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentDefaultPagingDRX
	ie.Value.DefaultPagingDRX = new(ngapType.PagingDRX)

	pagingDRX := ie.Value.DefaultPagingDRX
	pagingDRX.Value = ngapType.PagingDRXPresentV128

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	return &pdu, nil
}

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBDoesntSupportAnyTAC(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

	// Use a valid TAC to satisfy the decoder (missing IE → ErrorIndication
	// at the dispatcher layer, not NGSetupFailure here), then erase the
	// item list to exercise the "gNB advertised no supported TAs" branch.
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

	amfInstance := amf.New(&FakeDBInstance{
		Operator: &db.Operator{
			Mcc: "001",
			Mnc: "01",
		},
	}, nil, nil)

	decoded := decodeNGSetupRequestOrFatal(t, msg)
	decoded.SupportedTAItems = nil

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, decoded)

	if len(fakeNGAPSender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure to be sent, but got %d", len(fakeNGAPSender.SentNGSetupFailures))
	}

	cause := fakeNGAPSender.SentNGSetupFailures[0].Cause
	if cause.Present != ngapType.CausePresentMisc {
		t.Fatalf("expected Cause Present to be Miscellaneous, but got %v", cause.Present)
	}

	if cause.Misc.Value != ngapType.CauseMiscPresentUnspecified {
		t.Errorf("expected Cause Miscellaneous Value to be CauseMiscPresentUnspecified, but got %v", cause.Misc.Value)
	}

	if ran.RanID != nil {
		t.Error("RanID should remain nil after failed NG Setup")
	}
}

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBSupportsDifferentTAC(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

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

	amfInstance := amf.New(&FakeDBInstance{
		Operator: op,
	}, nil, nil)

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, decodeNGSetupRequestOrFatal(t, msg))

	if len(fakeNGAPSender.SentNGSetupFailures) != 1 {
		t.Fatalf("expected 1 NGSetupFailure to be sent, but got %d", len(fakeNGAPSender.SentNGSetupFailures))
	}

	cause := fakeNGAPSender.SentNGSetupFailures[0].Cause
	if cause.Present != ngapType.CausePresentMisc {
		t.Fatalf("expected Cause Present to be Miscellaneous, but got %v", cause.Present)
	}

	if cause.Misc.Value != ngapType.CauseMiscPresentUnknownPLMN {
		t.Errorf("expected Cause Miscellaneous Value to be CauseMiscPresentUnknownPLMN, but got %v", cause.Misc.Value)
	}

	if ran.RanID != nil {
		t.Error("RanID should remain nil after failed NG Setup")
	}
}

func TestHandleNGSetupRequest_NGSetupResponse(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

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

	amfInstance := amf.New(&FakeDBInstance{
		Operator: op,
	}, nil, nil)
	amfInstance.Name = "ella-core"

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, decodeNGSetupRequestOrFatal(t, msg))

	if len(fakeNGAPSender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse to be sent, but got %d", len(fakeNGAPSender.SentNGSetupResponses))
	}

	response := fakeNGAPSender.SentNGSetupResponses[0]

	if response.Guami == nil {
		t.Errorf("expected Guami to be set in NGSetupResponse, but it was nil")
	}

	if response.Guami.PlmnID.Mcc != "001" {
		t.Errorf("expected Guami PlmnID MCC to be '001', but got %s", response.Guami.PlmnID.Mcc)
	}

	if response.Guami.PlmnID.Mnc != "01" {
		t.Errorf("expected Guami PlmnID MNC to be '01', but got %s", response.Guami.PlmnID.Mnc)
	}

	if len(response.SnssaiList) != 1 {
		t.Fatalf("expected 1 slice in SnssaiList, got %d", len(response.SnssaiList))
	}

	if response.SnssaiList[0].Sst != 1 {
		t.Errorf("expected SnssaiList[0].Sst to be 1, got %d", response.SnssaiList[0].Sst)
	}

	if response.AmfName != "ella-core" {
		t.Errorf("expected AmfName to be 'ella-core', but got '%s'", response.AmfName)
	}

	if response.AmfRelativeCapacity != 0xff {
		t.Errorf("expected AmfRelativeCapacity to be 0xff, but got %d", response.AmfRelativeCapacity)
	}

	if ran.RanID == nil {
		t.Fatal("RanID should be set after successful NG Setup")
	}

	// Verify ran.SupportedTAIs was populated from the request
	if len(ran.SupportedTAIs) != 1 {
		t.Fatalf("expected 1 SupportedTAI, got %d", len(ran.SupportedTAIs))
	}

	if ran.SupportedTAIs[0].Tai.Tac != "000064" {
		t.Errorf("expected TAC '000064', got %q", ran.SupportedTAIs[0].Tai.Tac)
	}

	if len(ran.SupportedTAIs[0].SNssaiList) != 1 {
		t.Fatalf("expected 1 SNssai in SupportedTAI, got %d", len(ran.SupportedTAIs[0].SNssaiList))
	}

	if ran.SupportedTAIs[0].SNssaiList[0].Sst != 1 {
		t.Errorf("expected SNssai SST 1, got %d", ran.SupportedTAIs[0].SNssaiList[0].Sst)
	}

	if ran.SupportedTAIs[0].SNssaiList[0].Sd != "010203" {
		t.Errorf("expected SNssai SD '010203', got %q", ran.SupportedTAIs[0].SNssaiList[0].Sd)
	}
}

func TestHandleNGSetupRequest_MultipleSlicesInRequest(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

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

	amfInstance := amf.New(&FakeDBInstance{
		Operator: op,
	}, nil, nil)
	amfInstance.Name = "ella-core"

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, decodeNGSetupRequestOrFatal(t, msg))

	if len(fakeNGAPSender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse, got %d", len(fakeNGAPSender.SentNGSetupResponses))
	}

	// Verify ran.SupportedTAIs has all 3 slices from the request
	if len(ran.SupportedTAIs) != 1 {
		t.Fatalf("expected 1 SupportedTAI, got %d", len(ran.SupportedTAIs))
	}

	snssaiList := ran.SupportedTAIs[0].SNssaiList
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
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &amf.Radio{
		Log:           logger.AmfLog,
		NGAPSender:    fakeNGAPSender,
		SupportedTAIs: make([]amf.SupportedTAI, 0),
	}

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

	amfInstance := amf.New(&FakeDBInstance{
		Operator: op,
		Slices: []db.NetworkSlice{
			{ID: 1, Name: "eMBB", Sst: 1, Sd: &sd1},
			{ID: 2, Name: "URLLC", Sst: 2, Sd: &sd2},
			{ID: 3, Name: "mMTC", Sst: 3, Sd: nil},
		},
	}, nil, nil)
	amfInstance.Name = "ella-core"

	ngap.HandleNGSetupRequest(context.Background(), amfInstance, ran, decodeNGSetupRequestOrFatal(t, msg))

	if len(fakeNGAPSender.SentNGSetupResponses) != 1 {
		t.Fatalf("expected 1 NGSetupResponse, got %d", len(fakeNGAPSender.SentNGSetupResponses))
	}

	response := fakeNGAPSender.SentNGSetupResponses[0]

	// Verify the response carries all 3 configured slices from DB
	if len(response.SnssaiList) != 3 {
		t.Fatalf("expected 3 slices in response SnssaiList, got %d", len(response.SnssaiList))
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
		if response.SnssaiList[i].Sst != expected.sst {
			t.Errorf("SnssaiList[%d]: expected SST %d, got %d", i, expected.sst, response.SnssaiList[i].Sst)
		}

		if response.SnssaiList[i].Sd != expected.sd {
			t.Errorf("SnssaiList[%d]: expected SD %q, got %q", i, expected.sd, response.SnssaiList[i].Sd)
		}
	}
}
