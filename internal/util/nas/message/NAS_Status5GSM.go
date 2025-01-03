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

type Status5GSM struct {
	nasType.ExtendedProtocolDiscriminator
	nasType.PDUSessionID
	nasType.PTI
	nasType.STATUSMessageIdentity5GSM
	nasType.Cause5GSM
}

func NewStatus5GSM(iei uint8) (status5GSM *Status5GSM) {
	status5GSM = &Status5GSM{}
	return status5GSM
}

func (a *Status5GSM) EncodeStatus5GSM(buffer *bytes.Buffer) {
	binary.Write(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Write(buffer, binary.BigEndian, &a.PDUSessionID.Octet)
	binary.Write(buffer, binary.BigEndian, &a.PTI.Octet)
	binary.Write(buffer, binary.BigEndian, &a.STATUSMessageIdentity5GSM.Octet)
	binary.Write(buffer, binary.BigEndian, &a.Cause5GSM.Octet)
}

func (a *Status5GSM) DecodeStatus5GSM(byteArray *[]byte) {
	buffer := bytes.NewBuffer(*byteArray)
	binary.Read(buffer, binary.BigEndian, &a.ExtendedProtocolDiscriminator.Octet)
	binary.Read(buffer, binary.BigEndian, &a.PDUSessionID.Octet)
	binary.Read(buffer, binary.BigEndian, &a.PTI.Octet)
	binary.Read(buffer, binary.BigEndian, &a.STATUSMessageIdentity5GSM.Octet)
	binary.Read(buffer, binary.BigEndian, &a.Cause5GSM.Octet)
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
