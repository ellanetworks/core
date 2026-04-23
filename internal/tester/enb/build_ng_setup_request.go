package enb

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/tester/gnb"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type NGSetupRequestOpts struct {
	Name  string
	EnbID string
	Mcc   string
	Mnc   string
	Tac   string
	Sst   int32
	Sd    string
}

func BuildNGSetupRequest(opts *NGSetupRequestOpts) (ngapType.NGAPPDU, error) {
	if opts.Mcc == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("MCC is required to build NGSetupRequest")
	}

	if opts.Mnc == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("MNC is required to build NGSetupRequest")
	}

	plmnID, err := gnb.GetMccAndMncInOctets(opts.Mcc, opts.Mnc)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("could not get plmnID in octets: %v", err)
	}

	if opts.Sst == 0 {
		return ngapType.NGAPPDU{}, fmt.Errorf("SST is required to build NGSetupRequest")
	}

	sst, sd, err := gnb.GetSliceInBytes(opts.Sst, opts.Sd)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("could not get slice info in bytes: %v", err)
	}

	if opts.Tac == "" {
		return ngapType.NGAPPDU{}, fmt.Errorf("TAC is required to build NGSetupRequest")
	}

	tac, err := hex.DecodeString(opts.Tac)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("could not get tac in bytes: %v", err)
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
	globalRANNodeID.Present = ngapType.GlobalRANNodeIDPresentGlobalNgENBID
	globalRANNodeID.GlobalNgENBID = new(ngapType.GlobalNgENBID)

	globalNgENBID := globalRANNodeID.GlobalNgENBID
	globalNgENBID.PLMNIdentity.Value = plmnID
	globalNgENBID.NgENBID.Present = ngapType.NgENBIDPresentMacroNgENBID
	globalNgENBID.NgENBID.MacroNgENBID = new(aper.BitString)

	enbIDBytes, err := gnb.GetGnbIdInBytes(opts.EnbID)
	if err != nil {
		return ngapType.NGAPPDU{}, fmt.Errorf("could not decode EnbID: %v", err)
	}

	*globalNgENBID.NgENBID.MacroNgENBID = aper.BitString{
		Bytes:     []byte{enbIDBytes[0], enbIDBytes[1], enbIDBytes[2] & 0xF0},
		BitLength: 20,
	}

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDRANNodeName
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentRANNodeName
	ie.Value.RANNodeName = new(ngapType.RANNodeName)

	rANNodeName := ie.Value.RANNodeName
	rANNodeName.Value = opts.Name

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)
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

	ie = ngapType.NGSetupRequestIEs{}
	ie.Id.Value = ngapType.ProtocolIEIDDefaultPagingDRX
	ie.Criticality.Value = ngapType.CriticalityPresentIgnore
	ie.Value.Present = ngapType.NGSetupRequestIEsPresentDefaultPagingDRX
	ie.Value.DefaultPagingDRX = new(ngapType.PagingDRX)

	pagingDRX := ie.Value.DefaultPagingDRX
	pagingDRX.Value = ngapType.PagingDRXPresentV128

	nGSetupRequestIEs.List = append(nGSetupRequestIEs.List, ie)

	return pdu, nil
}
