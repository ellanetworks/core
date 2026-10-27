// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	QueryTargetNFType          = "target-nf-type"
	QueryRequesterNFType       = "requester-nf-type"
	QueryServiceNames          = "service-names"
	QueryTargetPlmnList        = "target-plmn-list"
	QueryRequesterPlmnList     = "requester-plmn-list"
	QueryRoutingIndicator      = "routing-indicator"
	QuerySupi                  = "supi"
	QueryRequesterNFInstanceID = "requester-nf-instance-id"
	QueryRequesterFeatures     = "requester-features"
)

type NFType string

const (
	NFTypeNRF  NFType = "NRF"
	NFTypeUDM  NFType = "UDM"
	NFTypeAMF  NFType = "AMF"
	NFTypeSMF  NFType = "SMF"
	NFTypeAUSF NFType = "AUSF"
	NFTypeNEF  NFType = "NEF"
	NFTypePCF  NFType = "PCF"
	NFTypeSMSF NFType = "SMSF"
	NFTypeNSSF NFType = "NSSF"
	NFTypeUDR  NFType = "UDR"
	NFTypeSEPP NFType = "SEPP"
	NFTypeUPF  NFType = "UPF"
	NFTypeSCP  NFType = "SCP"
)

var knownNFTypes = strings.Fields(`NRF UDM AMF SMF AUSF NEF PCF SMSF NSSF UDR LMF GMLC 5G_EIR SEPP UPF N3IWF AF UDSF
	BSF CHF NWDAF PCSCF CBCF HSS UCMF SOR_AF SPAF MME SCSAS SCEF SCP NSSAAF ICSCF SCSCF DRA IMS_AS AANF 5G_DDNMF
	NSACF MFAF EASDF DCCF MB_SMF TSCTSF ADRF GBA_BSF CEF MB_UPF NSWOF PKMF MNPF SMS_GMSC SMS_IWMSC MBSF MBSTF PANF
	IP_SM_GW SMS_ROUTER DCSF MRF MRFP MF SLPKMF RH EIF AIOTF ADM`)

func (t NFType) Known() bool {
	return slices.Contains(knownNFTypes, string(t))
}

type NFStatus string

const (
	NFStatusRegistered     NFStatus = "REGISTERED"
	NFStatusSuspended      NFStatus = "SUSPENDED"
	NFStatusUndiscoverable NFStatus = "UNDISCOVERABLE"
	NFStatusCanaryRelease  NFStatus = "CANARY_RELEASE"
)

func (s NFStatus) Known() bool {
	return oneOf(s, NFStatusRegistered, NFStatusSuspended, NFStatusUndiscoverable, NFStatusCanaryRelease)
}

type TransportProtocol string

const TransportTCP TransportProtocol = "TCP"

func (t TransportProtocol) Known() bool {
	return t == TransportTCP
}

type NFServiceVersion struct {
	APIVersionInURI string   `json:"apiVersionInUri"`
	APIFullVersion  string   `json:"apiFullVersion"`
	Expiry          DateTime `json:"expiry,omitempty"`
}

func (v NFServiceVersion) Check(p Policy) error {
	if err := firstError(
		present("/apiVersionInUri", v.APIVersionInURI != ""),
		present("/apiFullVersion", v.APIFullVersion != ""),
	); err != nil || p != Strict {
		return err
	}

	return checkDateTime("/expiry", v.Expiry, true)
}

type IPEndPoint struct {
	Ipv4Address string            `json:"ipv4Address,omitempty"`
	Ipv6Address string            `json:"ipv6Address,omitempty"`
	Transport   TransportProtocol `json:"transport,omitempty"`
	Port        *int              `json:"port,omitempty"`
}

func (e IPEndPoint) Check(p Policy) error {
	if err := checkOptionalRange("/port", e.Port, 0, 65535); err != nil || p != Strict {
		return err
	}

	if e.Ipv4Address != "" && e.Ipv6Address != "" {
		return Incorrect("/ipv6Address", true, "both ipv4Address and ipv6Address")
	}

	if e.Ipv4Address != "" {
		if ip := net.ParseIP(e.Ipv4Address); ip == nil || ip.To4() == nil || strings.Contains(e.Ipv4Address, ":") {
			return Incorrect("/ipv4Address", true, "invalid IPv4 address %q", e.Ipv4Address)
		}
	}

	if e.Ipv6Address != "" {
		if ip := net.ParseIP(e.Ipv6Address); ip == nil || !strings.Contains(e.Ipv6Address, ":") {
			return Incorrect("/ipv6Address", true, "invalid IPv6 address %q", e.Ipv6Address)
		}
	}

	return CheckEnum("/transport", e.Transport, p, true)
}

