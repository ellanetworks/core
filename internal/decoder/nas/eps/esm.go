// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import (
	"fmt"
	"net"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/nas"
	"github.com/ellanetworks/core/nas/eps"
)

type ESMHeader struct {
	MessageType                  utils.EnumField `json:"message_type"`
	EPSBearerIdentity            uint8           `json:"eps_bearer_identity"`
	ProcedureTransactionIdentity uint8           `json:"procedure_transaction_identity"`
}

// ESMMessage is a decoded ESM message: its header, plus the salient fields of
// the session-management messages the MME exchanges. Unlisted types decode to
// the header only.
type ESMMessage struct {
	ESMHeader ESMHeader `json:"esm_header"`
	Error     string    `json:"error,omitempty"`

	PDNConnectivityRequest      *PDNConnectivityRequest      `json:"pdn_connectivity_request,omitempty"`
	ActivateDefaultBearer       *ActivateDefaultBearer       `json:"activate_default_bearer,omitempty"`
	ActivateDefaultBearerAccept *ActivateDefaultBearerAccept `json:"activate_default_bearer_accept,omitempty"`
	ESMInformationRequest       *ESMInformationRequest       `json:"esm_information_request,omitempty"`
	ESMInformationResponse      *ESMInformationResponse      `json:"esm_information_response,omitempty"`
	PDNDisconnectRequest        *PDNDisconnectRequest        `json:"pdn_disconnect_request,omitempty"`
	DeactivateBearerAccept      *DeactivateBearerAccept      `json:"deactivate_bearer_accept,omitempty"`

	ActivateDefaultBearerReject       *ESMCauseOnly                      `json:"activate_default_bearer_reject,omitempty"`
	DeactivateBearerRequest           *ESMCauseOnly                      `json:"deactivate_bearer_request,omitempty"`
	ESMStatus                         *ESMCauseOnly                      `json:"esm_status,omitempty"`
	PDNConnectivityReject             *ESMCauseOnly                      `json:"pdn_connectivity_reject,omitempty"`
	PDNDisconnectReject               *ESMCauseOnly                      `json:"pdn_disconnect_reject,omitempty"`
	ModifyEPSBearerContextReject      *ESMCauseOnly                      `json:"modify_eps_bearer_context_reject,omitempty"`
	BearerResourceAllocationReject    *ESMCauseOnly                      `json:"bearer_resource_allocation_reject,omitempty"`
	BearerResourceModificationReject  *ESMCauseOnly                      `json:"bearer_resource_modification_reject,omitempty"`
	ModifyEPSBearerContextRequest     *ModifyEPSBearerContextRequest     `json:"modify_eps_bearer_context_request,omitempty"`
	ModifyEPSBearerContextAccept      *ModifyEPSBearerContextAccept      `json:"modify_eps_bearer_context_accept,omitempty"`
	BearerResourceAllocationRequest   *BearerResourceAllocationRequest   `json:"bearer_resource_allocation_request,omitempty"`
	BearerResourceModificationRequest *BearerResourceModificationRequest `json:"bearer_resource_modification_request,omitempty"`
}

// PDNAddress is the decoded PDN address IE (TS 24.301 §9.9.4.9): the assigned UE
// address. For IPv6 the network assigns only the 64-bit interface identifier
// (the prefix arrives via Router Advertisement / PCO).
type PDNAddress struct {
	Type            utils.EnumField `json:"type"`
	IPv4            string          `json:"ipv4,omitempty"`
	IPv6InterfaceID string          `json:"ipv6_interface_id,omitempty"`
}

