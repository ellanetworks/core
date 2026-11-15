// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"

	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

// QosFlowWithCause is a QoS flow the peer could not act on, and why
// (TS 38.413 §9.3.1.13).
type QosFlowWithCause struct {
	QosFlowIdentifier int64 `json:"qos_flow_identifier"`
	Cause             Cause `json:"cause"`
}

// QosFlowAddOrModifyRequest is a QoS flow the AMF asks the RAN to add or change.
// The QoS parameters are optional: a modify that only rebinds the E-RAB carries
// none (TS 38.413 §9.3.4.3).
type QosFlowAddOrModifyRequest struct {
	QosFlow *QosFlowSetupRequest `json:"qos_flow,omitempty"`

	QosFlowIdentifier     int64      `json:"qos_flow_identifier"`
	ERABID                *uint8     `json:"erab_id,omitempty"`
	ULNGUUPTNLInformation *GTPTunnel `json:"ul_ng_u_up_tnl_information,omitempty"`
}

type QosFlowAddOrModifyResponse struct {
	QosFlowIdentifier     int64      `json:"qos_flow_identifier"`
	DLNGUUPTNLInformation *GTPTunnel `json:"dl_ng_u_up_tnl_information,omitempty"`
}

type UserPlaneFailureIndication struct {
	UserPlaneFailureType  utils.EnumField `json:"user_plane_failure_type"`
	ULNGUUPTNLInformation GTPTunnel       `json:"ul_ng_u_up_tnl_information"`
	DLNGUUPTNLInformation GTPTunnel       `json:"dl_ng_u_up_tnl_information"`
}

// ULNGUUPTNLModify pairs the uplink and downlink endpoints a session moves to.
type ULNGUUPTNLModify struct {
	ULNGUUPTNLInformation          GTPTunnel  `json:"ul_ng_u_up_tnl_information"`
	DLNGUUPTNLInformation          GTPTunnel  `json:"dl_ng_u_up_tnl_information"`
	RedundantULNGUUPTNLInformation *GTPTunnel `json:"redundant_ul_ng_u_up_tnl_information,omitempty"`
	RedundantDLNGUUPTNLInformation *GTPTunnel `json:"redundant_dl_ng_u_up_tnl_information,omitempty"`
}

// QosFlowPerTNLInformation is a tunnel endpoint and the QoS flows carried on it
// (TS 38.413 §9.3.2.8).
type QosFlowPerTNLInformation struct {
	UPTransportLayerInformation GTPTunnel           `json:"up_transport_layer_information"`
	AssociatedQosFlowList       []AssociatedQosFlow `json:"associated_qos_flow_list,omitempty"`
}

// PDUSessionResourceModifyRequestTransferDecoded is the per-session transfer the
// AMF sends to change a session (TS 38.413 §9.3.4.3).
type PDUSessionResourceModifyRequestTransferDecoded struct {
	MaximumBitRate                           *MaximumBitRate             `json:"maximum_bit_rate,omitempty"`
	ULNGUUPTNLModify                         []ULNGUUPTNLModify          `json:"ul_ng_u_up_tnl_modify,omitempty"`
	NetworkInstance                          *uint16                     `json:"network_instance,omitempty"`
	QosFlowAddOrModifyRequest                []QosFlowAddOrModifyRequest `json:"qos_flow_add_or_modify_request,omitempty"`
	QosFlowToRelease                         []QosFlowWithCause          `json:"qos_flow_to_release,omitempty"`
	AdditionalULNGUUPTNLInformation          []GTPTunnel                 `json:"additional_ul_ng_u_up_tnl_information,omitempty"`
	AdditionalRedundantULNGUUPTNLInformation []GTPTunnel                 `json:"additional_redundant_ul_ng_u_up_tnl_information,omitempty"`
	RedundantULNGUUPTNLInformation           *GTPTunnel                  `json:"redundant_ul_ng_u_up_tnl_information,omitempty"`
	UserPlaneFailureIndication               *UserPlaneFailureIndication `json:"user_plane_failure_indication,omitempty"`
	SecurityIndication                       *SecurityIndication         `json:"security_indication,omitempty"`
	CommonNetworkInstance                    any                         `json:"common_network_instance,omitempty"`
	RedundantCommonNetworkInstance           any                         `json:"redundant_common_network_instance,omitempty"`
	MBSSessionSetuporModifyRequestList       any                         `json:"mbs_session_setupor_modify_request_list,omitempty"`
	MBSSessionToReleaseList                  any                         `json:"mbs_session_to_release_list,omitempty"`
}

