// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"errors"
	"fmt"
	"net/netip"
	"runtime"
	"strconv"
	"unsafe"

	"github.com/cilium/ebpf"
	"github.com/ellanetworks/core/internal/logger"
	"go.uber.org/zap"
)

const (
	MaxSdfFilters     = 288 // 2 * MaxPolicies; must match MAX_SDF_FILTERS in C
	MaxPolicies       = 144 // must match MAX_POLICIES in C
	MaxRulesPerFilter = 12  // must match MAX_RULES_PER_FILTER in C
	SdfProtoAny       = 255
	SdfActionAllow    = 0
	SdfActionDeny     = 1
	NoFilterIndex     = 0 // reserved; means "no filtering"
)

// PdrInfo holds all data needed to program a PDR into the BPF maps.
// FAR and QER are embedded directly so that only a single BPF map lookup
// is required per packet.
// FarID and QerID are kept as bookkeeping fields so that UpdateFAR/UpdateQER
// messages can locate which PDRs reference a given FAR or QER.
type PdrInfo struct {
	SEID               uint64
	OuterHeaderRemoval uint8
	PdrID              uint32
	FarID              uint32
	QerID              uint32
	UrrID              uint32
	IMSI               string
	Far                FarInfo
	Qer                QerInfo
	FilterMapIndex     uint32 // 0 = no SDF filter
}

type PortRange struct {
	LowerBound uint16
	UpperBound uint16
}

func (bpfObjects *BpfObjects) PutPdrUplink(teid uint32, pdrInfo PdrInfo) error {
	logger.UpfLog.Debug("Put PDR Uplink", logger.TEID(teid))

	pdrToStore := ToN3N6EntrypointPdrInfo(pdrInfo)

	return bpfObjects.PdrsUplink.Put(teid, unsafe.Pointer(&pdrToStore))
}

func (bpfObjects *BpfObjects) PutPdrDownlink(addr netip.Addr, pdrInfo PdrInfo) error {
	logger.UpfLog.Debug("Put PDR Downlink", logger.IPAddress(addr.String()))

	pdrToStore := ToN3N6EntrypointPdrInfo(pdrInfo)

	if addr.Is4() {
		key := addr.As4()
		return bpfObjects.PdrsDownlinkIp4.Put(key, unsafe.Pointer(&pdrToStore))
	}

	prefix := addr.As16()

	return bpfObjects.PdrsDownlinkIp6.Put(prefix, unsafe.Pointer(&pdrToStore))
}

// framedIP4Key / framedIP6Key mirror the C LPM-trie keys in pdr_maps.h: a
// prefix length (bits) followed by the network-order address.
type framedIP4Key struct {
	PrefixLen uint32
	Addr      [4]byte
}

type framedIP6Key struct {
	PrefixLen uint32
	Addr      [16]byte
}

// PutFramedDownlink installs a framed route (TS 29.244 §5.16) as an LPM entry
// whose value is the owning session's UE address. A downlink packet inside the
// prefix redirects to that UE's live downlink PDR, so the framed route follows
// every downlink-PDR change without a separate copy to keep in sync. ueAddr
// must match the prefix's address family.
func (bpfObjects *BpfObjects) PutFramedDownlink(prefix netip.Prefix, ueAddr netip.Addr) error {
	prefix = prefix.Masked()

	logger.UpfLog.Debug("Put framed route", logger.IPAddress(prefix.String()))

	if prefix.Addr().Is4() {
		key := framedIP4Key{PrefixLen: uint32(prefix.Bits()), Addr: prefix.Addr().As4()}
		ueIP := ueAddr.As4()

		return bpfObjects.FramedDownlinkIp4.Put(key, unsafe.Pointer(&ueIP))
	}

	key := framedIP6Key{PrefixLen: uint32(prefix.Bits()), Addr: prefix.Addr().As16()}
	uePrefix := ueAddr.As16()

	return bpfObjects.FramedDownlinkIp6.Put(key, unsafe.Pointer(&uePrefix))
}

// DeleteFramedDownlink removes a framed route's LPM entry.
func (bpfObjects *BpfObjects) DeleteFramedDownlink(prefix netip.Prefix) error {
	prefix = prefix.Masked()

	logger.UpfLog.Debug("Delete framed route", logger.IPAddress(prefix.String()))

	if prefix.Addr().Is4() {
		key := framedIP4Key{PrefixLen: uint32(prefix.Bits()), Addr: prefix.Addr().As4()}
		return bpfObjects.FramedDownlinkIp4.Delete(key)
	}

	key := framedIP6Key{PrefixLen: uint32(prefix.Bits()), Addr: prefix.Addr().As16()}

	return bpfObjects.FramedDownlinkIp6.Delete(key)
}

