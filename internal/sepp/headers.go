// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const (
	HeaderTargetAPIRoot        = "3gpp-Sbi-Target-apiRoot"
	HeaderOriginatingNetworkID = "3gpp-Sbi-Originating-Network-Id"
	HeaderN32HandshakeID       = "3gpp-Sbi-N32-Handshake-Id"
	HeaderInterPLMNPurpose     = "3gpp-Sbi-Interplmn-Purpose"
	HeaderVia                  = "Via"
	HeaderServer               = "Server"
)

var ErrHeader = errors.New("sepp: malformed header")

func headerError(name, format string, args ...any) error {
	return fmt.Errorf("%w %s: %s", ErrHeader, name, fmt.Sprintf(format, args...))
}

func trimOWS(s string) string {
	return strings.Trim(s, " \t")
}

type APIRoot struct {
	Scheme    string
	Authority string
	Prefix    string
}

func (r APIRoot) String() string {
	return r.Scheme + "://" + r.Authority + r.Prefix
}

func (r APIRoot) Hostname() string {
	return (&url.URL{Host: r.Authority}).Hostname()
}

func (r APIRoot) Port() string {
	return (&url.URL{Host: r.Authority}).Port()
}

func ParseAPIRoot(v string) (APIRoot, error) {
	s := trimOWS(v)

	scheme, rest, ok := strings.Cut(s, "://")
	if !ok {
		return APIRoot{}, headerError(HeaderTargetAPIRoot, "%q has no scheme", v)
	}

	scheme = strings.ToLower(scheme)
	if scheme != "http" && scheme != "https" {
		return APIRoot{}, headerError(HeaderTargetAPIRoot, "scheme %q is not http or https", scheme)
	}

	authority, prefix := rest, ""
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		authority, prefix = rest[:i], rest[i:]
	}

	if err := checkAuthority(authority); err != nil {
		return APIRoot{}, headerError(HeaderTargetAPIRoot, "%v", err)
	}

	if err := checkPrefix(prefix); err != nil {
		return APIRoot{}, headerError(HeaderTargetAPIRoot, "%v", err)
	}

	return APIRoot{Scheme: scheme, Authority: authority, Prefix: prefix}, nil
}

var hostnamePattern = regexp.MustCompile(`^[0-9A-Za-z]([-0-9A-Za-z]{0,61}[0-9A-Za-z])?(\.[0-9A-Za-z]([-0-9A-Za-z]{0,61}[0-9A-Za-z])?)*\.?$`)

func checkAuthority(a string) error {
	if a == "" {
		return errors.New("empty authority")
	}

	if strings.ContainsAny(a, "@?#") {
		return fmt.Errorf("authority %q has userinfo, query or fragment", a)
	}

	host, port := a, ""

	if strings.HasPrefix(a, "[") {
		end := strings.IndexByte(a, ']')
		if end < 0 {
			return fmt.Errorf("authority %q has an unterminated IPv6 literal", a)
		}

		host, port = a[1:end], a[end+1:]
		if ip := net.ParseIP(host); ip == nil || ip.To4() != nil {
			return fmt.Errorf("authority %q has an invalid IPv6 literal", a)
		}

		if port != "" {
			p, ok := strings.CutPrefix(port, ":")
			if !ok {
				return fmt.Errorf("authority %q has garbage after the IPv6 literal", a)
			}

			port = p
		}
	} else if i := strings.LastIndexByte(a, ':'); i >= 0 {
		host, port = a[:i], a[i+1:]
		if port == "" {
			port = "0"
		}
	}

	if !strings.HasPrefix(a, "[") && !hostnamePattern.MatchString(host) {
		return fmt.Errorf("host %q is not a host name or IPv4 address", host)
	}

	if port != "" {
		if _, err := strconv.ParseUint(port, 10, 16); err != nil {
			return fmt.Errorf("port %q is not a TCP port", port)
		}
	}

	return nil
}

func checkPrefix(p string) error {
	if p == "" {
		return nil
	}

	if strings.HasPrefix(p, "//") {
		return fmt.Errorf("prefix %q starts with an empty segment", p)
	}

	for i := 0; i < len(p); i++ {
		c := p[i]

		switch {
		case c == '%':
			if i+2 >= len(p) || !isHex(p[i+1:i+3]) {
				return fmt.Errorf("prefix %q has a malformed percent-encoding", p)
			}

			i += 2
		case c == '/', isPathChar(c):
		default:
			return fmt.Errorf("prefix %q is not an absolute path", p)
		}
	}

	return nil
}

func isPathChar(c byte) bool {
	switch {
	case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	default:
		return strings.IndexByte("-._~!$&'()*+,;=:@", c) >= 0
	}
}

type OriginatingNetworkID struct {
	PlmnID  wire.PlmnID
	Nid     string
	SrcType string
	SrcFQDN string
}

var srcFQDNPattern = regexp.MustCompile(`^[A-Za-z0-9.-]{4,}$`)

