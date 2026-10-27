// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"net/http"
	"regexp"
	"slices"
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

func headerError(name, format string, args ...any) error {
	return wire.Incorrect("header: "+name, false, format, args...)
}

func singleHeader(name string, values []string) (string, error) {
	switch len(values) {
	case 0:
		return "", wire.Missing("header: " + name)
	case 1:
		return values[0], nil
	default:
		return "", headerError(name, "repeated %d times", len(values))
	}
}

func trimOWS(s string) string {
	return strings.Trim(s, " \t")
}

func ParseTargetAPIRoot(values []string) (APIRoot, error) {
	v, err := singleHeader(HeaderTargetAPIRoot, values)
	if err != nil {
		return APIRoot{}, err
	}

	root, err := ParseAPIRoot(v)
	if err != nil {
		return APIRoot{}, headerError(HeaderTargetAPIRoot, "%v", err)
	}

	return root, nil
}

type OriginatingNetworkID struct {
	PlmnID  wire.PlmnID
	Nid     string
	SrcType string
	SrcFQDN string
}

var srcFQDNPattern = regexp.MustCompile(`^[A-Za-z0-9.-]{4,}$`)

func ParseOriginatingNetworkID(values []string) (OriginatingNetworkID, error) {
	v, err := singleHeader(HeaderOriginatingNetworkID, values)
	if err != nil {
		return OriginatingNetworkID{}, err
	}

	return parseOriginatingNetworkID(v)
}