// HasFramedDownlink reports whether a framed route's exact LPM entry is present.
func (bpfObjects *BpfObjects) HasFramedDownlink(prefix netip.Prefix) (bool, error) {
	prefix = prefix.Masked()

	var lookupErr error

	if prefix.Addr().Is4() {
		key := framedIP4Key{PrefixLen: uint32(prefix.Bits()), Addr: prefix.Addr().As4()}

		var val uint32

		lookupErr = bpfObjects.FramedDownlinkIp4.Lookup(key, &val)
	} else {
		key := framedIP6Key{PrefixLen: uint32(prefix.Bits()), Addr: prefix.Addr().As16()}

		var val [16]byte

		lookupErr = bpfObjects.FramedDownlinkIp6.Lookup(key, &val)
	}

	if lookupErr == nil {
		return true, nil
	}

	if errors.Is(lookupErr, ebpf.ErrKeyNotExist) {
		return false, nil
	}

	return false, lookupErr
}

func (bpfObjects *BpfObjects) DeletePdrUplink(teid uint32) error {
	logger.UpfLog.Debug("Delete PDR Uplink", logger.TEID(teid))
	return bpfObjects.PdrsUplink.Delete(teid)
}

func (bpfObjects *BpfObjects) DeletePdrDownlink(addr netip.Addr) error {
	logger.UpfLog.Debug("Delete PDR Downlink", logger.IPAddress(addr.String()))

	if addr.Is4() {
		key := addr.As4()
		return bpfObjects.PdrsDownlinkIp4.Delete(key)
	}

	key := addr.As16()

	return bpfObjects.PdrsDownlinkIp6.Delete(key)
}

// FarInfo holds Forwarding Action Rule parameters embedded directly in each PDR.
// RemoteIP and LocalIP use the IPv4-mapped IPv6 representation (::ffff:x.x.x.x)
// for IPv4 addresses and native IPv6 for IPv6 addresses, matching the C struct in6_addr layout.
type FarInfo struct {
	Action                uint8
	OuterHeaderCreation   uint8
	TeID                  uint32
	RemoteIP              [16]byte // struct in6_addr equivalent
	LocalIP               [16]byte // struct in6_addr equivalent
	TransportLevelMarking uint16
}

// IPToIn6Addr converts a netip.Addr to a [16]byte in6_addr.
// IPv4 addresses are stored as IPv4-mapped IPv6 (::ffff:x.x.x.x) via As16().
func IPToIn6Addr(addr netip.Addr) [16]byte {
	return addr.As16()
}

// In6AddrToIP converts a [16]byte in6_addr to a netip.Addr.
// IPv4-mapped addresses (::ffff:x.x.x.x) are returned as a pure IPv4 addr via Unmap().
func In6AddrToIP(b [16]byte) netip.Addr {
	addr := netip.AddrFrom16(b)
	if addr.Is4In6() {
		return addr.Unmap()
	}

	return addr
}

// QerInfo holds QoS Enforcement Rule parameters embedded directly in each PDR.
type QerInfo struct {
	GateStatusUL uint8
	GateStatusDL uint8
	Qfi          uint8
	MaxBitrateUL uint64
	MaxBitrateDL uint64
	StartUL      uint64
	StartDL      uint64
}

// SdfRule mirrors struct sdf_rule in pdr.h.
type SdfRule struct {
	RemoteIP  [16]byte // in6_addr: ::ffff:x.x.x.x for IPv4, native for IPv6
	PrefixLen uint8
	PortLow   uint16
	PortHigh  uint16
	Protocol  uint8
	Action    uint8
	_         [7]byte // padding to 32 bytes for verifier-friendly array indexing
}

// SdfFilterList mirrors struct sdf_filter_list in pdr.h.
type SdfFilterList struct {
	NumRules uint8
	_        [3]byte
	Rules    [MaxRulesPerFilter]SdfRule
}

func (bpfObjects *BpfObjects) NewUrr(seid uint64, id uint32) error {
	zeroVals := make([]uint64, runtime.NumCPU())

	err := bpfObjects.UrrMap.Put(N3N6EntrypointUrrKey{Seid: seid, UrrId: id}, zeroVals)
	if err != nil {
		return fmt.Errorf("failed to put urr id %d: %w", id, err)
	}

	return nil
}

func (bpfObjects *BpfObjects) DeleteUrr(seid uint64, id uint32) error {
	// A URR shared by several PDRs (the downlink and second PDR share one) is
	// deleted with the first PDR, so a later delete of the same key is a no-op.
	err := bpfObjects.UrrMap.Delete(N3N6EntrypointUrrKey{Seid: seid, UrrId: id})
	if err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
		return fmt.Errorf("failed to delete URR: %w", err)
	}

	return nil
}

