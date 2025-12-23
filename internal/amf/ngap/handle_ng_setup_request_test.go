// Copyright 2024 Ella Networks

package ngap_test

import (
	"context"
	"encoding/hex"
	"fmt"
	"testing"

	ctxt "github.com/ellanetworks/core/internal/amf/context"
	"github.com/ellanetworks/core/internal/amf/ngap"
	"github.com/ellanetworks/core/internal/db"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapConvert"
	"github.com/free5gc/ngap/ngapType"
)

type FakeSCTPConn struct{}

type NGSetupRequestOpts struct {
	Name  string
	GnbID string
	ID    int64
	Mcc   string
	Mnc   string
	Tac   string
	Sst   int32
	Sd    string
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

	if opts.Sst == 0 {
		return nil, fmt.Errorf("SST is required to build NGSetupRequest")
	}

	sst, sd, err := getSliceInBytes(opts.Sst, opts.Sd)
	if err != nil {
		return nil, fmt.Errorf("could not get slice info in bytes: %v", err)
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
		sliceSupportItem := ngapType.SliceSupportItem{}
		sliceSupportItem.SNSSAI.SST.Value = sst

		if sd != nil {
			sliceSupportItem.SNSSAI.SD = new(ngapType.SD)
			sliceSupportItem.SNSSAI.SD.Value = sd
		}

		sliceSupportList.List = append(sliceSupportList.List, sliceSupportItem)

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

func getMccAndMncInOctets(mccStr string, mncStr string) ([]byte, error) {
	mcc := reverse(mccStr)
	mnc := reverse(mncStr)

	var res string

	if len(mnc) == 2 {
		res = fmt.Sprintf("%c%cf%c%c%c", mcc[1], mcc[2], mcc[0], mnc[0], mnc[1])
	} else {
		res = fmt.Sprintf("%c%c%c%c%c%c", mcc[1], mcc[2], mnc[2], mcc[0], mnc[0], mnc[1])
	}

	resu, err := hex.DecodeString(res)
	if err != nil {
		return nil, fmt.Errorf("could not decode mcc/mnc to octets: %v", err)
	}

	return resu, nil
}

func reverse(s string) string {
	var aux string

	for _, valor := range s {
		aux = string(valor) + aux
	}

	return aux
}

func getSliceInBytes(sst int32, sd string) ([]byte, []byte, error) {
	sstBytes := []byte{byte(sst)}

	if sd != "" {
		sdBytes, err := hex.DecodeString(sd)
		if err != nil {
			return sstBytes, nil, fmt.Errorf("could not decode sd to bytes: %v", err)
		}

		return sstBytes, sdBytes, nil
	}

	return sstBytes, nil, nil
}

type FakeDBInstance struct {
	Operator *db.Operator
}

func (fdb *FakeDBInstance) GetOperator(ctx context.Context) (*db.Operator, error) {
	return fdb.Operator, nil
}

func (fdb *FakeDBInstance) GetDataNetworkByID(ctx context.Context, id int) (*db.DataNetwork, error) {
	return &db.DataNetwork{
		ID:   id,
		Name: "TestDataNetwork",
	}, nil
}

func (fdb *FakeDBInstance) GetPolicyByID(ctx context.Context, id int) (*db.Policy, error) {
	return &db.Policy{
		ID:   id,
		Name: "TestPolicy",
	}, nil
}

func (fdb *FakeDBInstance) GetSubscriber(ctx context.Context, imsi string) (*db.Subscriber, error) {
	return &db.Subscriber{
		Imsi: imsi,
	}, nil
}

type NGSetupFailure struct {
	Cause *ngapType.Cause
}

type NGSetupResponse struct {
	Guami               *models.Guami
	PlmnSupported       *models.PlmnSupportItem
	AmfName             string
	AmfRelativeCapacity int64
}

type FakeNGAPSender struct {
	SentNGSetupFailures  []*NGSetupFailure
	SentNGSetupResponses []*NGSetupResponse
}

func (fng *FakeNGAPSender) SendNGSetupFailure(ctx context.Context, cause *ngapType.Cause) error {
	fng.SentNGSetupFailures = append(fng.SentNGSetupFailures, &NGSetupFailure{Cause: cause})
	return nil
}

func (fng *FakeNGAPSender) SendNGSetupResponse(ctx context.Context, guami *models.Guami, plmnSupported *models.PlmnSupportItem, amfName string, amfRelativeCapacity int64) error {
	fng.SentNGSetupResponses = append(fng.SentNGSetupResponses, &NGSetupResponse{
		Guami:               guami,
		PlmnSupported:       plmnSupported,
		AmfName:             amfName,
		AmfRelativeCapacity: amfRelativeCapacity,
	})
	return nil
}

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBDoesntSupportAnyTAC(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &ctxt.AmfRan{
		Log:             logger.AmfLog,
		NGAPSender:      fakeNGAPSender,
		SupportedTAList: make([]ctxt.SupportedTAI, 0, ctxt.MaxNumOfTAI*ctxt.MaxNumOfBroadcastPLMNs),
	}

	msg, err := buildNGSetupRequest(&NGSetupRequestOpts{
		Name:  "TestRAN",
		GnbID: "ABCDE1",
		ID:    12345,
		Mcc:   "001",
		Mnc:   "01",
		Sst:   1,
		Sd:    "010203",
	})
	if err != nil {
		t.Fatalf("failed to build NGSetupRequest: %v", err)
	}

	amf := &ctxt.AMFContext{
		DBInstance: &FakeDBInstance{
			Operator: &db.Operator{
				Mcc: "001",
				Mnc: "01",
				Sst: 1,
			},
		},
	}

	ngap.HandleNGSetupRequest(context.Background(), amf, ran, msg)

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
}

func TestHandleNGSetupRequest_NGSetupFailure_gNodeBSupportsDifferentTAC(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &ctxt.AmfRan{
		Log:             logger.AmfLog,
		NGAPSender:      fakeNGAPSender,
		SupportedTAList: make([]ctxt.SupportedTAI, 0, ctxt.MaxNumOfTAI*ctxt.MaxNumOfBroadcastPLMNs),
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
		Sst: 1,
	}

	err = op.SetSupportedTacs([]string{"000065", "000066"})
	if err != nil {
		t.Fatalf("failed to set supported TACS: %v", err)
	}

	amf := &ctxt.AMFContext{
		DBInstance: &FakeDBInstance{
			Operator: op,
		},
	}

	ngap.HandleNGSetupRequest(context.Background(), amf, ran, msg)

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
}

func TestHandleNGSetupRequest_NGSetupResponse(t *testing.T) {
	fakeNGAPSender := &FakeNGAPSender{}

	ran := &ctxt.AmfRan{
		Log:             logger.AmfLog,
		NGAPSender:      fakeNGAPSender,
		SupportedTAList: make([]ctxt.SupportedTAI, 0, ctxt.MaxNumOfTAI*ctxt.MaxNumOfBroadcastPLMNs),
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
		Sst: 1,
	}

	err = op.SetSupportedTacs([]string{"000064", "000065"})
	if err != nil {
		t.Fatalf("failed to set supported TACS: %v", err)
	}

	amf := &ctxt.AMFContext{
		Name:             "ella-core",
		RelativeCapacity: 0xff,
		DBInstance: &FakeDBInstance{
			Operator: op,
		},
	}

	ngap.HandleNGSetupRequest(context.Background(), amf, ran, msg)

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

	if response.PlmnSupported == nil {
		t.Errorf("expected PlmnSupported to be set in NGSetupResponse, but it was nil")
	}

	if response.PlmnSupported.PlmnID.Mcc != "001" {
		t.Errorf("expected PlmnSupported PlmnID MCC to be '001', but got %s", response.PlmnSupported.PlmnID.Mcc)
	}

	if response.PlmnSupported.PlmnID.Mnc != "01" {
		t.Errorf("expected PlmnSupported PlmnID MNC to be '01', but got %s", response.PlmnSupported.PlmnID.Mnc)
	}

	if response.AmfName != "ella-core" {
		t.Errorf("expected AmfName to be 'ella-core', but got '%s'", response.AmfName)
	}

	if response.AmfRelativeCapacity != 0xff {
		t.Errorf("expected AmfRelativeCapacity to be 0xff, but got %d", response.AmfRelativeCapacity)
	}
}
