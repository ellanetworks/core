// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/rrc"
	"github.com/ellanetworks/core/s1ap"
)

type AMBR struct {
	DL uint64 `json:"dl"`
	UL uint64 `json:"ul"`
}

type ARP struct {
	PriorityLevel           uint8 `json:"priority_level"`
	PreemptionCapability    uint8 `json:"preemption_capability"`
	PreemptionVulnerability uint8 `json:"preemption_vulnerability"`
}

type ERABToBeSetup struct {
	ERABID                uint8   `json:"erab_id"`
	QCI                   uint8   `json:"qci"`
	ARP                   ARP     `json:"arp"`
	TransportLayerAddress string  `json:"transport_layer_address"`
	GTPTEID               uint32  `json:"gtp_teid"`
	NASPDU                *NASPDU `json:"nas_pdu,omitempty"` // absent on a reconnect ICS (no piggybacked NAS)
}

type ERABSetupItem struct {
	ERABID                uint8  `json:"erab_id"`
	TransportLayerAddress string `json:"transport_layer_address"`
	GTPTEID               uint32 `json:"gtp_teid"`
}

type UESecurityCapabilities struct {
	EncryptionAlgorithms          []string `json:"encryption_algorithms"`
	IntegrityProtectionAlgorithms []string `json:"integrity_protection_algorithms"`
}

// securityAlgorithms decodes a UE security-capability bitmap (TS 36.413
// §9.2.1.40): MSB-first, bit 15 = <prefix>1, bit 14 = <prefix>2, bit 13 =
// <prefix>3 (the null algorithm <prefix>0 is implicit and always supported).
func securityAlgorithms(bitmap uint16, prefix string) []string {
	var out []string

	for i, bit := 1, 15; i <= 7; i, bit = i+1, bit-1 {
		if bitmap>>uint(bit)&1 == 1 {
			out = append(out, fmt.Sprintf("%s%d", prefix, i))
		}
	}

	if len(out) == 0 {
		return []string{prefix + "0 (null)"}
	}

	return out
}

func erabToBeSetup(it s1ap.ERABToBeSetupItemCtxtSUReq) ERABToBeSetup {
	e := ERABToBeSetup{
		ERABID: uint8(it.ERABID),
		QCI:    uint8(it.QoS.QCI),
		ARP: ARP{
			PriorityLevel:           it.QoS.ARP.PriorityLevel,
			PreemptionCapability:    uint8(it.QoS.ARP.PreemptionCapability),
			PreemptionVulnerability: uint8(it.QoS.ARP.PreemptionVulnerability),
		},
		TransportLayerAddress: transportLayerAddress(it.TransportLayerAddress),
		GTPTEID:               uint32(it.GTPTEID),
	}

	// A reconnect ICS (Service Request / active TAU) carries no piggybacked NAS;
	// only attach the NAS-PDU when present.
	if len(it.NASPDU) > 0 {
		nas := nasPDU(it.NASPDU)
		e.NASPDU = &nas
	}

	return e
}

func buildInitialContextSetupRequest(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseInitialContextSetupRequest(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Initial Context Setup Request: %v", err)}, ""
	}

	erabs := make([]ERABToBeSetup, 0, len(m.ERABToBeSetup))
	for _, it := range m.ERABToBeSetup {
		erabs = append(erabs, erabToBeSetup(it))
	}

	ies := []IE{
		ie(s1ap.IDMMEUES1APID, s1ap.CriticalityReject, uint32(m.MMEUES1APID)),
		ie(s1ap.IDENBUES1APID, s1ap.CriticalityReject, uint32(m.ENBUES1APID)),
		ie(s1ap.IDUEAggregateMaximumBitrate, s1ap.CriticalityReject, AMBR{DL: uint64(m.UEAggregateMaximumBitRate.DL), UL: uint64(m.UEAggregateMaximumBitRate.UL)}),
		ie(s1ap.IDERABToBeSetupListCtxtSUReq, s1ap.CriticalityReject, erabs),
		ie(s1ap.IDUESecurityCapabilities, s1ap.CriticalityReject, UESecurityCapabilities{
			EncryptionAlgorithms:          securityAlgorithms(m.UESecurityCapabilities.EncryptionAlgorithms, "EEA"),
			IntegrityProtectionAlgorithms: securityAlgorithms(m.UESecurityCapabilities.IntegrityProtectionAlgorithms, "EIA"),
		}),
		ie(s1ap.IDSecurityKey, s1ap.CriticalityReject, hex.EncodeToString(m.SecurityKey[:])),
	}

	if len(m.UERadioCapability) > 0 {
		ies = append(ies, ie(s1ap.IDUERadioCapability, s1ap.CriticalityIgnore, rrc.DescribeS1AP(m.UERadioCapability)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Initial Context Setup Request (MME-UE %d, eNB-UE %d, %d E-RAB)", m.MMEUES1APID, m.ENBUES1APID, len(m.ERABToBeSetup))
}

func buildInitialContextSetupResponse(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseInitialContextSetupResponse(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Initial Context Setup Response: %v", err)}, ""
	}

	setup := make([]ERABSetupItem, 0, len(m.ERABSetup))
	for _, it := range m.ERABSetup {
		setup = append(setup, ERABSetupItem{
			ERABID:                uint8(it.ERABID),
			TransportLayerAddress: transportLayerAddress(it.TransportLayerAddress),
			GTPTEID:               uint32(it.GTPTEID),
		})
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	ies = append(ies, ie(s1ap.IDERABSetupListCtxtSURes, s1ap.CriticalityIgnore, setup))

	if len(m.ERABFailedToSetup) > 0 {
		failed := make([]ERABFailedItem, 0, len(m.ERABFailedToSetup))
		for _, it := range m.ERABFailedToSetup {
			failed = append(failed, ERABFailedItem{ERABID: uint8(it.ERABID), Cause: cause(it.Cause)})
		}

		ies = append(ies, ie(s1ap.IDERABFailedToSetupListCtxtSURes, s1ap.CriticalityIgnore, failed))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Initial Context Setup Response (MME-UE %s, eNB-UE %s, %d E-RAB)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID), len(m.ERABSetup))
}

// ERABFailedItem is a decoded E-RAB that the eNB failed to set up (TS 36.413
// §9.2.1.36): its bearer id and the cause.
type ERABFailedItem struct {
	ERABID uint8 `json:"erab_id"`
	Cause  Cause `json:"cause"`
}

func buildInitialContextSetupFailure(value []byte) (S1APMessageValue, string) {
	m, err := s1ap.ParseInitialContextSetupFailure(value)
	if err != nil {
		return S1APMessageValue{Error: fmt.Sprintf("parse Initial Context Setup Failure: %v", err)}, ""
	}

	var ies []IE

	if m.MMEUES1APID != nil {
		ies = append(ies, ie(s1ap.IDMMEUES1APID, s1ap.CriticalityIgnore, uint32(*m.MMEUES1APID)))
	}

	if m.ENBUES1APID != nil {
		ies = append(ies, ie(s1ap.IDENBUES1APID, s1ap.CriticalityIgnore, uint32(*m.ENBUES1APID)))
	}

	if m.Cause != nil {
		ies = append(ies, ie(s1ap.IDCause, s1ap.CriticalityIgnore, cause(*m.Cause)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(s1ap.IDCriticalityDiagnostics, s1ap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	ies = appendUnknownIEs(ies, m.UnknownIEs())

	return S1APMessageValue{IEs: ies}, fmt.Sprintf("Initial Context Setup Failure (MME-UE %s, eNB-UE %s)", ueIDText(m.MMEUES1APID), ueIDText(m.ENBUES1APID))
}
