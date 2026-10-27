// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"errors"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const (
	N32cHandshakePath          = "/n32c-handshake/v1"
	N32cExchangeCapabilityPath = N32cHandshakePath + "/exchange-capability"
)

const (
	CauseRequestedParamMismatch         wire.Cause = "REQUESTED_PARAM_MISMATCH"
	CauseNegotiationNotAllowed          wire.Cause = "NEGOTIATION_NOT_ALLOWED"
	CauseN32cExchangeCapabilityOngoing  wire.Cause = "N32C_EXCHANGE_CAPABILITY_ONGOING"
	CauseSecurityParamExchangeCollision wire.Cause = "SECURITY_PARAM_EXCHANGE_COLLISION"
)

const (
	FeatureNFTLST  = 1
	FeaturePSEPRO  = 2
	FeaturePSIU    = 3
	FeatureSNDN32F = 4
	FeatureTLSCOR  = 5
)

const LocalFeatures wire.SupportedFeatures = "1"

type SecurityCapability string

const (
	SecurityTLS   SecurityCapability = "TLS"
	SecurityPRINS SecurityCapability = "PRINS"
	SecurityNone  SecurityCapability = "NONE"
)

func (c SecurityCapability) Known() bool {
	return c == SecurityTLS || c == SecurityPRINS || c == SecurityNone
}

type N32Purpose string

const (
	PurposeRoaming                   N32Purpose = "ROAMING"
	PurposeInterPLMNMobility         N32Purpose = "INTER_PLMN_MOBILITY"
	PurposeSMSInterconnect           N32Purpose = "SMS_INTERCONNECT"
	PurposeRoamingTest               N32Purpose = "ROAMING_TEST"
	PurposeInterPLMNMobilityTest     N32Purpose = "INTER_PLMN_MOBILITY_TEST"
	PurposeSMSInterconnectTest       N32Purpose = "SMS_INTERCONNECT_TEST"
	PurposeSNPNInterconnect          N32Purpose = "SNPN_INTERCONNECT"
	PurposeSNPNInterconnectTest      N32Purpose = "SNPN_INTERCONNECT_TEST"
	PurposeDisasterRoaming           N32Purpose = "DISASTER_ROAMING"
	PurposeDisasterRoamingTest       N32Purpose = "DISASTER_ROAMING_TEST"
	PurposeDataAnalyticsExchange     N32Purpose = "DATA_ANALYTICS_EXCHANGE"
	PurposeDataAnalyticsExchangeTest N32Purpose = "DATA_ANALYTICS_EXCHANGE_TEST"
)

func (p N32Purpose) Known() bool {
	return slices.Contains([]N32Purpose{
		PurposeRoaming, PurposeInterPLMNMobility, PurposeSMSInterconnect, PurposeRoamingTest,
		PurposeInterPLMNMobilityTest, PurposeSMSInterconnectTest, PurposeSNPNInterconnect,
		PurposeSNPNInterconnectTest, PurposeDisasterRoaming, PurposeDisasterRoamingTest,
		PurposeDataAnalyticsExchange, PurposeDataAnalyticsExchangeTest,
	}, p)
}

var (
	acceptedPurposes = []N32Purpose{PurposeRoaming, PurposeInterPLMNMobility}
	defaultPurposes  = []N32Purpose{PurposeRoaming, PurposeInterPLMNMobility}
)

type IntendedN32Purpose struct {
	UsagePurpose   N32Purpose `json:"usagePurpose"`
	AdditionalInfo string     `json:"additionalInfo,omitempty"`
	Cause          string     `json:"cause,omitempty"`
}

func (ip IntendedN32Purpose) Check(p wire.Policy) error {
	return wire.CheckEnum("/usagePurpose", ip.UsagePurpose, p, false)
}

type purposeList int

const (
	requestedPurposes purposeList = iota
	allowedPurposes
	rejectedPurposes
)

func checkPurposes(param string, list []IntendedN32Purpose, p wire.Policy, kind purposeList) error {
	for i, ip := range list {
		pol := p
		if kind == rejectedPurposes {
			pol = wire.Lenient
		}

		if err := ip.Check(pol); err != nil {
			return wire.NestIndex(param, i, err)
		}

		if p == wire.Strict && kind == allowedPurposes && ip.Cause != "" {
			return wire.NestIndex(param, i, wire.Incorrect("/cause", true, "cause in an allowed purpose"))
		}
	}

	return nil
}