// PDUSessionResourceModifyResponseTransferDecoded is what the RAN reports back
// per session (TS 38.413 §9.3.4.4).
type PDUSessionResourceModifyResponseTransferDecoded struct {
	DLNGUUPTNLInformation                         *GTPTunnel                   `json:"dl_ng_u_up_tnl_information,omitempty"`
	ULNGUUPTNLInformation                         *GTPTunnel                   `json:"ul_ng_u_up_tnl_information,omitempty"`
	QosFlowAddOrModifyResponse                    []QosFlowAddOrModifyResponse `json:"qos_flow_add_or_modify_response,omitempty"`
	AdditionalDLQosFlowPerTNLInformation          []QosFlowPerTNLInformation   `json:"additional_dl_qos_flow_per_tnl_information,omitempty"`
	QosFlowFailedToAddOrModify                    []QosFlowWithCause           `json:"qos_flow_failed_to_add_or_modify,omitempty"`
	AdditionalNGUUPTNLInformation                 []ULNGUUPTNLModify           `json:"additional_ng_u_up_tnl_information,omitempty"`
	RedundantDLNGUUPTNLInformation                *GTPTunnel                   `json:"redundant_dl_ng_u_up_tnl_information,omitempty"`
	RedundantULNGUUPTNLInformation                *GTPTunnel                   `json:"redundant_ul_ng_u_up_tnl_information,omitempty"`
	AdditionalRedundantDLQosFlowPerTNLInformation []QosFlowPerTNLInformation   `json:"additional_redundant_dl_qos_flow_per_tnl_information,omitempty"`
	AdditionalRedundantNGUUPTNLInformation        []ULNGUUPTNLModify           `json:"additional_redundant_ng_u_up_tnl_information,omitempty"`
}

// PDUSessionResourceModifyIndicationTransferDecoded is the RAN-initiated
// indication's per-session transfer (TS 38.413 §9.3.4.6).
type PDUSessionResourceModifyIndicationTransferDecoded struct {
	DLQosFlowPerTNLInformation                    QosFlowPerTNLInformation   `json:"dl_qos_flow_per_tnl_information"`
	AdditionalDLQosFlowPerTNLInformation          []QosFlowPerTNLInformation `json:"additional_dl_qos_flow_per_tnl_information,omitempty"`
	RedundantDLQosFlowPerTNLInformation           *QosFlowPerTNLInformation  `json:"redundant_dl_qos_flow_per_tnl_information,omitempty"`
	AdditionalRedundantDLQosFlowPerTNLInformation []QosFlowPerTNLInformation `json:"additional_redundant_dl_qos_flow_per_tnl_information,omitempty"`
}

// PDUSessionResourceModifyConfirmTransferDecoded is the AMF's confirmation of a
// RAN-initiated modification (TS 38.413 §9.3.4.7).
type PDUSessionResourceModifyConfirmTransferDecoded struct {
	QosFlowModifyConfirm                   []int64            `json:"qos_flow_modify_confirm,omitempty"`
	ULNGUUPTNLInformation                  GTPTunnel          `json:"ul_ng_u_up_tnl_information"`
	AdditionalNGUUPTNLInformation          []ULNGUUPTNLModify `json:"additional_ng_u_up_tnl_information,omitempty"`
	QosFlowFailedToModify                  []QosFlowWithCause `json:"qos_flow_failed_to_modify,omitempty"`
	RedundantULNGUUPTNLInformation         *GTPTunnel         `json:"redundant_ul_ng_u_up_tnl_information,omitempty"`
	AdditionalRedundantNGUUPTNLInformation []ULNGUUPTNLModify `json:"additional_redundant_ng_u_up_tnl_information,omitempty"`
}

