// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

// Package s1ap decodes S1AP (4G) network-event payloads into a structured,
// JSON-friendly view for the UI events drawer, mirroring internal/decoder/ngap.
// The decoded shape matches the NGAP decoder's so the front-end renderer is
// shared; coverage is partial and grows per message (TS 36.413).
package s1ap

import (
	"fmt"
	"reflect"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

// IE is one decoded Information Element. It is structurally identical to the
// NGAP decoder's IE so the UI renderer treats both protocols uniformly.
type IE struct {
	ID          utils.EnumField[int64]  `json:"id"`
	Criticality utils.EnumField[uint64] `json:"criticality"`
	Value       any                     `json:"value,omitempty"`
	ValueType   string                  `json:"value_type,omitempty"`

	Error string `json:"error,omitempty"` // reserved field for decoding errors
}

type S1APMessageValue struct {
	IEs   []IE   `json:"ies,omitempty"`
	Error string `json:"error,omitempty"` // reserved field for decoding errors
}

type S1APMessage struct {
	Summary       string                  `json:"summary,omitempty"`
	PDUType       string                  `json:"pdu_type"`
	ProcedureCode utils.EnumField[int64]  `json:"procedure_code"`
	Criticality   utils.EnumField[uint64] `json:"criticality"`
	Value         S1APMessageValue        `json:"value"`
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
	case s1ap.ProcPaging:
		msg.Value, msg.Summary = buildPaging(m.Value)
	case s1ap.ProcErrorIndication:
		msg.Value, msg.Summary = buildErrorIndication(m.Value)
	default:
		msg.Value = unsupportedProcedure(m.ProcedureCode)
	}

	setIEValueTypes(msg.Value.IEs)

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
	default:
		msg.Value = unsupportedProcedure(m.ProcedureCode)
	}

	setIEValueTypes(msg.Value.IEs)

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
	default:
		msg.Value = unsupportedProcedure(m.ProcedureCode)
	}

	setIEValueTypes(msg.Value.IEs)

	return msg
}

func unsupportedProcedure(code s1ap.ProcedureCode) S1APMessageValue {
	return S1APMessageValue{
		Error: fmt.Sprintf("decoding not implemented for procedure code %d (%s)", code, procedureCodeName(code)),
	}
}

func inferValueType(v any) string {
	if v == nil {
		return ""
	}

	switch v.(type) {
	case int64, int32, int16, int, uint64, uint32, uint16, uint8:
		return "integer"
	case string:
		return "string"
	case []byte:
		return "bytes"
	case NASPDU:
		return "nas_pdu"
	case utils.Enum:
		return "enum"
	default:
		if reflect.TypeOf(v).Kind() == reflect.Slice {
			return "array"
		}

		return "object"
	}
}

func setIEValueTypes(ies []IE) {
	for i := range ies {
		ies[i].ValueType = inferValueType(ies[i].Value)
	}
}
