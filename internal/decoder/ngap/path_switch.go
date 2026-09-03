// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

// SecurityContext is the {NCC, NH} pair the AMF hands the target node so it can
// derive the next access-stratum key (TS 38.413 §9.3.1.88).
type SecurityContext struct {
	NextHopChainingCount int64  `json:"next_hop_chaining_count"`
	NextHopNH            string `json:"next_hop_nh"`
}

// SecurityResult reports whether the target node actually applied the protection
// the network asked for (TS 38.413 §9.3.1.59).
type SecurityResult struct {
	IntegrityProtectionResult       utils.EnumField `json:"integrity_protection_result"`
	ConfidentialityProtectionResult utils.EnumField `json:"confidentiality_protection_result"`
}

// UserPlaneSecurityInformation pairs what was asked for with what was applied.
type UserPlaneSecurityInformation struct {
	SecurityResult     SecurityResult      `json:"security_result"`
	SecurityIndication *SecurityIndication `json:"security_indication,omitempty"`
}

// PathSwitchRequestTransferDecoded is the per-session transfer the target node
// sends to move the downlink tunnel to itself (TS 38.413 §9.3.4.8).
type PathSwitchRequestTransferDecoded struct {
	DLNGUUPTNLInformation        GTPTunnel                     `json:"dl_ng_u_up_tnl_information"`
	DLNGUTNLInformationReused    *bool                         `json:"dl_ng_u_tnl_information_reused,omitempty"`
	UserPlaneSecurityInformation *UserPlaneSecurityInformation `json:"user_plane_security_information,omitempty"`
	QosFlowAccepted              []int64                       `json:"qos_flow_accepted,omitempty"`
}

// PathSwitchRequestAcknowledgeTransferDecoded is the transfer the AMF returns per
// switched session (TS 38.413 §9.3.4.9).
type PathSwitchRequestAcknowledgeTransferDecoded struct {
	ULNGUUPTNLInformation *GTPTunnel          `json:"ul_ng_u_up_tnl_information,omitempty"`
	SecurityIndication    *SecurityIndication `json:"security_indication,omitempty"`
}

// CauseTransferDecoded is a transfer carrying only a cause, as the failed and
// released path-switch lists do.
type CauseTransferDecoded struct {
	Cause Cause `json:"cause"`
}

// PathSwitchSession is one PDU session in a path-switch list: its id, and the
// transfer that travelled with it when the decoder could read it.
type PathSwitchSession struct {
	PDUSessionID int64 `json:"pdu_session_id"`

	PathSwitchRequestTransfer            *PathSwitchRequestTransferDecoded            `json:"path_switch_request_transfer,omitempty"`
	PathSwitchRequestAcknowledgeTransfer *PathSwitchRequestAcknowledgeTransferDecoded `json:"path_switch_request_acknowledge_transfer,omitempty"`
	CauseTransfer                        *CauseTransferDecoded                        `json:"cause_transfer,omitempty"`

	TransferHex string `json:"transfer_hex,omitempty"` // set when the transfer did not decode
	Error       string `json:"error,omitempty"`
}

func securityResult(r ngap.SecurityResult) SecurityResult {
	return SecurityResult{
		IntegrityProtectionResult:       utils.NamedEnum(uint8(r.IntegrityProtectionResult), r.IntegrityProtectionResult.Name()),
		ConfidentialityProtectionResult: utils.NamedEnum(uint8(r.ConfidentialityProtectionResult), r.ConfidentialityProtectionResult.Name()),
	}
}

func libPathSwitchRequestTransfer(raw ngap.TransferContainer) (*PathSwitchRequestTransferDecoded, error) {
	t, err := ngap.ParsePathSwitchRequestTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &PathSwitchRequestTransferDecoded{DLNGUUPTNLInformation: libGTPTunnel(t.DLNGUUPTNLInformation)}

	if t.DLNGUTNLInformationReused != nil {
		reused := *t.DLNGUTNLInformationReused == ngap.DLNGUTNLInformationReusedTrue
		out.DLNGUTNLInformationReused = &reused
	}

	if u := t.UserPlaneSecurityInformation; u != nil {
		indication := u.SecurityIndication
		out.UserPlaneSecurityInformation = &UserPlaneSecurityInformation{
			SecurityResult:     securityResult(u.SecurityResult),
			SecurityIndication: securityIndication(&indication),
		}
	}

	for _, flow := range t.QosFlowAccepted {
		out.QosFlowAccepted = append(out.QosFlowAccepted, int64(flow.QosFlowIdentifier))
	}

	return out, nil
}

func libPathSwitchRequestAcknowledgeTransfer(raw ngap.TransferContainer) (*PathSwitchRequestAcknowledgeTransferDecoded, error) {
	t, err := ngap.ParsePathSwitchRequestAcknowledgeTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &PathSwitchRequestAcknowledgeTransferDecoded{SecurityIndication: securityIndication(t.SecurityIndication)}

	if t.ULNGUUPTNLInformation != nil {
		tunnel := libGTPTunnel(*t.ULNGUUPTNLInformation)
		out.ULNGUUPTNLInformation = &tunnel
	}

	return out, nil
}

