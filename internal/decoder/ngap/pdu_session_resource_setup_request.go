package ngap

import (
	"encoding/hex"

	"github.com/ellanetworks/core/internal/decoder/nas"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

func buildPDUSessionResourceSetupRequest(pduSessionResourceSetupRequest *ngapType.PDUSessionResourceSetupRequest) *PDUSessionResourceSetupRequest {
	if pduSessionResourceSetupRequest == nil {
		return nil
	}

	ieList := &PDUSessionResourceSetupRequest{}

	AMFUENGAPID := int64(0)

	for i := 0; i < len(pduSessionResourceSetupRequest.ProtocolIEs.List); i++ {
		ie := pduSessionResourceSetupRequest.ProtocolIEs.List[i]
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
		case ngapType.ProtocolIEIDRANPagingPriority:
			ieList.IEs = append(ieList.IEs, IE{
				ID:                protocolIEIDToString(ie.Id.Value),
				Criticality:       criticalityToString(ie.Criticality.Value),
				RANPagingPriority: &ie.Value.RANPagingPriority.Value,
			})
		case ngapType.ProtocolIEIDNASPDU:
			nasContextInfo := &nas.NasContextInfo{
				AMFUENGAPID: AMFUENGAPID,
				Direction:   nas.DirDownlink,
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
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSUReq:
			nasContextInfo := &nas.NasContextInfo{
				AMFUENGAPID: AMFUENGAPID,
				Direction:   nas.DirDownlink,
			}
			ieList.IEs = append(ieList.IEs, IE{
				ID:                               protocolIEIDToString(ie.Id.Value),
				Criticality:                      criticalityToString(ie.Criticality.Value),
				PDUSessionResourceSetupListSUReq: buildPDUSessionResourceSetupListSUReq(ie.Value.PDUSessionResourceSetupListSUReq, nasContextInfo),
			})
		case ngapType.ProtocolIEIDUEAggregateMaximumBitRate:
			ieList.IEs = append(ieList.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToString(ie.Criticality.Value),
				UEAggregateMaximumBitRate: &UEAggregateMaximumBitRate{
					Downlink: ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateDL.Value,
					Uplink:   ie.Value.UEAggregateMaximumBitRate.UEAggregateMaximumBitRateUL.Value,
				},
			})
		default:
			logger.EllaLog.Warn("Unsupported IE in PDUSessionResourceSetupRequest", zap.Int64("id", ie.Id.Value))
		}
	}

	return ieList
}

func buildPDUSessionResourceSetupListSUReq(list *ngapType.PDUSessionResourceSetupListSUReq, nasContextInfo *nas.NasContextInfo) []PDUSessionResourceSetupSUReq {
	if list == nil {
		return nil
	}

	var reqList []PDUSessionResourceSetupSUReq
	for _, item := range list.List {
		pduSUReq := PDUSessionResourceSetupSUReq{
			PDUSessionID:                           item.PDUSessionID.Value,
			SNSSAI:                                 *buildSNSSAI(&item.SNSSAI),
			PDUSessionResourceSetupRequestTransfer: item.PDUSessionResourceSetupRequestTransfer,
		}

		if item.PDUSessionNASPDU != nil {
			decodednNasPdu, err := nas.DecodeNASMessage(item.PDUSessionNASPDU.Value, nasContextInfo)
			if err != nil {
				logger.EllaLog.Warn("Failed to decode NAS PDU", zap.Error(err))
			}
			pduSUReq.PDUSessionNASPDU = &NASPDU{
				Raw:     item.PDUSessionNASPDU.Value,
				Decoded: decodednNasPdu,
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
