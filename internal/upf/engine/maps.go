// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"net/netip"

	"github.com/ellanetworks/core/internal/upf/ebpf"
)

type Maps interface {
	PutPdrUplink(teid uint32, pdrInfo ebpf.PdrInfo) error
	PutPdrDownlink(addr netip.Addr, pdrInfo ebpf.PdrInfo) error
	DeletePdrUplink(teid uint32) error
	DeletePdrDownlink(addr netip.Addr) error
	PutFramedDownlink(prefix netip.Prefix, ueAddr netip.Addr) error
	DeleteFramedDownlink(prefix netip.Prefix) error
	NewUrr(seid uint64, id uint32) error
	DeleteUrr(seid uint64, id uint32) error
	PutSdfFilterList(index uint32, list ebpf.SdfFilterList) error
	DeleteSdfFilterList(index uint32) error
	MarkNotified(d ebpf.DataNotification)
	IsAlreadyNotified(d ebpf.DataNotification) bool
	ClearNotified(seid uint64, pdrid uint16)
	ClearNotifiedForSEID(seid uint64)
}