// GetAndResetUrr returns the (SEID, id) byte counter summed across CPUs and
// resets it to zero.
func (bpfObjects *BpfObjects) GetAndResetUrr(seid uint64, id uint32) (uint64, error) {
	key := N3N6EntrypointUrrKey{Seid: seid, UrrId: id}

	var perCPU []uint64
	if err := bpfObjects.UrrMap.Lookup(key, &perCPU); err != nil {
		return 0, fmt.Errorf("failed to lookup URR: %w", err)
	}

	zeroes := make([]uint64, runtime.NumCPU())
	if err := bpfObjects.UrrMap.Update(key, zeroes, ebpf.UpdateAny); err != nil {
		return 0, fmt.Errorf("failed to reset URR: %w", err)
	}

	var total uint64
	for _, v := range perCPU {
		total += v
	}

	return total, nil
}

// AddUrr adds bytes to the (SEID, id) counter. The read-modify-write can drop
// datapath increments landing between the lookup and the update, the same bound
// GetAndResetUrr's reset carries; it runs only on the rare report-failure path.
func (bpfObjects *BpfObjects) AddUrr(seid uint64, id uint32, bytes uint64) error {
	key := N3N6EntrypointUrrKey{Seid: seid, UrrId: id}

	var perCPU []uint64
	if err := bpfObjects.UrrMap.Lookup(key, &perCPU); err != nil {
		return fmt.Errorf("failed to lookup URR: %w", err)
	}

	perCPU[0] += bytes
	if err := bpfObjects.UrrMap.Update(key, perCPU, ebpf.UpdateAny); err != nil {
		return fmt.Errorf("failed to restore URR: %w", err)
	}

	return nil
}

// ToN3N6EntrypointPdrInfo converts a PdrInfo (with embedded FAR and QER) to
// the auto-generated BPF map value type.
func ToN3N6EntrypointPdrInfo(defaultPdr PdrInfo) N3N6EntrypointPdrInfo {
	var pdrToStore N3N6EntrypointPdrInfo

	pdrToStore.LocalSeid = defaultPdr.SEID
	pdrToStore.OuterHeaderRemoval = defaultPdr.OuterHeaderRemoval
	pdrToStore.PdrId = defaultPdr.PdrID
	pdrToStore.UrrId = defaultPdr.UrrID

	imsiUint64, err := strconv.ParseUint(defaultPdr.IMSI, 10, 64)
	if err != nil {
		logger.UpfLog.Error("failed to parse IMSI", logger.IMSI(defaultPdr.IMSI), zap.Error(err))
		return pdrToStore
	}

	pdrToStore.Imsi = imsiUint64

	pdrToStore.Far.Action = defaultPdr.Far.Action
	pdrToStore.Far.OuterHeaderCreation = defaultPdr.Far.OuterHeaderCreation
	pdrToStore.Far.Teid = defaultPdr.Far.TeID
	pdrToStore.Far.Remoteip.In6U.U6Addr8 = defaultPdr.Far.RemoteIP
	pdrToStore.Far.Localip.In6U.U6Addr8 = defaultPdr.Far.LocalIP
	pdrToStore.Far.TransportLevelMarking = defaultPdr.Far.TransportLevelMarking

	pdrToStore.Qer.UlGateStatus = defaultPdr.Qer.GateStatusUL
	pdrToStore.Qer.DlGateStatus = defaultPdr.Qer.GateStatusDL
	pdrToStore.Qer.Qfi = defaultPdr.Qer.Qfi
	pdrToStore.Qer.UlMaximumBitrate = defaultPdr.Qer.MaxBitrateUL
	pdrToStore.Qer.DlMaximumBitrate = defaultPdr.Qer.MaxBitrateDL
	pdrToStore.Qer.UlStart = defaultPdr.Qer.StartUL
	pdrToStore.Qer.DlStart = defaultPdr.Qer.StartDL

	pdrToStore.FilterMapIndex = defaultPdr.FilterMapIndex

	return pdrToStore
}

// PutSdfFilterList writes a filter list into the sdf_filters BPF array.
func (b *BpfObjects) PutSdfFilterList(index uint32, list SdfFilterList) error {
	return b.SdfFilters.Put(index, unsafe.Pointer(&list))
}

// DeleteSdfFilterList zeroes the slot at index.
// BPF_MAP_TYPE_ARRAY cannot truly delete entries; zeroing is correct.
func (b *BpfObjects) DeleteSdfFilterList(index uint32) error {
	var empty SdfFilterList
	return b.SdfFilters.Put(index, unsafe.Pointer(&empty))
}
