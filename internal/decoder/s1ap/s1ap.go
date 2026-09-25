// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package s1ap decodes S1AP (4G) network-event payloads into a structured,
// JSON-friendly view for the UI events drawer, mirroring internal/decoder/ngap.
// The decoded shape matches the NGAP decoder's so the front-end renderer is
// shared; coverage is partial and grows per message (TS 36.413).
package s1ap

import (
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

// IE is one decoded Information Element. It is structurally identical to the
// NGAP decoder's IE so the UI renderer treats both protocols uniformly.
type IE struct {
	ID          utils.EnumField `json:"id"`
	Criticality utils.EnumField `json:"criticality"`
	Value       any             `json:"value,omitempty"`

	Error string `json:"error,omitempty"` // reserved field for decoding errors
}

type S1APMessageValue struct {
	IEs   []IE   `json:"ies,omitempty"`
	Error string `json:"error,omitempty"` // reserved field for decoding errors
}

type S1APMessage struct {
	Summary       string           `json:"summary,omitempty"`
	PDUType       string           `json:"pdu_type"`
	ProcedureCode utils.EnumField  `json:"procedure_code"`
	Criticality   utils.EnumField  `json:"criticality"`
	Value         S1APMessageValue `json:"value"`
}

// DecodeS1APMessage decodes a raw S1AP PDU. A decode error is reported in the
// returned message rather than surfaced as a Go error, matching the NGAP path.
func DecodeS1APMessage(raw []byte) S1APMessage {
	pdu, err := s1ap.Unmarshal(raw)
	if err != nil {
		return S1APMessage{
			Value: S1APMessageValue{Error: fmt.Sprintf("could not decode raw s1ap message: %v", err)},
		}
	}

	switch p := pdu.(type) {
	case *s1ap.InitiatingMessage:
		return decodeInitiatingMessage(p)
	case *s1ap.SuccessfulOutcome:
		return decodeSuccessfulOutcome(p)
	case *s1ap.UnsuccessfulOutcome:
		return decodeUnsuccessfulOutcome(p)
	default:
		return S1APMessage{
			PDUType: "Unknown",
			Value:   S1APMessageValue{Error: fmt.Sprintf("unknown S1AP PDU type: %T", pdu)},
		}
	}
}

func decodeInitiatingMessage(m *s1ap.InitiatingMessage) S1APMessage {
	msg := S1APMessage{
		PDUType:       "InitiatingMessage",
		ProcedureCode: procedureCodeToEnum(m.ProcedureCode),
		Criticality:   criticalityToEnum(m.Criticality),
	}

	switch m.ProcedureCode {
	case s1ap.ProcS1Setup:
		msg.Value, msg.Summary = buildS1SetupRequest(m.Value)
	case s1ap.ProcInitialUEMessage:
		msg.Value, msg.Summary = buildInitialUEMessage(m.Value)
	case s1ap.ProcUplinkNASTransport:
		msg.Value, msg.Summary = buildUplinkNASTransport(m.Value)
	case s1ap.ProcDownlinkNASTransport:
		msg.Value, msg.Summary = buildDownlinkNASTransport(m.Value)
	case s1ap.ProcInitialContextSetup:
		msg.Value, msg.Summary = buildInitialContextSetupRequest(m.Value)
	case s1ap.ProcUEContextReleaseRequest:
		msg.Value, msg.Summary = buildUEContextReleaseRequest(m.Value)
	case s1ap.ProcUEContextRelease:
		msg.Value, msg.Summary = buildUEContextReleaseCommand(m.Value)
	case s1ap.ProcUECapabilityInfoIndication:
		msg.Value, msg.Summary = buildUECapabilityInfoIndication(m.Value)
	case s1ap.ProcERABSetup:
		msg.Value, msg.Summary = buildERABSetupRequest(m.Value)
	case s1ap.ProcERABRelease:
		msg.Value, msg.Summary = buildERABReleaseCommand(m.Value)
	case s1ap.ProcReset:
		msg.Value, msg.Summary = buildReset(m.Value)
	case s1ap.ProcPathSwitchRequest:
		msg.Value, msg.Summary = buildPathSwitchRequest(m.Value)
	case s1ap.ProcNASNonDeliveryIndication:
		msg.Value, msg.Summary = buildNASNonDeliveryIndication(m.Value)
	case s1ap.ProcERABModify:
		msg.Value, msg.Summary = buildERABModifyRequest(m.Value)
	case s1ap.ProcERABModificationIndication:
		msg.Value, msg.Summary = buildERABModificationIndication(m.Value)
	case s1ap.ProcENBConfigurationUpdate:
		msg.Value, msg.Summary = buildENBConfigurationUpdate(m.Value)
	case s1ap.ProcMMEConfigurationUpdate:
		msg.Value, msg.Summary = buildMMEConfigurationUpdate(m.Value)
	case s1ap.ProcENBConfigurationTransfer:
		msg.Value, msg.Summary = buildENBConfigurationTransfer(m.Value)
	case s1ap.ProcMMEConfigurationTransfer:
		msg.Value, msg.Summary = buildMMEConfigurationTransfer(m.Value)
	case s1ap.ProcLocationReport:
		msg.Value, msg.Summary = buildLocationReport(m.Value)
	case s1ap.ProcDownlinkNonUEAssociatedLPPaTransport:
		msg.Value, msg.Summary = buildDownlinkNonUEAssociatedLPPaTransport(m.Value)
	case s1ap.ProcUplinkNonUEAssociatedLPPaTransport:
		msg.Value, msg.Summary = buildUplinkNonUEAssociatedLPPaTransport(m.Value)
	case s1ap.ProcPaging:
		msg.Value, msg.Summary = buildPaging(m.Value)
	case s1ap.ProcErrorIndication:
		msg.Value, msg.Summary = buildErrorIndication(m.Value)
	case s1ap.ProcHandoverPreparation:
		msg.Value, msg.Summary = buildHandoverRequired(m.Value)
	case s1ap.ProcHandoverResourceAllocation:
		msg.Value, msg.Summary = buildHandoverRequest(m.Value)
	case s1ap.ProcHandoverNotification:
		msg.Value, msg.Summary = buildHandoverNotify(m.Value)
	case s1ap.ProcHandoverCancel:
		msg.Value, msg.Summary = buildHandoverCancel(m.Value)
	case s1ap.ProcENBStatusTransfer:
		msg.Value, msg.Summary = buildENBStatusTransfer(m.Value)
	case s1ap.ProcMMEStatusTransfer:
		msg.Value, msg.Summary = buildMMEStatusTransfer(m.Value)
	case s1ap.ProcDownlinkUEAssociatedLPPaTransport:
		msg.Value, msg.Summary = buildDownlinkUEAssociatedLPPaTransport(m.Value)
	case s1ap.ProcUplinkUEAssociatedLPPaTransport:
		msg.Value, msg.Summary = buildUplinkUEAssociatedLPPaTransport(m.Value)
	default:
		msg.Value = unsupportedProcedure(m.ProcedureCode)
	}

	return msg
}

func decodeSuccessfulOutcome(m *s1ap.SuccessfulOutcome) S1APMessage {
	msg := S1APMessage{
		PDUType:       "SuccessfulOutcome",
		ProcedureCode: procedureCodeToEnum(m.ProcedureCode),
		Criticality:   criticalityToEnum(m.Criticality),
	}

	switch m.ProcedureCode {
	case s1ap.ProcS1Setup:
		msg.Value, msg.Summary = buildS1SetupResponse(m.Value)
	case s1ap.ProcInitialContextSetup:
		msg.Value, msg.Summary = buildInitialContextSetupResponse(m.Value)
	case s1ap.ProcUEContextRelease:
		msg.Value, msg.Summary = buildUEContextReleaseComplete(m.Value)
	case s1ap.ProcERABSetup:
		msg.Value, msg.Summary = buildERABSetupResponse(m.Value)
	case s1ap.ProcERABRelease:
		msg.Value, msg.Summary = buildERABReleaseResponse(m.Value)
	case s1ap.ProcReset:
		msg.Value, msg.Summary = buildResetAcknowledge(m.Value)
	case s1ap.ProcPathSwitchRequest:
		msg.Value, msg.Summary = buildPathSwitchRequestAcknowledge(m.Value)
	case s1ap.ProcERABModify:
		msg.Value, msg.Summary = buildERABModifyResponse(m.Value)
	case s1ap.ProcERABModificationIndication:
		msg.Value, msg.Summary = buildERABModificationConfirm(m.Value)
	case s1ap.ProcENBConfigurationUpdate:
		msg.Value, msg.Summary = buildENBConfigurationUpdateAcknowledge(m.Value)
	case s1ap.ProcMMEConfigurationUpdate:
		msg.Value, msg.Summary = buildMMEConfigurationUpdateAcknowledge(m.Value)
	case s1ap.ProcHandoverPreparation:
		msg.Value, msg.Summary = buildHandoverCommand(m.Value)
	case s1ap.ProcHandoverResourceAllocation:
		msg.Value, msg.Summary = buildHandoverRequestAcknowledge(m.Value)
	case s1ap.ProcHandoverCancel:
		msg.Value, msg.Summary = buildHandoverCancelAcknowledge(m.Value)
	default:
		msg.Value = unsupportedProcedure(m.ProcedureCode)
	}

	return msg
}

func decodeUnsuccessfulOutcome(m *s1ap.UnsuccessfulOutcome) S1APMessage {
	msg := S1APMessage{
		PDUType:       "UnsuccessfulOutcome",
		ProcedureCode: procedureCodeToEnum(m.ProcedureCode),
		Criticality:   criticalityToEnum(m.Criticality),
	}

	switch m.ProcedureCode {
	case s1ap.ProcS1Setup:
		msg.Value, msg.Summary = buildS1SetupFailure(m.Value)
	case s1ap.ProcInitialContextSetup:
		msg.Value, msg.Summary = buildInitialContextSetupFailure(m.Value)
	case s1ap.ProcHandoverPreparation:
		msg.Value, msg.Summary = buildHandoverPreparationFailure(m.Value)
	case s1ap.ProcHandoverResourceAllocation:
		msg.Value, msg.Summary = buildHandoverFailure(m.Value)
	case s1ap.ProcPathSwitchRequest:
		msg.Value, msg.Summary = buildPathSwitchRequestFailure(m.Value)
	case s1ap.ProcENBConfigurationUpdate:
		msg.Value, msg.Summary = buildENBConfigurationUpdateFailure(m.Value)
	case s1ap.ProcMMEConfigurationUpdate:
		msg.Value, msg.Summary = buildMMEConfigurationUpdateFailure(m.Value)
	default:
		msg.Value = unsupportedProcedure(m.ProcedureCode)
	}

	return msg
}

func unsupportedProcedure(code s1ap.ProcedureCode) S1APMessageValue {
	name, _ := s1ap.ProcedureCodeName(code)

	return S1APMessageValue{
		Error: fmt.Sprintf("decoding not implemented for procedure code %d (%s)", code, name),
	}
}
