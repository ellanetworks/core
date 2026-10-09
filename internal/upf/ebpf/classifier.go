// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: Apache-2.0

package ebpf

import (
	"errors"
	"fmt"
	"net/netip"
	"unsafe"

	"github.com/cilium/ebpf"
)

const (
	MaxClassifierRules   = 32
	MaxClassifierTargets = 16

	ClassifierUplink   = 1
	ClassifierDownlink = 2

	ClassifierFamilyAny  = 0
	ClassifierFamilyIPv4 = 4
	ClassifierFamilyIPv6 = 6

	PdrFlagSDF         = 0x01
	PdrFlagLocalSwitch = 0x02
	PdrFlagFallback    = 0x04
)

type ClassifierRule struct {
	Direction   uint8
	Tunnel      uint32
	Family      uint8
	Remote      netip.Prefix
	Protocol    uint8
	RemotePorts PortRange
	LocalPorts  PortRange
	Target      uint8
	QFI         uint8
}

type ClassifierTarget struct {
	PdrID uint32
	QerID uint32
	UrrID uint32
	Far   FarInfo
	Qer   QerInfo
}

type Classifier struct {
	Rules   []ClassifierRule
	Targets []ClassifierTarget
}

func (c Classifier) toBpf() (N3N6EntrypointSdfClassifier, error) {
	var out N3N6EntrypointSdfClassifier

	if len(c.Rules) > MaxClassifierRules {
		return out, fmt.Errorf("%d SDF filters exceed the session limit of %d", len(c.Rules), MaxClassifierRules)
	}

	if len(c.Targets) > MaxClassifierTargets {
		return out, fmt.Errorf("%d SDF PDRs exceed the session limit of %d", len(c.Targets), MaxClassifierTargets)
	}

	out.NumRules = uint8(len(c.Rules))

	for i, r := range c.Rules {
		if int(r.Target) >= len(c.Targets) {
			return out, fmt.Errorf("SDF filter %d names PDR slot %d of %d", i, r.Target, len(c.Targets))
		}

		dst := &out.Rules[i]
		dst.Direction = r.Direction
		dst.Tunnel = r.Tunnel
		dst.Family = r.Family
		dst.Protocol = r.Protocol
		dst.RemotePortLow, dst.RemotePortHigh = r.RemotePorts.LowerBound, r.RemotePorts.UpperBound
		dst.LocalPortLow, dst.LocalPortHigh = r.LocalPorts.LowerBound, r.LocalPorts.UpperBound
		dst.Target = r.Target
		dst.Qfi = r.QFI

		if r.Remote.IsValid() {
			remote := r.Remote.Masked()
			dst.Remote.In6U.U6Addr8 = remote.Addr().As16()
			dst.PrefixLen = uint8(remote.Bits())

			if remote.Addr().Is4() {
				dst.PrefixLen += 96
			}
		}
	}

	for i, t := range c.Targets {
		var p N3N6EntrypointPdrInfo

		fillFarQer(&p, t.Far, t.Qer)

		dst := &out.Targets[i]
		dst.PdrId, dst.QerId, dst.UrrId = t.PdrID, t.QerID, t.UrrID
		dst.Far, dst.Qer = p.Far, p.Qer
	}

	return out, nil
}

func (bpfObjects *BpfObjects) PutClassifier(seid uint64, c Classifier) error {
	value, err := c.toBpf()
	if err != nil {
		return err
	}

	return bpfObjects.putTracked(bpfObjects.SdfClassifiers, MapSdfClassifiers, seid, unsafe.Pointer(&value))
}

func (bpfObjects *BpfObjects) DeleteClassifier(seid uint64) error {
	err := bpfObjects.deleteTracked(bpfObjects.SdfClassifiers, MapSdfClassifiers, seid)
	if err != nil && !errors.Is(err, ebpf.ErrKeyNotExist) {
		return fmt.Errorf("delete SDF classifier: %w", err)
	}

	return nil
}
