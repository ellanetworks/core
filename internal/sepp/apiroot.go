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
)

var ErrTargetURI = errors.New("sepp: invalid target URI")

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
		return APIRoot{}, fmt.Errorf("%q has no scheme", v)
	}

	scheme = strings.ToLower(scheme)
	if scheme != "http" && scheme != "https" {
		return APIRoot{}, fmt.Errorf("scheme %q is not http or https", scheme)
	}

	authority, prefix := rest, ""
	if i := strings.IndexByte(rest, '/'); i >= 0 {
		authority, prefix = rest[:i], rest[i:]
	}

	authority, err := normalizeAuthority(authority)
	if err != nil {
		return APIRoot{}, err
	}

	if err := checkPrefix(prefix); err != nil {
		return APIRoot{}, err
	}

	return APIRoot{Scheme: scheme, Authority: authority, Prefix: prefix}, nil
}

var (
	hostnamePattern = regexp.MustCompile(`^[0-9A-Za-z]([-0-9A-Za-z]{0,61}[0-9A-Za-z])?(\.[0-9A-Za-z]([-0-9A-Za-z]{0,61}[0-9A-Za-z])?)*\.?$`)
	numericHost     = regexp.MustCompile(`^[0-9.]+$`)
)

func normalizeAuthority(a string) (string, error) {
	if a == "" {
		return "", errors.New("empty authority")
	}

	if strings.ContainsAny(a, "@?#") {
		return "", fmt.Errorf("authority %q has userinfo, query or fragment", a)
	}

	host, port, hasPort := a, "", false

	if strings.HasPrefix(a, "[") {
		end := strings.IndexByte(a, ']')
		if end < 0 {
			return "", fmt.Errorf("authority %q has an unterminated IPv6 literal", a)
		}

		host, port = a[1:end], a[end+1:]
		if ip := net.ParseIP(host); ip == nil || ip.To4() != nil || strings.Contains(host, "%") {
			return "", fmt.Errorf("authority %q has an invalid IPv6 literal", a)
		}

		if port != "" {
			p, ok := strings.CutPrefix(port, ":")
			if !ok {
				return "", fmt.Errorf("authority %q has garbage after the IPv6 literal", a)
			}

			port, hasPort = p, true
		}

		host = "[" + strings.ToLower(host) + "]"
	} else {
		if i := strings.LastIndexByte(a, ':'); i >= 0 {
			host, port, hasPort = a[:i], a[i+1:], true
		}

		if !hostnamePattern.MatchString(host) {
			return "", fmt.Errorf("host %q is not a host name or IPv4 address", host)
		}

		if numericHost.MatchString(strings.TrimSuffix(host, ".")) {
			if ip := net.ParseIP(host); ip == nil || ip.To4() == nil {
				return "", fmt.Errorf("host %q is not an IPv4 address", host)
			}
		}

		host = strings.ToLower(host)
	}

	if !hasPort || port == "" {
		return host, nil
	}

	if _, err := strconv.ParseUint(port, 10, 16); err != nil {
		return "", fmt.Errorf("port %q is not a TCP port", port)
	}

	return host + ":" + port, nil
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

			if b, _ := strconv.ParseUint(p[i+1:i+3], 16, 8); b < 0x20 || b == 0x7f {
				return fmt.Errorf("prefix %q encodes a control character", p)
			}

			i += 2
		case c == '/', isPathChar(c):
		default:
			return fmt.Errorf("prefix %q is not an absolute path", p)
		}
	}

	if hasDotSegment(p) {
		return fmt.Errorf("prefix %q has a dot segment", p)
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

func hasDotSegment(escapedPath string) bool {
	for seg := range strings.SplitSeq(escapedPath, "/") {
		if s, err := url.PathUnescape(seg); err == nil && (s == "." || s == "..") {
			return true
		}
	}

	return false
}

func TargetAPIRoot(target *url.URL) (APIRoot, error) {
	if target == nil || !target.IsAbs() || target.Host == "" {
		return APIRoot{}, fmt.Errorf("%w: %v is not absolute", ErrTargetURI, target)
	}

	if target.User != nil {
		return APIRoot{}, fmt.Errorf("%w: %s carries userinfo", ErrTargetURI, target.Redacted())
	}

	root, err := ParseAPIRoot(strings.ToLower(target.Scheme) + "://" + target.Host)
	if err != nil {
		return APIRoot{}, fmt.Errorf("%w: %w", ErrTargetURI, err)
	}

	return root, nil
}

func RewriteToN32f(target *url.URL, n32fBase *url.URL) (*url.URL, APIRoot, error) {
	root, err := TargetAPIRoot(target)
	if err != nil {
		return nil, APIRoot{}, err
	}

	if n32fBase == nil || !n32fBase.IsAbs() || n32fBase.Host == "" {
		return nil, APIRoot{}, fmt.Errorf("%w: N32-f base %v is not absolute", ErrTargetURI, n32fBase)
	}

	if hasDotSegment(target.EscapedPath()) {
		return nil, APIRoot{}, fmt.Errorf("%w: %s has a dot segment", ErrTargetURI, target.Redacted())
	}

	out := &url.URL{
		Scheme:   n32fBase.Scheme,
		Host:     n32fBase.Host,
		Path:     joinPath(n32fBase.Path, target.Path),
		RawQuery: target.RawQuery,
	}

	if target.RawPath != "" || n32fBase.RawPath != "" {
		out.RawPath = joinPath(n32fBase.EscapedPath(), target.EscapedPath())
	}

	return out, root, nil
}

func TargetURL(root APIRoot, received *url.URL, localPrefix string) (*url.URL, error) {
	if received == nil {
		return nil, fmt.Errorf("%w: no request URI", ErrTargetURI)
	}

	escaped := received.EscapedPath()

	if localPrefix != "" {
		lp := strings.TrimSuffix(localPrefix, "/")

		rest, ok := strings.CutPrefix(escaped, lp)
		if !ok || rest != "" && rest[0] != '/' {
			return nil, fmt.Errorf("%w: path %q is outside %q", ErrTargetURI, escaped, localPrefix)
		}

		escaped = rest
	}

	escaped = joinPath(root.Prefix, escaped)

	if hasDotSegment(escaped) {
		return nil, fmt.Errorf("%w: path %q has a dot segment", ErrTargetURI, escaped)
	}

	path, err := url.PathUnescape(escaped)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrTargetURI, err)
	}

	out := &url.URL{
		Scheme:   root.Scheme,
		Host:     root.Authority,
		Path:     path,
		RawQuery: received.RawQuery,
	}
	out.RawPath = escaped

	if out.EscapedPath() != escaped {
		out.RawPath = ""
	}

	return out, nil
}

func joinPath(base, p string) string {
	base = strings.TrimSuffix(base, "/")
	if p == "" {
		return base
	}

	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	return base + p
}
