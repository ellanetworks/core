// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s6a

import (
	"slices"

	"github.com/ellanetworks/core/diameter"
	"github.com/ellanetworks/core/diameter/tgpp"
)

const (
	AVPGMLCNumber           uint32 = 1474
	AVPLCSPrivacyException  uint32 = 1475
	AVPSSStatus             uint32 = 1477
	AVPNotificationToUEUser uint32 = 1478
	AVPPLMNClient           uint32 = 1482
)

const (
	SSCodeAllLCSPrivacyException byte = 0xB0
	SSCodePLMNOperator           byte = 0xB4
)

type SSStatus byte

const (
	SSStatusActive      SSStatus = 1 << 0
	SSStatusRegistered  SSStatus = 1 << 1
	SSStatusProvisioned SSStatus = 1 << 2
	SSStatusQuiescent   SSStatus = 1 << 3
)

func (s SSStatus) ProvisionedAndActive() bool {
	return s&(SSStatusProvisioned|SSStatusActive) == SSStatusProvisioned|SSStatusActive
}

type PLMNClient uint32

const (
	PLMNClientBroadcastService          PLMNClient = 0
	PLMNClientOAndMHPLMN                PLMNClient = 1
	PLMNClientOAndMVPLMN                PLMNClient = 2
	PLMNClientAnonymousLocation         PLMNClient = 3
	PLMNClientTargetUESubscribedService PLMNClient = 4
)

var plmnClientNames = tgpp.EnumNames{
	"BROADCAST_SERVICE", "O_AND_M_HPLMN", "O_AND_M_VPLMN", "ANONYMOUS_LOCATION", "TARGET_UE_SUBSCRIBED_SERVICE",
}

func (c PLMNClient) String() string {
	return plmnClientNames.Name("PLMNClient", uint32(c))
}

func (c PLMNClient) Known() bool { return plmnClientNames.Has(uint32(c)) }

type LCSPrivacyException struct {
	SSCode      byte
	SSStatus    SSStatus
	PLMNClients []PLMNClient
}

type LCSInfo struct {
	GMLCNumbers       []string
	PrivacyExceptions []LCSPrivacyException
}

func (i *LCSInfo) AllowsPLMNClient(c PLMNClient) bool {
	if i == nil {
		return false
	}

	for _, e := range i.PrivacyExceptions {
		if e.SSCode == SSCodePLMNOperator && e.SSStatus.ProvisionedAndActive() && slices.Contains(e.PLMNClients, c) {
			return true
		}
	}

	return false
}

func (i *LCSInfo) withdraw(gmlcList bool, ssCodes [][]byte) *LCSInfo {
	if i == nil {
		return nil
	}

	out := LCSInfo{GMLCNumbers: slices.Clone(i.GMLCNumbers), PrivacyExceptions: slices.Clone(i.PrivacyExceptions)}

	if gmlcList {
		out.GMLCNumbers = nil
	}

	for _, code := range ssCodes {
		if len(code) != 1 {
			continue
		}

		out.PrivacyExceptions = slices.DeleteFunc(out.PrivacyExceptions, func(e LCSPrivacyException) bool {
			return code[0] == SSCodeAllLCSPrivacyException || e.SSCode == code[0]
		})
	}

	return &out
}

func (i LCSInfo) avp() (diameter.AVP, error) {
	var (
		inner []diameter.AVP
		err   error
	)

	for _, n := range i.GMLCNumbers {
		if inner, err = appendE164(inner, AVPGMLCNumber, "GMLC-Number", n); err != nil {
			return diameter.AVP{}, err
		}
	}

	for _, e := range i.PrivacyExceptions {
		group := []diameter.AVP{
			octetsAVP(AVPSSCode, []byte{e.SSCode}),
			octetsAVP(AVPSSStatus, []byte{byte(e.SSStatus)}),
		}

		for _, c := range e.PLMNClients {
			if !c.Known() {
				return diameter.AVP{}, invalidf("PLMN-Client %d", uint32(c))
			}

			group = append(group, unsignedAVP(AVPPLMNClient, uint32(c)))
		}

		inner = append(inner, groupedAVP(AVPLCSPrivacyException, group...))
	}

	return groupedAVP(AVPLCSInfo, inner...), nil
}

func parseLCSInfo(a diameter.AVP) *LCSInfo {
	inner, err := a.Grouped()
	if err != nil {
		return nil
	}

	var info LCSInfo

	for _, g := range diameter.FindAll(inner, AVPGMLCNumber, tgpp.VendorID) {
		if n, err := tgpp.DecodeE164(g.Data); err == nil && n != "" {
			info.GMLCNumbers = append(info.GMLCNumbers, n)
		}
	}

	for _, p := range diameter.FindAll(inner, AVPLCSPrivacyException, tgpp.VendorID) {
		if e, ok := parseLCSPrivacyException(p); ok {
			info.PrivacyExceptions = append(info.PrivacyExceptions, e)
		}
	}

	return &info
}

func parseLCSPrivacyException(a diameter.AVP) (LCSPrivacyException, bool) {
	inner, err := a.Grouped()
	if err != nil {
		return LCSPrivacyException{}, false
	}

	code, ok := diameter.Find(inner, AVPSSCode, tgpp.VendorID)
	if !ok || len(code.Data) != 1 {
		return LCSPrivacyException{}, false
	}

	status, ok := diameter.Find(inner, AVPSSStatus, tgpp.VendorID)
	if !ok || len(status.Data) < 1 || len(status.Data) > 5 {
		return LCSPrivacyException{}, false
	}

	e := LCSPrivacyException{SSCode: code.Data[0], SSStatus: SSStatus(status.Data[0] & 0x0f)}

	for _, c := range diameter.FindAll(inner, AVPPLMNClient, tgpp.VendorID) {
		v, err := tgpp.Unsigned32(c)
		if err != nil || !PLMNClient(v).Known() {
			continue
		}

		e.PLMNClients = append(e.PLMNClients, PLMNClient(v))
	}

	return e, true
}
