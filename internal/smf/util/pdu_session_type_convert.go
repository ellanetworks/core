package util

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/omec-project/nas/nasMessage"
)

func PDUSessionTypeToModels(nasPduSessType uint8) (pduSessType models.PduSessionType) {
	switch nasPduSessType {
	case nasMessage.PDUSessionTypeIPv4:
		pduSessType = models.PduSessionType_IPV4
	case nasMessage.PDUSessionTypeIPv6:
		pduSessType = models.PduSessionType_IPV6
	case nasMessage.PDUSessionTypeIPv4IPv6:
		pduSessType = models.PduSessionType_IPV4_V6
	case nasMessage.PDUSessionTypeUnstructured:
		pduSessType = models.PduSessionType_UNSTRUCTURED
	case nasMessage.PDUSessionTypeEthernet:
		pduSessType = models.PduSessionType_ETHERNET
	}

	return
}
