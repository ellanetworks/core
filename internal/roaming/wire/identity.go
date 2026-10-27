// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	imsiSupiPattern  = regexp.MustCompile(`^imsi-([0-9]{5,15})$`)
	supiPattern      = regexp.MustCompile(`^(imsi-[0-9]{5,15}|nai-.+|gci-.+|gli-.+)$`)
	suciPattern      = regexp.MustCompile(`^suci-(0-[0-9]{3}-[0-9]{2,3}|[1-7]-.+)-[0-9]{1,4}-(0-0-.*|[a-fA-F1-9]-([1-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])-[a-fA-F0-9]+)$`)
	gpsiPattern      = regexp.MustCompile(`^(msisdn-[0-9]{5,15}|extid-[^@]+@[^@]+)$`)
	peiPattern       = regexp.MustCompile(`^(imei-[0-9]{15}|imeisv-[0-9]{16}|mac((-[0-9a-fA-F]{2}){6})(-untrusted)?|eui((-[0-9a-fA-F]{2}){8}))$`)
	uuidPattern      = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	fqdnPattern      = regexp.MustCompile(`^([0-9A-Za-z]([-0-9A-Za-z]{0,61}[0-9A-Za-z])?\.)+[A-Za-z]{2,63}\.?$`)
	routingIndicator = regexp.MustCompile(`^[0-9]{1,4}$`)
	dnnPattern       = regexp.MustCompile(`^[0-9A-Za-z]([-0-9A-Za-z]{0,61}[0-9A-Za-z])?(\.[0-9A-Za-z]([-0-9A-Za-z]{0,61}[0-9A-Za-z])?)*$`)
)

func ValidSupi(s string) bool {
	return supiPattern.MatchString(s)
}

func ValidSuci(s string) bool {
	return suciPattern.MatchString(s)
}

func ValidSupiOrSuci(s string) bool {
	return ValidSupi(s) || ValidSuci(s)
}

func ValidGpsi(s string) bool {
	return gpsiPattern.MatchString(s)
}

func ValidPei(s string) bool {
	return peiPattern.MatchString(s)
}

func ValidNfInstanceID(s string) bool {
	return uuidPattern.MatchString(s)
}

func ValidFqdn(s string) bool {
	return len(s) >= 4 && len(s) <= 253 && fqdnPattern.MatchString(s)
}

func ValidRoutingIndicator(s string) bool {
	return routingIndicator.MatchString(s)
}

func ValidDnn(s string) bool {
	return len(s) <= 100 && dnnPattern.MatchString(s)
}

func IMSIFromSupi(supi string) (string, bool) {
	m := imsiSupiPattern.FindStringSubmatch(supi)
	if m == nil {
		return "", false
	}

	return m[1], true
}

func SupiInPLMN(supi string, p PlmnID) bool {
	imsi, ok := IMSIFromSupi(supi)

	return ok && p.Valid() && len(imsi) > len(p.Mcc)+len(p.Mnc) && strings.HasPrefix(imsi, p.Mcc+p.Mnc)
}

type Suci struct {
	Mcc              string
	Mnc              string
	RoutingIndicator string
	ProtectionScheme int
	HNPublicKeyID    int
	SchemeOutput     string
}

func ParseSuci(s string) (Suci, error) {
	if !ValidSuci(s) {
		return Suci{}, fmt.Errorf("wire: %q is not a SUCI", s)
	}

	parts := strings.SplitN(s, "-", 8)
	if len(parts) != 8 || parts[1] != "0" {
		return Suci{}, fmt.Errorf("wire: %q is not an IMSI-based SUCI", s)
	}

	scheme, err := strconv.ParseUint(parts[5], 16, 8)
	if err != nil {
		return Suci{}, fmt.Errorf("wire: SUCI %q protection scheme: %w", s, err)
	}

	keyID, err := strconv.Atoi(parts[6])
	if err != nil {
		return Suci{}, fmt.Errorf("wire: SUCI %q key identifier: %w", s, err)
	}

	return Suci{
		Mcc:              parts[2],
		Mnc:              parts[3],
		RoutingIndicator: parts[4],
		ProtectionScheme: int(scheme),
		HNPublicKeyID:    keyID,
		SchemeOutput:     parts[7],
	}, nil
}

func (s Suci) PlmnID() PlmnID {
	return PlmnID{Mcc: s.Mcc, Mnc: s.Mnc}
}

type DateTime string

func DateTimeOf(t time.Time) DateTime {
	return DateTime(t.Format(time.RFC3339Nano))
}

func (d DateTime) Time() (time.Time, error) {
	return time.Parse(time.RFC3339Nano, string(d))
}

func checkDateTime(param string, d DateTime, optional bool) error {
	if d == "" {
		if optional {
			return nil
		}

		return missing(param)
	}

	if _, err := d.Time(); err != nil {
		return incorrect(param, optional, "%q is not an RFC 3339 date-time", string(d))
	}

	return nil
}

func checkAbsoluteURI(param string, s string, optional bool) error {
	if s == "" {
		if optional {
			return nil
		}

		return missing(param)
	}

	u, err := url.Parse(s)
	if err != nil || !u.IsAbs() || u.Host == "" {
		return incorrect(param, optional, "%q is not an absolute URI", s)
	}

	return nil
}

func checkString(param string, s string, valid func(string) bool, optional bool) error {
	if s == "" {
		if optional {
			return nil
		}

		return missing(param)
	}

	if !valid(s) {
		return incorrect(param, optional, "invalid value %q", s)
	}

	return nil
}

func equalDnn(a, b string) bool {
	return strings.EqualFold(a, b)
}
