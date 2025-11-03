package ngap

import (
	"fmt"

	"github.com/omec-project/ngap/ngapType"
)

func buildUEContextReleaseCommand(ueContextReleaseCommand ngapType.UEContextReleaseCommand) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(ueContextReleaseCommand.ProtocolIEs.List); i++ {
		ie := ueContextReleaseCommand.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDUENGAPIDs:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUENGAPIDs(*ie.Value.UENGAPIDs),
			})
		case ngapType.ProtocolIEIDCause:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       CauseToEnum(*ie.Value.Cause),
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

type UENGAPIDPair struct {
	AMFUENGAPID int64 `json:"amf_ue_ngap_id"`
	RANUENGAPID int64 `json:"ran_ue_ngap_id"`
}

type UENGAPIDs struct {
	AMFUENGAPID  int64        `json:"amf_ue_ngap_id"`
	UENGAPIDPair UENGAPIDPair `json:"ue_ngap_id_pair"`
}

func buildUENGAPIDs(ueNgapIDs ngapType.UENGAPIDs) UENGAPIDs {
	amfuengapID := int64(0)
	if ueNgapIDs.AMFUENGAPID != nil {
		amfuengapID = ueNgapIDs.AMFUENGAPID.Value
	}

	uengapidPairAMFUENGAPID := int64(0)
	if ueNgapIDs.UENGAPIDPair != nil {
		uengapidPairAMFUENGAPID = ueNgapIDs.UENGAPIDPair.AMFUENGAPID.Value
	}

	uengapidPairRANUENGAPID := int64(0)
	if ueNgapIDs.UENGAPIDPair != nil {
		uengapidPairRANUENGAPID = ueNgapIDs.UENGAPIDPair.RANUENGAPID.Value
	}

	return UENGAPIDs{
		AMFUENGAPID: amfuengapID,
		UENGAPIDPair: UENGAPIDPair{
			AMFUENGAPID: uengapidPairAMFUENGAPID,
			RANUENGAPID: uengapidPairRANUENGAPID,
		},
	}
}
