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

type SecurityModeReject struct {
	nasType.ExtendedProtocolDiscriminator
	nasType.SpareHalfOctetAndSecurityHeaderType
	nasType.SecurityModeRejectMessageIdentity
	nasType.Cause5GMM
}

func NewSecurityModeReject(iei uint8) (securityModeReject *SecurityModeReject) {
	securityModeReject = &SecurityModeReject{}
	return securityModeReject
}

func (a *SecurityModeReject) EncodeSecurityModeReject(buffer *bytes.Buffer) {
	binary.Write(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Write(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Write(buffer, binary.BigEndian, &a.SecurityModeRejectMessageIdentity.Octet)
	binary.Write(buffer, binary.BigEndian, &a.Cause5GMM.Octet)
}

func (a *SecurityModeReject) DecodeSecurityModeReject(byteArray *[]byte) {
	buffer := bytes.NewBuffer(*byteArray)
	binary.Read(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Read(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Read(buffer, binary.BigEndian, &a.SecurityModeRejectMessageIdentity.Octet)
	binary.Read(buffer, binary.BigEndian, &a.Cause5GMM.Octet)
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
