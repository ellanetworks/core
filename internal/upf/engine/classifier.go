// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package engine

import (
	"cmp"
	"errors"
	"fmt"
	"slices"

	"github.com/ellanetworks/core/internal/models"
	"github.com/ellanetworks/core/internal/upf/ebpf"
)

func sdfDownlink(spdrInfo SPDRInfo) bool {
	return spdrInfo.UEIP.IsValid() && len(spdrInfo.SDF) > 0
}

func keyedDownlink(spdrInfo SPDRInfo) bool {
	return spdrInfo.UEIP.IsValid() && len(spdrInfo.SDF) == 0
}

func byPrecedence(a, b SPDRInfo) int {
	return cmp.Or(cmp.Compare(a.Precedence, b.Precedence), cmp.Compare(a.PdrID, b.PdrID))
}

func classifierRule(f models.SDFFilter, direction uint8) ebpf.ClassifierRule {
	r := ebpf.ClassifierRule{
		Direction:   direction,
		Remote:      f.Remote,
		Protocol:    ebpf.SdfProtoAny,
		RemotePorts: ebpf.PortRange{LowerBound: f.RemotePort, UpperBound: f.RemotePort},
		LocalPorts:  ebpf.PortRange{LowerBound: f.LocalPort, UpperBound: f.LocalPort},
	}

	if f.Protocol != 0 {
		r.Protocol = f.Protocol
	}

	if f.Remote.IsValid() {
		r.Family = addressFamily(f.Remote.Addr().Is4())
	}

	return r
}

func addressFamily(v4 bool) uint8 {
	if v4 {
		return ebpf.ClassifierFamilyIPv4
	}

	return ebpf.ClassifierFamilyIPv6
}

func buildClassifier(session *Session) ebpf.Classifier {
	var downlink, uplink []SPDRInfo

	for _, p := range session.ListPDRs() {
		switch {
		case sdfDownlink(p):
			downlink = append(downlink, p)
		case !p.UEIP.IsValid() && len(p.SDF) > 0:
			uplink = append(uplink, p)
		}
	}

	slices.SortFunc(downlink, byPrecedence)
	slices.SortFunc(uplink, byPrecedence)

	var c ebpf.Classifier

	for _, p := range downlink {
		target := addTarget(&c, p)

		for _, f := range p.SDF {
			r := classifierRule(f, ebpf.ClassifierDownlink)
			r.Target = target

			ueFamily := addressFamily(p.UEIP.Is4())
			if r.Family != ebpf.ClassifierFamilyAny && r.Family != ueFamily {
				continue
			}

			r.Family = ueFamily
			c.Rules = append(c.Rules, r)
		}
	}

	for _, p := range uplink {
		target := addTarget(&c, p)

		for _, f := range p.SDF {
			r := classifierRule(f, ebpf.ClassifierUplink)
			r.Tunnel = p.TeID
			r.Target = target
			r.QFI = p.PdrInfo.QFI
			c.Rules = append(c.Rules, r)
		}
	}

	return c
}

func addTarget(c *ebpf.Classifier, p SPDRInfo) uint8 {
	c.Targets = append(c.Targets, ebpf.ClassifierTarget{
		PdrID: p.PdrID,
		QerID: p.PdrInfo.QerID,
		UrrID: p.PdrInfo.UrrID,
		Far:   p.PdrInfo.Far,
		Qer:   p.PdrInfo.Qer,
	})

	return uint8(len(c.Targets) - 1)
}

func (conn *SessionEngine) syncClassifier(session *Session) error {
	return conn.writeClassifier(session, true)
}

func (conn *SessionEngine) writeClassifier(session *Session, reapply bool) error {
	c := buildClassifier(session)
	classified := slices.ContainsFunc(c.Rules, func(r ebpf.ClassifierRule) bool { return r.Direction == ebpf.ClassifierDownlink })
	present := len(c.Rules) > 0

	bpfObjects := conn.BpfObjects

	if present && !session.classifierWritten(c) {
		if err := bpfObjects.PutClassifier(session.SEID, c); err != nil {
			session.setWrittenClassifier(nil)
			return fmt.Errorf("install the session's SDF filters: %w", err)
		}

		session.setWrittenClassifier(&c)
	}

	if !present {
		session.setWrittenClassifier(nil)
	}

	var errs []error

	if session.SetClassified(classified) && reapply {
		for _, p := range session.ListPDRs() {
			if !keyedDownlink(p) {
				continue
			}

			if err := applyPDR(p, session, bpfObjects); err != nil {
				errs = append(errs, err)
			}
		}
	}

	if session.SetFiltered(present) && !present {
		if err := bpfObjects.DeleteClassifier(session.SEID); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
