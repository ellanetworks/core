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

type IdentityRequest struct {
	nasType.ExtendedProtocolDiscriminator
	nasType.SpareHalfOctetAndSecurityHeaderType
	nasType.IdentityRequestMessageIdentity
	nasType.SpareHalfOctetAndIdentityType
}

func NewIdentityRequest(iei uint8) (identityRequest *IdentityRequest) {
	identityRequest = &IdentityRequest{}
	return identityRequest
}

func (a *IdentityRequest) EncodeIdentityRequest(buffer *bytes.Buffer) {
	binary.Write(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Write(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Write(buffer, binary.BigEndian, &a.IdentityRequestMessageIdentity.Octet)
	binary.Write(buffer, binary.BigEndian, &a.SpareHalfOctetAndIdentityType.Octet)
}

func (a *IdentityRequest) DecodeIdentityRequest(byteArray *[]byte) {
	buffer := bytes.NewBuffer(*byteArray)
	binary.Read(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Read(buffer, binary.BigEndian, &a.SpareHalfOctetAndSecurityHeaderType.Octet)
	binary.Read(buffer, binary.BigEndian, &a.IdentityRequestMessageIdentity.Octet)
	binary.Read(buffer, binary.BigEndian, &a.SpareHalfOctetAndIdentityType.Octet)
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
