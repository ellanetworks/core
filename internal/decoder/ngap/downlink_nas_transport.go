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

type NASPDU struct {
	Raw     []byte          `json:"raw"`
	Decoded *nas.NASMessage `json:"decoded"`
}

type RATRestriction struct {
	PLMNID                    PLMNID `json:"plmn_id"`
	RATRestrictionInformation string `json:"rat_restriction_information"`
}

type ForbiddenAreaInformation struct {
	PLMNID        PLMNID   `json:"plmn_id"`
	ForbiddenTACs []string `json:"forbidden_tacs"`
}

type ServiceAreaInformation struct {
	PLMNID         PLMNID   `json:"plmn_id"`
	AllowedTACs    []string `json:"allowed_tacs,omitempty"`
	NotAllowedTACs []string `json:"not_allowed_tacs,omitempty"`
}

type MobilityRestrictionList struct {
	ServingPLMN              PLMNID                     `json:"serving_plmn"`
	EquivalentPLMNs          []PLMNID                   `json:"equivalent_plmns,omitempty"`
	RATRestrictions          []RATRestriction           `json:"rat_restrictions,omitempty"`
	ForbiddenAreaInformation []ForbiddenAreaInformation `json:"forbidden_area_information,omitempty"`
	ServiceAreaInformation   []ServiceAreaInformation   `json:"service_area_information,omitempty"`
}

type UEAggregateMaximumBitRate struct {
	Downlink int64 `json:"downlink"`
	Uplink   int64 `json:"uplink"`
}

//	type NGAPMessageValue struct {
//		IEs   []IE   `json:"ies,omitempty"`
//		Error string `json:"error,omitempty"`
//	}

func buildDownlinkNASTransport(downlinkNASTransport ngapType.DownlinkNASTransport) NGAPMessageValue {
	AMFUENGAPID := int64(0)

	ies := make([]IE, 0)

	for i := 0; i < len(downlinkNASTransport.ProtocolIEs.List); i++ {
		ie := downlinkNASTransport.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			AMFUENGAPID = ie.Value.AMFUENGAPID.Value
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDOldAMF:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildAMFNameIE(*ie.Value.OldAMF),
			})
		case ngapType.ProtocolIEIDRANPagingPriority:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANPagingPriority.Value,
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
			nasPdu := NASPDU{
				Raw:     ie.Value.NASPDU.Value,
				Decoded: decodednNasPdu,
			}
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       nasPdu,
			})
		case ngapType.ProtocolIEIDMobilityRestrictionList:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildMobilityRestrictionListIE(*ie.Value.MobilityRestrictionList),
			})
		case ngapType.ProtocolIEIDIndexToRFSP:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.IndexToRFSP.Value,
			})
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUEAggregateMaximumBitRateIE(*ie.Value.UEAggregateMaximumBitRate),
			})
		case ngapType.ProtocolIEIDAllowedNSSAI:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildAllowedNSSAI(*ie.Value.AllowedNSSAI),
			})
		default:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value: UnknownIE{
					Reason: fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
				},
			})
		}
	}
	return NGAPMessageValue{
		IEs: ies,
	}
}

func buildUEAggregateMaximumBitRateIE(ueambr ngapType.UEAggregateMaximumBitRate) UEAggregateMaximumBitRate {
	return UEAggregateMaximumBitRate{
		Downlink: ueambr.UEAggregateMaximumBitRateDL.Value,
		Uplink:   ueambr.UEAggregateMaximumBitRateUL.Value,
	}
}

func buildMobilityRestrictionListIE(mrl ngapType.MobilityRestrictionList) MobilityRestrictionList {
	mobilityRestrictionList := MobilityRestrictionList{}

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
