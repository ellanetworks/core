package ngap

import (
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/ngap/ngapType"
	"go.uber.org/zap"
)

type PDUSessionResourceSetupResponse struct {
	IEs []IE `json:"ies"`
}

func buildPDUSessionResourceSetupResponse(pduSessionResourceSetupResponse *ngapType.PDUSessionResourceSetupResponse) *PDUSessionResourceSetupResponse {
	if pduSessionResourceSetupResponse == nil {
		return nil
	}

	psrs := &PDUSessionResourceSetupResponse{}

	for i := 0; i < len(pduSessionResourceSetupResponse.ProtocolIEs.List); i++ {
		ie := pduSessionResourceSetupResponse.ProtocolIEs.List[i]
		switch ie.Id.Value {
		case ngapType.ProtocolIEIDAMFUENGAPID:
			psrs.IEs = append(psrs.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				AMFUENGAPID: &ie.Value.AMFUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDRANUENGAPID:
			psrs.IEs = append(psrs.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
				RANUENGAPID: &ie.Value.RANUENGAPID.Value,
			})
		case ngapType.ProtocolIEIDPDUSessionResourceSetupListSURes:
			psrs.IEs = append(psrs.IEs, IE{
				ID:                               protocolIEIDToString(ie.Id.Value),
				Criticality:                      criticalityToEnum(ie.Criticality.Value),
				PDUSessionResourceSetupListSURes: buildPDUSessionResourceSetupListSUResIE(ie.Value.PDUSessionResourceSetupListSURes),
			})
		case ngapType.ProtocolIEIDPDUSessionResourceFailedToSetupListSURes:
			psrs.IEs = append(psrs.IEs, IE{
				ID:                                       protocolIEIDToString(ie.Id.Value),
				Criticality:                              criticalityToEnum(ie.Criticality.Value),
				PDUSessionResourceFailedToSetupListSURes: buildPDUSessionResourceFailedToSetupListSUResIE(ie.Value.PDUSessionResourceFailedToSetupListSURes),
			})
		case ngapType.ProtocolIEIDCriticalityDiagnostics:
			psrs.IEs = append(psrs.IEs, IE{
				ID:                     protocolIEIDToString(ie.Id.Value),
				Criticality:            criticalityToEnum(ie.Criticality.Value),
				CriticalityDiagnostics: buildCriticalityDiagnosticsIE(ie.Value.CriticalityDiagnostics),
			})
		default:
			psrs.IEs = append(psrs.IEs, IE{
				ID:          protocolIEIDToString(ie.Id.Value),
				Criticality: criticalityToEnum(ie.Criticality.Value),
			})
			logger.EllaLog.Warn("Unsupported ie type", zap.Int64("type", ie.Id.Value))
		}
	}

	return psrs
}

func buildPDUSessionResourceSetupListSUResIE(pduList *ngapType.PDUSessionResourceSetupListSURes) []PDUSessionResourceSetupSURes {
	if pduList == nil {
		return nil
	}

	pduSessionList := make([]PDUSessionResourceSetupSURes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]

		pduSessionList = append(pduSessionList, PDUSessionResourceSetupSURes{
			PDUSessionID:                            item.PDUSessionID.Value,
			PDUSessionResourceSetupResponseTransfer: item.PDUSessionResourceSetupResponseTransfer,
		})
	}

	return pduSessionList
}

func buildPDUSessionResourceFailedToSetupListSUResIE(pduList *ngapType.PDUSessionResourceFailedToSetupListSURes) []PDUSessionResourceFailedToSetupSURes {
	if pduList == nil {
		return nil
	}

	pduSessionList := make([]PDUSessionResourceFailedToSetupSURes, 0)

	for i := 0; i < len(pduList.List); i++ {
		item := pduList.List[i]

		pduSessionList = append(pduSessionList, PDUSessionResourceFailedToSetupSURes{
			PDUSessionID: item.PDUSessionID.Value,
			PDUSessionResourceSetupUnsuccessfulTransfer: item.PDUSessionResourceSetupUnsuccessfulTransfer,
		})
	}

	return pduSessionList
}
