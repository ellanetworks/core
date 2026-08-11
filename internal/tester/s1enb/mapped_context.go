// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1enb

import (
	"encoding/binary"
	"fmt"

	"github.com/ellanetworks/core/internal/util/ueauth"
	"github.com/ellanetworks/core/nas"
)

// fcKASMEPrimeHandover is TS 33.501 Annex A.14.2: K_AMF to K'ASME, with the
// downlink 5G NAS COUNT as P0. Spelled out here rather than shared with the
// core, so the tester checks the core's derivation instead of agreeing with it.
const fcKASMEPrimeHandover = "74"

// MappedFrom5GS is what a UE handed over from 5GS needs to build its mapped EPS
// security context (TS 33.501 §8.3.2 steps 8-9).
type MappedFrom5GS struct {
	// KAMF is the 5G NAS key the UE holds.
	KAMF []byte

	// DownlinkNASCount is the 5G downlink NAS COUNT the AMF used to derive
	// K'ASME, which the UE rebuilds from the 8 LSB the Handover Command carried
	// in the N1 mode to S1 mode NAS transparent container.
	DownlinkNASCount nas.Count

	// UplinkNASCount is the UE's current 5G uplink NAS COUNT. The mapped EPS
	// context continues both counts rather than resetting them (§8.6.1).
	UplinkNASCount nas.Count

	// Ciphering and Integrity are the EPS NAS algorithms the AMF provisioned in
	// an earlier 5G security mode command. The UE cannot be told them now.
	Ciphering uint8
	Integrity uint8

	// EKSI is the key set identifier value, taken from the ngKSI.
	EKSI uint8
}

// EstimateDownlinkNASCount rebuilds the downlink 5G NAS COUNT the AMF used from
// the 8 least significant bits the Handover Command carried and the count the UE
// holds. TS 33.501 §8.3.2 step 8 requires the estimate to exceed the stored
// value, so a replayed container cannot roll the count back.
func EstimateDownlinkNASCount(stored nas.Count, sequenceNumber uint8) (nas.Count, error) {
	estimate := nas.MakeCount(stored.Overflow(), sequenceNumber)
	if estimate.Value() <= stored.Value() {
		// The sequence number wrapped past the stored one.
		estimate = nas.MakeCount(stored.Overflow()+1, sequenceNumber)
	}

	if estimate.Value() <= stored.Value() {
		return 0, fmt.Errorf("s1enb: downlink NAS COUNT estimate %d does not exceed the stored %d",
			estimate.Value(), stored.Value())
	}

	return estimate, nil
}

// InstallMappedSecurityContext takes into use the EPS security context this UE
// derives on a handover from 5GS (TS 33.501 §8.3.2 step 9): K'ASME from K_AMF
// and the downlink 5G NAS COUNT, then the EPS NAS keys from the algorithms the
// AMF provisioned earlier. Both NAS COUNTs continue from the 5G context.
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

	// §8.6.1: the EPS NAS COUNTs are the 5G context's, not zero. The uplink one
	// is what the post-handover TRACKING AREA UPDATE REQUEST is protected with.
	ue.ulCount = in.UplinkNASCount.SQN()

	return nil
}

// EPSNASAlgorithmsInUse reports the pair the mapped context was built with.
func (ue *UE) EPSNASAlgorithmsInUse() (ciphering, integrity uint8) {
	return ue.eea, ue.eia
}

// MappedNASKeys reports the EPS NAS keys the mapped context produced.
func (ue *UE) MappedNASKeys() (knasEnc, knasInt [16]byte) {
	return ue.knasEnc, ue.knasInt
}

// MappedKASME reports the K'ASME this UE derived.
func (ue *UE) MappedKASME() []byte { return ue.kasme }

// NewUnboundUE creates a UE with no serving eNB, for deriving and checking a
// mapped security context on its own.
func NewUnboundUE() *UE {
	return &UE{netCapEEA: 0xf0, netCapEIA: 0x70, pti: 1}
}
