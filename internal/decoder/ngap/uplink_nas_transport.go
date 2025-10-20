package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/omec-project/ngap/ngapType"
)

func buildUplinkNASTransport(uplinkNASTransport ngapType.UplinkNASTransport) NGAPMessageValue {
	ies := make([]IE, 0)

	AMFUENGAPID := int64(0)

	for i := 0; i < len(uplinkNASTransport.ProtocolIEs.List); i++ {
		ie := uplinkNASTransport.ProtocolIEs.List[i]
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
		case ngapType.ProtocolIEIDNASPDU:
			nasContextInfo := &nas.NasContextInfo{
				Direction:   nas.DirUplink,
				AMFUENGAPID: AMFUENGAPID,
			}

			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value: NASPDU{
					Raw:     ie.Value.NASPDU.Value,
					Decoded: nas.DecodeNASMessage(ie.Value.NASPDU.Value, nasContextInfo),
				},
			})
		case ngapType.ProtocolIEIDUserLocationInformation:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUserLocationInformationIE(*ie.Value.UserLocationInformation),
			})
		default:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Error:       fmt.Sprintf("unsupported ie type %d", ie.Id.Value),
			})
		}
	}

	return NGAPMessageValue{
		IEs: ies,
	}
}
