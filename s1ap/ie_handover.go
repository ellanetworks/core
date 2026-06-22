// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/s1ap/aper"
)

// HandoverType ::= ENUMERATED { intralte, ltetoutran, ltetogeran, utrantolte,
// gerantolte, ..., eps-to-5gs, fivegs-to-eps } (extensible). Only intralte is in
// scope for S1 handover within E-UTRAN (TS 36.413 §9.2.1.4).
type HandoverType uint8

const (
	HandoverTypeIntraLTE HandoverType = iota
	HandoverTypeLTEtoUTRAN
	HandoverTypeLTEtoGERAN
	HandoverTypeUTRANtoLTE
	HandoverTypeGERANtoLTE

	handoverTypeRootCount = 5
)

func (t HandoverType) encode(w *aper.Writer) error {
	return w.WriteEnum(int(t), handoverTypeRootCount, true, false)
}

func decodeHandoverType(r *aper.Reader) (HandoverType, error) {
	idx, ext, err := r.ReadEnum(handoverTypeRootCount, true)
	if err != nil {
		return 0, err
	}

	if ext {
		return 0, fmt.Errorf("s1ap: unsupported HandoverType extension value")
	}

	return HandoverType(idx), nil
}

// TransparentContainer ::= OCTET STRING. The Source-to-Target and
// Target-to-Source containers carry the source/target RAN's RRC information
// (TS 36.413 §9.2.1.x); the MME relays them opaquely (TS 36.300).
type TransparentContainer []byte

func (c TransparentContainer) encode(w *aper.Writer) error {
	return w.WriteOctetString(c, 0, aper.Unbounded, false)
}

func decodeTransparentContainer(r *aper.Reader) (TransparentContainer, error) {
	b, err := r.ReadOctetString(0, aper.Unbounded, false)
	return TransparentContainer(b), err
}

// TargeteNB-ID ::= SEQUENCE { global-ENB-ID, selected-TAI, iE-Extensions
// OPTIONAL } (extensible). It names the target eNB and the TAI of the target
// cell (TS 36.413 §9.2.1.x).
type TargeteNBID struct {
	GlobalENBID GlobalENBID
	SelectedTAI TAI
}

func (t TargeteNBID) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := t.GlobalENBID.encode(w); err != nil {
		return err
	}

	return t.SelectedTAI.encode(w)
}

func decodeTargeteNBID(r *aper.Reader) (TargeteNBID, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return TargeteNBID{}, err
	}

	enb, err := decodeGlobalENBID(r)
	if err != nil {
		return TargeteNBID{}, err
	}

	tai, err := decodeTAI(r)
	if err != nil {
		return TargeteNBID{}, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return TargeteNBID{}, err
	}

	return TargeteNBID{GlobalENBID: enb, SelectedTAI: tai}, nil
}

// TargetID ::= CHOICE { targeteNB-ID, targetRNC-ID, cGI, ..., targetgNgRanNode-ID }.
// Only the first root alternative (targeteNB-ID), an intra-E-UTRAN handover
// target, is modeled; the others are out of scope (TS 36.413 §9.2.1.40).
type TargetID struct {
	TargeteNBID TargeteNBID
}

const targetIDRootCount = 3 // targeteNB-ID, targetRNC-ID, cGI

func (t TargetID) encode(w *aper.Writer) error {
	if err := w.WriteChoiceIndex(0, targetIDRootCount, true, false); err != nil {
		return err
	}

	return t.TargeteNBID.encode(w)
}

func decodeTargetID(r *aper.Reader) (TargetID, error) {
	idx, isExt, err := r.ReadChoiceIndex(targetIDRootCount, true)
	if err != nil {
		return TargetID{}, err
	}

	if isExt || idx != 0 {
		return TargetID{}, fmt.Errorf("s1ap: unsupported TargetID alternative %d (only targeteNB-ID)", idx)
	}

	enb, err := decodeTargeteNBID(r)
	if err != nil {
		return TargetID{}, err
	}

	return TargetID{TargeteNBID: enb}, nil
}

// ERABToBeSetupItemHOReq ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, e-RABlevelQosParameters, iE-Extensions OPTIONAL } (extensible). The
// transport address and TEID are the serving GW's S1-U uplink endpoint the
// target eNB sends uplink user data to (TS 36.413 §9.1.5.4). The optional
// Data-Forwarding-Not-Possible extension is not modeled; data forwarding is not
// requested.
type ERABToBeSetupItemHOReq struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	QoS                   ERABLevelQoSParameters
}

