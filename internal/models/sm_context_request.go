// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package models

import (
	"github.com/ellanetworks/core/etsi"
	"github.com/ellanetworks/core/nas/fgs"
)

type SmContextCreateRequest struct {
	SUPI              etsi.SUPI
	PDUSessionID      uint8
	DNN               string
	Snssai            *Snssai
	RequestType       fgs.RequestType
	N1SmMsg           []byte
	EPSBearerIdentity uint8
	UE                SessionUEInfo
}

type SessionTransferRequest struct {
	SUPI              etsi.SUPI
	PDUSessionID      uint8
	EPSBearerIdentity uint8
	DNN               string
	Snssai            *Snssai
	UE                SessionUEInfo
}
