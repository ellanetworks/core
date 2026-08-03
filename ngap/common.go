// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package ngap encodes and decodes NG Application Protocol messages
// (3GPP TS 38.413) using Aligned PER. It is a pure codec: no transport, no
// state, no procedure logic.
//
// Marshal returns a complete NGAP-PDU; ParseXxx takes the open-type payload of
// one.
//
// A field is a value type only where §10.3.5 stops the message reaching the
// caller without it, which is exactly the required IEs of reject criticality;
// every other IE is nil-able. Marshal applies the stricter §10.3.3 rule and
// fails on any unset required IE.
//
// Decoding returns no message with either [TransferSyntaxError] or
// [AbstractSyntaxError]. Anything §10.3.4.2 and §10.3.5 let a receiver carry on
// past is not an error: the message is returned and [Diagnostics] reports it.
//
// Unmodeled IEs are preserved and re-emitted on encode. Preservation and
// diagnostics are both bounded, since a peer chooses how many IEs to send.
//
// NGAP closes a CHOICE with a choice-Extensions alternative rather than an
// extension marker (§9.3); an alternative this version does not model is an
// error, not a skipped value.
package ngap

import (
	"fmt"

	"github.com/ellanetworks/core/per"
)

func Ptr[T any](v T) *T { return &v }

// Not extensible.
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

// An extension value decodes as nRoot+k, which narrows back onto a root value
// in these small Go enum types, so it is refused: §10.3.1 case 6 handles it on
// the IE's criticality.
func decodeRootEnumerated(r *per.Reader, enc per.Encoding, nRoot int64, name string) (int64, error) {
	idx, err := per.DecodeEnumerated(r, enc, nRoot, true)
	if err != nil {
		return 0, err
	}

	if idx >= nRoot {
		return 0, fmt.Errorf("%w: %s extension value %d", errNotComprehended, name, idx)
	}

	return idx, nil
}

// presence is the TS 38.413 §9.2 tabular "Presence" column.
type presence uint8

const (
	presenceOptional presence = iota
	presenceConditional
	presenceMandatory
)

type TriggeringMessage uint8

const (
	TriggeringInitiatingMessage TriggeringMessage = iota
	TriggeringSuccessfulOutcome
	TriggeringUnsuccessfulOutcome

	triggeringMessageRootCount = 3
)

// ProtocolIEID ::= INTEGER (0..65535).
type ProtocolIEID uint16

// ProcedureCode ::= INTEGER (0..255). The named values are the NGAP elementary
// procedure codes (TS 38.413, NGAP-Constants).
type ProcedureCode uint8

