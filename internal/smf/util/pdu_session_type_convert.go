package util

import (
	"github.com/ellanetworks/core/internal/models"
	"github.com/free5gc/nas/nasMessage"
)

func PDUSessionTypeToModels(nasPduSessType uint8) (pduSessType models.PduSessionType) {
	switch nasPduSessType {
	case nasMessage.PDUSessionTypeIPv4:
		pduSessType = models.PduSessionTypeIPv4
	case nasMessage.PDUSessionTypeIPv6:
		pduSessType = models.PduSessionTypeIPv6
	case nasMessage.PDUSessionTypeIPv4IPv6:
		pduSessType = models.PduSessionTypeIPv4v6
	case nasMessage.PDUSessionTypeUnstructured:
		pduSessType = models.PduSessionTypeUnstructured
	case nasMessage.PDUSessionTypeEthernet:
		pduSessType = models.PduSessionTypeEthernet
	}

	return
}

func ModelsToPDUSessionType(pduSessType models.PduSessionType) (nasPduSessType uint8) {
	switch pduSessType {
	case models.PduSessionTypeIPv4:
		nasPduSessType = nasMessage.PDUSessionTypeIPv4
	case models.PduSessionTypeIPv6:
		nasPduSessType = nasMessage.PDUSessionTypeIPv6
	case models.PduSessionTypeIPv4v6:
		nasPduSessType = nasMessage.PDUSessionTypeIPv4IPv6
	case models.PduSessionTypeUnstructured:
		nasPduSessType = nasMessage.PDUSessionTypeUnstructured
	case models.PduSessionTypeEthernet:
		nasPduSessType = nasMessage.PDUSessionTypeEthernet
	}
	return
}