// ModifySession is one PDU session in a modify list: its id, and whichever
// transfer travelled with it.
type ModifySession struct {
	PDUSessionID int64   `json:"pdu_session_id"`
	NASPDU       *NASPDU `json:"nas_pdu,omitempty"`
	SNSSAI       *SNSSAI `json:"snssai,omitempty"`

	ModifyRequestTransfer    *PDUSessionResourceModifyRequestTransferDecoded    `json:"modify_request_transfer,omitempty"`
	ModifyResponseTransfer   *PDUSessionResourceModifyResponseTransferDecoded   `json:"modify_response_transfer,omitempty"`
	ModifyIndicationTransfer *PDUSessionResourceModifyIndicationTransferDecoded `json:"modify_indication_transfer,omitempty"`
	ModifyConfirmTransfer    *PDUSessionResourceModifyConfirmTransferDecoded    `json:"modify_confirm_transfer,omitempty"`
	CauseTransfer            *CauseTransferDecoded                              `json:"cause_transfer,omitempty"`

	TransferHex string `json:"transfer_hex,omitempty"` // set when the transfer did not decode
	Error       string `json:"error,omitempty"`
}

func qosFlowsWithCause(list ngap.QosFlowListWithCause) []QosFlowWithCause {
	out := make([]QosFlowWithCause, 0, len(list))
	for _, f := range list {
		out = append(out, QosFlowWithCause{QosFlowIdentifier: int64(f.QosFlowIdentifier), Cause: cause(f.Cause)})
	}

	return out
}

func qosFlowPerTNLInformation(q ngap.QosFlowPerTNLInformation) QosFlowPerTNLInformation {
	out := QosFlowPerTNLInformation{UPTransportLayerInformation: libGTPTunnel(q.UPTransportLayerInformation)}
	for _, f := range q.AssociatedQosFlowList {
		out.AssociatedQosFlowList = append(out.AssociatedQosFlowList, AssociatedQosFlow{QosFlowIdentifier: int64(f.QosFlowIdentifier)})
	}

	return out
}

func qosFlowPerTNLInformationList(list ngap.QosFlowPerTNLInformationList) []QosFlowPerTNLInformation {
	out := make([]QosFlowPerTNLInformation, 0, len(list))
	for _, q := range list {
		out = append(out, qosFlowPerTNLInformation(q.QosFlowPerTNLInformation))
	}

	return out
}

func libPDUSessionResourceModifyRequestTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParsePDUSessionResourceModifyRequestTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionResourceModifyRequestTransferDecoded{}

	if ambr := t.PDUSessionAggregateMaximumBitRate; ambr != nil {
		out.MaximumBitRate = &MaximumBitRate{
			UplinkNAggregateMaximumBitRate:   uint64(ambr.UL),
			DownlinkNAggregateMaximumBitRate: uint64(ambr.DL),
			Unit:                             "bps",
		}
	}

	for _, m := range t.ULNGUUPTNLModify {
		out.ULNGUUPTNLModify = append(out.ULNGUUPTNLModify, ULNGUUPTNLModify{
			ULNGUUPTNLInformation:          libGTPTunnel(m.ULNGUUPTNLInformation),
			DLNGUUPTNLInformation:          libGTPTunnel(m.DLNGUUPTNLInformation),
			RedundantULNGUUPTNLInformation: optionalGTPTunnel(m.RedundantULNGUUPTNLInformation),
			RedundantDLNGUUPTNLInformation: optionalGTPTunnel(m.RedundantDLNGUUPTNLInformation),
		})
	}

	if t.NetworkInstance != nil {
		v := uint16(*t.NetworkInstance)
		out.NetworkInstance = &v
	}

	for _, f := range t.QosFlowAddOrModifyRequest {
		entry := QosFlowAddOrModifyRequest{QosFlowIdentifier: int64(f.QosFlowIdentifier)}

		if f.QosFlowLevelQosParameters != nil {
			flow := libQosFlowSetupRequest(ngap.QosFlowSetupRequestItem{
				QosFlowIdentifier:         f.QosFlowIdentifier,
				QosFlowLevelQosParameters: *f.QosFlowLevelQosParameters,
			})
			entry.QosFlow = &flow
		}

		if f.ERABID != nil {
			id := uint8(*f.ERABID)
			entry.ERABID = &id
		}

		entry.ULNGUUPTNLInformation = optionalGTPTunnel(f.ULNGUUPTNLInformation)

		out.QosFlowAddOrModifyRequest = append(out.QosFlowAddOrModifyRequest, entry)
	}

	out.QosFlowToRelease = qosFlowsWithCause(t.QosFlowToRelease)

	for _, item := range t.AdditionalULNGUUPTNLInformation {
		out.AdditionalULNGUUPTNLInformation = append(out.AdditionalULNGUUPTNLInformation, libGTPTunnel(item.NGUUPTNLInformation))
	}

	out.AdditionalRedundantULNGUUPTNLInformation = gtpTunnelList(t.AdditionalRedundantULNGUUPTNLInformation)
	out.RedundantULNGUUPTNLInformation = optionalGTPTunnel(t.RedundantULNGUUPTNLInformation)

	if u := t.UserPlaneFailureIndication; u != nil {
		out.UserPlaneFailureIndication = &UserPlaneFailureIndication{
			UserPlaneFailureType:  utils.NamedEnum(uint8(u.UserPlaneFailureType), u.UserPlaneFailureType.Name()),
			ULNGUUPTNLInformation: libGTPTunnel(u.ULNGUUPTNLInformation),
			DLNGUUPTNLInformation: libGTPTunnel(u.DLNGUUPTNLInformation),
		}
	}

	out.SecurityIndication = securityIndication(t.SecurityIndication)
	out.CommonNetworkInstance = renderPresent(t.CommonNetworkInstance)
	out.RedundantCommonNetworkInstance = renderPresent(t.RedundantCommonNetworkInstance)
	out.MBSSessionSetuporModifyRequestList = renderPresent(t.MBSSessionSetuporModifyRequestList)
	out.MBSSessionToReleaseList = renderPresent(t.MBSSessionToReleaseList)

	return out, nil
}

func libPDUSessionResourceModifyResponseTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParsePDUSessionResourceModifyResponseTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionResourceModifyResponseTransferDecoded{
		AdditionalDLQosFlowPerTNLInformation: qosFlowPerTNLInformationList(t.AdditionalDLQosFlowPerTNLInformation),
		QosFlowFailedToAddOrModify:           qosFlowsWithCause(t.QosFlowFailedToAddOrModify),
	}

	if t.DLNGUUPTNLInformation != nil {
		tunnel := libGTPTunnel(*t.DLNGUUPTNLInformation)
		out.DLNGUUPTNLInformation = &tunnel
	}

	if t.ULNGUUPTNLInformation != nil {
		tunnel := libGTPTunnel(*t.ULNGUUPTNLInformation)
		out.ULNGUUPTNLInformation = &tunnel
	}

	for _, f := range t.QosFlowAddOrModifyResponse {
		out.QosFlowAddOrModifyResponse = append(out.QosFlowAddOrModifyResponse, QosFlowAddOrModifyResponse{
			QosFlowIdentifier:     int64(f.QosFlowIdentifier),
			DLNGUUPTNLInformation: optionalGTPTunnel(f.DLNGUUPTNLInformation),
		})
	}

	out.AdditionalNGUUPTNLInformation = gtpTunnelPairs(t.AdditionalNGUUPTNLInformation)
	out.RedundantDLNGUUPTNLInformation = optionalGTPTunnel(t.RedundantDLNGUUPTNLInformation)
	out.RedundantULNGUUPTNLInformation = optionalGTPTunnel(t.RedundantULNGUUPTNLInformation)
	out.AdditionalRedundantDLQosFlowPerTNLInformation = qosFlowPerTNLInformationList(t.AdditionalRedundantDLQosFlowPerTNLInformation)
	out.AdditionalRedundantNGUUPTNLInformation = gtpTunnelPairs(t.AdditionalRedundantNGUUPTNLInformation)

	return out, nil
}

func libPDUSessionResourceModifyIndicationTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParsePDUSessionResourceModifyIndicationTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionResourceModifyIndicationTransferDecoded{
		DLQosFlowPerTNLInformation:                    qosFlowPerTNLInformation(t.DLQosFlowPerTNLInformation),
		AdditionalDLQosFlowPerTNLInformation:          qosFlowPerTNLInformationList(t.AdditionalDLQosFlowPerTNLInformation),
		AdditionalRedundantDLQosFlowPerTNLInformation: qosFlowPerTNLInformationList(t.AdditionalRedundantDLQosFlowPerTNLInformation),
	}

	if t.RedundantDLQosFlowPerTNLInformation != nil {
		q := qosFlowPerTNLInformation(*t.RedundantDLQosFlowPerTNLInformation)
		out.RedundantDLQosFlowPerTNLInformation = &q
	}

	return out, nil
}

func libPDUSessionResourceModifyConfirmTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParsePDUSessionResourceModifyConfirmTransfer(raw)
	if err != nil {
		return nil, err
	}

	out := &PDUSessionResourceModifyConfirmTransferDecoded{
		ULNGUUPTNLInformation: libGTPTunnel(t.ULNGUUPTNLInformation),
		QosFlowFailedToModify: qosFlowsWithCause(t.QosFlowFailedToModify),
	}

	for _, f := range t.QosFlowModifyConfirm {
		out.QosFlowModifyConfirm = append(out.QosFlowModifyConfirm, int64(f.QosFlowIdentifier))
	}

	out.AdditionalNGUUPTNLInformation = gtpTunnelPairs(t.AdditionalNGUUPTNLInformation)
	out.RedundantULNGUUPTNLInformation = optionalGTPTunnel(t.RedundantULNGUUPTNLInformation)
	out.AdditionalRedundantNGUUPTNLInformation = gtpTunnelPairs(t.AdditionalRedundantNGUUPTNLInformation)

	return out, nil
}

func libPDUSessionResourceModifyIndicationUnsuccessfulTransfer(raw ngap.TransferContainer) (any, error) {
	t, err := ngap.ParsePDUSessionResourceModifyIndicationUnsuccessfulTransfer(raw)
	if err != nil {
		return nil, err
	}

	return &CauseTransferDecoded{Cause: cause(t.Cause)}, nil
}

// modifySession decodes one list item. A transfer the codec cannot model is kept
// as hex rather than dropped.
func modifySession(id ngap.PDUSessionID, raw ngap.TransferContainer, decode func(ngap.TransferContainer) (any, error)) ModifySession {
	s := ModifySession{PDUSessionID: int64(id)}

	if decode == nil {
		s.TransferHex = hex.EncodeToString(raw)
		return s
	}

	decoded, err := decode(raw)
	if err != nil {
		s.Error = err.Error()
		s.TransferHex = hex.EncodeToString(raw)

		return s
	}

	switch v := decoded.(type) {
	case *PDUSessionResourceModifyRequestTransferDecoded:
		s.ModifyRequestTransfer = v
	case *PDUSessionResourceModifyResponseTransferDecoded:
		s.ModifyResponseTransfer = v
	case *PDUSessionResourceModifyIndicationTransferDecoded:
		s.ModifyIndicationTransfer = v
	case *PDUSessionResourceModifyConfirmTransferDecoded:
		s.ModifyConfirmTransfer = v
	case *CauseTransferDecoded:
		s.CauseTransfer = v
	}

	return s
}