func buildESMMessage(b []byte) *ESMMessage {
	hdr, err := eps.ParseESMHeader(b)
	if err != nil {
		return &ESMMessage{Error: err.Error()}
	}

	m := &ESMMessage{ESMHeader: ESMHeader{
		MessageType:                  esmTypeToEnum(hdr.MessageType),
		EPSBearerIdentity:            uint8(hdr.EPSBearerIdentity),
		ProcedureTransactionIdentity: uint8(hdr.PTI),
	}}

	msg, err := eps.ParseMessage(b, nas.DirectionUplink)
	if err != nil && !nas.SoftOnly(err) {
		m.Error = err.Error()

		return m
	}

	switch msg := msg.(type) {
	case *eps.ActivateDefaultEPSBearerContextRequest:
		m.ActivateDefaultBearer = buildActivateDefaultBearer(msg)
	case *eps.PDNConnectivityRequest:
		m.PDNConnectivityRequest = buildPDNConnectivityRequest(msg)
	case *eps.ActivateDefaultEPSBearerContextAccept:
		m.ActivateDefaultBearerAccept = buildActivateDefaultBearerAccept(msg)
	case *eps.ESMInformationRequest:
		m.ESMInformationRequest = buildESMInformationRequest(msg)
	case *eps.ESMInformationResponse:
		m.ESMInformationResponse = buildESMInformationResponse(msg)
	case *eps.PDNDisconnectRequest:
		m.PDNDisconnectRequest = buildPDNDisconnectRequest(msg)
	case *eps.DeactivateEPSBearerContextAccept:
		m.DeactivateBearerAccept = buildDeactivateBearerAccept(msg)
	case *eps.ActivateDefaultEPSBearerContextReject:
		m.ActivateDefaultBearerReject = esmCauseOnly(msg.Cause, msg.Unrecognized)
	case *eps.DeactivateEPSBearerContextRequest:
		m.DeactivateBearerRequest = esmCauseOnly(msg.Cause, msg.Unrecognized)
	case *eps.ESMStatus:
		m.ESMStatus = esmCauseOnly(msg.Cause, msg.Unrecognized)
	case *eps.PDNConnectivityReject:
		m.PDNConnectivityReject = esmCauseOnly(msg.Cause, msg.Unrecognized)
	case *eps.PDNDisconnectReject:
		m.PDNDisconnectReject = esmCauseOnly(msg.Cause, msg.Unrecognized)
	case *eps.ModifyEPSBearerContextReject:
		m.ModifyEPSBearerContextReject = esmCauseOnly(msg.Cause, msg.Unrecognized)
	case *eps.BearerResourceAllocationReject:
		m.BearerResourceAllocationReject = esmCauseOnly(msg.Cause, msg.Unrecognized)
	case *eps.BearerResourceModificationReject:
		m.BearerResourceModificationReject = esmCauseOnly(msg.Cause, msg.Unrecognized)
	case *eps.ModifyEPSBearerContextRequest:
		m.ModifyEPSBearerContextRequest = buildModifyEPSBearerContextRequest(msg)
	case *eps.ModifyEPSBearerContextAccept:
		m.ModifyEPSBearerContextAccept = buildModifyEPSBearerContextAccept(msg)
	case *eps.BearerResourceAllocationRequest:
		m.BearerResourceAllocationRequest = buildBearerResourceAllocationRequest(msg)
	case *eps.BearerResourceModificationRequest:
		m.BearerResourceModificationRequest = buildBearerResourceModificationRequest(msg)
	}

	return m
}

// decodeESMContainer decodes an ESM message carried inside an EMM message (e.g.
// the PDN Connectivity Request in Attach Request). An empty container yields nil.
func decodeESMContainer(b []byte) *ESMMessage {
	if len(b) == 0 {
		return nil
	}

	return buildESMMessage(b)
}

func requestTypeToEnum(v uint8) utils.EnumField {
	return utils.NamedEnum(v, eps.RequestType(v).Name())
}

func pdnTypeToEnum(v eps.PDNType) utils.EnumField {
	return utils.NamedEnum(uint8(v), v.Name())
}

// pdnAddress decodes the PDN address IE into the assigned UE address (TS 24.301
// §9.9.4.9). An undecodable value yields nil.
func pdnAddress(addr eps.PDNAddress) *PDNAddress {
	out := &PDNAddress{Type: pdnTypeToEnum(addr.PDNType)}

	switch addr.PDNType {
	case 1: // IPv4
		out.IPv4 = net.IP(addr.IPv4[:]).String()
	case 2: // IPv6 (interface identifier only)
		out.IPv6InterfaceID = interfaceID(addr.IPv6IID)
	case 3: // IPv4v6
		out.IPv4 = net.IP(addr.IPv4[:]).String()
		out.IPv6InterfaceID = interfaceID(addr.IPv6IID)
	}

	return out
}

// interfaceID renders a 64-bit IPv6 interface identifier in colon-separated hex.
func interfaceID(iid [8]byte) string {
	return fmt.Sprintf("%02x%02x:%02x%02x:%02x%02x:%02x%02x",
		iid[0], iid[1], iid[2], iid[3], iid[4], iid[5], iid[6], iid[7])
}
