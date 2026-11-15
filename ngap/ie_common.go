// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

// nameMaxLen bounds AMFName / RANNodeName (PrintableString (SIZE(1..150,...))).
const nameMaxLen = 150

// PLMNIdentity ::= OCTET STRING (SIZE(3)).
type PLMNIdentity [3]byte

// Name is an AMFName / RANNodeName: PrintableString (SIZE(1..150,...)). In the
// ALIGNED variant PrintableString encodes 8 bits per character (X.691 §30.5.2).
type Name string

func (n Name) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeKnownMultiplierString(w, enc, per.CharPrintableString, 1, nameMaxLen, true, true, true, string(n))
}

func (n *Name) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	s, err := per.DecodeKnownMultiplierString(r, enc, per.CharPrintableString, 1, nameMaxLen, true, true, true)
	if err != nil {
		return err
	}

	*n = Name(s)

	return nil
}

type AMFNameVisibleString string

func (n AMFNameVisibleString) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeKnownMultiplierString(w, enc, per.CharVisibleString, 1, nameMaxLen, true, true, true, string(n))
}

func (n *AMFNameVisibleString) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	s, err := per.DecodeKnownMultiplierString(r, enc, per.CharVisibleString, 1, nameMaxLen, true, true, true)
	if err != nil {
		return err
	}

	*n = AMFNameVisibleString(s)

	return nil
}

type AMFNameUTF8String string

func (n AMFNameUTF8String) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeString(w, enc, []byte(n))
}

func (n *AMFNameUTF8String) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeString(r, enc)
	if err != nil {
		return err
	}

	*n = AMFNameUTF8String(b)

	return nil
}

type ExtendedAMFName struct {
	AMFNameVisibleString *AMFNameVisibleString  `per:",optional"`
	AMFNameUTF8String    *AMFNameUTF8String     `per:",optional"`
	iEExtensions         extensionMeta          `per:",exttable"`
	extensions           per.ExtensionAdditions `per:",extraw"`
}

// GTP-TEID ::= OCTET STRING (SIZE(4)).
type GTPTEID uint32

func (t GTPTEID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 4, 4, true, true, false, []byte{byte(t >> 24), byte(t >> 16), byte(t >> 8), byte(t)})
}

func (t *GTPTEID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 4, 4, true, true, false)
	if err != nil {
		return err
	}

	*t = GTPTEID(uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]))

	return nil
}

