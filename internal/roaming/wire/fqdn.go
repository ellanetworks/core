// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"strings"
)

const (
	FQDNLabelNRF  = "nrf"
	FQDNLabelAUSF = "ausf"
	FQDNLabelUDM  = "udm"
	FQDNLabelAMF  = "amf"
	FQDNLabelSMF  = "smf"
	FQDNLabelSEPP = "sepp"
)

func (p PlmnID) HomeNetworkDomain() string {
	return "5gc.mnc" + p.mnc3() + ".mcc" + p.Mcc + ".3gppnetwork.org"
}

func (p PlmnID) NFFQDN(label string) string {
	return label + "." + p.HomeNetworkDomain()
}

type NetworkFQDN struct {
	Label string
	Mcc   string
	Mnc   string
}

func ParseNetworkFQDN(host string) (NetworkFQDN, bool) {
	h := strings.ToLower(strings.TrimSuffix(host, "."))

	rest, ok := strings.CutSuffix(h, ".3gppnetwork.org")
	if !ok {
		return NetworkFQDN{}, false
	}

	i := strings.LastIndex(rest, ".5gc.mnc")
	if i <= 0 {
		return NetworkFQDN{}, false
	}

	label, tail := rest[:i], rest[i+len(".5gc.mnc"):]

	mnc, mcc, ok := strings.Cut(tail, ".mcc")
	if !ok || len(mnc) != 3 || !allDigits(mnc) || len(mcc) != 3 || !allDigits(mcc) {
		return NetworkFQDN{}, false
	}

	if !ValidFqdn(h) {
		return NetworkFQDN{}, false
	}

	return NetworkFQDN{Label: label, Mcc: mcc, Mnc: mnc}, true
}

func (f NetworkFQDN) InPLMN(p PlmnID) bool {
	return p.Valid() && f.Mcc == p.Mcc && f.Mnc == p.mnc3()
}

func (f NetworkFQDN) String() string {
	return f.Label + ".5gc.mnc" + f.Mnc + ".mcc" + f.Mcc + ".3gppnetwork.org"
}

func InHomeNetworkDomain(host string, p PlmnID) bool {
	f, ok := ParseNetworkFQDN(host)

	return ok && f.InPLMN(p)
}
