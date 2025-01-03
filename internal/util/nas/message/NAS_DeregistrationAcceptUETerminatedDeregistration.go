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

type DeregistrationAcceptUETerminatedDeregistration struct {
	nasType.ExtendedProtocolDiscriminator
	nasType.SpareHalfOctetAndSecurityHeaderType
	nasType.DeregistrationAcceptMessageIdentity
}

func NewDeregistrationAcceptUETerminatedDeregistration(iei uint8) (deregistrationAcceptUETerminatedDeregistration *DeregistrationAcceptUETerminatedDeregistration) {
	deregistrationAcceptUETerminatedDeregistration = &DeregistrationAcceptUETerminatedDeregistration{}
	return deregistrationAcceptUETerminatedDeregistration
}

func (a *DeregistrationAcceptUETerminatedDeregistration) EncodeDeregistrationAcceptUETerminatedDeregistration(buffer *bytes.Buffer) {
	binary.Write(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Write(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Write(buffer, binary.BigEndian, &a.DeregistrationAcceptMessageIdentity.Octet)
}

func (a *DeregistrationAcceptUETerminatedDeregistration) DecodeDeregistrationAcceptUETerminatedDeregistration(byteArray *[]byte) {
	buffer := bytes.NewBuffer(*byteArray)
	binary.Read(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Read(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Read(buffer, binary.BigEndian, &a.DeregistrationAcceptMessageIdentity.Octet)
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
		default:
		}
	}
}