// GTPTunnel ::= SEQUENCE { transportLayerAddress, gTP-TEID, iE-Extensions
// OPTIONAL } (extensible) — TS 38.413 §9.3.2.2.
type GTPTunnel struct {
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

// UPTransportLayerInformation ::= CHOICE { gTPTunnel, choice-Extensions } —
// TS 38.413 §9.3.2.2. S1AP carries the user-plane endpoint as a bare
// TransportLayerAddress and GTP-TEID pair inside each E-RAB item instead.
type UPTransportLayerInformation struct {
	GTPTunnel       *GTPTunnel       `per:",choice:0"`
	ChoiceExtension *SingleContainer `per:",choice:1,extie"`
}

// UPTransportLayerInformationItem ::= SEQUENCE { nGU-UP-TNLInformation,
// iE-Extensions OPTIONAL } (extensible).
type UPTransportLayerInformationItem struct {
	NGUUPTNLInformation   UPTransportLayerInformation
	CommonNetworkInstance *CommonNetworkInstance `per:",extie:IDCommonNetworkInstance,crit:ignore"`
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

// UPTransportLayerInformationList ::= SEQUENCE
// (SIZE(1..maxnoofMultiConnectivityMinusOne)) OF
// UPTransportLayerInformationItem — the additional legs of a multi-connectivity
// session, so the bound excludes the first.
type UPTransportLayerInformationList []UPTransportLayerInformationItem

// UPTransportLayerInformationPairItem ::= SEQUENCE { uL-NGU-UP-TNLInformation,
// dL-NGU-UP-TNLInformation, iE-Extensions OPTIONAL } (extensible).
type UPTransportLayerInformationPairItem struct {
	ULNGUUPTNLInformation UPTransportLayerInformation
	DLNGUUPTNLInformation UPTransportLayerInformation
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

// UPTransportLayerInformationPairList ::= SEQUENCE
// (SIZE(1..maxnoofMultiConnectivityMinusOne)) OF
// UPTransportLayerInformationPairItem.
type UPTransportLayerInformationPairList []UPTransportLayerInformationPairItem

// ULNGUUPTNLModifyItem ::= SEQUENCE { uL-NGU-UP-TNLInformation,
// dL-NGU-UP-TNLInformation, iE-Extensions OPTIONAL } (extensible). Structurally
// identical to UPTransportLayerInformationPairItem but a distinct ASN.1 type,
// and its list is bounded by maxnoofMultiConnectivity rather than one less.
type ULNGUUPTNLModifyItem struct {
	ULNGUUPTNLInformation          UPTransportLayerInformation
	DLNGUUPTNLInformation          UPTransportLayerInformation
	RedundantULNGUUPTNLInformation *UPTransportLayerInformation `per:",extie:IDRedundantULNGUUPTNLInformation,crit:ignore"`
	RedundantDLNGUUPTNLInformation *UPTransportLayerInformation `per:",extie:IDRedundantDLNGUUPTNLInformation,crit:ignore"`
	iEExtensions                   extensionMeta                `per:",exttable"`
	extensions                     per.ExtensionAdditions       `per:",extraw"`
}

// ULNGUUPTNLModifyList ::= SEQUENCE (SIZE(1..maxnoofMultiConnectivity)) OF
// UL-NGU-UP-TNLModifyItem.
type ULNGUUPTNLModifyList []ULNGUUPTNLModifyItem

// DRBID ::= INTEGER (1..32, ...) — TS 38.413 §9.3.1.53.
type DRBID uint8

func (d DRBID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeExtensibleInt(w, enc, 1, 32, int64(d))
}

func (d *DRBID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := decodeExtensibleInt[DRBID](r, enc, 1, 32)
	if err != nil {
		return err
	}

	*d = v

	return nil
}

func (v DRBID) Diagnose(*ap.Diagnosis[ProtocolIEID]) bool { return outsideRoot(int64(v), 1, 32) }

// DataForwardingResponseDRBItem ::= SEQUENCE { dRB-ID,
// dLForwardingUP-TNLInformation OPTIONAL, uLForwardingUP-TNLInformation
// OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type DataForwardingResponseDRBItem struct {
	DRBID                        DRBID
	DLForwardingUPTNLInformation *UPTransportLayerInformation `per:",optional"`
	ULForwardingUPTNLInformation *UPTransportLayerInformation `per:",optional"`
	iEExtensions                 extensionMeta                `per:",exttable"`
	extensions                   per.ExtensionAdditions       `per:",extraw"`
}

// DataForwardingResponseDRBList ::= SEQUENCE (SIZE(1..maxnoofDRBs)) OF
// DataForwardingResponseDRBItem.
type DataForwardingResponseDRBList []DataForwardingResponseDRBItem

type DataForwardingResponseERABListItem struct {
	ERABID                       ERABID
	DLForwardingUPTNLInformation UPTransportLayerInformation
	iEExtensions                 extensionMeta          `per:",exttable"`
	extensions                   per.ExtensionAdditions `per:",extraw"`
}

type DataForwardingResponseERABList []DataForwardingResponseERABListItem

func (l DataForwardingResponseERABList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofERABs, []DataForwardingResponseERABListItem(l))
}

func (l *DataForwardingResponseERABList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[DataForwardingResponseERABListItem](r, enc, 1, maxnoofERABs)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type AdditionalDLUPTNLInformationForHOItem struct {
	AdditionalDLNGUUPTNLInformation          UPTransportLayerInformation
	AdditionalQosFlowSetupResponse           QosFlowListWithDataForwarding
	AdditionalDLForwardingUPTNLInformation   *UPTransportLayerInformation `per:",optional"`
	AdditionalRedundantDLNGUUPTNLInformation *UPTransportLayerInformation `per:",extie:IDAdditionalRedundantDLNGUUPTNLInformation,crit:ignore"`
	iEExtensions                             extensionMeta                `per:",exttable"`
	extensions                               per.ExtensionAdditions       `per:",extraw"`
}

type AdditionalDLUPTNLInformationForHOList []AdditionalDLUPTNLInformationForHOItem

func (l AdditionalDLUPTNLInformationForHOList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofMultiConnectivityMinusOne, []AdditionalDLUPTNLInformationForHOItem(l))
}

func (l *AdditionalDLUPTNLInformationForHOList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[AdditionalDLUPTNLInformationForHOItem](r, enc, 1, maxnoofMultiConnectivityMinusOne)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type UserPlaneFailureType uint8

const (
	UserPlaneFailureTypeGTPUErrorIndicationReceived UserPlaneFailureType = iota
	UserPlaneFailureTypeUPPathFailure
)

type UserPlaneFailureIndication struct {
	UserPlaneFailureType  UserPlaneFailureType
	ULNGUUPTNLInformation UPTransportLayerInformation
	DLNGUUPTNLInformation UPTransportLayerInformation
	iEExtensions          extensionMeta          `per:",exttable"`
	extensions            per.ExtensionAdditions `per:",extraw"`
}

func encodeExtensibleInt(w *per.Writer, enc per.Encoding, lb, ub, n int64) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: lb, HasLB: true, UB: ub, HasUB: true, Extensible: true}, n)
}

