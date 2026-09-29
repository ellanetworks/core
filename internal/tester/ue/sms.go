// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ue

import (
	"fmt"

	"github.com/ellanetworks/core/nas/fgs"
)

func (ue *UE) SMSAllowed() bool {
	return ue.smsAllowed.Load()
}

func (ue *UE) sendSMS(cp []byte) error {
	plain, err := (&fgs.ULNASTransport{PayloadContainerType: fgs.PayloadContainerTypeSMS, PayloadContainer: cp}).MarshalBinary()
	if err != nil {
		return fmt.Errorf("build UL NAS Transport: %w", err)
	}

	ue.smsMu.Lock()
	defer ue.smsMu.Unlock()

	wire, err := ue.EncodeNasPduWithSecurity(plain, uint8(fgs.SHTIntegrityProtectedCiphered))
	if err != nil {
		return err
	}

	return ue.Gnb.SendUplinkNAS(wire, ue.lastAMFUENGAPID.Load(), ue.lastRANUENGAPID.Load())
}
