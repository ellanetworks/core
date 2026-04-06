package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/free5gc/aper"
	"github.com/free5gc/ngap/ngapType"
)

type PDUSessionResourceReleaseCommandTransferDecoded struct {
	Cause utils.EnumField[uint64] `json:"cause"`
}

type PDUSessionResourceToReleaseListRelCmd struct {
	PDUSessionID                             int64                                            `json:"pdu_session_id"`
	PDUSessionResourceReleaseCommandTransfer *PDUSessionResourceReleaseCommandTransferDecoded `json:"pdu_session_resource_release_command_transfer,omitempty"`

	Error string `json:"error,omitempty"`
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
					Protocol: "NAS",
					RawHex:   hex.EncodeToString(ie.Value.NASPDU.Value),
					Decoded:  nas.DecodeNASMessage(ie.Value.NASPDU.Value),
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
		entry := PDUSessionResourceToReleaseListRelCmd{
			PDUSessionID: item.PDUSessionID.Value,
		}

		transfer, err := decodeReleaseCommandTransfer(item.PDUSessionResourceReleaseCommandTransfer)
		if err != nil {
			entry.Error = fmt.Sprintf("failed to decode release command transfer: %v", err)
		} else {
			entry.PDUSessionResourceReleaseCommandTransfer = transfer
		}

		pduSessionList = append(pduSessionList, entry)
	}

	return pduSessionList
}

func decodeReleaseCommandTransfer(transfer aper.OctetString) (*PDUSessionResourceReleaseCommandTransferDecoded, error) {
	if transfer == nil {
		return nil, fmt.Errorf("transfer is nil")
	}

	pdu := &ngapType.PDUSessionResourceReleaseCommandTransfer{}

	err := aper.UnmarshalWithParams(transfer, pdu, "valueExt")
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal release command transfer: %v", err)
	}

	return &PDUSessionResourceReleaseCommandTransferDecoded{
		Cause: causeToEnum(pdu.Cause),
	}, nil
}
