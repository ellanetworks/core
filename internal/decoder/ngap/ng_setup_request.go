package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

func buildGlobalRANNodeIDIE(grn *ngapType.GlobalRANNodeID) *GlobalRANNodeIDIE {
	if grn == nil {
		return nil
	}

	ie := &GlobalRANNodeIDIE{}

	if grn.GlobalGNBID != nil && grn.GlobalGNBID.GNBID.GNBID != nil {
		ie.GlobalGNBID = bitStringToHex(grn.GlobalGNBID.GNBID.GNBID)
	}

	if grn.GlobalNgENBID != nil && grn.GlobalNgENBID.NgENBID.MacroNgENBID != nil {
		ie.GlobalNgENBID = bitStringToHex(grn.GlobalNgENBID.NgENBID.MacroNgENBID)
	}

	if grn.GlobalN3IWFID != nil && grn.GlobalN3IWFID.N3IWFID.N3IWFID != nil {
		ie.GlobalN3IWFID = bitStringToHex(grn.GlobalN3IWFID.N3IWFID.N3IWFID)
	}

	return ie
}

func buildSupportedTAListIE(stal *ngapType.SupportedTAList) []SupportedTA {
	if stal == nil {
		return nil
	}

	supportedTAs := make([]SupportedTA, len(stal.List))
	for i := 0; i < len(stal.List); i++ {
		supportedTAs[i] = SupportedTA{
			TAC:               hex.EncodeToString(stal.List[i].TAC.Value),
			BroadcastPLMNList: buildPLMNList(stal.List[i].BroadcastPLMNList),
		}
	}

	return supportedTAs
}

func buildPLMNList(bpl ngapType.BroadcastPLMNList) []PLMN {
	plmns := make([]PLMN, len(bpl.List))
	for i := 0; i < len(bpl.List); i++ {
		plmns[i] = PLMN{
			PLMNID:           plmnIDToModels(bpl.List[i].PLMNIdentity),
			SliceSupportList: buildSNSSAIList(bpl.List[i].TAISliceSupportList),
		}
	}

	return plmns
}

func buildSNSSAIList(sssl ngapType.SliceSupportList) []SNSSAI {
	snssais := make([]SNSSAI, len(sssl.List))
	for i := 0; i < len(sssl.List); i++ {
		snssai := buildSNSSAI(&sssl.List[i].SNSSAI)
		snssais[i] = *snssai
	}

	return snssais
}

func buildRanNodeNameIE(rnn *ngapType.RANNodeName) *string {
	if rnn == nil || rnn.Value == "" {
		return nil
	}

	s := rnn.Value

	return &s
}

func buildDefaultPagingDRXIE(dpd *ngapType.PagingDRX) *string {
	if dpd == nil {
		return nil
	}

	switch dpd.Value {
	case ngapType.PagingDRXPresentV32:
		return strPtr("v32")
	case ngapType.PagingDRXPresentV64:
		return strPtr("v64")
	case ngapType.PagingDRXPresentV128:
		return strPtr("v128")
	case ngapType.PagingDRXPresentV256:
		return strPtr("v256")
	default:
		return strPtr(fmt.Sprintf("Unknown (%d)", dpd.Value))
	}
}

type NGSetupRequest struct {
	IEs []IE `json:"ies"`
}

func buildNGSetupRequest(ngSetupRequest *ngapType.NGSetupRequest) *NGSetupRequest {
	if ngSetupRequest == nil {
		return nil
	}

	ngSetup := &NGSetupRequest{}

	for i := 0; i < len(ngSetupRequest.ProtocolIEs.List); i++ {
		ie := ngSetupRequest.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDGlobalRANNodeID:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToString(ie.Criticality.Value),
				GlobalRANNodeID: buildGlobalRANNodeIDIE(ie.Value.GlobalRANNodeID),
			})
		case ngapType.ProtocolIEIDSupportedTAList:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:              protocolIEIDToString(ie.Id.Value),
				Criticality:     criticalityToString(ie.Criticality.Value),
				SupportedTAList: buildSupportedTAListIE(ie.Value.SupportedTAList),
			})
		case ngapType.ProtocolIEIDRANNodeName:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				RANNodeName: buildRanNodeNameIE(ie.Value.RANNodeName),
			})
		case ngapType.ProtocolIEIDDefaultPagingDRX:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:               protocolIEIDToString(ie.Id.Value),
				Criticality:      criticalityToString(ie.Criticality.Value),
				DefaultPagingDRX: buildDefaultPagingDRXIE(ie.Value.DefaultPagingDRX),
			})
		case ngapType.ProtocolIEIDUERetentionInformation:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToString(ie.Criticality.Value),
				UERetentionInformation: buildUERetentionInformationIE(ie.Value.UERetentionInformation),
			})
		default:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return ngSetup
}

func buildUERetentionInformationIE(uri *ngapType.UERetentionInformation) *string {
	if uri == nil {
		return nil
	}

	switch uri.Value {
	case ngapType.UERetentionInformationPresentUesRetained:
		return strPtr("present")
	default:
		return strPtr(fmt.Sprintf("unknown (%d)", uri.Value))
	}
}