var handshakeIDPattern = regexp.MustCompile(`^[A-Fa-f0-9]{16}$`)

type SecNegotiateReqData struct {
	Sender                     string                 `json:"sender"`
	N32HandshakeID             string                 `json:"n32HandshakeId,omitempty"`
	SupportedSecCapabilityList []SecurityCapability   `json:"supportedSecCapabilityList"`
	TargetAPIRootSupported     bool                   `json:"3GppSbiTargetApiRootSupported,omitempty"`
	PlmnIDList                 []wire.PlmnID          `json:"plmnIdList,omitempty"`
	SnpnIDList                 []wire.PlmnIDNid       `json:"snpnIdList,omitempty"`
	TargetPlmnID               *wire.PlmnID           `json:"targetPlmnId,omitempty"`
	TargetSnpnID               *wire.PlmnIDNid        `json:"targetSnpnId,omitempty"`
	IntendedUsagePurpose       []IntendedN32Purpose   `json:"intendedUsagePurpose,omitempty"`
	SupportedFeatures          wire.SupportedFeatures `json:"supportedFeatures,omitempty"`
	SenderN32fFqdn             string                 `json:"senderN32fFqdn,omitempty"`
	SenderN32fPortList         []int                  `json:"senderN32fPortList,omitempty"`
	N32KeepaliveTimer          *int                   `json:"n32KeepaliveTimer,omitempty"`
}

func (r SecNegotiateReqData) Teardown() bool {
	return slices.Contains(r.SupportedSecCapabilityList, SecurityNone)
}

func (r SecNegotiateReqData) Check(p wire.Policy) error {
	if err := firstError(
		checkFqdn("/sender", r.Sender, false),
		present("/supportedSecCapabilityList", len(r.SupportedSecCapabilityList) > 0),
		wire.CheckEnums("/supportedSecCapabilityList", r.SupportedSecCapabilityList, p),
	); err != nil {
		return err
	}

	if r.Teardown() && len(r.SupportedSecCapabilityList) != 1 {
		return wire.Incorrect("/supportedSecCapabilityList", false, "NONE combined with other capabilities")
	}

	if err := firstError(
		wire.CheckAll("/plmnIdList", r.PlmnIDList, p),
		wire.CheckOptional("/targetPlmnId", r.TargetPlmnID, p),
		checkPurposes("/intendedUsagePurpose", r.IntendedUsagePurpose, p, requestedPurposes),
		wire.Nest("/supportedFeatures", r.SupportedFeatures.Check(p)),
	); err != nil || p != wire.Strict {
		return err
	}

	if r.N32HandshakeID != "" && !handshakeIDPattern.MatchString(r.N32HandshakeID) {
		return wire.Incorrect("/n32HandshakeId", true, "not 16 hexadecimal digits")
	}

	for i, port := range r.SenderN32fPortList {
		if port < 0 || port > 65535 {
			return wire.Incorrect("/senderN32fPortList/"+strconv.Itoa(i), true, "not a TCP port")
		}
	}

	return firstError(
		checkFqdn("/senderN32fFqdn", r.SenderN32fFqdn, true),
		wire.CheckAll("/snpnIdList", r.SnpnIDList, p),
		wire.CheckOptional("/targetSnpnId", r.TargetSnpnID, p),
	)
}

type SecNegotiateRspData struct {
	Sender                 string                 `json:"sender"`
	SelectedSecCapability  SecurityCapability     `json:"selectedSecCapability"`
	N32HandshakeID         string                 `json:"n32HandshakeId,omitempty"`
	TargetAPIRootSupported bool                   `json:"3GppSbiTargetApiRootSupported,omitempty"`
	PlmnIDList             []wire.PlmnID          `json:"plmnIdList,omitempty"`
	SnpnIDList             []wire.PlmnIDNid       `json:"snpnIdList,omitempty"`
	AllowedUsagePurpose    []IntendedN32Purpose   `json:"allowedUsagePurpose,omitempty"`
	RejectedUsagePurpose   []IntendedN32Purpose   `json:"rejectedUsagePurpose,omitempty"`
	SupportedFeatures      wire.SupportedFeatures `json:"supportedFeatures,omitempty"`
	SenderN32fFqdn         string                 `json:"senderN32fFqdn,omitempty"`
	SenderN32fPort         *int                   `json:"senderN32fPort,omitempty"`
	N32KeepaliveTimer      *int                   `json:"n32KeepaliveTimer,omitempty"`
}

