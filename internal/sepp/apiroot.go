// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

var ErrTargetURI = errors.New("sepp: invalid target URI")

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
