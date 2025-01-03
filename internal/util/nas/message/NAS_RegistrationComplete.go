// Copyright 2019 free5GC.org
//
// SPDX-License-Identifier: Apache-2.0
//

package nasMessage

import (
	"bytes"
	"encoding/binary"

	nasType "github.com/ellanetworks/core/internal/util/nas/type"
)

type RegistrationComplete struct {
	nasType.ExtendedProtocolDiscriminator
	nasType.SpareHalfOctetAndSecurityHeaderType
	nasType.RegistrationCompleteMessageIdentity
	*nasType.SORTransparentContainer
}

func NewRegistrationComplete(iei uint8) (registrationComplete *RegistrationComplete) {
	registrationComplete = &RegistrationComplete{}
	return registrationComplete
}

const (
	RegistrationCompleteSORTransparentContainerType uint8 = 0x73
)

func (a *RegistrationComplete) EncodeRegistrationComplete(buffer *bytes.Buffer) {
	binary.Write(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Write(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Write(buffer, binary.BigEndian, &a.RegistrationCompleteMessageIdentity.Octet)
	if a.SORTransparentContainer != nil {
		binary.Write(buffer, binary.BigEndian, a.SORTransparentContainer.GetIei())
		binary.Write(buffer, binary.BigEndian, a.SORTransparentContainer.GetLen())
		binary.Write(buffer, binary.BigEndian, &a.SORTransparentContainer.Buffer)
	}
}

func (a *RegistrationComplete) DecodeRegistrationComplete(byteArray *[]byte) {
	buffer := bytes.NewBuffer(*byteArray)
	binary.Read(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Read(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Read(buffer, binary.BigEndian, &a.RegistrationCompleteMessageIdentity.Octet)
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
		case RegistrationCompleteSORTransparentContainerType:
			a.SORTransparentContainer = nasType.NewSORTransparentContainer(ieiN)
			binary.Read(buffer, binary.BigEndian, &a.SORTransparentContainer.Len)
			a.SORTransparentContainer.SetLen(a.SORTransparentContainer.GetLen())
			binary.Read(buffer, binary.BigEndian, a.SORTransparentContainer.Buffer[:a.SORTransparentContainer.GetLen()])
		default:
		}
	}
}
