// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/per"
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
	_                     [0]struct{} `per:"extseq"`
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	_                     ieExtensions `per:",skip"`
}

// UPTransportLayerInformation CHOICE alternatives. The CHOICE is not
// extensible, so choice-Extensions is a plain alternative index.
const (
	upTransportLayerInformationGTPTunnel = iota
	upTransportLayerInformationChoiceExtensions

	upTransportLayerInformationAlternatives = 2
)

// UPTransportLayerInformation ::= CHOICE { gTPTunnel, choice-Extensions } —
// TS 38.413 §9.3.2.2. S1AP carries the user-plane endpoint as a bare
// TransportLayerAddress and GTP-TEID pair inside each E-RAB item instead.
type UPTransportLayerInformation struct {
	GTPTunnel GTPTunnel
}

func (u UPTransportLayerInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	if err := per.EncodeConstrainedWholeNumber(w, enc, 0, upTransportLayerInformationAlternatives-1, upTransportLayerInformationGTPTunnel); err != nil {
		return err
	}

	return u.GTPTunnel.MarshalPER(w, enc)
}

func (u *UPTransportLayerInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	idx, err := per.DecodeConstrainedWholeNumber(r, enc, 0, upTransportLayerInformationAlternatives-1)
	if err != nil {
		return err
	}

	if idx == upTransportLayerInformationChoiceExtensions {
		return decodeChoiceExtension(r, enc, "UPTransportLayerInformation")
	}

	return u.GTPTunnel.UnmarshalPER(r, enc)
}

// UPTransportLayerInformationItem ::= SEQUENCE { nGU-UP-TNLInformation,
// iE-Extensions OPTIONAL } (extensible).
type UPTransportLayerInformationItem struct {
	_                   [0]struct{} `per:"extseq"`
	NGUUPTNLInformation UPTransportLayerInformation
	_                   ieExtensions `per:",skip"`
}

// UPTransportLayerInformationList ::= SEQUENCE
// (SIZE(1..maxnoofMultiConnectivityMinusOne)) OF
// UPTransportLayerInformationItem — the additional legs of a multi-connectivity
// session, so the bound excludes the first.
type UPTransportLayerInformationList []UPTransportLayerInformationItem

// UPTransportLayerInformationPairItem ::= SEQUENCE { uL-NGU-UP-TNLInformation,
// dL-NGU-UP-TNLInformation, iE-Extensions OPTIONAL } (extensible).
type UPTransportLayerInformationPairItem struct {
	_                     [0]struct{} `per:"extseq"`
	ULNGUUPTNLInformation UPTransportLayerInformation
	DLNGUUPTNLInformation UPTransportLayerInformation
	_                     ieExtensions `per:",skip"`
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
	_                     [0]struct{} `per:"extseq"`
	ULNGUUPTNLInformation UPTransportLayerInformation
	DLNGUUPTNLInformation UPTransportLayerInformation
	_                     ieExtensions `per:",skip"`
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
	v, err := decodeExtensibleInt(r, enc, 1, 32, "DRB-ID")
	if err != nil {
		return err
	}

	*d = DRBID(v)

	return nil
}

// DataForwardingResponseDRBItem ::= SEQUENCE { dRB-ID,
// dLForwardingUP-TNLInformation OPTIONAL, uLForwardingUP-TNLInformation
// OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type DataForwardingResponseDRBItem struct {
	_                            [0]struct{} `per:"extseq"`
	DRBID                        DRBID
	DLForwardingUPTNLInformation *UPTransportLayerInformation `per:",optional"`
	ULForwardingUPTNLInformation *UPTransportLayerInformation `per:",optional"`
	_                            ieExtensions                 `per:",skip"`
}

// DataForwardingResponseDRBList ::= SEQUENCE (SIZE(1..maxnoofDRBs)) OF
// DataForwardingResponseDRBItem.
type DataForwardingResponseDRBList []DataForwardingResponseDRBItem

// encodeExtensibleInt encodes an INTEGER whose constraint carries an extension
// marker but defines no extension additions: X.691 §13.1 spends one bit, then
// the root value follows. A value outside the root is refused rather than sent
// as an extension, because 3GPP has defined no such addition to send.
func encodeExtensibleInt(w *per.Writer, enc per.Encoding, lb, ub, n int64) error {
	if n < lb || n > ub {
		return fmt.Errorf("value %d outside the extension root %d..%d", n, lb, ub)
	}

	w.WriteBit(false)

	return per.EncodeInteger(w, enc, per.Bounds{LB: lb, HasLB: true, UB: ub, HasUB: true}, n)
}

// decodeExtensibleInt consumes an extension value before reporting it, so the
// reader is left positioned after the field: X.691 §13.1 sends an out-of-root
// value as an unconstrained integer (§13.2.4-13.2.6), which carries its own
// length determinant and is therefore skippable without knowing the addition.
// Criticality then decides what to do with the enclosing IE (§10.3.1 case 6).
func decodeExtensibleInt(r *per.Reader, enc per.Encoding, lb, ub int64, name string) (int64, error) {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: lb, HasLB: true, UB: ub, HasUB: true, Extensible: true})
	if err != nil {
		return 0, err
	}

	if v < lb || v > ub {
		return 0, fmt.Errorf("%w: %s extension value %d", errNotComprehended, name, v)
	}

	return v, nil
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

// TransportLayerAddress ::= BIT STRING (SIZE(1..160, ...)). Holds the address
// octets (IPv4 = 4, IPv6 = 16).
type TransportLayerAddress []byte

func (a TransportLayerAddress) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeBitString(w, enc, 1, 160, true, true, true, a, len(a)*8)
}

func (a *TransportLayerAddress) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, nbits, err := per.DecodeBitString(r, enc, 1, 160, true, true, true)
	if err != nil {
		return err
	}

	*a = TransportLayerAddress(b[:(nbits+7)/8])

	return nil
}

// IPs returns the addresses the bit string carries. TS 38.414 §5.1 packs an
// IPv4 address, an IPv6 address, or both with the IPv4 one first; a return is
// invalid when that address is absent.
func (a TransportLayerAddress) IPs() (ipv4, ipv6 netip.Addr) {
	switch len(a) {
	case 4:
		ipv4, _ = netip.AddrFromSlice(a)
	case 16:
		ipv6, _ = netip.AddrFromSlice(a)
	case 20:
		ipv4, _ = netip.AddrFromSlice(a[:4])
		ipv6, _ = netip.AddrFromSlice(a[4:])
	}

	return ipv4, ipv6
}

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