func buildPDUSessionResourceModifyRequest(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePDUSessionResourceModifyRequest(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PDU Session Resource Modify Request: %v", err)}
	}

	sessions := make([]ModifySession, 0, len(m.PDUSessionResourceModify))

	for _, it := range m.PDUSessionResourceModify {
		s := modifySession(it.PDUSessionID, it.Transfer, libPDUSessionResourceModifyRequestTransfer)

		if it.NASPDU != nil {
			pdu := libNASPDU(*it.NASPDU)
			s.NASPDU = &pdu
		}

		if it.SNSSAI != nil {
			snssai := buildSNSSAIValue(*it.SNSSAI)
			s.SNSSAI = &snssai
		}

		sessions = append(sessions, s)
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDPDUSessionResourceModifyListModReq, ngap.CriticalityReject, sessions),
	}

	ies = appendTyped(ies, []typedIE{
		{ngap.IDRANPagingPriority, ngap.CriticalityIgnore, m.RANPagingPriority},
	})

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildPDUSessionResourceModifyResponse(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePDUSessionResourceModifyResponse(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PDU Session Resource Modify Response: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if len(m.PDUSessionResourceModify) > 0 {
		sessions := make([]ModifySession, 0, len(m.PDUSessionResourceModify))
		for _, it := range m.PDUSessionResourceModify {
			sessions = append(sessions, modifySession(it.PDUSessionID, it.Transfer, libPDUSessionResourceModifyResponseTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceModifyListModRes, ngap.CriticalityIgnore, sessions))
	}

	if len(m.PDUSessionResourceFailed) > 0 {
		failed := make([]ModifySession, 0, len(m.PDUSessionResourceFailed))
		for _, it := range m.PDUSessionResourceFailed {
			failed = append(failed, modifySession(it.PDUSessionID, it.Transfer, libPDUSessionResourceModifyIndicationUnsuccessfulTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceFailedToModifyListModRes, ngap.CriticalityIgnore, failed))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(ngap.IDUserLocationInformation, ngap.CriticalityIgnore, userLocationInformation(*m.UserLocationInformation)))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildPDUSessionResourceModifyIndication(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePDUSessionResourceModifyIndication(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PDU Session Resource Modify Indication: %v", err)}
	}

	sessions := make([]ModifySession, 0, len(m.PDUSessionResourceModify))
	for _, it := range m.PDUSessionResourceModify {
		sessions = append(sessions, modifySession(it.PDUSessionID, it.Transfer, libPDUSessionResourceModifyIndicationTransfer))
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDPDUSessionResourceModifyListModInd, ngap.CriticalityReject, sessions),
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(ngap.IDUserLocationInformation, ngap.CriticalityIgnore, userLocationInformation(*m.UserLocationInformation)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

func buildPDUSessionResourceModifyConfirm(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePDUSessionResourceModifyConfirm(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PDU Session Resource Modify Confirm: %v", err)}
	}

	var ies []IE

	if m.AMFUENGAPID != nil {
		ies = append(ies, ie(ngap.IDAMFUENGAPID, ngap.CriticalityIgnore, int64(*m.AMFUENGAPID)))
	}

	if m.RANUENGAPID != nil {
		ies = append(ies, ie(ngap.IDRANUENGAPID, ngap.CriticalityIgnore, int64(*m.RANUENGAPID)))
	}

	if len(m.PDUSessionResourceModify) > 0 {
		sessions := make([]ModifySession, 0, len(m.PDUSessionResourceModify))
		for _, it := range m.PDUSessionResourceModify {
			sessions = append(sessions, modifySession(it.PDUSessionID, it.Transfer, libPDUSessionResourceModifyConfirmTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceModifyListModCfm, ngap.CriticalityIgnore, sessions))
	}

	if len(m.PDUSessionResourceFailed) > 0 {
		failed := make([]ModifySession, 0, len(m.PDUSessionResourceFailed))
		for _, it := range m.PDUSessionResourceFailed {
			failed = append(failed, modifySession(it.PDUSessionID, it.Transfer, libPDUSessionResourceModifyIndicationUnsuccessfulTransfer))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceFailedToModifyListModCfm, ngap.CriticalityIgnore, failed))
	}

	if m.CriticalityDiagnostics != nil {
		ies = append(ies, ie(ngap.IDCriticalityDiagnostics, ngap.CriticalityIgnore, criticalityDiagnostics(*m.CriticalityDiagnostics)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}

// The codec models no PDU Session Resource Notify transfer, so the per-session
// containers are reported as hex.
func buildPDUSessionResourceNotify(value []byte) NGAPMessageValue {
	m, err := ngap.ParsePDUSessionResourceNotify(value)
	if err != nil {
		return NGAPMessageValue{Error: fmt.Sprintf("parse PDU Session Resource Notify: %v", err)}
	}

	notified := make([]ModifySession, 0, len(m.PDUSessionResourceNotify))
	for _, it := range m.PDUSessionResourceNotify {
		notified = append(notified, modifySession(it.PDUSessionID, it.Transfer, nil))
	}

	ies := []IE{
		ie(ngap.IDAMFUENGAPID, ngap.CriticalityReject, int64(m.AMFUENGAPID)),
		ie(ngap.IDRANUENGAPID, ngap.CriticalityReject, int64(m.RANUENGAPID)),
		ie(ngap.IDPDUSessionResourceNotifyList, ngap.CriticalityReject, notified),
	}

	if len(m.PDUSessionResourceReleased) > 0 {
		released := make([]ModifySession, 0, len(m.PDUSessionResourceReleased))
		for _, it := range m.PDUSessionResourceReleased {
			released = append(released, modifySession(it.PDUSessionID, it.Transfer, nil))
		}

		ies = append(ies, ie(ngap.IDPDUSessionResourceReleasedListNot, ngap.CriticalityIgnore, released))
	}

	if m.UserLocationInformation != nil {
		ies = append(ies, ie(ngap.IDUserLocationInformation, ngap.CriticalityIgnore, userLocationInformation(*m.UserLocationInformation)))
	}

	return NGAPMessageValue{IEs: append(ies, unmodeledIEs(m.UnknownIEs())...)}
}
