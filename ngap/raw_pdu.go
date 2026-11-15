// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per/ap"
	"github.com/ellanetworks/core/per/ap/apimpl"
)

var (
	ErrIENotPresent = ap.ErrIENotPresent
	ErrDuplicateIE  = ap.ErrDuplicateIE
)

type RawPDU struct {
	*ap.RawPDU[ProtocolIEID, ProcedureCode, TriggeringMessage]
}

func ParseRawPDU(b []byte) (*RawPDU, error) {
	m, err := apimpl.ParseRawPDU[ProtocolIEID, ProcedureCode, TriggeringMessage](profile, b)
	if err != nil {
		return nil, err
	}

	return &RawPDU{m}, nil
}
