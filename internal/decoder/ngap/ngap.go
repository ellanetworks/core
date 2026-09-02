// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"fmt"

	fgsdec "github.com/ellanetworks/core/internal/decoder/nas/fgs"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type NGAPMessageValue struct {
	IEs   []IE   `json:"ies,omitempty"`
	Error string `json:"error,omitempty"` // reserved field for decoding errors
}

type NGAPMessage struct {
	Summary       string           `json:"summary,omitempty"`
	PDUType       string           `json:"pdu_type"`
	ProcedureCode utils.EnumField  `json:"procedure_code"`
	Criticality   utils.EnumField  `json:"criticality"`
	Value         NGAPMessageValue `json:"value"`
}

// DecodeNGAPMessage decodes a raw NGAP PDU. A decode error is reported in the
// returned message rather than surfaced as a Go error, matching the S1AP path.
func DecodeNGAPMessage(raw []byte) NGAPMessage {
	pdu, err := ngap.Unmarshal(raw)
	if err != nil {
		return NGAPMessage{
			Value: NGAPMessageValue{Error: fmt.Sprintf("Could not decode raw ngap message: %v", err)},
		}
	}

	var msg NGAPMessage

	switch p := pdu.(type) {
	case *ngap.InitiatingMessage:
		msg = NGAPMessage{
			PDUType:       "InitiatingMessage",
			ProcedureCode: procedureCodeToEnum(p.ProcedureCode),
			Criticality:   criticalityToEnum(p.Criticality),
			Value:         buildInitiatingMessage(p),
		}
	case *ngap.SuccessfulOutcome:
		msg = NGAPMessage{
			PDUType:       "SuccessfulOutcome",
			ProcedureCode: procedureCodeToEnum(p.ProcedureCode),
			Criticality:   criticalityToEnum(p.Criticality),
			Value:         buildSuccessfulOutcome(p),
		}
	case *ngap.UnsuccessfulOutcome:
		msg = NGAPMessage{
			PDUType:       "UnsuccessfulOutcome",
			ProcedureCode: procedureCodeToEnum(p.ProcedureCode),
			Criticality:   criticalityToEnum(p.Criticality),
			Value:         buildUnsuccessfulOutcome(p),
		}
	default:
		return NGAPMessage{
			PDUType: "Unknown",
			Value:   NGAPMessageValue{Error: fmt.Sprintf("unknown NGAP PDU type: %T", pdu)},
		}
	}

	msg.Summary = buildNGAPSummary(msg)

	return msg
}

// buildNGAPSummary generates a one-line summary from the
// procedure code and key IEs. Example: "InitialUEMessage, RAN-UE=1, NAS=RegistrationRequest"
func buildNGAPSummary(msg NGAPMessage) string {
	summary := msg.ProcedureCode.Label
	if summary == "" {
		summary = msg.PDUType
	}

	for _, ie := range msg.Value.IEs {
		switch ngap.ProtocolIEID(ie.ID.Value) {
		case ngap.IDAMFUENGAPID:
			summary += fmt.Sprintf(", AMF-UE=%d", ie.Value)
		case ngap.IDRANUENGAPID:
			summary += fmt.Sprintf(", RAN-UE=%d", ie.Value)
		case ngap.IDNASPDU:
			if nasPdu, ok := ie.Value.(NASPDU); ok && nasPdu.Decoded != nil {
				summary += ", NAS=" + nasMessageTypeName(nasPdu.Decoded)
			}
		case ngap.IDNRPPaPDU:
			if nrppaPdu, ok := ie.Value.(NRPPaPDU); ok && nrppaPdu.Decoded != nil && nrppaPdu.Decoded.Kind.Label != "" {
				summary += ", NRPPa=" + nrppaPdu.Decoded.Kind.Label
			}
		case ngap.IDCause:
			if c, ok := ie.Value.(Cause); ok {
				summary += ", Cause=" + c.Value.Label
			}
		}
	}

	return summary
}

func nasMessageTypeName(msg *fgsdec.NASMessage) string {
	if msg.Encrypted {
		return "Encrypted"
	}

	if msg.GmmMessage != nil {
		return msg.GmmMessage.GmmHeader.MessageType.Label
	}

	if msg.GsmMessage != nil {
		return msg.GsmMessage.GsmHeader.MessageType.Label
	}

	return "Unknown"
}

