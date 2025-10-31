package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/omec-project/ngap/ngapType"
)

type PDUSessionResourceToReleaseListRelCmd struct {
	PDUSessionID                             int64  `json:"pdu_session_id"`
	PDUSessionResourceReleaseCommandTransfer []byte `json:"pdu_session_resource_release_command_transfer"`
}

func buildPDUSessionResourceReleaseCommand(cmd ngapType.PDUSessionResourceReleaseCommand) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(cmd.ProtocolIEs.List); i++ {
		ie := cmd.ProtocolIEs.List[i]

		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
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
		case ngapType.ProtocolIEIDRANPagingPriority:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       ie.Value.RANPagingPriority.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value: NASPDU{
					Raw:     ie.Value.NASPDU.Value,
					Decoded: nas.DecodeNASMessage(ie.Value.NASPDU.Value),
				},
			})
		case ngapType.ProtocolIEIDPDUSessionResourceToReleaseListRelCmd:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceToReleaseListRelCmd(*ie.Value.PDUSessionResourceToReleaseListRelCmd),
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

func buildPDUSessionResourceToReleaseListRelCmd(pduList ngapType.PDUSessionResourceToReleaseListRelCmd) []PDUSessionResourceToReleaseListRelCmd {
	pduSessionList := make([]PDUSessionResourceToReleaseListRelCmd, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]

		pduSessionList = append(pduSessionList, PDUSessionResourceToReleaseListRelCmd{
			PDUSessionID:                             item.PDUSessionID.Value,
			PDUSessionResourceReleaseCommandTransfer: item.PDUSessionResourceReleaseCommandTransfer,
		})
	}

	return pduSessionList
}