func parseOriginatingNetworkID(v string) (OriginatingNetworkID, error) {
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

	if len(src) < 5 || !strings.EqualFold(src[:4], "src:") || (src[4] != ' ' && src[4] != '\t') {
		return OriginatingNetworkID{}, headerError(HeaderOriginatingNetworkID, "%q has a malformed srcinfo", v)
	}

	rest := strings.TrimLeft(src[4:], " \t")

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

func (o OriginatingNetworkID) Matches(p wire.PlmnID) bool {
	return o.Nid == "" && o.PlmnID == p
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

func ParseN32HandshakeID(values []string) (string, error) {
	v, err := singleHeader(HeaderN32HandshakeID, values)
	if err != nil {
		return "", err
	}

	s := trimOWS(v)
	if len(s) != 16 || !isHex(s) {
		return "", headerError(HeaderN32HandshakeID, "%q is not 16 hexadecimal digits", v)
	}

	return s, nil
}

type InterPLMNPurpose struct {
	Purpose        N32Purpose
	AdditionalInfo string
}

func ParseInterPLMNPurpose(values []string) (InterPLMNPurpose, error) {
	v, err := singleHeader(HeaderInterPLMNPurpose, values)
	if err != nil {
		return InterPLMNPurpose{}, err
	}

	purpose, info, hasInfo := strings.Cut(trimOWS(v), ":")
	info = trimOWS(info)

	if !isToken(purpose) || hasInfo && !isToken(info) {
		return InterPLMNPurpose{}, headerError(HeaderInterPLMNPurpose, "%q is not <purpose>[: <additional-info>]", v)
	}

	return InterPLMNPurpose{Purpose: N32Purpose(purpose), AdditionalInfo: info}, nil
}

func CheckInterPLMNPurpose(values []string, allowed []N32Purpose) error {
	if len(values) == 0 {
		if slices.Contains(allowed, PurposeRoaming) || slices.Contains(allowed, PurposeInterPLMNMobility) {
			return nil
		}

		return forbidden(wire.CauseRequestedPurposeNotAllowed, "no purpose given, roaming is not allowed")
	}

	p, err := ParseInterPLMNPurpose(values)
	if err != nil {
		return err
	}

	if !slices.Contains(allowed, p.Purpose) {
		return forbidden(wire.CauseRequestedPurposeNotAllowed, "purpose "+string(p.Purpose)+" is not allowed")
	}

	return nil
}

func forbidden(cause wire.Cause, detail string) error {
	return wire.NewProblem(http.StatusForbidden, cause, detail)
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

type NetworkEntity struct {
	Type wire.NFType
	ID   string
}

func (e ViaEntry) NetworkEntity() (NetworkEntity, bool) {
	return parseEntity(e.ReceivedBy)
}

func parseEntity(s string) (NetworkEntity, bool) {
	typ, id, ok := strings.Cut(s, "-")
	id, _, _ = strings.Cut(id, "/")

	if !ok || !isToken(typ) || id == "" {
		return NetworkEntity{}, false
	}

	return NetworkEntity{Type: wire.NFType(strings.ToUpper(typ)), ID: id}, true
}

func IsSEPP(value, fqdn string) bool {
	e, ok := parseEntity(value)

	return ok && e.Type == wire.NFTypeSEPP && strings.EqualFold(strings.TrimSuffix(e.ID, "."), strings.TrimSuffix(fqdn, "."))
}

func ParseVia(values []string) ([]ViaEntry, error) {
	var out []ViaEntry

	for _, v := range values {
		entries, err := parseViaLine(v)
		if err != nil {
			return nil, err
		}

		out = append(out, entries...)
	}

	if len(out) == 0 {
		return nil, headerError(HeaderVia, "empty")
	}

	return out, nil
}

func parseViaLine(v string) ([]ViaEntry, error) {
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

		elem := trimOWS(v[start:min(i, len(v))])
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

	return out, nil
}

func parseViaElement(elem string) (ViaEntry, error) {
	proto, rest, ok := cutRWS(elem)
	if !ok {
		return ViaEntry{}, headerError(HeaderVia, "%q has no received-by", elem)
	}

	by, comment, _ := cutRWS(rest)

	if !isReceivedProtocol(proto) || !isReceivedBy(by) {
		return ViaEntry{}, headerError(HeaderVia, "%q is malformed", elem)
	}

	if comment != "" && !isComment(comment) {
		return ViaEntry{}, headerError(HeaderVia, "%q has a malformed comment", elem)
	}

	return ViaEntry{Protocol: proto, ReceivedBy: by, Comment: comment}, nil
}

func isReceivedProtocol(s string) bool {
	name, version, hasName := strings.Cut(s, "/")
	if !hasName {
		return isToken(s)
	}

	return isToken(name) && isToken(version)
}

func isReceivedBy(s string) bool {
	host, port := s, ""

	if strings.HasPrefix(s, "[") {
		end := strings.IndexByte(s, ']')
		if end < 0 {
			return false
		}

		host, port = s[1:end], s[end+1:]
		if !isIPv6Literal(host) {
			return false
		}

		if port == "" {
			return true
		}

		p, ok := strings.CutPrefix(port, ":")

		return ok && isDigits(p)
	}

	if i := strings.IndexByte(s, ':'); i >= 0 {
		host, port = s[:i], s[i+1:]
		if !isDigits(port) {
			return false
		}
	}

	return isToken(host)
}

func isIPv6Literal(s string) bool {
	if s == "" {
		return false
	}

	for i := range len(s) {
		c := s[i]
		if !isHex(s[i:i+1]) && c != ':' && c != '.' {
			return false
		}
	}

	return true
}

func isDigits(s string) bool {
	if s == "" || len(s) > 5 {
		return false
	}

	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
}

func isComment(s string) bool {
	if len(s) < 2 || s[0] != '(' {
		return false
	}

	depth := 0

	for i := 0; i < len(s); i++ {
		c := s[i]

		switch {
		case c == '\\':
			if i+1 >= len(s) || !isQuotedPairChar(s[i+1]) {
				return false
			}

			i++
		case c == '(':
			depth++
		case c == ')':
			depth--
			if depth == 0 && i != len(s)-1 {
				return false
			}
		case c != '\t' && (c < 0x20 || c == 0x7f):
			return false
		}
	}

	return depth == 0
}

func isQuotedPairChar(c byte) bool {
	return c == '\t' || c >= 0x20 && c != 0x7f
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

func ParseServer(values []string) (NetworkEntity, bool) {
	if len(values) != 1 {
		return NetworkEntity{}, false
	}

	product, _, _ := cutRWS(trimOWS(values[0]))
	if !isReceivedProtocol(product) {
		return NetworkEntity{}, false
	}

	return parseEntity(product)
}