func outsideRoot(v, lb, ub int64) bool { return v < lb || v > ub }

func decodeExtensibleInt[T per.Integer](r *per.Reader, enc per.Encoding, lb, ub int64) (T, error) {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: lb, HasLB: true, UB: ub, HasUB: true, Extensible: true})
	if err != nil {
		return 0, err
	}

	if t := T(v); int64(t) == v {
		return t, nil
	}

	return 0, per.ErrOverflow
}

// TransferContainer is an OCTET STRING (CONTAINING XxxTransfer): a nested PER
// encoding NGAP carries opaquely between the NG-RAN node and the SMF. S1AP has
// no such construct.
type TransferContainer []byte

func (c TransferContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *TransferContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = TransferContainer(b)

	return nil
}

type TransportLayerAddress = ap.TransportLayerAddress

// Most significant bit first, matching BIT STRING storage.
func uintToBits(v uint64, nbits int) []byte {
	out := make([]byte, (nbits+7)/8)

	for i := 0; i < nbits; i++ {
		if v&(1<<uint(nbits-1-i)) != 0 {
			out[i/8] |= 1 << uint(7-i%8)
		}
	}

	return out
}

// nbits is clamped to the available bits so a caller cannot over-index on
// malformed input.
func bitsToUint(b []byte, nbits int) uint64 {
	if nbits > len(b)*8 {
		nbits = len(b) * 8
	}

	var v uint64

	for i := 0; i < nbits; i++ {
		if b[i/8]&(1<<uint(7-i%8)) != 0 {
			v |= 1 << uint(nbits-1-i)
		}
	}

	return v
}

const uint64RangeOctets = 8

func encodeUint64Range(w *per.Writer, enc per.Encoding, v uint64) error {
	if enc == per.Unaligned {
		w.WriteBits(v, 64)

		return nil
	}

	octets := 1
	for x := v >> 8; x != 0; x >>= 8 {
		octets++
	}

	return per.EncodeLength(w, enc, 1, uint64RangeOctets, true, int64(octets), func(int64) error {
		w.AlignToByte()

		for i := octets - 1; i >= 0; i-- {
			w.WriteBits(v>>(8*i)&0xff, 8)
		}

		return nil
	})
}

func decodeUint64Range(r *per.Reader, enc per.Encoding) (uint64, error) {
	if enc == per.Unaligned {
		return r.ReadBits(64)
	}

	var v uint64

	err := per.DecodeLength(r, enc, 1, uint64RangeOctets, true, func(count int64) error {
		r.AlignToByte()

		for range count {
			b, err := r.ReadBits(8)
			if err != nil {
				return err
			}

			v = v<<8 | b
		}

		return nil
	})

	return v, err
}
