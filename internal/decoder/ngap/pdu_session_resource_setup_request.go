package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/free5gc/ngap/ngapType"
)

type PDUSessionResourceSetupSUReq struct {
	PDUSessionID                           int64   `json:"pdu_session_id"`
	PDUSessionNASPDU                       *NASPDU `json:"pdu_session_nas_pdu,omitempty"`
	SNSSAI                                 SNSSAI  `json:"snssai"`
	PDUSessionResourceSetupRequestTransfer []byte  `json:"pdu_session_resource_setup_request_transfer"`
}

func buildPDUSessionResourceSetupRequest(pduSessionResourceSetupRequest ngapType.PDUSessionResourceSetupRequest) NGAPMessageValue {
	ies := make([]IE, 0)

	for i := 0; i < len(pduSessionResourceSetupRequest.ProtocolIEs.List); i++ {
		ie := pduSessionResourceSetupRequest.ProtocolIEs.List[i]
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
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildPDUSessionResourceSetupListSUReq(*ie.Value.PDUSessionResourceSetupListSUReq),
			})
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ies = append(ies, IE{
				ID:          protocolIEIDToEnum(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				Value:       buildUEAggregateMaximumBitRateIE(*ie.Value.UEAggregateMaximumBitRate),
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

func buildPDUSessionResourceSetupListSUReq(list ngapType.PDUSessionResourceSetupListSUReq) []PDUSessionResourceSetupSUReq {
	var reqList []PDUSessionResourceSetupSUReq
	for _, item := range list.List {
		pduSUReq := PDUSessionResourceSetupSUReq{
			PDUSessionID:                           item.PDUSessionID.Value,
			SNSSAI:                                 *buildSNSSAI(&item.SNSSAI),
			PDUSessionResourceSetupRequestTransfer: item.PDUSessionResourceSetupRequestTransfer,
		}

		if item.PDUSessionNASPDU != nil {
			pduSUReq.PDUSessionNASPDU = &NASPDU{
				Raw:     item.PDUSessionNASPDU.Value,
				Decoded: nas.DecodeNASMessage(item.PDUSessionNASPDU.Value),
			}
		}

		reqList = append(reqList, pduSUReq)
	}

	return reqList
}

func buildSNSSAI(ngapSnssai *ngapType.SNSSAI) *SNSSAI {
	if ngapSnssai == nil {
		return nil
	}

	snssai := &SNSSAI{
		SST: int32(ngapSnssai.SST.Value[0]),
	}

	if ngapSnssai.SD != nil {
		sd := hex.EncodeToString(ngapSnssai.SD.Value)
		snssai.SD = &sd
	}

	return snssai
}
