package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/omec-project/ngap/ngapType"
)

type NGSetupRequest struct {
	IEs []IE `json:"ies"`
}

type SNSSAI struct {
	SST int32   `json:"sst"`
	SD  *string `json:"sd,omitempty"`
}

type GlobalRANNodeIDIE struct {
	GlobalGNBID   string `json:"global_gnb_id,omitempty"`
	GlobalNgENBID string `json:"global_ng_enb_id,omitempty"`
	GlobalN3IWFID string `json:"global_n3iwf_id,omitempty"`
}

type SupportedTA struct {
	TAC               string `json:"tac"`
	BroadcastPLMNList []PLMN `json:"broadcast_plmn_list,omitempty"`
}

type PLMNID struct {
	Mcc string `json:"mcc"`
	Mnc string `json:"mnc"`
}

type PLMN struct {
	PLMNID           PLMNID   `json:"plmn_id"`
	SliceSupportList []SNSSAI `json:"slice_support_list,omitempty"`
}

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

func buildDefaultPagingDRXIE(dpd *ngapType.PagingDRX) *EnumField {
	if dpd == nil {
		return nil
	}

	switch dpd.Value {
	case ngapType.PagingDRXPresentV32:
		return &EnumField{Label: "v32", Value: int(dpd.Value)}
	case ngapType.PagingDRXPresentV64:
		return &EnumField{Label: "v64", Value: int(dpd.Value)}
	case ngapType.PagingDRXPresentV128:
		return &EnumField{Label: "v128", Value: int(dpd.Value)}
	case ngapType.PagingDRXPresentV256:
		return &EnumField{Label: "v256", Value: int(dpd.Value)}
	default:
		return &EnumField{Label: "Unknown", Value: int(dpd.Value)}
	}
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
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildGlobalRANNodeIDIE(ie.Value.GlobalRANNodeID),
			})
		case ngapType.ProtocolIEIDSupportedTAList:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildSupportedTAListIE(ie.Value.SupportedTAList),
			})
		case ngapType.ProtocolIEIDRANNodeName:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildRanNodeNameIE(ie.Value.RANNodeName),
			})
		case ngapType.ProtocolIEIDDefaultPagingDRX:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildDefaultPagingDRXIE(ie.Value.DefaultPagingDRX),
			})
		case ngapType.ProtocolIEIDUERetentionInformation:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUERetentionInformationIE(ie.Value.UERetentionInformation),
			})
		default:
			ngSetup.IEs = append(ngSetup.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value: UnknownIE{
					Reason: fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
				},
			})
		}
	}

	return ngSetup
}

func buildUERetentionInformationIE(uri *ngapType.UERetentionInformation) *EnumField {
	if uri == nil {
		return nil
	}

	switch uri.Value {
	case ngapType.UERetentionInformationPresentUesRetained:
		return &EnumField{Label: "present", Value: int(uri.Value)}
	default:
		return &EnumField{Label: "unknown ", Value: int(uri.Value)}
	}
}