type NFService struct {
	ServiceInstanceID string             `json:"serviceInstanceId"`
	ServiceName       ServiceName        `json:"serviceName"`
	Versions          []NFServiceVersion `json:"versions"`
	Scheme            URIScheme          `json:"scheme"`
	NfServiceStatus   NFStatus           `json:"nfServiceStatus"`
	Fqdn              string             `json:"fqdn,omitempty"`
	InterPlmnFqdn     string             `json:"interPlmnFqdn,omitempty"`
	IPEndPoints       []IPEndPoint       `json:"ipEndPoints,omitempty"`
	APIPrefix         string             `json:"apiPrefix,omitempty"`
	AllowedPlmns      []PlmnID           `json:"allowedPlmns,omitempty"`
	AllowedNfTypes    []NFType           `json:"allowedNfTypes,omitempty"`
	Priority          *int               `json:"priority,omitempty"`
	Capacity          *int               `json:"capacity,omitempty"`
	Load              *int               `json:"load,omitempty"`
	SupportedFeatures SupportedFeatures  `json:"supportedFeatures,omitempty"`
}

func (s NFService) Check(p Policy) error {
	if err := firstError(
		present("/serviceInstanceId", s.ServiceInstanceID != ""),
		present("/serviceName", s.ServiceName != ""),
		present("/versions", len(s.Versions) > 0),
		CheckEnum("/scheme", s.Scheme, p, false),
		CheckEnum("/nfServiceStatus", s.NfServiceStatus, p, false),
		CheckAll("/versions", s.Versions, p),
		CheckAll("/ipEndPoints", s.IPEndPoints, p),
		CheckAll("/allowedPlmns", s.AllowedPlmns, p),
	); err != nil || p != Strict {
		return err
	}

	return firstError(
		checkString("/fqdn", s.Fqdn, ValidFqdn, true),
		checkString("/interPlmnFqdn", s.InterPlmnFqdn, ValidFqdn, true),
		checkAPIPrefix(s.APIPrefix),
		CheckEnums("/allowedNfTypes", s.AllowedNfTypes, p),
		checkOptionalRange("/priority", s.Priority, 0, 65535),
		checkOptionalRange("/capacity", s.Capacity, 0, 65535),
		checkOptionalRange("/load", s.Load, 0, 100),
		Nest("/supportedFeatures", s.SupportedFeatures.Check(p)),
	)
}

func (s NFService) supportsVersion(v string) bool {
	return slices.ContainsFunc(s.Versions, func(sv NFServiceVersion) bool { return sv.APIVersionInURI == v })
}

func checkAPIPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}

	u, err := url.Parse(prefix)
	if err != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(prefix, "/") {
		return Incorrect("/apiPrefix", true, "%q is not an absolute path", prefix)
	}

	return nil
}

type NFProfile struct {
	NfInstanceID   string               `json:"nfInstanceId"`
	NfInstanceName string               `json:"nfInstanceName,omitempty"`
	NfType         NFType               `json:"nfType"`
	NfStatus       NFStatus             `json:"nfStatus"`
	PlmnList       []PlmnID             `json:"plmnList,omitempty"`
	Fqdn           string               `json:"fqdn,omitempty"`
	InterPlmnFqdn  string               `json:"interPlmnFqdn,omitempty"`
	Ipv4Addresses  []string             `json:"ipv4Addresses,omitempty"`
	Ipv6Addresses  []string             `json:"ipv6Addresses,omitempty"`
	AllowedPlmns   []PlmnID             `json:"allowedPlmns,omitempty"`
	AllowedNfTypes []NFType             `json:"allowedNfTypes,omitempty"`
	Priority       *int                 `json:"priority,omitempty"`
	Capacity       *int                 `json:"capacity,omitempty"`
	Load           *int                 `json:"load,omitempty"`
	Locality       string               `json:"locality,omitempty"`
	NfServices     []NFService          `json:"nfServices,omitempty"`
	NfServiceList  map[string]NFService `json:"nfServiceList,omitempty"`
}