func (it ERABToBeSetupItemHOReq) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{false})

	if err := it.ERABID.encode(w); err != nil {
		return err
	}

	if err := it.TransportLayerAddress.encode(w); err != nil {
		return err
	}

	if err := it.GTPTEID.encode(w); err != nil {
		return err
	}

	return it.QoS.encode(w)
}

func decodeERABToBeSetupItemHOReq(r *aper.Reader) (ERABToBeSetupItemHOReq, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 1)
	if err != nil {
		return ERABToBeSetupItemHOReq{}, err
	}

	var it ERABToBeSetupItemHOReq

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if it.TransportLayerAddress, err = decodeTransportLayerAddress(r); err != nil {
		return it, err
	}

	if it.GTPTEID, err = decodeGTPTEID(r); err != nil {
		return it, err
	}

	if it.QoS, err = decodeERABLevelQoSParameters(r); err != nil {
		return it, err
	}

	if err := skipSequenceExtensions(r, opt[0], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

func decodeERABToBeSetupListHOReq(r *aper.Reader) ([]ERABToBeSetupItemHOReq, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABToBeSetupItemHOReq)
}

// ERABAdmittedItem ::= SEQUENCE { e-RAB-ID, transportLayerAddress, gTP-TEID,
// dL-transportLayerAddress OPTIONAL, dL-gTP-TEID OPTIONAL, uL-TransportLayerAddress
// OPTIONAL, uL-GTP-TEID OPTIONAL, iE-Extensions OPTIONAL } (extensible). The
// mandatory transport address and TEID are the target eNB's S1-U downlink
// endpoint; the optional DL/UL pairs are data-forwarding tunnels the MME does not
// use (TS 36.413 §9.1.5.5).
type ERABAdmittedItem struct {
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	DLTransportLayerAddr  TransportLayerAddress
	DLGTPTEID             *GTPTEID
	ULTransportLayerAddr  TransportLayerAddress
	ULGTPTEID             *GTPTEID
}

func (it ERABAdmittedItem) encode(w *aper.Writer) error {
	w.WriteSequencePreamble(true, false, []bool{
		it.DLTransportLayerAddr != nil,
		it.DLGTPTEID != nil,
		it.ULTransportLayerAddr != nil,
		it.ULGTPTEID != nil,
		false,
	})

	if err := it.ERABID.encode(w); err != nil {
		return err
	}

	if err := it.TransportLayerAddress.encode(w); err != nil {
		return err
	}

	if err := it.GTPTEID.encode(w); err != nil {
		return err
	}

	if it.DLTransportLayerAddr != nil {
		if err := it.DLTransportLayerAddr.encode(w); err != nil {
			return err
		}
	}

	if it.DLGTPTEID != nil {
		if err := it.DLGTPTEID.encode(w); err != nil {
			return err
		}
	}

	if it.ULTransportLayerAddr != nil {
		if err := it.ULTransportLayerAddr.encode(w); err != nil {
			return err
		}
	}

	if it.ULGTPTEID != nil {
		if err := it.ULGTPTEID.encode(w); err != nil {
			return err
		}
	}

	return nil
}

func decodeERABAdmittedItem(r *aper.Reader) (ERABAdmittedItem, error) {
	extPresent, opt, err := r.ReadSequencePreamble(true, 5)
	if err != nil {
		return ERABAdmittedItem{}, err
	}

	var it ERABAdmittedItem

	if it.ERABID, err = decodeERABID(r); err != nil {
		return it, err
	}

	if it.TransportLayerAddress, err = decodeTransportLayerAddress(r); err != nil {
		return it, err
	}

	if it.GTPTEID, err = decodeGTPTEID(r); err != nil {
		return it, err
	}

	if opt[0] {
		if it.DLTransportLayerAddr, err = decodeTransportLayerAddress(r); err != nil {
			return it, err
		}
	}

	if opt[1] {
		teid, err := decodeGTPTEID(r)
		if err != nil {
			return it, err
		}

		it.DLGTPTEID = &teid
	}

	if opt[2] {
		if it.ULTransportLayerAddr, err = decodeTransportLayerAddress(r); err != nil {
			return it, err
		}
	}

	if opt[3] {
		teid, err := decodeGTPTEID(r)
		if err != nil {
			return it, err
		}

		it.ULGTPTEID = &teid
	}

	if err := skipSequenceExtensions(r, opt[4], extPresent); err != nil {
		return it, err
	}

	return it, nil
}

func decodeERABAdmittedList(r *aper.Reader) ([]ERABAdmittedItem, error) {
	return decodeItemList(r, maxnoofERABs, decodeERABAdmittedItem)
}
