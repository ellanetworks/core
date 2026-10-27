// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	ServiceNnrfDisc            = "nnrf-disc"
	NnrfDiscAPIRoot            = "/nnrf-disc/v1"
	NnrfDiscNFInstances        = NnrfDiscAPIRoot + "/nf-instances"
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

type ServiceName string

const (
	ServiceNameNnrfDisc  ServiceName = ServiceNnrfDisc
	ServiceNameNausfAuth ServiceName = ServiceNausfAuth
	ServiceNameNudmSDM   ServiceName = ServiceNudmSDM
	ServiceNameNudmUECM  ServiceName = ServiceNudmUECM
)

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
	if v.APIVersionInURI == "" {
		return missing("/apiVersionInUri")
	}

	if p != Strict {
		return nil
	}

	if v.APIFullVersion == "" {
		return missing("/apiFullVersion")
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
	if e.Port != nil {
		if err := checkRange("/port", *e.Port, 0, 65535, true); err != nil {
			return err
		}
	}

	if p != Strict {
		return nil
	}

	if e.Ipv4Address != "" && e.Ipv6Address != "" {
		return incorrect("/ipv6Address", true, "both ipv4Address and ipv6Address")
	}

	if e.Ipv4Address != "" {
		if ip := net.ParseIP(e.Ipv4Address); ip == nil || ip.To4() == nil || strings.Contains(e.Ipv4Address, ":") {
			return incorrect("/ipv4Address", true, "invalid IPv4 address %q", e.Ipv4Address)
		}
	}

	if e.Ipv6Address != "" {
		if ip := net.ParseIP(e.Ipv6Address); ip == nil || !strings.Contains(e.Ipv6Address, ":") {
			return incorrect("/ipv6Address", true, "invalid IPv6 address %q", e.Ipv6Address)
		}
	}

	return checkEnum("/transport", e.Transport, p, true)
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
	if s.ServiceName == "" {
		return missing("/serviceName")
	}

	if s.Scheme == "" {
		return missing("/scheme")
	}

	if s.NfServiceStatus == "" {
		return missing("/nfServiceStatus")
	}

	if err := firstError(
		checkString("/fqdn", s.Fqdn, ValidFqdn, true),
		checkAll("/ipEndPoints", s.IPEndPoints, p),
		checkAPIPrefix(s.APIPrefix),
	); err != nil {
		return err
	}

	if p != Strict {
		return nil
	}

	if s.ServiceInstanceID == "" {
		return missing("/serviceInstanceId")
	}

	if len(s.Versions) == 0 {
		return missing("/versions")
	}

	return firstError(
		checkAll("/versions", s.Versions, p),
		checkEnum("/scheme", s.Scheme, p, false),
		checkEnum("/nfServiceStatus", s.NfServiceStatus, p, false),
		checkString("/interPlmnFqdn", s.InterPlmnFqdn, ValidFqdn, true),
		checkAll("/allowedPlmns", s.AllowedPlmns, p),
		checkEnums("/allowedNfTypes", s.AllowedNfTypes, p),
		checkOptionalMax("/priority", s.Priority, 65535),
		checkOptionalMax("/capacity", s.Capacity, 65535),
		checkOptionalMax("/load", s.Load, 100),
		Nest("/supportedFeatures", s.SupportedFeatures.Check(p)),
	)
}

func checkAPIPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}

	u, err := url.Parse(prefix)
	if err != nil || u.Scheme != "" || u.Host != "" || u.RawQuery != "" || u.Fragment != "" || !strings.HasPrefix(prefix, "/") {
		return incorrect("/apiPrefix", true, "%q is not an absolute path", prefix)
	}

	return nil
}