// raw is the PDU as captured. Procedures rendered from the in-house library
// parse it themselves; the reference decode above still selects the branch.
func buildInitiatingMessage(m *ngap.InitiatingMessage) NGAPMessageValue {
	switch m.ProcedureCode {
	case ngap.ProcNGSetup:
		return buildNGSetupRequest(m.Value)
	case ngap.ProcInitialUEMessage:
		return buildInitialUEMessage(m.Value)
	case ngap.ProcDownlinkNASTransport:
		return buildDownlinkNASTransport(m.Value)
	case ngap.ProcUplinkNASTransport:
		return buildUplinkNASTransport(m.Value)
	case ngap.ProcInitialContextSetup:
		return buildInitialContextSetupRequest(m.Value)
	case ngap.ProcPDUSessionResourceSetup:
		return buildPDUSessionResourceSetupRequest(m.Value)
	case ngap.ProcUEContextReleaseRequest:
		return buildUEContextReleaseRequest(m.Value)
	case ngap.ProcUEContextRelease:
		return buildUEContextReleaseCommand(m.Value)
	case ngap.ProcPDUSessionResourceRelease:
		return buildPDUSessionResourceReleaseCommand(m.Value)
	case ngap.ProcUERadioCapabilityInfoIndication:
		return buildUERadioCapabilityInfoIndication(m.Value)
	case ngap.ProcAMFStatusIndication:
		return buildAMFStatusIndication(m.Value)
	case ngap.ProcPaging:
		return buildPaging(m.Value)
	case ngap.ProcDownlinkUEAssociatedNRPPaTransport:
		return buildDownlinkUEAssociatedNRPPaTransport(m.Value)
	case ngap.ProcUplinkUEAssociatedNRPPaTransport:
		return buildUplinkUEAssociatedNRPPaTransport(m.Value)
	case ngap.ProcDownlinkNonUEAssociatedNRPPaTransport:
		return buildDownlinkNonUEAssociatedNRPPaTransport(m.Value)
	case ngap.ProcUplinkNonUEAssociatedNRPPaTransport:
		return buildUplinkNonUEAssociatedNRPPaTransport(m.Value)
	case ngap.ProcErrorIndication:
		return buildErrorIndication(m.Value)
	case ngap.ProcLocationReport:
		return buildLocationReport(m.Value)
	case ngap.ProcLocationReportingControl:
		return buildLocationReportingControl(m.Value)
	case ngap.ProcNGReset:
		return buildNGReset(m.Value)
	case ngap.ProcPathSwitchRequest:
		return buildPathSwitchRequest(m.Value)
	case ngap.ProcNASNonDeliveryIndication:
		return buildNASNonDeliveryIndication(m.Value)
	case ngap.ProcPDUSessionResourceModify:
		return buildPDUSessionResourceModifyRequest(m.Value)
	case ngap.ProcPDUSessionResourceModifyIndication:
		return buildPDUSessionResourceModifyIndication(m.Value)
	case ngap.ProcPDUSessionResourceNotify:
		return buildPDUSessionResourceNotify(m.Value)
	case ngap.ProcRANConfigurationUpdate:
		return buildRANConfigurationUpdate(m.Value)
	case ngap.ProcUplinkRANConfigurationTransfer:
		return buildUplinkRANConfigurationTransfer(m.Value)
	case ngap.ProcDownlinkRANConfigurationTransfer:
		return buildDownlinkRANConfigurationTransfer(m.Value)
	case ngap.ProcUplinkRANStatusTransfer:
		return buildUplinkRANStatusTransfer(m.Value)
	case ngap.ProcDownlinkRANStatusTransfer:
		return buildDownlinkRANStatusTransfer(m.Value)
	case ngap.ProcHandoverPreparation:
		return buildHandoverRequired(m.Value)
	case ngap.ProcHandoverResourceAllocation:
		return buildHandoverRequest(m.Value)
	case ngap.ProcHandoverNotification:
		return buildHandoverNotify(m.Value)
	case ngap.ProcHandoverCancel:
		return buildHandoverCancel(m.Value)
	default:
		return unsupportedProcedure(m.ProcedureCode)
	}
}

func buildSuccessfulOutcome(m *ngap.SuccessfulOutcome) NGAPMessageValue {
	switch m.ProcedureCode {
	case ngap.ProcNGSetup:
		return buildNGSetupResponse(m.Value)
	case ngap.ProcInitialContextSetup:
		return buildInitialContextSetupResponse(m.Value)
	case ngap.ProcPDUSessionResourceSetup:
		return buildPDUSessionResourceSetupResponse(m.Value)
	case ngap.ProcUEContextRelease:
		return buildUEContextReleaseComplete(m.Value)
	case ngap.ProcPDUSessionResourceRelease:
		return buildPDUSessionResourceReleaseResponse(m.Value)
	case ngap.ProcNGReset:
		return buildNGResetAcknowledge(m.Value)
	case ngap.ProcPathSwitchRequest:
		return buildPathSwitchRequestAcknowledge(m.Value)
	case ngap.ProcPDUSessionResourceModify:
		return buildPDUSessionResourceModifyResponse(m.Value)
	case ngap.ProcPDUSessionResourceModifyIndication:
		return buildPDUSessionResourceModifyConfirm(m.Value)
	case ngap.ProcRANConfigurationUpdate:
		return buildRANConfigurationUpdateAcknowledge(m.Value)
	case ngap.ProcHandoverPreparation:
		return buildHandoverCommand(m.Value)
	case ngap.ProcHandoverResourceAllocation:
		return buildHandoverRequestAcknowledge(m.Value)
	case ngap.ProcHandoverCancel:
		return buildHandoverCancelAcknowledge(m.Value)
	default:
		return unsupportedProcedure(m.ProcedureCode)
	}
}

func buildUnsuccessfulOutcome(m *ngap.UnsuccessfulOutcome) NGAPMessageValue {
	switch m.ProcedureCode {
	case ngap.ProcNGSetup:
		return buildNGSetupFailure(m.Value)
	case ngap.ProcInitialContextSetup:
		return buildInitialContextSetupFailure(m.Value)
	case ngap.ProcPathSwitchRequest:
		return buildPathSwitchRequestFailure(m.Value)
	case ngap.ProcRANConfigurationUpdate:
		return buildRANConfigurationUpdateFailure(m.Value)
	case ngap.ProcHandoverPreparation:
		return buildHandoverPreparationFailure(m.Value)
	case ngap.ProcHandoverResourceAllocation:
		return buildHandoverFailure(m.Value)
	default:
		return unsupportedProcedure(m.ProcedureCode)
	}
}

// unsupportedProcedure reports a procedure this decoder does not render. The
// message still shows its procedure code and criticality.
func unsupportedProcedure(p ngap.ProcedureCode) NGAPMessageValue {
	return NGAPMessageValue{Error: fmt.Sprintf("Unsupported message %s", p)}
}
