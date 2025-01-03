// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasMessage

import (
	"bytes"
	"encoding/binary"
	"net"

	nasType "github.com/ellanetworks/core/internal/util/nas/type"
)

type DLNASTransport struct {
	nasType.ExtendedProtocolDiscriminator
	nasType.SpareHalfOctetAndSecurityHeaderType
	nasType.DLNASTRANSPORTMessageIdentity
	nasType.SpareHalfOctetAndPayloadContainerType
	nasType.PayloadContainer
	*nasType.PduSessionID2Value
	*nasType.AdditionalInformation
	*nasType.Cause5GMM
	*nasType.BackoffTimerValue
	Ipaddr string
}

func NewDLNASTransport(iei uint8) (dLNASTransport *DLNASTransport) {
	dLNASTransport = &DLNASTransport{}
	return dLNASTransport
}

const (
	DLNASTransportPduSessionID2ValueType    uint8 = 0x12
	DLNASTransportAdditionalInformationType uint8 = 0x24
	DLNASTransportCause5GMMType             uint8 = 0x58
	DLNASTransportBackoffTimerValueType     uint8 = 0x37
)

func (a *DLNASTransport) EncodeDLNASTransport(buffer *bytes.Buffer) {
	binary.Write(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Write(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Write(buffer, binary.BigEndian, &a.DLNASTRANSPORTMessageIdentity.Octet)
	binary.Write(buffer, binary.BigEndian, &a.SpareHalfOctetAndPayloadContainerType.Octet)
	binary.Write(buffer, binary.BigEndian, a.PayloadContainer.GetLen())
	binary.Write(buffer, binary.BigEndian, &a.PayloadContainer.Buffer)
	if a.PduSessionID2Value != nil {
		binary.Write(buffer, binary.BigEndian, a.PduSessionID2Value.GetIei())
		binary.Write(buffer, binary.BigEndian, &a.PduSessionID2Value.Octet)
	}
	if a.AdditionalInformation != nil {
		binary.Write(buffer, binary.BigEndian, a.AdditionalInformation.GetIei())
		binary.Write(buffer, binary.BigEndian, a.AdditionalInformation.GetLen())
		binary.Write(buffer, binary.BigEndian, &a.AdditionalInformation.Buffer)
	}
	if a.Cause5GMM != nil {
		binary.Write(buffer, binary.BigEndian, a.Cause5GMM.GetIei())
		binary.Write(buffer, binary.BigEndian, &a.Cause5GMM.Octet)
	}
	if a.BackoffTimerValue != nil {
		binary.Write(buffer, binary.BigEndian, a.BackoffTimerValue.GetIei())
		binary.Write(buffer, binary.BigEndian, a.BackoffTimerValue.GetLen())
		binary.Write(buffer, binary.BigEndian, &a.BackoffTimerValue.Octet)
	}
}

func (a *DLNASTransport) DecodeDLNASTransport(byteArray *[]byte) {
	buffer := bytes.NewBuffer(*byteArray)
	binary.Read(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Read(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Read(buffer, binary.BigEndian, &a.DLNASTRANSPORTMessageIdentity.Octet)
	binary.Read(buffer, binary.BigEndian, &a.SpareHalfOctetAndPayloadContainerType.Octet)
	binary.Read(buffer, binary.BigEndian, &a.PayloadContainer.Len)
	a.PayloadContainer.SetLen(a.PayloadContainer.GetLen())
	binary.Read(buffer, binary.BigEndian, &a.PayloadContainer.Buffer)
	if a.SpareHalfOctetAndPayloadContainerType.Octet == uint8(1) {
		esmMsg := bytes.NewBuffer(a.PayloadContainer.Buffer)
		var pd uint8
		binary.Read(esmMsg, binary.BigEndian, &pd)
		binary.Read(esmMsg, binary.BigEndian, &pd)
		binary.Read(esmMsg, binary.BigEndian, &pd)
		binary.Read(esmMsg, binary.BigEndian, &pd)
		binary.Read(esmMsg, binary.BigEndian, &pd)
		var len uint16
		binary.Read(esmMsg, binary.BigEndian, &len)
		var qos [9]uint8
		binary.Read(esmMsg, binary.BigEndian, &qos)
		var ambr_len uint8
		binary.Read(esmMsg, binary.BigEndian, &ambr_len)
		var ambr [6]uint8
		binary.Read(esmMsg, binary.BigEndian, &ambr)
	forLoop:
		for esmMsg.Len() > 0 {
			var ieiN uint8
			binary.Read(esmMsg, binary.BigEndian, &ieiN)
			switch ieiN {
			case 89:
				var cause uint8
				binary.Read(esmMsg, binary.BigEndian, &cause)
			case 41:
				var iplen uint8
				binary.Read(esmMsg, binary.BigEndian, &iplen)
				var iptype uint8
				binary.Read(esmMsg, binary.BigEndian, &iptype)
				var ipaddr [4]uint8
				binary.Read(esmMsg, binary.BigEndian, &ipaddr)
				ip := net.IPv4(ipaddr[0], ipaddr[1], ipaddr[2], ipaddr[3])
				a.Ipaddr = ip.String()
				break forLoop // we just need ip address nothing more
			default:
			}
		}

	}

	for buffer.Len() > 0 {
		var ieiN uint8
		var tmpIeiN uint8
		binary.Read(buffer, binary.BigEndian, &ieiN)
		if ieiN >= 0x80 {
			tmpIeiN = (ieiN & 0xf0) >> 4
		} else {
			tmpIeiN = ieiN
		}
		switch tmpIeiN {
		case DLNASTransportPduSessionID2ValueType:
			a.PduSessionID2Value = nasType.NewPduSessionID2Value(ieiN)
			binary.Read(buffer, binary.BigEndian, &a.PduSessionID2Value.Octet)
		case DLNASTransportAdditionalInformationType:
			a.AdditionalInformation = nasType.NewAdditionalInformation(ieiN)
			binary.Read(buffer, binary.BigEndian, &a.AdditionalInformation.Len)
			a.AdditionalInformation.SetLen(a.AdditionalInformation.GetLen())
			binary.Read(buffer, binary.BigEndian, a.AdditionalInformation.Buffer[:a.AdditionalInformation.GetLen()])
		case DLNASTransportCause5GMMType:
			a.Cause5GMM = nasType.NewCause5GMM(ieiN)
			binary.Read(buffer, binary.BigEndian, &a.Cause5GMM.Octet)
		case DLNASTransportBackoffTimerValueType:
			a.BackoffTimerValue = nasType.NewBackoffTimerValue(ieiN)
			binary.Read(buffer, binary.BigEndian, &a.BackoffTimerValue.Len)
			a.BackoffTimerValue.SetLen(a.BackoffTimerValue.GetLen())
			binary.Read(buffer, binary.BigEndian, &a.BackoffTimerValue.Octet)
		default:
		}
	}
}
