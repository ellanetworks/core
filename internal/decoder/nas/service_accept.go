package nas

import (
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/internal/logger"
	"github.com/omec-project/nas/nasConvert"
	"github.com/omec-project/nas/nasMessage"
)

type PDUSessionCause struct {
	PDUSessionID uint8                  `json:"pdu_session_id"`
	Cause        utils.EnumField[uint8] `json:"cause"`
}

type PDUSessionReactivateResultPDU struct {
	PDUSessionID int  `json:"pdu_session_id"`
	Active       bool `json:"active"`
}

type ServiceAccept struct {
	ExtendedProtocolDiscriminator          uint8                           `json:"extended_protocol_discriminator"`
	SpareHalfOctetAndSecurityHeaderType    uint8                           `json:"spare_half_octet_and_security_header_type"`
	PDUSessionStatus                       []PDUSessionStatusPDU           `json:"pdu_session_status,omitempty"`
	PDUSessionReactivationResult           []PDUSessionReactivateResultPDU `json:"pdu_session_reactivation_result,omitempty"`
	PDUSessionReactivationResultErrorCause []PDUSessionCause               `json:"pdu_session_reactivation_result_error_cause,omitempty"`
	EAPMessage                             []byte                          `json:"eap_message,omitempty"`
}

func buildServiceAccept(msg *nasMessage.ServiceAccept) *ServiceAccept {
	if msg == nil {
		return nil
	}

	serviceAccept := &ServiceAccept{
		ExtendedProtocolDiscriminator:       msg.ExtendedProtocolDiscriminator.Octet,
		SpareHalfOctetAndSecurityHeaderType: msg.SpareHalfOctetAndSecurityHeaderType.Octet,
	}

	if msg.PDUSessionStatus != nil {
		pduSessionStatus := []PDUSessionStatusPDU{}
		psiArray := nasConvert.PSIToBooleanArray(msg.PDUSessionStatus.Buffer)
		for pduSessionID, isActive := range psiArray {
			pduSessionStatus = append(pduSessionStatus, PDUSessionStatusPDU{
				PDUSessionID: pduSessionID,
				Active:       isActive,
			})
		}
		serviceAccept.PDUSessionStatus = pduSessionStatus
	}

	if msg.PDUSessionReactivationResult != nil {
		pduSessionReactivationResult := []PDUSessionReactivateResultPDU{}
		psiArray := nasConvert.PSIToBooleanArray(msg.PDUSessionReactivationResult.Buffer)
		for pduSessionID, isActive := range psiArray {
			pduSessionReactivationResult = append(pduSessionReactivationResult, PDUSessionReactivateResultPDU{
				PDUSessionID: pduSessionID,
				Active:       isActive,
			})
		}
		serviceAccept.PDUSessionReactivationResult = pduSessionReactivationResult
	}

	if msg.PDUSessionReactivationResultErrorCause != nil {
		pduSessionIDAndCause := msg.PDUSessionReactivationResultErrorCause.GetPDUSessionIDAndCauseValue()
		pduSessionIDs, causes := bufToPDUSessionReactivationResultErrorCause(pduSessionIDAndCause)
		if len(pduSessionIDs) != len(causes) {
			logger.EllaLog.Warn("PDUSessionReactivationResultErrorCause: invalid length")
		} else {
			var pduSessionCauses []PDUSessionCause
			for i := range pduSessionIDs {
				pduSessionCauses = append(pduSessionCauses, PDUSessionCause{
					PDUSessionID: pduSessionIDs[i],
					Cause:        cause5GMMToEnum(causes[i]),
				})
			}
			serviceAccept.PDUSessionReactivationResultErrorCause = pduSessionCauses
		}
	}

	if msg.EAPMessage != nil {
		serviceAccept.EAPMessage = msg.EAPMessage.GetEAPMessage()
	}

	return serviceAccept
}

func bufToPDUSessionReactivationResultErrorCause(buf []uint8) (errPduSessionId, errCause []uint8) {
	if len(buf)%2 != 0 {
		return nil, nil
	}

	n := len(buf) / 2
	errPduSessionId = make([]uint8, 0, n)
	errCause = make([]uint8, 0, n)

	for i := 0; i < len(buf); i += 2 {
		errPduSessionId = append(errPduSessionId, buf[i])
		errCause = append(errCause, buf[i+1])
	}
	return
}