func (r SecNegotiateRspData) Check(p wire.Policy) error {
	if err := firstError(
		checkFqdn("/sender", r.Sender, false),
		wire.CheckEnum("/selectedSecCapability", r.SelectedSecCapability, p, false),
		wire.CheckAll("/plmnIdList", r.PlmnIDList, p),
		checkPurposes("/allowedUsagePurpose", r.AllowedUsagePurpose, p, allowedPurposes),
		checkPurposes("/rejectedUsagePurpose", r.RejectedUsagePurpose, p, rejectedPurposes),
		wire.Nest("/supportedFeatures", r.SupportedFeatures.Check(p)),
	); err != nil || p != wire.Strict {
		return err
	}

	if r.N32HandshakeID != "" && !handshakeIDPattern.MatchString(r.N32HandshakeID) {
		return wire.Incorrect("/n32HandshakeId", true, "not 16 hexadecimal digits")
	}

	if r.SenderN32fPort != nil && (*r.SenderN32fPort < 0 || *r.SenderN32fPort > 65535) {
		return wire.Incorrect("/senderN32fPort", true, "not a TCP port")
	}

	return firstError(
		checkFqdn("/senderN32fFqdn", r.SenderN32fFqdn, true),
		wire.CheckAll("/snpnIdList", r.SnpnIDList, p),
	)
}

func (r SecNegotiateRspData) Purposes() []N32Purpose {
	if len(r.AllowedUsagePurpose) == 0 {
		return slices.Clone(defaultPurposes)
	}

	out := make([]N32Purpose, 0, len(r.AllowedUsagePurpose))
	for _, ip := range r.AllowedUsagePurpose {
		out = append(out, ip.UsagePurpose)
	}

	return out
}

func checkFqdn(param, v string, optional bool) error {
	if v == "" {
		if optional {
			return nil
		}

		return wire.Missing(param)
	}

	if !wire.ValidFqdn(v) {
		return wire.Incorrect(param, optional, "%q is not an FQDN", v)
	}

	return nil
}

func present(param string, ok bool) error {
	if !ok {
		return wire.Missing(param)
	}

	return nil
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

var ErrNegotiation = errors.New("sepp: N32-c negotiation failed")

type NegotiationError struct {
	Reason          string
	TeardownAllowed bool
}

func (e *NegotiationError) Error() string {
	return ErrNegotiation.Error() + ": " + e.Reason
}

func (e *NegotiationError) Unwrap() error {
	return ErrNegotiation
}

type Negotiation struct {
	LocalFQDN         string
	LocalPlmnID       wire.PlmnID
	PeerFQDN          string
	PeerPlmnID        wire.PlmnID
	PeerCertDNSNames  []string
	SupportedFeatures wire.SupportedFeatures
}

func (n Negotiation) Request() SecNegotiateReqData {
	target := n.PeerPlmnID

	return SecNegotiateReqData{
		Sender:                     n.LocalFQDN,
		SupportedSecCapabilityList: []SecurityCapability{SecurityTLS},
		TargetAPIRootSupported:     true,
		PlmnIDList:                 []wire.PlmnID{n.LocalPlmnID},
		TargetPlmnID:               &target,
		SupportedFeatures:          n.SupportedFeatures,
	}
}

func (n Negotiation) TeardownRequest() SecNegotiateReqData {
	return SecNegotiateReqData{
		Sender:                     n.LocalFQDN,
		SupportedSecCapabilityList: []SecurityCapability{SecurityNone},
	}
}

type Outcome int

const (
	OutcomeEstablish Outcome = iota + 1
	OutcomeTeardown
)

type Established struct {
	NFTLST   bool
	Purposes []N32Purpose
}

func sameFQDN(a, b string) bool {
	return strings.EqualFold(strings.TrimSuffix(a, "."), strings.TrimSuffix(b, "."))
}

func (n Negotiation) senderMatches(sender string) bool {
	return sameFQDN(sender, n.PeerFQDN) && slices.ContainsFunc(n.PeerCertDNSNames, func(name string) bool { return sameFQDN(name, sender) })
}

func LocalPrecedes(local, peer string) bool {
	return strings.ToLower(strings.TrimSuffix(local, ".")) < strings.ToLower(strings.TrimSuffix(peer, "."))
}

func (n Negotiation) Respond(req SecNegotiateReqData) (SecNegotiateRspData, Outcome, error) {
	if !n.senderMatches(req.Sender) {
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "sender does not match the partner SEPP")
	}

	if req.Teardown() {
		return SecNegotiateRspData{Sender: n.LocalFQDN, SelectedSecCapability: SecurityNone}, OutcomeTeardown, nil
	}

	switch {
	case !slices.Contains(req.SupportedSecCapabilityList, SecurityTLS):
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "TLS is the only supported security capability")
	case !req.TargetAPIRootSupported:
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "3gpp-Sbi-Target-apiRoot support is required")
	case len(req.SnpnIDList) > 0 || req.TargetSnpnID != nil:
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "SNPN interconnect is not supported")
	case len(req.PlmnIDList) > 0 && !slices.Contains(req.PlmnIDList, n.PeerPlmnID):
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "plmnIdList does not contain the partner PLMN")
	case req.TargetPlmnID != nil && *req.TargetPlmnID != n.LocalPlmnID:
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "targetPlmnId is not the local PLMN")
	}

	rsp := SecNegotiateRspData{
		Sender:                 n.LocalFQDN,
		SelectedSecCapability:  SecurityTLS,
		TargetAPIRootSupported: true,
		PlmnIDList:             []wire.PlmnID{n.LocalPlmnID},
		SupportedFeatures:      req.SupportedFeatures.Intersect(n.SupportedFeatures),
	}

	if len(req.IntendedUsagePurpose) == 0 {
		return rsp, OutcomeEstablish, nil
	}

	for _, ip := range req.IntendedUsagePurpose {
		list := &rsp.RejectedUsagePurpose
		if slices.Contains(acceptedPurposes, ip.UsagePurpose) {
			list = &rsp.AllowedUsagePurpose
		}

		if !slices.ContainsFunc(*list, func(x IntendedN32Purpose) bool { return x.UsagePurpose == ip.UsagePurpose }) {
			*list = append(*list, IntendedN32Purpose{UsagePurpose: ip.UsagePurpose})
		}
	}

	if rsp.AllowedUsagePurpose == nil {
		return SecNegotiateRspData{}, 0, forbidden(wire.CauseRequestedPurposeNotAllowed, "only ROAMING and INTER_PLMN_MOBILITY are allowed")
	}

	return rsp, OutcomeEstablish, nil
}

