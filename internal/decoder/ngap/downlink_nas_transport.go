package ngap

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

type DownlinkNASTransport struct {
	IEs []IE `json:"ies"`
}

func buildDownlinkNASTransport(downlinkNASTransport *ngapType.DownlinkNASTransport) *DownlinkNASTransport {
	if downlinkNASTransport == nil {
		return nil
	}

	AMFUENGAPID := int64(0)

	ieList := &DownlinkNASTransport{}

	for i := 0; i < len(downlinkNASTransport.ProtocolIEs.List); i++ {
		ie := downlinkNASTransport.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			AMFUENGAPID = ie.Value.AMFUENGAPID.Value
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				AMFUENGAPID: &ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				RANUENGAPID: &ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDOldAMF:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				OldAMF:      buildAMFNameIE(ie.Value.OldAMF),
			})
		case ngapType.ProtocolIEIDRANPagingPriority:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                protocolIEIDToString(ie.Id.Value),
				Criticality:       criticalityToString(ie.Criticality.Value),
				RANPagingPriority: &ie.Value.RANPagingPriority.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			nasContextInfo := &nas.NasContextInfo{
				Direction:   nas.DirDownlink,
				AMFUENGAPID: AMFUENGAPID,
			}
			decodednNasPdu, err := nas.DecodeNASMessage(ie.Value.NASPDU.Value, nasContextInfo)
			if err != nil {
				logger.EllaLog.Warn("Failed to decode NAS PDU", zap.Error(err))
			}
			nasPdu := &NASPDU{
				Raw:     ie.Value.NASPDU.Value,
				Decoded: decodednNasPdu,
			}
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				NASPDU:      nasPdu,
			})
		case ngapType.ProtocolIEIDMobilityRestrictionList:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                      protocolIEIDToString(ie.Id.Value),
				Criticality:             criticalityToString(ie.Criticality.Value),
				MobilityRestrictionList: buildMobilityRestrictionListIE(ie.Value.MobilityRestrictionList),
			})
		case ngapType.ProtocolIEIDIndexToRFSP:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				IndexToRFSP: &ie.Value.IndexToRFSP.Value,
			})
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                        protocolIEIDToString(ie.Id.Value),
				Criticality:               criticalityToString(ie.Criticality.Value),
				UEAggregateMaximumBitRate: buildUEAggregateMaximumBitRateIE(ie.Value.UEAggregateMaximumBitRate),
			})
		case ngapType.ProtocolIEIDAllowedNSSAI:
			ieList.IEs = append(ieList.IEs, IE{
				ID:           protocolIEIDToString(ie.Id.Value),
				Criticality:  criticalityToString(ie.Criticality.Value),
				AllowedNSSAI: buildAllowedNSSAI(ie.Value.AllowedNSSAI),
			})
		default:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}
	return ieList
}

func buildUEAggregateMaximumBitRateIE(ueambr *ngapType.UEAggregateMaximumBitRate) *UEAggregateMaximumBitRate {
	if ueambr == nil {
		return nil
	}

	return &UEAggregateMaximumBitRate{
		Downlink: ueambr.UEAggregateMaximumBitRateDL.Value,
		Uplink:   ueambr.UEAggregateMaximumBitRateUL.Value,
	}
}

func buildMobilityRestrictionListIE(mrl *ngapType.MobilityRestrictionList) *MobilityRestrictionList {
	if mrl == nil {
		return nil
	}

	mobilityRestrictionList := &MobilityRestrictionList{}

	mobilityRestrictionList.ServingPLMN = plmnIDToModels(mrl.ServingPLMN)

	if mrl.EquivalentPLMNs != nil {
		eqPlmns := make([]PLMNID, 0)
		for i := 0; i < len(mrl.EquivalentPLMNs.List); i++ {
			eqPlmns = append(eqPlmns, plmnIDToModels(mrl.EquivalentPLMNs.List[i]))
		}
		mobilityRestrictionList.EquivalentPLMNs = eqPlmns
	}

	if mrl.RATRestrictions != nil {
		ratRestrictions := make([]RATRestriction, 0)
		for i := 0; i < len(mrl.RATRestrictions.List); i++ {
			ratRestrictions = append(ratRestrictions, RATRestriction{
				PLMNID:                    plmnIDToModels(mrl.RATRestrictions.List[i].PLMNIdentity),
				RATRestrictionInformation: ratRestrictionInfoToString(mrl.RATRestrictions.List[i].RATRestrictionInformation),
			})
		}
		mobilityRestrictionList.RATRestrictions = ratRestrictions
	}

	if mrl.ForbiddenAreaInformation != nil {
		faiList := make([]ForbiddenAreaInformation, 0)
		for i := 0; i < len(mrl.ForbiddenAreaInformation.List); i++ {
			tacList := make([]string, 0)
			for j := 0; j < len(mrl.ForbiddenAreaInformation.List[i].ForbiddenTACs.List); j++ {
				tacList = append(tacList, hex.EncodeToString(mrl.ForbiddenAreaInformation.List[i].ForbiddenTACs.List[j].Value))
			}
			faiList = append(faiList, ForbiddenAreaInformation{
				PLMNID:        plmnIDToModels(mrl.ForbiddenAreaInformation.List[i].PLMNIdentity),
				ForbiddenTACs: tacList,
			})
		}
		mobilityRestrictionList.ForbiddenAreaInformation = faiList
	}

	if mrl.ServiceAreaInformation != nil {
		saiList := make([]ServiceAreaInformation, 0)
		for i := 0; i < len(mrl.ServiceAreaInformation.List); i++ {
			allowedTACs := make([]string, 0)
			for j := 0; j < len(mrl.ServiceAreaInformation.List[i].AllowedTACs.List); j++ {
				allowedTACs = append(allowedTACs, hex.EncodeToString(mrl.ServiceAreaInformation.List[i].AllowedTACs.List[j].Value))
			}
			notAllowedTACs := make([]string, 0)
			for j := 0; j < len(mrl.ServiceAreaInformation.List[i].NotAllowedTACs.List); j++ {
				notAllowedTACs = append(notAllowedTACs, hex.EncodeToString(mrl.ServiceAreaInformation.List[i].NotAllowedTACs.List[j].Value))
			}
			saiList = append(saiList, ServiceAreaInformation{
				PLMNID:         plmnIDToModels(mrl.ServiceAreaInformation.List[i].PLMNIdentity),
				AllowedTACs:    allowedTACs,
				NotAllowedTACs: notAllowedTACs,
			})
		}
		mobilityRestrictionList.ServiceAreaInformation = saiList
	}
	return mobilityRestrictionList
}

func ratRestrictionInfoToString(ratType ngapType.RATRestrictionInformation) string {
	if bytes.Equal(ratType.Value.Bytes, []byte{0x40}) {
		return "NR"
	} else if bytes.Equal(ratType.Value.Bytes, []byte{0x80}) {
		return "EUTRA"
	} else {
		return fmt.Sprintf("Unknown (%v)", ratType.Value)
	}
}
