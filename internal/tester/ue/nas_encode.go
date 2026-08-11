/*
*
// Modified by Ella Networks Inc.
  - SPDX-License-Identifier: BUSL-1.1
  - SPDX-FileCopyrightText: Ella Networks Inc.
  - © Copyright 2023 Hewlett Packard Enterprise Development LP
    *
  - Modified by Ella Networks.
*/
package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/fgs"
)

// EncodeNasPduWithSecurity wraps a plaintext NAS PDU in the given security header,
// ciphering (when the header type indicates), computing the NAS-MAC over the
// sequence-number octet and payload, and advancing the uplink NAS COUNT — the
// mechanism the UE previously hand-rolled (TS 24.501 §4.4).
func (ue *UE) EncodeNasPduWithSecurity(pdu []byte, securityHeaderType uint8) ([]byte, error) {
	if ue == nil {
		return nil, fmt.Errorf("ue is nil")
	}

	if pdu == nil {
		return nil, fmt.Errorf("nas message is nil")
	}

	sc, err := ue.securityContext()
	if err != nil {
		return nil, err
	}

	wire, err := fgs.Protect(pdu, fgs.SecurityHeaderType(securityHeaderType), ue.UeSecurity.ULCount,
		nas.DirectionUplink, sc)
	if err != nil {
		return nil, fmt.Errorf("could not protect NAS message: %w", err)
	}

	ue.UeSecurity.ULCount = ue.UeSecurity.ULCount.Next()

	return wire, nil
}
