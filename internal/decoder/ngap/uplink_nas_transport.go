package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

type UplinkNASTransport struct {
	IEs []IE `json:"ies"`
}

func buildUplinkNASTransport(uplinkNASTransport *ngapType.UplinkNASTransport) *UplinkNASTransport {
	if uplinkNASTransport == nil {
		return nil
	}

	ieList := &UplinkNASTransport{}

	AMFUENGAPID := int64(0)

	for i := 0; i < len(uplinkNASTransport.ProtocolIEs.List); i++ {
		ie := uplinkNASTransport.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			AMFUENGAPID = ie.Value.AMFUENGAPID.Value
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				AMFUENGAPID: &ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			nasContextInfo := &nas.NasContextInfo{
				Direction:   nas.DirUplink,
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

			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       nasPdu,
			})
		case ngapType.ProtocolIEIDUserLocationInformation:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                      protocolIEIDToEnum(ie.Id.Value),
				Criticality:             criticalityToEnum(ie.Criticality.Value),
				UserLocationInformation: buildUserLocationInformationIE(ie.Value.UserLocationInformation),
			})
		default:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value: UnknownIE{
					Reason: fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
				},
			})
		}
	}

	return ieList
}
