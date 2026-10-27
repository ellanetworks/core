// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const (
	N32cAPIRoot            = "/n32c-handshake/v1"
	N32cExchangeCapability = N32cAPIRoot + "/exchange-capability"
)

const (
	CauseRequestedParamMismatch         = "REQUESTED_PARAM_MISMATCH"
	CauseRequestedPurposeNotAllowed     = "REQUESTED_PURPOSE_NOT_ALLOWED"
	CauseNegotiationNotAllowed          = "NEGOTIATION_NOT_ALLOWED"
	CauseN32cExchangeCapabilityOngoing  = "N32C_EXCHANGE_CAPABILITY_ONGOING"
	CauseSecurityParamExchangeCollision = "SECURITY_PARAM_EXCHANGE_COLLISION"
)

const (
	FeatureNFTLST  = 1
	FeaturePSEPRO  = 2
	FeaturePSIU    = 3
	FeatureSNDN32F = 4
	FeatureTLSCOR  = 5
)

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

type IntendedN32Purpose struct {
	UsagePurpose   N32Purpose `json:"usagePurpose"`
	AdditionalInfo string     `json:"additionalInfo,omitempty"`
	Cause          string     `json:"cause,omitempty"`
}

func (ip IntendedN32Purpose) Check(p wire.Policy) error {
	if ip.UsagePurpose == "" {
		return &wire.IEError{Param: "/usagePurpose", Missing: true}
	}

	if p == wire.Strict && !ip.UsagePurpose.Known() {
		return &wire.IEError{Param: "/usagePurpose", Reason: "unknown value " + strconv.Quote(string(ip.UsagePurpose))}
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

func (r SecNegotiateReqData) Check(p wire.Policy) error {
	if err := checkFqdn("/sender", r.Sender, false); err != nil {
		return err
	}

	if len(r.SupportedSecCapabilityList) == 0 {
		return &wire.IEError{Param: "/supportedSecCapabilityList", Missing: true}
	}

	for i, c := range r.SupportedSecCapabilityList {
		if err := checkCapability("/supportedSecCapabilityList/"+strconv.Itoa(i), c, p); err != nil {
			return err
		}
	}

	if err := checkPlmnIDs("/plmnIdList", r.PlmnIDList, p); err != nil {
		return err
	}

	if err := checkOptionalPlmnID("/targetPlmnId", r.TargetPlmnID, p); err != nil {
		return err
	}

	if err := checkPurposes("/intendedUsagePurpose", r.IntendedUsagePurpose, p); err != nil {
		return err
	}

	if err := checkFeatures(r.SupportedFeatures, p); err != nil {
		return err
	}

	if p != wire.Strict {
		return nil
	}

	if r.N32HandshakeID != "" && !handshakeIDPattern.MatchString(r.N32HandshakeID) {
		return &wire.IEError{Param: "/n32HandshakeId", Optional: true, Reason: "not 16 hexadecimal digits"}
	}

	if err := checkFqdn("/senderN32fFqdn", r.SenderN32fFqdn, true); err != nil {
		return err
	}

	for i, port := range r.SenderN32fPortList {
		if port < 0 || port > 65535 {
			return &wire.IEError{Param: "/senderN32fPortList/" + strconv.Itoa(i), Optional: true, Reason: "not a TCP port"}
		}
	}

	return checkSnpnIDs(r.SnpnIDList, r.TargetSnpnID, p)
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
	if err := checkFqdn("/sender", r.Sender, false); err != nil {
		return err
	}

	if err := checkCapability("/selectedSecCapability", r.SelectedSecCapability, p); err != nil {
		return err
	}

	if err := checkPlmnIDs("/plmnIdList", r.PlmnIDList, p); err != nil {
		return err
	}

	if err := checkFeatures(r.SupportedFeatures, p); err != nil {
		return err
	}

	if p != wire.Strict {
		return nil
	}

	if r.N32HandshakeID != "" && !handshakeIDPattern.MatchString(r.N32HandshakeID) {
		return &wire.IEError{Param: "/n32HandshakeId", Optional: true, Reason: "not 16 hexadecimal digits"}
	}

	if err := checkPurposes("/allowedUsagePurpose", r.AllowedUsagePurpose, p); err != nil {
		return err
	}

	if err := checkPurposes("/rejectedUsagePurpose", r.RejectedUsagePurpose, p); err != nil {
		return err
	}

	if err := checkFqdn("/senderN32fFqdn", r.SenderN32fFqdn, true); err != nil {
		return err
	}

	if r.SenderN32fPort != nil && (*r.SenderN32fPort < 0 || *r.SenderN32fPort > 65535) {
		return &wire.IEError{Param: "/senderN32fPort", Optional: true, Reason: "not a TCP port"}
	}

	return checkSnpnIDs(r.SnpnIDList, nil, p)
}

func checkFqdn(param, v string, optional bool) error {
	if v == "" {
		if optional {
			return nil
		}

		return &wire.IEError{Param: param, Missing: true}
	}

	if !wire.ValidFqdn(v) {
		return &wire.IEError{Param: param, Optional: optional, Reason: strconv.Quote(v) + " is not an FQDN"}
	}

	return nil
}

func checkCapability(param string, c SecurityCapability, p wire.Policy) error {
	if c == "" {
		return &wire.IEError{Param: param, Missing: true}
	}

	if p == wire.Strict && !c.Known() {
		return &wire.IEError{Param: param, Reason: "unknown value " + strconv.Quote(string(c))}
	}

	return nil
}

func checkPlmnIDs(param string, list []wire.PlmnID, p wire.Policy) error {
	for i, id := range list {
		if err := id.Check(p); err != nil {
			return wire.Nest(param+"/"+strconv.Itoa(i), err)
		}
	}

	return nil
}

func checkOptionalPlmnID(param string, id *wire.PlmnID, p wire.Policy) error {
	if id == nil {
		return nil
	}

	if err := id.Check(p); err != nil {
		return wire.Nest(param, err)
	}

	return nil
}

func checkSnpnIDs(list []wire.PlmnIDNid, target *wire.PlmnIDNid, p wire.Policy) error {
	for i, id := range list {
		if err := id.Check(p); err != nil {
			return wire.Nest("/snpnIdList/"+strconv.Itoa(i), err)
		}
	}

	if target != nil {
		if err := target.Check(p); err != nil {
			return wire.Nest("/targetSnpnId", err)
		}
	}

	return nil
}

func checkPurposes(param string, list []IntendedN32Purpose, p wire.Policy) error {
	for i, ip := range list {
		if err := ip.Check(p); err != nil {
			return wire.Nest(param+"/"+strconv.Itoa(i), err)
		}
	}

	return nil
}

func checkFeatures(f wire.SupportedFeatures, p wire.Policy) error {
	if err := f.Check(p); err != nil {
		return wire.Nest("/supportedFeatures", err)
	}

	return nil
}

var (
	ErrNegotiation = errors.New("sepp: N32-c negotiation failed")
	LocalFeatures  = wire.Features(FeatureNFTLST)
)

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
	OutcomeEstablish Outcome = iota
	OutcomeTeardown
)

func forbidden(cause, detail string) error {
	return wire.NewProblem(http.StatusForbidden, cause, detail)
}

func (n Negotiation) senderMatches(sender string) bool {
	if !strings.EqualFold(strings.TrimSuffix(sender, "."), strings.TrimSuffix(n.PeerFQDN, ".")) {
		return false
	}

	return slices.ContainsFunc(n.PeerCertDNSNames, func(name string) bool {
		return strings.EqualFold(strings.TrimSuffix(name, "."), strings.TrimSuffix(sender, "."))
	})
}

func (n Negotiation) Respond(req SecNegotiateReqData) (SecNegotiateRspData, Outcome, error) {
	if !n.senderMatches(req.Sender) {
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "sender does not match the partner SEPP")
	}

	if slices.Contains(req.SupportedSecCapabilityList, SecurityNone) {
		return SecNegotiateRspData{Sender: n.LocalFQDN, SelectedSecCapability: SecurityNone}, OutcomeTeardown, nil
	}

	if !slices.Contains(req.SupportedSecCapabilityList, SecurityTLS) {
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "TLS is the only supported security capability")
	}

	if !req.TargetAPIRootSupported {
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "3gpp-Sbi-Target-apiRoot support is required")
	}

	if len(req.PlmnIDList) > 0 && !slices.Contains(req.PlmnIDList, n.PeerPlmnID) {
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "plmnIdList does not contain the partner PLMN")
	}

	if req.TargetPlmnID != nil && *req.TargetPlmnID != n.LocalPlmnID {
		return SecNegotiateRspData{}, 0, forbidden(CauseNegotiationNotAllowed, "targetPlmnId is not the local PLMN")
	}

	rsp := SecNegotiateRspData{
		Sender:                 n.LocalFQDN,
		SelectedSecCapability:  SecurityTLS,
		TargetAPIRootSupported: true,
		PlmnIDList:             []wire.PlmnID{n.LocalPlmnID},
		SupportedFeatures:      req.SupportedFeatures.Intersect(n.SupportedFeatures),
	}

	if len(req.IntendedUsagePurpose) > 0 {
		for _, ip := range req.IntendedUsagePurpose {
			if ip.UsagePurpose == PurposeRoaming {
				rsp.AllowedUsagePurpose = []IntendedN32Purpose{{UsagePurpose: PurposeRoaming}}
			} else {
				rsp.RejectedUsagePurpose = append(rsp.RejectedUsagePurpose, IntendedN32Purpose{UsagePurpose: ip.UsagePurpose})
			}
		}

		if rsp.AllowedUsagePurpose == nil {
			return SecNegotiateRspData{}, 0, forbidden(CauseRequestedPurposeNotAllowed, "only ROAMING is allowed")
		}
	}

	return rsp, OutcomeEstablish, nil
}

func (n Negotiation) CheckResponse(rsp SecNegotiateRspData) (bool, error) {
	nftlst := rsp.SelectedSecCapability == SecurityTLS && rsp.SupportedFeatures.Has(FeatureNFTLST)

	switch {
	case !n.senderMatches(rsp.Sender):
		return nftlst, fmt.Errorf("%w: sender %q does not match the partner SEPP", ErrNegotiation, rsp.Sender)
	case rsp.SelectedSecCapability != SecurityTLS:
		return nftlst, fmt.Errorf("%w: selected security capability %q", ErrNegotiation, rsp.SelectedSecCapability)
	case !rsp.TargetAPIRootSupported:
		return nftlst, fmt.Errorf("%w: peer does not support 3gpp-Sbi-Target-apiRoot", ErrNegotiation)
	case len(rsp.PlmnIDList) > 0 && !slices.Contains(rsp.PlmnIDList, n.PeerPlmnID):
		return nftlst, fmt.Errorf("%w: plmnIdList does not contain the partner PLMN %s", ErrNegotiation, n.PeerPlmnID)
	}

	return nftlst, nil
}
