// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

// HandoverType ::= ENUMERATED { intralte, ltetoutran, ltetogeran, utrantolte,
// gerantolte, ..., eps-to-5gs, fivegs-to-eps } (extensible). Only intralte is in
// scope for S1 handover within E-UTRAN (TS 36.413).
type HandoverType uint8

const (
	HandoverTypeIntraLTE HandoverType = iota
	HandoverTypeLTEtoUTRAN
	HandoverTypeLTEtoGERAN
	HandoverTypeUTRANtoLTE
	HandoverTypeGERANtoLTE

	handoverTypeRootCount = 5
)

func (t HandoverType) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeEnumerated(w, enc, handoverTypeRootCount, true, int64(t))
}

func (t *HandoverType) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeEnumerated(r, enc, handoverTypeRootCount, true)
	if err != nil {
		return err
	}

	if idx >= handoverTypeRootCount {
		return fmt.Errorf("s1ap: unsupported HandoverType extension value")
	}

	*t = HandoverType(idx)

	return nil
}

// TransparentContainer ::= OCTET STRING. The Source-to-Target and
// Target-to-Source containers carry the source/target RAN's RRC information
// (TS 36.413); the MME relays them opaquely (TS 36.300).
type TransparentContainer []byte

func (c TransparentContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *TransparentContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = TransparentContainer(b)

	return nil
}

// TargeteNB-ID ::= SEQUENCE { global-ENB-ID, selected-TAI, iE-Extensions
// OPTIONAL } (extensible). It names the target eNB and the TAI of the target
// cell (TS 36.413).
type TargeteNBID struct {
	_           [0]struct{} `per:"extseq"`
	GlobalENBID GlobalENBID
	SelectedTAI TAI
	_           ieExtensions `per:",skip"`
}

// TargetID ::= CHOICE { targeteNB-ID, targetRNC-ID, cGI, ..., targetgNgRanNode-ID }.
// Only the first root alternative (targeteNB-ID), an intra-E-UTRAN handover
// target, is modeled; the others are out of scope (TS 36.413).
type TargetID struct {
	TargeteNBID TargeteNBID
}

const targetIDRootCount = 3 // targeteNB-ID, targetRNC-ID, cGI

func (t TargetID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, targetIDRootCount-1, 0); err != nil {
		return err
	}

	return t.TargeteNBID.MarshalPER(w, enc)
}

func (t *TargetID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	isExt, err := r.ReadBit()
	if err != nil {
		return err
	}

	if isExt {
		return fmt.Errorf("s1ap: unsupported TargetID extension alternative")
	}

	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, targetIDRootCount-1)
	if err != nil {
		return err
	}

	if idx != 0 {
		return fmt.Errorf("s1ap: unsupported TargetID alternative %d (only targeteNB-ID)", idx)
	}

	return t.TargeteNBID.UnmarshalPER(r, enc)
}

// ERABToBeSetupItemHOReq ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, e-RABlevelQosParameters, iE-Extensions OPTIONAL } (extensible). The
// transport address and TEID are the serving GW's S1-U uplink endpoint the
// target eNB sends uplink user data to (TS 36.413). The optional
// Data-Forwarding-Not-Possible extension is not modeled; data forwarding is not
// requested.
type ERABToBeSetupItemHOReq struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	QoS                   ERABLevelQoSParameters
	_                     ieExtensions `per:",skip"`
}

// ERABAdmittedItem ::= SEQUENCE { e-RAB-ID, transportLayerAddress, gTP-TEID,
// dL-transportLayerAddress OPTIONAL, dL-gTP-TEID OPTIONAL, uL-TransportLayerAddress
// OPTIONAL, uL-GTP-TEID OPTIONAL, iE-Extensions OPTIONAL } (extensible). The
// mandatory transport address and TEID are the target eNB's S1-U downlink
// endpoint; the optional DL/UL pairs are data-forwarding tunnels the MME does not
// use (TS 36.413).
type ERABAdmittedItem struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	DLTransportLayerAddr  TransportLayerAddress `per:",optional"`
	DLGTPTEID             *GTPTEID              `per:",optional"`
	ULTransportLayerAddr  TransportLayerAddress `per:",optional"`
	ULGTPTEID             *GTPTEID              `per:",optional"`
	_                     ieExtensions          `per:",skip"`
}