func checkOptionalMax(param string, v *int, hi int) error {
	if v == nil {
		return nil
	}

	return checkRange(param, *v, 0, hi, true)
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

func (n NFProfile) Check(p Policy) error {
	if n.NfInstanceID == "" {
		return missing("/nfInstanceId")
	}

	if err := firstError(
		checkEnum("/nfType", n.NfType, p, false),
		checkEnum("/nfStatus", n.NfStatus, p, false),
		checkString("/fqdn", n.Fqdn, ValidFqdn, true),
		checkAll("/plmnList", n.PlmnList, p),
		checkAll("/nfServices", n.NfServices, p),
	); err != nil {
		return err
	}

	for _, id := range slicesSorted(n.NfServiceList) {
		if err := n.NfServiceList[id].Check(p); err != nil {
			return Nest("/nfServiceList/"+id, err)
		}
	}

	if p != Strict {
		return nil
	}

	return firstError(
		checkString("/nfInstanceId", n.NfInstanceID, ValidNfInstanceID, false),
		checkString("/interPlmnFqdn", n.InterPlmnFqdn, ValidFqdn, true),
		checkAll("/allowedPlmns", n.AllowedPlmns, p),
		checkEnums("/allowedNfTypes", n.AllowedNfTypes, p),
		checkOptionalMax("/priority", n.Priority, 65535),
		checkOptionalMax("/capacity", n.Capacity, 65535),
		checkOptionalMax("/load", n.Load, 100),
	)
}

func (n NFProfile) Services() []NFService {
	out := append([]NFService(nil), n.NfServices...)

	for _, id := range slicesSorted(n.NfServiceList) {
		s := n.NfServiceList[id]
		if s.ServiceInstanceID == "" {
			s.ServiceInstanceID = id
		}

		out = append(out, s)
	}

	return out
}

func (n NFProfile) Service(name ServiceName) (NFService, bool) {
	for _, s := range n.Services() {
		if s.ServiceName == name && s.NfServiceStatus == NFStatusRegistered {
			return s, true
		}
	}

	return NFService{}, false
}

func (n NFProfile) APIRoot(s NFService) (string, error) {
	if s.Scheme != SchemeHTTP && s.Scheme != SchemeHTTPS {
		return "", fmt.Errorf("wire: service %s has scheme %q", s.ServiceName, s.Scheme)
	}

	host := s.Fqdn
	if host == "" {
		host = n.Fqdn
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
		return "", fmt.Errorf("wire: service %s: %w", s.ServiceName, err)
	}

	return string(s.Scheme) + "://" + authority + strings.TrimSuffix(s.APIPrefix, "/"), nil
}

type SearchResult struct {
	ValidityPeriod       int               `json:"validityPeriod"`
	NfInstances          []NFProfile       `json:"nfInstances"`
	SearchID             string            `json:"searchId,omitempty"`
	NrfSupportedFeatures SupportedFeatures `json:"nrfSupportedFeatures,omitempty"`
}

func (r *SearchResult) UnmarshalJSON(data []byte) error {
	if err := requireKeys(data, "validityPeriod", "nfInstances"); err != nil {
		return err
	}

	type plain SearchResult

	return json.Unmarshal(data, (*plain)(r))
}

func (r SearchResult) Check(p Policy) error {
	if r.ValidityPeriod < 0 {
		return incorrect("/validityPeriod", false, "negative validity period %d", r.ValidityPeriod)
	}

	if err := checkAll("/nfInstances", r.NfInstances, p); err != nil {
		return err
	}

	if p == Strict {
		return Nest("/nrfSupportedFeatures", r.NrfSupportedFeatures.Check(p))
	}

	return nil
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

	if err := checkEnum("query "+QueryTargetNFType, q.TargetNFType, Strict, false); err != nil {
		return nil, err
	}

	if err := checkEnum("query "+QueryRequesterNFType, q.RequesterNFType, Strict, false); err != nil {
		return nil, err
	}

	v.Set(QueryTargetNFType, string(q.TargetNFType))
	v.Set(QueryRequesterNFType, string(q.RequesterNFType))

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

	if q.RoutingIndicator != "" {
		if !ValidRoutingIndicator(q.RoutingIndicator) {
			return nil, queryError(QueryRoutingIndicator, fmt.Errorf("invalid routing indicator %q", q.RoutingIndicator))
		}

		v.Set(QueryRoutingIndicator, q.RoutingIndicator)
	}

	if q.Supi != "" {
		if !ValidSupi(q.Supi) {
			return nil, queryError(QuerySupi, fmt.Errorf("invalid SUPI %q", q.Supi))
		}

		v.Set(QuerySupi, q.Supi)
	}

	if q.RequesterNFInstanceID != "" {
		if !ValidNfInstanceID(q.RequesterNFInstanceID) {
			return nil, queryError(QueryRequesterNFInstanceID, fmt.Errorf("invalid NF instance ID %q", q.RequesterNFInstanceID))
		}

		v.Set(QueryRequesterNFInstanceID, q.RequesterNFInstanceID)
	}

	if q.RequesterFeatures != "" {
		if err := q.RequesterFeatures.Check(Strict); err != nil {
			return nil, queryError(QueryRequesterFeatures, err)
		}

		v.Set(QueryRequesterFeatures, string(q.RequesterFeatures))
	}

	return v, nil
}
