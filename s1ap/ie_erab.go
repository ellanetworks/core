// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"fmt"
	"net/netip"

	"github.com/ellanetworks/core/per"
)

// ERABID ::= INTEGER (0..15, ...) (extensible).
type ERABID uint8

func (id ERABID) MarshalPER(w *per.Writer, enc per.Encoding) error {
	w.WriteBit(false)

	return per.EncodeConstrainedWholeNumber(w, enc, 0, 15, int64(id))
}

func (id *ERABID) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	ext, err := r.ReadBit()
	if err != nil {
		return err
	}

	if ext {
		return fmt.Errorf("%w: E-RAB-ID extension value", errNotComprehended)
	}

	v, err := per.DecodeConstrainedWholeNumber(r, enc, 0, 15)
	if err != nil {
		return err
	}

	*id = ERABID(v)

	return nil
}

// QCI ::= INTEGER (0..255).
type QCI uint8

func (q QCI) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true}, int64(q))
}

func (q *QCI) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 255, HasUB: true})
	if err != nil {
		return err
	}

	*q = QCI(v)

	return nil
}

// Pre-emptionCapability ::= ENUMERATED (not extensible).
type PreemptionCapability uint8

const (
	PreemptionShallNotTrigger PreemptionCapability = iota
	PreemptionMayTrigger
)

// Pre-emptionVulnerability ::= ENUMERATED (not extensible).
type PreemptionVulnerability uint8

const (
	PreemptionNotPreemptable PreemptionVulnerability = iota
	PreemptionPreemptable
)

// AllocationAndRetentionPriority ::= SEQUENCE { priorityLevel,
// pre-emptionCapability, pre-emptionVulnerability, iE-Extensions OPTIONAL }
// (extensible). PriorityLevel ::= INTEGER (0..15).
type AllocationAndRetentionPriority struct {
	_                       [0]struct{}             `per:"extseq"`
	PriorityLevel           uint8                   `per:",range:0..15"`
	PreemptionCapability    PreemptionCapability    `per:"ENUMERATED,range:0..1"`
	PreemptionVulnerability PreemptionVulnerability `per:"ENUMERATED,range:0..1"`
	_                       ieExtensions            `per:",skip"`
}

// GBRQosInformation ::= SEQUENCE { four BitRates, iE-Extensions OPTIONAL }
// (extensible).
type GBRQosInformation struct {
	_                   [0]struct{} `per:"extseq"`
	MaximumBitrateDL    BitRate
	MaximumBitrateUL    BitRate
	GuaranteedBitrateDL BitRate
	GuaranteedBitrateUL BitRate
	_                   ieExtensions `per:",skip"`
}

// ERABLevelQoSParameters ::= SEQUENCE { qCI, allocationRetentionPriority,
// gbrQosInformation OPTIONAL, iE-Extensions OPTIONAL } (extensible).
type ERABLevelQoSParameters struct {
	_   [0]struct{} `per:"extseq"`
	QCI QCI
	ARP AllocationAndRetentionPriority
	GBR *GBRQosInformation `per:",optional"`
	_   ieExtensions       `per:",skip"`
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

// IPs returns the addresses the bit string carries. TS 36.414 §5.1 packs an
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

// GTPTEID ::= OCTET STRING (SIZE(4)).
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

// ERABToBeSetupItemCtxtSUReq ::= SEQUENCE { e-RAB-ID, e-RABlevelQoSParameters,
// transportLayerAddress, gTP-TEID, nAS-PDU OPTIONAL, iE-Extensions OPTIONAL }
// (extensible).
type ERABToBeSetupItemCtxtSUReq struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	QoS                   ERABLevelQoSParameters
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	NASPDU                NASPDU       `per:",optional"`
	_                     ieExtensions `per:",skip"`
}

// ERABSetupItemCtxtSURes ::= SEQUENCE { e-RAB-ID, transportLayerAddress,
// gTP-TEID, iE-Extensions OPTIONAL } (extensible).
type ERABSetupItemCtxtSURes struct {
	_                     [0]struct{} `per:"extseq"`
	ERABID                ERABID
	TransportLayerAddress TransportLayerAddress
	GTPTEID               GTPTEID
	_                     ieExtensions `per:",skip"`
}

// ERABItem ::= SEQUENCE { e-RAB-ID, cause, iE-Extensions OPTIONAL } (extensible).
type ERABItem struct {
	_      [0]struct{} `per:"extseq"`
	ERABID ERABID
	Cause  Cause
	_      ieExtensions `per:",skip"`
}
