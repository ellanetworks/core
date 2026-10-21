// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package psi

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

const wildcardDelimiter = "!"

var (
	ErrInvalidURI        = errors.New("invalid URI")
	ErrInvalidWildcard   = errors.New("invalid wildcarded public service identity")
	errUnsupportedScheme = errors.New("scheme must be sip, sips or tel")
)

type parsed struct {
	scheme string
	user   string
	host   string
	rest   string
}

func Canonical(uri string) (string, error) {
	p, err := parse(uri, true)
	if err != nil {
		return "", err
	}

	return p.String(), nil
}

func CanonicalServerName(uri string) (string, error) {
	p, err := parse(uri, false)
	if err != nil {
		return "", err
	}

	if p.scheme == "tel" {
		return "", fmt.Errorf("%w: %q: an application server name must be a SIP URI", ErrInvalidURI, uri)
	}

	return p.String(), nil
}

func Wildcarded(uri string) bool {
	return strings.Count(uri, wildcardDelimiter) >= 2
}

func Overlap(a, b string) bool {
	if !Wildcarded(a) || !Wildcarded(b) {
		return false
	}

	pa, errA := parse(a, true)
	pb, errB := parse(b, true)

	if errA != nil || errB != nil {
		return false
	}

	if pa.scheme == "tel" || pb.scheme == "tel" {
		return pa.scheme == pb.scheme
	}

	return pa.host == pb.host
}

func parse(uri string, allowWildcard bool) (parsed, error) {
	colon := strings.IndexByte(uri, ':')
	if colon <= 0 {
		return parsed{}, fmt.Errorf("%w: %q has no scheme", ErrInvalidURI, uri)
	}

	p := parsed{scheme: strings.ToLower(uri[:colon])}
	body := uri[colon+1:]

	if strings.TrimSpace(uri) != uri || body == "" {
		return parsed{}, fmt.Errorf("%w: %q", ErrInvalidURI, uri)
	}

	wildcarded := strings.Contains(body, wildcardDelimiter)
	if wildcarded && !allowWildcard {
		return parsed{}, fmt.Errorf("%w: %q must not contain %q", ErrInvalidURI, uri, wildcardDelimiter)
	}

	switch p.scheme {
	case "sip", "sips":
		return p.sip(uri, body, wildcarded)
	case "tel":
		return p.tel(uri, body, wildcarded)
	default:
		return parsed{}, fmt.Errorf("%w: %q: %w", ErrInvalidURI, uri, errUnsupportedScheme)
	}
}

func (p parsed) sip(uri, body string, wildcarded bool) (parsed, error) {
	searchFrom := 0

	if wildcarded {
		end, err := wildcardEnd(uri, body)
		if err != nil {
			return parsed{}, err
		}

		searchFrom = end
	}

	at := strings.IndexByte(body[searchFrom:], '@')

	switch {
	case at >= 0:
		at += searchFrom
		p.user = body[:at]
		body = body[at+1:]
	case wildcarded:
		return parsed{}, fmt.Errorf("%w: %q: the wildcard must be in the user part", ErrInvalidWildcard, uri)
	}

	hostEnd := strings.IndexAny(body, ";?")
	if hostEnd < 0 {
		hostEnd = len(body)
	}

	p.host, p.rest = strings.ToLower(body[:hostEnd]), body[hostEnd:]

	if p.host == "" || (p.user == "" && strings.Contains(body, "@")) {
		return parsed{}, fmt.Errorf("%w: %q has no host", ErrInvalidURI, uri)
	}

	if strings.Contains(p.host, wildcardDelimiter) {
		return parsed{}, fmt.Errorf("%w: %q: the wildcard must be in the user part", ErrInvalidWildcard, uri)
	}

	return p, nil
}

func (p parsed) tel(uri, body string, wildcarded bool) (parsed, error) {
	paramStart := strings.IndexByte(body, ';')

	if wildcarded {
		end, err := wildcardEnd(uri, body)
		if err != nil {
			return parsed{}, err
		}

		if next := strings.IndexByte(body[end:], ';'); next >= 0 {
			paramStart = end + next
		} else {
			paramStart = -1
		}
	}

	if paramStart < 0 {
		paramStart = len(body)
	}

	subscriber, rest := body[:paramStart], body[paramStart:]

	start, end := strings.Index(subscriber, wildcardDelimiter), strings.LastIndex(subscriber, wildcardDelimiter)
	if wildcarded {
		if strings.Trim(stripVisualSeparators(subscriber[:start]+subscriber[end+1:]), "+") == "" {
			return parsed{}, fmt.Errorf("%w: %q: a tel wildcard needs a fixed part", ErrInvalidWildcard, uri)
		}

		subscriber = stripVisualSeparators(subscriber[:start]) + subscriber[start:end+1] + stripVisualSeparators(subscriber[end+1:])
	} else {
		subscriber = stripVisualSeparators(subscriber)
	}

	if subscriber == "" || subscriber == "+" {
		return parsed{}, fmt.Errorf("%w: %q has no number", ErrInvalidURI, uri)
	}

	p.user, p.rest = subscriber, rest

	return p, nil
}

func wildcardEnd(uri, body string) (int, error) {
	start, end := strings.Index(body, wildcardDelimiter), strings.LastIndex(body, wildcardDelimiter)
	if start == end {
		return 0, fmt.Errorf("%w: %q: the wildcard needs an opening and a closing %q", ErrInvalidWildcard, uri, wildcardDelimiter)
	}

	if _, err := regexp.CompilePOSIX(body[start+1 : end]); err != nil {
		return 0, fmt.Errorf("%w: %q: %w", ErrInvalidWildcard, uri, err)
	}

	return end + 1, nil
}

func stripVisualSeparators(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '-', '.', '(', ')':
			return -1
		}

		return r
	}, s)
}

func (p parsed) String() string {
	switch {
	case p.scheme == "tel":
		return p.scheme + ":" + p.user + p.rest
	case p.user == "":
		return p.scheme + ":" + p.host + p.rest
	default:
		return p.scheme + ":" + p.user + "@" + p.host + p.rest
	}
}
