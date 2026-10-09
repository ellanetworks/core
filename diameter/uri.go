// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

const DefaultSecurePort uint16 = 5658

var ErrInvalidURI = errors.New("diameter: invalid DiameterURI")

type URI struct {
	Host      string
	Port      uint16
	Transport Transport
	Secure    bool
}

func ParseURI(s string) (URI, error) {
	var u URI

	scheme, rest, ok := strings.Cut(s, "://")
	secure := strings.EqualFold(scheme, "aaas")

	if !ok || (!secure && !strings.EqualFold(scheme, "aaa")) {
		return URI{}, fmt.Errorf("%w: %q has no aaa:// or aaas:// scheme", ErrInvalidURI, s)
	}

	u.Secure = secure
	u.Port = DefaultPort
	u.Transport = TransportTCP

	if secure {
		u.Port = DefaultSecurePort
	}

	authority, params, hasParams := strings.Cut(rest, ";")

	host, port, hasPort := strings.Cut(authority, ":")
	if !isDiameterIdentity(host) {
		return URI{}, fmt.Errorf("%w: %q has no valid FQDN", ErrInvalidURI, s)
	}

	u.Host = host

	if hasPort {
		p, err := strconv.ParseUint(port, 10, 16)
		if err != nil || p == 0 {
			return URI{}, fmt.Errorf("%w: %q has an invalid port", ErrInvalidURI, s)
		}

		u.Port = uint16(p)
	}

	if !hasParams {
		return u, nil
	}

	seen := make(map[string]bool)

	for param := range strings.SplitSeq(params, ";") {
		key, value, ok := strings.Cut(param, "=")
		key, value = strings.ToLower(key), strings.ToLower(value)

		if !ok || seen[key] {
			return URI{}, fmt.Errorf("%w: %q has an invalid parameter %q", ErrInvalidURI, s, param)
		}

		seen[key] = true

		switch key {
		case "transport":
			switch value {
			case "tcp":
				u.Transport = TransportTCP
			case "sctp":
				u.Transport = TransportSCTP
			default:
				return URI{}, fmt.Errorf("%w: %q has a transport Diameter cannot use", ErrInvalidURI, s)
			}
		case "protocol":
			if value != "diameter" {
				return URI{}, fmt.Errorf("%w: %q is not a Diameter URI", ErrInvalidURI, s)
			}
		default:
			return URI{}, fmt.Errorf("%w: %q has an unknown parameter %q", ErrInvalidURI, s, key)
		}
	}

	return u, nil
}

func (u URI) String() string {
	scheme := "aaa://"
	if u.Secure {
		scheme = "aaas://"
	}

	return scheme + u.Host + ":" + strconv.FormatUint(uint64(u.Port), 10) + ";transport=" + u.Transport.String()
}

func isDiameterIdentity(s string) bool {
	if s == "" || len(s) > 253 {
		return false
	}

	for label := range strings.SplitSeq(s, ".") {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}

		for _, c := range label {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' {
				return false
			}
		}
	}

	return true
}
