// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
)

const fcKASMEPrimeHandover = "74"

type MappedFrom5GS struct {
	KAMF             []byte
	DownlinkNASCount nas.Count
	UplinkNASCount   nas.Count
	Ciphering        uint8
	Integrity        uint8
	EKSI             uint8
}

func EstimateDownlinkNASCount(stored nas.Count, sequenceNumber uint8) (nas.Count, error) {
	estimate := nas.MakeCount(stored.Overflow(), sequenceNumber)
	if estimate.Value() <= stored.Value() {
		estimate = nas.MakeCount(stored.Overflow()+1, sequenceNumber)
	}

	if estimate.Value() <= stored.Value() {
		return 0, fmt.Errorf("s1enb: downlink NAS COUNT estimate %d does not exceed the stored %d",
			estimate.Value(), stored.Value())
	}

	return estimate, nil
}

func (ue *UE) InstallMappedSecurityContext(in MappedFrom5GS) error {
	if len(in.KAMF) != 32 {
		return fmt.Errorf("s1enb: K_AMF is %d octets, want 32", len(in.KAMF))
	}

	p0 := make([]byte, 4)
	binary.BigEndian.PutUint32(p0, in.DownlinkNASCount.Value())

	kasme, err := ueauth.GetKDFValue(in.KAMF, fcKASMEPrimeHandover, p0, ueauth.KDFLen(p0))
	if err != nil {
		return fmt.Errorf("s1enb: derive K'ASME: %w", err)
	}

	ue.kasme = kasme
	ue.eea, ue.eia = in.Ciphering, in.Integrity

	if err := ue.deriveNASKeys(); err != nil {
		return err
	}

	ue.ulCount = in.UplinkNASCount.SQN()

	return nil
}

func (ue *UE) EPSNASAlgorithmsInUse() (ciphering, integrity uint8) {
	return ue.eea, ue.eia
}

func (ue *UE) MappedNASKeys() (knasEnc, knasInt [16]byte) {
	return ue.knasEnc, ue.knasInt
}

func (ue *UE) MappedKASME() []byte { return ue.kasme }

func NewUnboundUE() *UE {
	return &UE{netCapEEA: 0xf0, netCapEIA: 0x70, pti: 1}
}