func ParseOriginatingNetworkID(v string) (OriginatingNetworkID, error) {
	s := trimOWS(v)

	id, src, hasSrc := strings.Cut(s, ";")

	parts := strings.Split(id, "-")
	if len(parts) < 2 || len(parts) > 3 {
		return OriginatingNetworkID{}, headerError(HeaderOriginatingNetworkID, "%q is not <mcc>-<mnc>[-<nid>]", v)
	}

	out := OriginatingNetworkID{PlmnID: wire.PlmnID{Mcc: parts[0], Mnc: parts[1]}}
	if !out.PlmnID.Valid() {
		return OriginatingNetworkID{}, headerError(HeaderOriginatingNetworkID, "%q has an invalid PLMN", v)
	}

	if len(parts) == 3 {
		if len(parts[2]) != 11 || !isHex(parts[2]) {
			return OriginatingNetworkID{}, headerError(HeaderOriginatingNetworkID, "%q has an invalid NID", v)
		}

		out.Nid = parts[2]
	}

	if !hasSrc {
		return out, nil
	}

	src = strings.TrimLeft(src, " \t")

	rest, ok := strings.CutPrefix(src, "src:")
	if !ok || rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return OriginatingNetworkID{}, headerError(HeaderOriginatingNetworkID, "%q has a malformed srcinfo", v)
	}

	rest = strings.TrimLeft(rest, " \t")

	typ, fqdn, ok := strings.Cut(rest, "-")
	typ = strings.ToUpper(typ)

	if !ok || (typ != "SCP" && typ != "SEPP") || !srcFQDNPattern.MatchString(fqdn) {
		return OriginatingNetworkID{}, headerError(HeaderOriginatingNetworkID, "%q has a malformed srcinfo", v)
	}

	out.SrcType, out.SrcFQDN = typ, fqdn

	return out, nil
}

func (o OriginatingNetworkID) String() string {
	s := o.PlmnID.String()
	if o.Nid != "" {
		s += "-" + o.Nid
	}

	if o.SrcType != "" {
		s += "; src: " + o.SrcType + "-" + o.SrcFQDN
	}

	return s
}

func isHex(s string) bool {
	for i := range len(s) {
		c := s[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}

	return true
}

func ParseN32HandshakeID(v string) (string, error) {
	s := trimOWS(v)
	if len(s) != 16 || !isHex(s) {
		return "", headerError(HeaderN32HandshakeID, "%q is not 16 hexadecimal digits", v)
	}

	return s, nil
}

type ViaEntry struct {
	Protocol   string
	ReceivedBy string
	Comment    string
}

func SEPPVia(fqdn string) string {
	return "2.0 SEPP-" + fqdn
}

func SEPPServer(fqdn string) string {
	return "SEPP-" + fqdn
}

func (e ViaEntry) String() string {
	s := e.Protocol + " " + e.ReceivedBy
	if e.Comment != "" {
		s += " " + e.Comment
	}

	return s
}

func (e ViaEntry) NetworkEntity() (string, string, bool) {
	return splitEntity(e.ReceivedBy)
}

func splitEntity(s string) (string, string, bool) {
	typ, id, ok := strings.Cut(s, "-")
	if !ok || typ == "" || id == "" {
		return "", "", false
	}

	return typ, id, true
}

func ParseVia(v string) ([]ViaEntry, error) {
	var (
		out   []ViaEntry
		depth int
		start int
	)

	for i := 0; i <= len(v); i++ {
		if i < len(v) {
			c := v[i]

			switch {
			case c == '\\' && depth > 0:
				i++

				continue
			case c == '(':
				depth++

				continue
			case c == ')' && depth > 0:
				depth--

				continue
			case c != ',' || depth > 0:
				continue
			}
		}

		elem := trimOWS(v[start:i])
		start = i + 1

		if elem == "" {
			continue
		}

		e, err := parseViaElement(elem)
		if err != nil {
			return nil, err
		}

		out = append(out, e)
	}

	if depth != 0 {
		return nil, headerError(HeaderVia, "%q has an unterminated comment", v)
	}

	if len(out) == 0 {
		return nil, headerError(HeaderVia, "empty")
	}

	return out, nil
}

func parseViaElement(elem string) (ViaEntry, error) {
	proto, rest, ok := cutRWS(elem)
	if !ok {
		return ViaEntry{}, headerError(HeaderVia, "%q has no received-by", elem)
	}

	by, comment, _ := cutRWS(rest)

	if !isToken(strings.Replace(proto, "/", "", 1)) || !isViaReceivedBy(by) {
		return ViaEntry{}, headerError(HeaderVia, "%q is malformed", elem)
	}

	if comment != "" && (!strings.HasPrefix(comment, "(") || !strings.HasSuffix(comment, ")")) {
		return ViaEntry{}, headerError(HeaderVia, "%q has trailing garbage", elem)
	}

	return ViaEntry{Protocol: proto, ReceivedBy: by, Comment: comment}, nil
}

func cutRWS(s string) (string, string, bool) {
	i := strings.IndexAny(s, " \t")
	if i < 0 {
		return s, "", false
	}

	return s[:i], strings.TrimLeft(s[i:], " \t"), true
}

func isTokenChar(c byte) bool {
	switch {
	case c >= '0' && c <= '9', c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z':
		return true
	default:
		return strings.IndexByte("!#$%&'*+-.^_`|~", c) >= 0
	}
}

func isToken(s string) bool {
	if s == "" {
		return false
	}

	for i := range len(s) {
		if !isTokenChar(s[i]) {
			return false
		}
	}

	return true
}

func isViaReceivedBy(s string) bool {
	if s == "" {
		return false
	}

	for i := range len(s) {
		if !isTokenChar(s[i]) && s[i] != ':' && s[i] != '[' && s[i] != ']' {
			return false
		}
	}

	return true
}

func ParseServer(v string) (string, string, bool) {
	product, _, _ := cutRWS(trimOWS(v))
	if !isToken(strings.Replace(product, "/", "", 1)) {
		return "", "", false
	}

	return splitEntity(product)
}