func hasPurpose(list []IntendedN32Purpose, p N32Purpose) bool {
	return slices.ContainsFunc(list, func(ip IntendedN32Purpose) bool { return ip.UsagePurpose == p })
}

func (n Negotiation) CheckResponse(rsp SecNegotiateRspData) (Established, error) {
	nftlst := rsp.SupportedFeatures.Has(FeatureNFTLST)
	fail := func(reason string) (Established, error) {
		return Established{}, &NegotiationError{Reason: reason, TeardownAllowed: rsp.SelectedSecCapability == SecurityTLS && nftlst}
	}

	switch {
	case !n.senderMatches(rsp.Sender):
		return fail("sender " + strconv.Quote(rsp.Sender) + " does not match the partner SEPP")
	case rsp.SelectedSecCapability != SecurityTLS:
		return fail("selected security capability " + strconv.Quote(string(rsp.SelectedSecCapability)))
	case !rsp.TargetAPIRootSupported:
		return fail("peer does not support 3gpp-Sbi-Target-apiRoot")
	case len(rsp.PlmnIDList) > 0 && !slices.Contains(rsp.PlmnIDList, n.PeerPlmnID):
		return fail("plmnIdList does not contain the partner PLMN " + n.PeerPlmnID.String())
	case len(rsp.SnpnIDList) > 0:
		return fail("peer answered with SNPN IDs")
	case hasPurpose(rsp.RejectedUsagePurpose, PurposeRoaming),
		len(rsp.AllowedUsagePurpose) > 0 && !hasPurpose(rsp.AllowedUsagePurpose, PurposeRoaming):
		return fail("ROAMING is not an allowed purpose")
	}

	return Established{NFTLST: nftlst, Purposes: rsp.Purposes()}, nil
}

func (n Negotiation) CheckTeardownResponse(rsp SecNegotiateRspData) error {
	switch {
	case !n.senderMatches(rsp.Sender):
		return &NegotiationError{Reason: "sender " + strconv.Quote(rsp.Sender) + " does not match the partner SEPP"}
	case rsp.SelectedSecCapability != SecurityNone:
		return &NegotiationError{Reason: "teardown answered with " + strconv.Quote(string(rsp.SelectedSecCapability))}
	}

	return nil
}