const (
	ProcAMFConfigurationUpdate                ProcedureCode = 0
	ProcAMFStatusIndication                   ProcedureCode = 1
	ProcCellTrafficTrace                      ProcedureCode = 2
	ProcDeactivateTrace                       ProcedureCode = 3
	ProcDownlinkNASTransport                  ProcedureCode = 4
	ProcDownlinkNonUEAssociatedNRPPaTransport ProcedureCode = 5
	ProcDownlinkRANConfigurationTransfer      ProcedureCode = 6
	ProcDownlinkRANStatusTransfer             ProcedureCode = 7
	ProcDownlinkUEAssociatedNRPPaTransport    ProcedureCode = 8
	ProcErrorIndication                       ProcedureCode = 9
	ProcHandoverCancel                        ProcedureCode = 10
	ProcHandoverNotification                  ProcedureCode = 11
	ProcHandoverPreparation                   ProcedureCode = 12
	ProcHandoverResourceAllocation            ProcedureCode = 13
	ProcInitialContextSetup                   ProcedureCode = 14
	ProcInitialUEMessage                      ProcedureCode = 15
	ProcLocationReportingControl              ProcedureCode = 16
	ProcLocationReportingFailureIndication    ProcedureCode = 17
	ProcLocationReport                        ProcedureCode = 18
	ProcNASNonDeliveryIndication              ProcedureCode = 19
	ProcNGReset                               ProcedureCode = 20
	ProcNGSetup                               ProcedureCode = 21
	ProcOverloadStart                         ProcedureCode = 22
	ProcOverloadStop                          ProcedureCode = 23
	ProcPaging                                ProcedureCode = 24
	ProcPathSwitchRequest                     ProcedureCode = 25
	ProcPDUSessionResourceModify              ProcedureCode = 26
	ProcPDUSessionResourceModifyIndication    ProcedureCode = 27
	ProcPDUSessionResourceRelease             ProcedureCode = 28
	ProcPDUSessionResourceSetup               ProcedureCode = 29
	ProcPDUSessionResourceNotify              ProcedureCode = 30
	ProcPrivateMessage                        ProcedureCode = 31
	ProcPWSCancel                             ProcedureCode = 32
	ProcPWSFailureIndication                  ProcedureCode = 33
	ProcPWSRestartIndication                  ProcedureCode = 34
	ProcRANConfigurationUpdate                ProcedureCode = 35
	ProcRerouteNASRequest                     ProcedureCode = 36
	ProcRRCInactiveTransitionReport           ProcedureCode = 37
	ProcTraceFailureIndication                ProcedureCode = 38
	ProcTraceStart                            ProcedureCode = 39
	ProcUEContextModification                 ProcedureCode = 40
	ProcUEContextRelease                      ProcedureCode = 41
	ProcUEContextReleaseRequest               ProcedureCode = 42
	ProcUERadioCapabilityCheck                ProcedureCode = 43
	ProcUERadioCapabilityInfoIndication       ProcedureCode = 44
	ProcUETNLABindingRelease                  ProcedureCode = 45
	ProcUplinkNASTransport                    ProcedureCode = 46
	ProcUplinkNonUEAssociatedNRPPaTransport   ProcedureCode = 47
	ProcUplinkRANConfigurationTransfer        ProcedureCode = 48
	ProcUplinkRANStatusTransfer               ProcedureCode = 49
	ProcUplinkUEAssociatedNRPPaTransport      ProcedureCode = 50
	ProcWriteReplaceWarning                   ProcedureCode = 51
	ProcSecondaryRATDataUsageReport           ProcedureCode = 52
	ProcUplinkRIMInformationTransfer          ProcedureCode = 53
	ProcDownlinkRIMInformationTransfer        ProcedureCode = 54
	ProcRetrieveUEInformation                 ProcedureCode = 55
	ProcUEInformationTransfer                 ProcedureCode = 56
	ProcRANCPRelocationIndication             ProcedureCode = 57
	ProcUEContextResume                       ProcedureCode = 58
	ProcUEContextSuspend                      ProcedureCode = 59
	ProcUERadioCapabilityIDMapping            ProcedureCode = 60
	ProcHandoverSuccess                       ProcedureCode = 61
	ProcUplinkRANEarlyStatusTransfer          ProcedureCode = 62
	ProcDownlinkRANEarlyStatusTransfer        ProcedureCode = 63
	ProcAMFCPRelocationIndication             ProcedureCode = 64
	ProcConnectionEstablishmentIndication     ProcedureCode = 65
	ProcBroadcastSessionModification          ProcedureCode = 66
	ProcBroadcastSessionRelease               ProcedureCode = 67
	ProcBroadcastSessionSetup                 ProcedureCode = 68
	ProcDistributionSetup                     ProcedureCode = 69
	ProcDistributionRelease                   ProcedureCode = 70
	ProcMulticastSessionActivation            ProcedureCode = 71
	ProcMulticastSessionDeactivation          ProcedureCode = 72
	ProcMulticastSessionUpdate                ProcedureCode = 73
	ProcMulticastGroupPaging                  ProcedureCode = 74
	ProcBroadcastSessionReleaseRequired       ProcedureCode = 75
	ProcTimingSynchronisationStatus           ProcedureCode = 76
	ProcTimingSynchronisationStatusReport     ProcedureCode = 77
	ProcMTCommunicationHandling               ProcedureCode = 78
	ProcRANPagingRequest                      ProcedureCode = 79
	ProcBroadcastSessionTransport             ProcedureCode = 80
)