func pathSwitchSession(id ngap.PDUSessionID, raw ngap.TransferContainer, decode func(ngap.TransferContainer) (any, error)) PathSwitchSession {
	s := PathSwitchSession{PDUSessionID: int64(id)}

	decoded, err := decode(raw)
	if err != nil {
		s.Error = err.Error()
		s.TransferHex = hex.EncodeToString(raw)

		return s
	}

	switch v := decoded.(type) {
	case *PathSwitchRequestTransferDecoded:
		s.PathSwitchRequestTransfer = v
	case *PathSwitchRequestAcknowledgeTransferDecoded:
		s.PathSwitchRequestAcknowledgeTransfer = v
	case *CauseTransferDecoded:
		s.CauseTransfer = v
	}

	return s
}

func decodeSetupFailedTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParsePathSwitchRequestSetupFailedTransfer(raw)
	if err != nil {
		return nil, err
	}

	return &CauseTransferDecoded{Cause: cause(t.Cause)}, nil
}

func decodeUnsuccessfulTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParsePathSwitchRequestUnsuccessfulTransfer(raw)
	if err != nil {
		return nil, err
	}

	return &CauseTransferDecoded{Cause: cause(t.Cause)}, nil
}

func buildPathSwitchRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePathSwitchRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Path Switch Request: %v", err)}
	}

	switched := make([]PathSwitchSession, 0, len(m.PDUSessionResourceToBeSwitchedDLList))
	for _, it := range m.PDUSessionResourceToBeSwitchedDLList {
		switched = append(switched, pathSwitchSession(it.PDUSessionID, it.Transfer, func(raw ngap.TransferContainer) (any, error) {
			return libPathSwitchRequestTransfer(raw)
		}))
	}

	ies := []IE{
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDSourceAMFUENGAPID, ngap.CriticalityReject, int64(m.SourceAMFUENGAPID)),
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(ngap.IDUserLocationInformation, ngap.CriticalityIgnore, userLocationInformation(*m.UserLocationInformation)))
	}

	if m.UESecurityCapabilities != nil {
		ies = append(ies, ie(ngap.IDUESecurityCapabilities, ngap.CriticalityIgnore, libUESecurityCapabilities(*m.UESecurityCapabilities)))
	}

	ies = append(ies, ie(ngap.IDPDUSessionResourceToBeSwitchedDLList, ngap.CriticalityReject, switched))

	if len(m.PDUSessionResourceFailedToSetup) > 0 {
		failed := make([]PathSwitchSession, 0, len(m.PDUSessionResourceFailedToSetup))
		for _, it := range m.PDUSessionResourceFailedToSetup {
			failed = append(failed, pathSwitchSession(it.PDUSessionID, it.Transfer, decodeSetupFailedTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceFailedToSetupListPSReq, ngap.CriticalityIgnore, failed))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildPathSwitchRequestAcknowledge(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePathSwitchRequestAcknowledge(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Path Switch Request Acknowledge: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if m.UESecurityCapabilities != nil {
		ies = append(ies, ie(ngap.IDUESecurityCapabilities, ngap.CriticalityReject, libUESecurityCapabilities(*m.UESecurityCapabilities)))
	}

	ies = append(ies, ie(ngap.IDSecurityContext, ngap.CriticalityReject, SecurityContext{
		NextHopChainingCount: int64(m.SecurityContext.NextHopChainingCount),
		NextHopNH:            hex.EncodeToString(m.SecurityContext.NextHopNH[:]),
	}))

	if len(m.PDUSessionResourceSwitchedList) > 0 {
		switched := make([]PathSwitchSession, 0, len(m.PDUSessionResourceSwitchedList))
		for _, it := range m.PDUSessionResourceSwitchedList {
			switched = append(switched, pathSwitchSession(it.PDUSessionID, it.Transfer, func(raw ngap.TransferContainer) (any, error) {
				return libPathSwitchRequestAcknowledgeTransfer(raw)
			}))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceSwitchedList, ngap.CriticalityIgnore, switched))
	}

	if len(m.PDUSessionResourceReleased) > 0 {
		released := make([]PathSwitchSession, 0, len(m.PDUSessionResourceReleased))
		for _, it := range m.PDUSessionResourceReleased {
			released = append(released, pathSwitchSession(it.PDUSessionID, it.Transfer, decodeUnsuccessfulTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceReleasedListPSAck, ngap.CriticalityIgnore, released))
	}

	if len(m.AllowedNSSAI) > 0 {
		allowed := make([]SNSSAI, 0, len(m.AllowedNSSAI))
		for _, it := range m.AllowedNSSAI {
			allowed = append(allowed, buildSNSSAIValue(it.SNSSAI))
		}

		ies = append(ies, ie(ngap.IDAllowedNSSAI, ngap.CriticalityReject, allowed))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildPathSwitchRequestFailure(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePathSwitchRequestFailure(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse Path Switch Request Failure: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if len(m.PDUSessionResourceReleased) > 0 {
		released := make([]PathSwitchSession, 0, len(m.PDUSessionResourceReleased))
		for _, it := range m.PDUSessionResourceReleased {
			released = append(released, pathSwitchSession(it.PDUSessionID, it.Transfer, decodeUnsuccessfulTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceReleasedListPSFail, ngap.CriticalityIgnore, released))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildNASNonDeliveryIndication(value []byte) NGAPMessageValue {
	m, err := ngap.ParseNASNonDeliveryIndication(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse NAS Non Delivery Indication: %v", err)}
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDNASPDU, ngap.CriticalityIgnore, libNASPDU(m.NASPDU)),
	}

	if m.Cause != nil {
		ies = append(ies, ie(ngap.IDCause, ngap.CriticalityIgnore, cause(*m.Cause)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
