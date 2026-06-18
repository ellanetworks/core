// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package s1ap encodes and decodes S1 Application Protocol messages
// (3GPP TS 36.413) using Aligned PER. It is a pure codec: no transport, no
// state, no procedure logic.
package s1ap

import "fmt"

// Criticality ::= ENUMERATED { reject, ignore, notify } (not extensible).
type Criticality uint8

const (
	CriticalityReject Criticality = iota
	CriticalityIgnore
	CriticalityNotify

	criticalityRootCount = 3
)

func (c Criticality) String() string {
	switch c {
	case CriticalityReject:
		return "reject"
	case CriticalityIgnore:
		return "ignore"
	case CriticalityNotify:
		return "notify"
	default:
		return fmt.Sprintf("Criticality(%d)", uint8(c))
	}
}

// Presence ::= ENUMERATED { optional, conditional, mandatory }. It drives the
// IE-container engine's handling of a missing IE.
type Presence uint8

const (
	PresenceOptional Presence = iota
	PresenceConditional
	PresenceMandatory
)

// TriggeringMessage ::= ENUMERATED { initiating-message, successful-outcome,
// unsuccessful-outcome } (used by CriticalityDiagnostics).
type TriggeringMessage uint8

const (
	TriggeringInitiatingMessage TriggeringMessage = iota
	TriggeringSuccessfulOutcome
	TriggeringUnsuccessfulOutcome

	triggeringMessageRootCount = 3
)

// ProtocolIEID ::= INTEGER (0..65535).
type ProtocolIEID uint16

// ProcedureCode ::= INTEGER (0..255). The named values are the S1AP elementary
// procedure codes (TS 36.413 §9.3, S1AP-Constants).
type ProcedureCode uint8

const (
	ProcHandoverPreparation                ProcedureCode = 0
	ProcHandoverResourceAllocation         ProcedureCode = 1
	ProcHandoverNotification               ProcedureCode = 2
	ProcPathSwitchRequest                  ProcedureCode = 3
	ProcHandoverCancel                     ProcedureCode = 4
	ProcERABSetup                          ProcedureCode = 5
	ProcERABModify                         ProcedureCode = 6
	ProcERABRelease                        ProcedureCode = 7
	ProcERABReleaseIndication              ProcedureCode = 8
	ProcInitialContextSetup                ProcedureCode = 9
	ProcPaging                             ProcedureCode = 10
	ProcDownlinkNASTransport               ProcedureCode = 11
	ProcInitialUEMessage                   ProcedureCode = 12
	ProcUplinkNASTransport                 ProcedureCode = 13
	ProcReset                              ProcedureCode = 14
	ProcErrorIndication                    ProcedureCode = 15
	ProcNASNonDeliveryIndication           ProcedureCode = 16
	ProcS1Setup                            ProcedureCode = 17
	ProcUEContextReleaseRequest            ProcedureCode = 18
	ProcDownlinkS1cdma2000tunnelling       ProcedureCode = 19
	ProcUplinkS1cdma2000tunnelling         ProcedureCode = 20
	ProcUEContextModification              ProcedureCode = 21
	ProcUECapabilityInfoIndication         ProcedureCode = 22
	ProcUEContextRelease                   ProcedureCode = 23
	ProcENBStatusTransfer                  ProcedureCode = 24
	ProcMMEStatusTransfer                  ProcedureCode = 25
	ProcDeactivateTrace                    ProcedureCode = 26
	ProcTraceStart                         ProcedureCode = 27
	ProcTraceFailureIndication             ProcedureCode = 28
	ProcENBConfigurationUpdate             ProcedureCode = 29
	ProcMMEConfigurationUpdate             ProcedureCode = 30
	ProcLocationReportingControl           ProcedureCode = 31
	ProcLocationReportingFailureIndication ProcedureCode = 32
	ProcLocationReport                     ProcedureCode = 33
	ProcOverloadStart                      ProcedureCode = 34
	ProcOverloadStop                       ProcedureCode = 35
	ProcWriteReplaceWarning                ProcedureCode = 36
	ProcENBDirectInformationTransfer       ProcedureCode = 37
	ProcMMEDirectInformationTransfer       ProcedureCode = 38
	ProcPrivateMessage                     ProcedureCode = 39
	ProcENBConfigurationTransfer           ProcedureCode = 40
	ProcMMEConfigurationTransfer           ProcedureCode = 41
	ProcCellTrafficTrace                   ProcedureCode = 42
	ProcKill                               ProcedureCode = 43
	ProcDownlinkUEAssociatedLPPaTransport  ProcedureCode = 44
	ProcUplinkUEAssociatedLPPaTransport    ProcedureCode = 45
	ProcDownlinkNonUEAssociatedLPPa        ProcedureCode = 46
	ProcUplinkNonUEAssociatedLPPa          ProcedureCode = 47
	ProcUERadioCapabilityMatch             ProcedureCode = 48
	ProcPWSRestartIndication               ProcedureCode = 49
	ProcERABModificationIndication         ProcedureCode = 50
	ProcPWSFailureIndication               ProcedureCode = 51
	ProcRerouteNASRequest                  ProcedureCode = 52
	ProcUEContextModificationIndication    ProcedureCode = 53
	ProcConnectionEstablishmentIndication  ProcedureCode = 54
	ProcUEContextSuspend                   ProcedureCode = 55
	ProcUEContextResume                    ProcedureCode = 56
	ProcNASDeliveryIndication              ProcedureCode = 57
	ProcRetrieveUEInformation              ProcedureCode = 58
	ProcUEInformationTransfer              ProcedureCode = 59
	ProcENBCPRelocationIndication          ProcedureCode = 60
	ProcMMECPRelocationIndication          ProcedureCode = 61
	ProcSecondaryRATDataUsageReport        ProcedureCode = 62
	ProcUERadioCapabilityIDMapping         ProcedureCode = 63
	ProcHandoverSuccess                    ProcedureCode = 64
	ProcENBEarlyStatusTransfer             ProcedureCode = 65
	ProcMMEEarlyStatusTransfer             ProcedureCode = 66
)