func (n *NFProfile) UnmarshalJSON(data []byte) error {
	type plain NFProfile

	aux := struct {
		*plain
		NfServices    []json.RawMessage          `json:"nfServices"`
		NfServiceList map[string]json.RawMessage `json:"nfServiceList"`
	}{plain: (*plain)(n)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	n.NfServices = decodeValid[NFService](aux.NfServices)
	n.NfServiceList = decodeValidMap(aux.NfServiceList, func(string) bool { return true }, func(id string, s *NFService) {
		if s.ServiceInstanceID == "" {
			s.ServiceInstanceID = id
		}
	})

	return nil
}

func (n NFProfile) Check(p Policy) error {
	if err := firstError(
		present("/nfInstanceId", n.NfInstanceID != ""),
		CheckEnum("/nfType", n.NfType, p, false),
		CheckEnum("/nfStatus", n.NfStatus, p, false),
		CheckAll("/plmnList", n.PlmnList, p),
		CheckAll("/allowedPlmns", n.AllowedPlmns, p),
		CheckAll("/nfServices", n.NfServices, p),
	); err != nil {
		return err
	}

	for _, id := range sortedKeys(n.NfServiceList) {
		if err := n.NfServiceList[id].Check(p); err != nil {
			return Nest("/nfServiceList/"+id, err)
		}
	}

	if p != Strict {
		return nil
	}

	return firstError(
		checkString("/nfInstanceId", n.NfInstanceID, ValidNfInstanceID, false),
		checkString("/fqdn", n.Fqdn, ValidFqdn, true),
		checkString("/interPlmnFqdn", n.InterPlmnFqdn, ValidFqdn, true),
		CheckEnums("/allowedNfTypes", n.AllowedNfTypes, p),
		checkOptionalRange("/priority", n.Priority, 0, 65535),
		checkOptionalRange("/capacity", n.Capacity, 0, 65535),
		checkOptionalRange("/load", n.Load, 0, 100),
	)
}

func (n NFProfile) Services() []NFService {
	out := append([]NFService(nil), n.NfServices...)

	for _, id := range sortedKeys(n.NfServiceList) {
		s := n.NfServiceList[id]
		if s.ServiceInstanceID == "" {
			s.ServiceInstanceID = id
		}

		out = append(out, s)
	}

	return out
}

type Selector struct {
	Service   ServiceName
	Target    PlmnID
	Requester PlmnID
}

type Candidate struct {
	NfInstanceID string
	Service      NFService
	APIRoot      string
}

func (n NFProfile) Candidate(sel Selector) (Candidate, bool) {
	if n.NfStatus != NFStatusRegistered {
		return Candidate{}, false
	}

	if len(n.PlmnList) > 0 && !slices.Contains(n.PlmnList, sel.Target) {
		return Candidate{}, false
	}

	version := sel.Service.Version()

	for _, s := range n.Services() {
		if s.ServiceName != sel.Service || s.NfServiceStatus != NFStatusRegistered || !s.supportsVersion(version) {
			continue
		}

		allowed := s.AllowedPlmns
		if len(allowed) == 0 {
			allowed = n.AllowedPlmns
		}

		if len(allowed) > 0 && !slices.Contains(allowed, sel.Requester) {
			continue
		}

		root, err := n.APIRoot(s)
		if err != nil {
			continue
		}

		return Candidate{NfInstanceID: n.NfInstanceID, Service: s, APIRoot: root}, true
	}

	return Candidate{}, false
}

func (n NFProfile) APIRoot(s NFService) (string, error) {
	if s.Scheme != SchemeHTTP && s.Scheme != SchemeHTTPS {
		return "", fmt.Errorf("wire: service %s has scheme %q", s.ServiceName, s.Scheme)
	}

	var host string

	for _, h := range []string{s.InterPlmnFqdn, n.InterPlmnFqdn, s.Fqdn, n.Fqdn} {
		if h != "" {
			host = h

			break
		}
	}

	if host == "" {
		return "", fmt.Errorf("wire: service %s of NF %s has no FQDN", s.ServiceName, n.NfInstanceID)
	}

	if !ValidFqdn(host) {
		return "", fmt.Errorf("wire: service %s of NF %s has invalid FQDN %q", s.ServiceName, n.NfInstanceID, host)
	}

	authority := strings.TrimSuffix(host, ".")

	for _, e := range s.IPEndPoints {
		if e.Port != nil {
			if *e.Port < 1 || *e.Port > 65535 {
				return "", fmt.Errorf("wire: service %s has invalid port %d", s.ServiceName, *e.Port)
			}

			authority = net.JoinHostPort(authority, strconv.Itoa(*e.Port))

			break
		}
	}

	if err := checkAPIPrefix(s.APIPrefix); err != nil {
		return "", fmt.Errorf("wire: service %s: %v", s.ServiceName, err)
	}

	return string(s.Scheme) + "://" + authority + strings.TrimSuffix(s.APIPrefix, "/"), nil
}

type SearchResult struct {
	ValidityPeriod       *int              `json:"validityPeriod"`
	NfInstances          []NFProfile       `json:"nfInstances"`
	SearchID             string            `json:"searchId,omitempty"`
	NrfSupportedFeatures SupportedFeatures `json:"nrfSupportedFeatures,omitempty"`
}

func (r *SearchResult) UnmarshalJSON(data []byte) error {
	type plain SearchResult

	aux := struct {
		*plain
		NfInstances []json.RawMessage `json:"nfInstances"`
	}{plain: (*plain)(r)}

	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	r.NfInstances = decodeValid[NFProfile](aux.NfInstances)

	return nil
}

func (r SearchResult) Check(p Policy) error {
	if err := firstError(
		present("/validityPeriod", r.ValidityPeriod != nil),
		present("/nfInstances", r.NfInstances != nil),
	); err != nil {
		return err
	}

	if *r.ValidityPeriod < 0 {
		return Incorrect("/validityPeriod", false, "negative validity period %d", *r.ValidityPeriod)
	}

	if err := CheckAll("/nfInstances", r.NfInstances, p); err != nil || p != Strict {
		return err
	}

	return Nest("/nrfSupportedFeatures", r.NrfSupportedFeatures.Check(p))
}

func (r SearchResult) Candidates(sel Selector) []Candidate {
	var out []Candidate

	for _, n := range r.NfInstances {
		if c, ok := n.Candidate(sel); ok {
			out = append(out, c)
		}
	}

	return out
}

type DiscoveryQuery struct {
	TargetNFType          NFType
	RequesterNFType       NFType
	ServiceNames          []ServiceName
	TargetPlmnList        []PlmnID
	RequesterPlmnList     []PlmnID
	RoutingIndicator      string
	Supi                  string
	RequesterNFInstanceID string
	RequesterFeatures     SupportedFeatures
}

func (q DiscoveryQuery) Values() (url.Values, error) {
	v := url.Values{}

	for name, t := range map[string]NFType{QueryTargetNFType: q.TargetNFType, QueryRequesterNFType: q.RequesterNFType} {
		if !t.Known() {
			return nil, queryError(name, fmt.Errorf("invalid NF type %q", t))
		}

		v.Set(name, string(t))
	}

	if len(q.ServiceNames) > 0 {
		names := make([]string, 0, len(q.ServiceNames))
		seen := map[ServiceName]bool{}

		for _, s := range q.ServiceNames {
			if s == "" || strings.Contains(string(s), ",") || seen[s] {
				return nil, queryError(QueryServiceNames, fmt.Errorf("invalid or repeated service name %q", s))
			}

			seen[s] = true

			names = append(names, string(s))
		}

		v.Set(QueryServiceNames, strings.Join(names, ","))
	}

	if err := checkInterPLMNTarget(q.TargetPlmnList, q.RequesterPlmnList); err != nil {
		return nil, err
	}

	for _, param := range []struct {
		name string
		list []PlmnID
	}{{QueryTargetPlmnList, q.TargetPlmnList}, {QueryRequesterPlmnList, q.RequesterPlmnList}} {
		if len(param.list) == 0 {
			continue
		}

		for i, p := range param.list {
			if err := p.Check(Strict); err != nil {
				return nil, queryError(param.name, NestIndex("", i, err))
			}
		}

		b, err := json.Marshal(param.list)
		if err != nil {
			return nil, queryError(param.name, err)
		}

		v.Set(param.name, string(b))
	}

	for _, opt := range []struct {
		name, value string
		valid       func(string) bool
	}{
		{QueryRoutingIndicator, q.RoutingIndicator, ValidRoutingIndicator},
		{QuerySupi, q.Supi, ValidSupi},
		{QueryRequesterNFInstanceID, q.RequesterNFInstanceID, ValidNfInstanceID},
		{QueryRequesterFeatures, string(q.RequesterFeatures), func(s string) bool { return SupportedFeatures(s).Check(Strict) == nil }},
	} {
		if opt.value == "" {
			continue
		}

		if !opt.valid(opt.value) {
			return nil, queryError(opt.name, fmt.Errorf("invalid value %q", opt.value))
		}

		v.Set(opt.name, opt.value)
	}

	return v, nil
}

func checkInterPLMNTarget(target, requester []PlmnID) error {
	if len(target) == 0 {
		return nil
	}

	if len(requester) == 0 {
		return queryError(QueryRequesterPlmnList, errors.New("required with target-plmn-list"))
	}

	interPLMN := slices.ContainsFunc(target, func(p PlmnID) bool { return !slices.Contains(requester, p) })
	if interPLMN && len(target) > 1 {
		return queryError(QueryTargetPlmnList, errors.New("inter-PLMN discovery takes one target PLMN"))
	}

	return nil
}
