// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package psi

import (
	"errors"
	"fmt"
	"maps"
	"regexp"
	"regexp/syntax"
	"slices"
	"strings"
	"unicode"
)

const wildcardDelimiter = "!"

var (
	ErrInvalidURI        = errors.New("invalid URI")
	ErrInvalidWildcard   = errors.New("invalid wildcarded public service identity")
	errUnsupportedScheme = errors.New("scheme must be sip, sips or tel")
)

type mode int

const (
	modeStored mode = iota
	modeLiteral
	modeServerName
)

type parsed struct {
	scheme string
	user   string
	host   string
	rest   string
}

func Canonical(uri string) (string, error) {
	p, err := parse(uri, modeStored)
	if err != nil {
		return "", err
	}

	return p.String(), nil
}

func Literal(uri string) (string, error) {
	p, err := parse(uri, modeLiteral)
	if err != nil {
		return "", err
	}

	return p.String(), nil
}

func CanonicalServerName(uri string) (string, error) {
	p, err := parse(uri, modeServerName)
	if err != nil {
		return "", err
	}

	if p.scheme == "tel" {
		return "", fmt.Errorf("%w: %q: an application server name must be a SIP URI", ErrInvalidURI, uri)
	}

	return p.String(), nil
}

func Wildcarded(stored string) bool {
	return strings.Count(stored, wildcardDelimiter) >= 2
}

func Host(stored string) string {
	p, err := parse(stored, modeStored)
	if err != nil {
		return ""
	}

	return p.host
}

func Rehome(uri, from, to string) (string, bool) {
	m := modeServerName
	if strings.Contains(uri, wildcardDelimiter) {
		m = modeStored
	}

	p, err := parse(uri, m)
	if err != nil || p.scheme == "tel" || strings.HasPrefix(p.host, "[") {
		return uri, false
	}

	name, port := p.host, ""
	if i := strings.LastIndexByte(p.host, ':'); i >= 0 {
		name, port = p.host[:i], p.host[i:]
	}

	switch {
	case name == from:
		name = to
	case strings.HasSuffix(name, "."+from):
		name = strings.TrimSuffix(name, from) + to
	default:
		return uri, false
	}

	p.host = name + port

	return p.String(), true
}

func User(stored string) string {
	p, err := parse(stored, modeStored)
	if err != nil {
		return ""
	}

	return p.user
}

func Overlap(a, b string) bool {
	if !Wildcarded(a) || !Wildcarded(b) {
		return false
	}

	pa, errA := wildcardProgram(a)
	pb, errB := wildcardProgram(b)

	if errA != nil || errB != nil {
		return false
	}

	return intersect(pa, pb)
}

func Covers(stored, pattern string) bool {
	if !Wildcarded(stored) {
		return false
	}

	ps, err := wildcardProgram(stored)
	if err != nil {
		return false
	}

	re, err := syntax.Parse(pattern, syntax.POSIX)
	if err != nil {
		return false
	}

	pp, err := syntax.Compile(re.Simplify())
	if err != nil {
		return false
	}

	return intersect(ps, pp)
}

type Match struct {
	Owner    string
	Wildcard string
}

type Matcher struct {
	distinct  map[string]string
	wildcards []wildcard
}

type wildcard struct {
	stored string
	owner  string
	re     *regexp.Regexp
}

func NewMatcher(identities map[string]string) *Matcher {
	m := &Matcher{distinct: make(map[string]string)}

	for _, stored := range slices.Sorted(maps.Keys(identities)) {
		owner := identities[stored]

		if !Wildcarded(stored) {
			if key, err := Canonical(stored); err == nil {
				m.distinct[key] = owner
			}

			continue
		}

		re, err := compileWildcard(stored)
		if err != nil {
			continue
		}

		m.wildcards = append(m.wildcards, wildcard{stored: stored, owner: owner, re: re})
	}

	return m
}

func (m *Matcher) Match(identity string) (Match, bool) {
	key, err := Literal(identity)
	if err != nil {
		return Match{}, false
	}

	if owner, ok := m.distinct[key]; ok {
		return Match{Owner: owner}, true
	}

	for _, w := range m.wildcards {
		if w.re.MatchString(key) {
			return Match{Owner: w.owner, Wildcard: w.stored}, true
		}
	}

	return Match{}, false
}

func compileWildcard(stored string) (*regexp.Regexp, error) {
	pattern, err := wildcardPattern(stored)
	if err != nil {
		return nil, err
	}

	re, err := regexp.CompilePOSIX(pattern)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInvalidWildcard, stored, err)
	}

	return re, nil
}

func wildcardPattern(stored string) (string, error) {
	canonical, err := Canonical(stored)
	if err != nil {
		return "", err
	}

	start, end := strings.Index(canonical, wildcardDelimiter), strings.LastIndex(canonical, wildcardDelimiter)
	if start == end {
		return "", fmt.Errorf("%w: %q has no wildcard", ErrInvalidWildcard, stored)
	}

	return "^" + regexp.QuoteMeta(canonical[:start]) + "(" + canonical[start+1:end] + ")" + regexp.QuoteMeta(canonical[end+1:]) + "$", nil
}

func wildcardProgram(stored string) (*syntax.Prog, error) {
	pattern, err := wildcardPattern(stored)
	if err != nil {
		return nil, err
	}

	re, err := syntax.Parse(pattern, syntax.POSIX)
	if err != nil {
		return nil, fmt.Errorf("%w: %q: %w", ErrInvalidWildcard, stored, err)
	}

	return syntax.Compile(re.Simplify())
}

func intersect(a, b *syntax.Prog) bool {
	type state struct{ a, b uint32 }

	start := state{uint32(a.Start), uint32(b.Start)}
	seen := map[state]bool{start: true}
	queue := []state{start}

	for len(queue) > 0 {
		s := queue[0]
		queue = queue[1:]

		ca, cb := closure(a, s.a), closure(b, s.b)

		for _, pa := range ca {
			ia := &a.Inst[pa]

			for _, pb := range cb {
				ib := &b.Inst[pb]

				if ia.Op == syntax.InstMatch && ib.Op == syntax.InstMatch {
					return true
				}

				if !consumes(ia.Op) || !consumes(ib.Op) || !runesIntersect(runeRanges(ia), runeRanges(ib)) {
					continue
				}

				next := state{ia.Out, ib.Out}
				if !seen[next] {
					seen[next] = true
					queue = append(queue, next)
				}
			}
		}
	}

	return false
}

func closure(p *syntax.Prog, pc uint32) []uint32 {
	var out []uint32

	seen := map[uint32]bool{}
	stack := []uint32{pc}

	for len(stack) > 0 {
		pc := stack[len(stack)-1]
		stack = stack[:len(stack)-1]

		if seen[pc] {
			continue
		}

		seen[pc] = true
		inst := &p.Inst[pc]

		switch inst.Op {
		case syntax.InstAlt, syntax.InstAltMatch:
			stack = append(stack, inst.Out, inst.Arg)
		case syntax.InstCapture, syntax.InstNop, syntax.InstEmptyWidth:
			stack = append(stack, inst.Out)
		case syntax.InstFail:
		default:
			out = append(out, pc)
		}
	}

	return out
}

func consumes(op syntax.InstOp) bool {
	switch op {
	case syntax.InstRune, syntax.InstRune1, syntax.InstRuneAny, syntax.InstRuneAnyNotNL:
		return true
	default:
		return false
	}
}

func runeRanges(inst *syntax.Inst) [][2]rune {
	switch inst.Op {
	case syntax.InstRuneAny:
		return [][2]rune{{0, unicode.MaxRune}}
	case syntax.InstRuneAnyNotNL:
		return [][2]rune{{0, '\n' - 1}, {'\n' + 1, unicode.MaxRune}}
	}

	if len(inst.Rune) == 1 {
		r := inst.Rune[0]
		out := [][2]rune{{r, r}}

		if syntax.Flags(inst.Arg)&syntax.FoldCase != 0 {
			for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
				out = append(out, [2]rune{f, f})
			}
		}

		return out
	}

	out := make([][2]rune, 0, len(inst.Rune)/2)
	for i := 0; i+1 < len(inst.Rune); i += 2 {
		out = append(out, [2]rune{inst.Rune[i], inst.Rune[i+1]})
	}

	return out
}

func runesIntersect(a, b [][2]rune) bool {
	for _, ra := range a {
		for _, rb := range b {
			if ra[0] <= rb[1] && rb[0] <= ra[1] {
				return true
			}
		}
	}

	return false
}

func parse(uri string, m mode) (parsed, error) {
	colon := strings.IndexByte(uri, ':')
	if colon <= 0 {
		return parsed{}, fmt.Errorf("%w: %q has no scheme", ErrInvalidURI, uri)
	}

	p := parsed{scheme: strings.ToLower(uri[:colon])}
	body := uri[colon+1:]

	if strings.TrimSpace(uri) != uri || body == "" {
		return parsed{}, fmt.Errorf("%w: %q", ErrInvalidURI, uri)
	}

	wildcarded := m == modeStored && strings.Contains(body, wildcardDelimiter)
	if m == modeServerName && strings.Contains(body, wildcardDelimiter) {
		return parsed{}, fmt.Errorf("%w: %q must not contain %q", ErrInvalidURI, uri, wildcardDelimiter)
	}

	switch p.scheme {
	case "sip", "sips":
		return p.sip(uri, body, wildcarded, m == modeServerName)
	case "tel":
		return p.tel(uri, body, wildcarded)
	default:
		return parsed{}, fmt.Errorf("%w: %q: %w", ErrInvalidURI, uri, errUnsupportedScheme)
	}
}

func (p parsed) sip(uri, body string, wildcarded, keepParameters bool) (parsed, error) {
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

	p.host = strings.ToLower(body[:hostEnd])

	if keepParameters {
		p.rest = body[hostEnd:]
	}

	if !validHostport(p.host) {
		return parsed{}, fmt.Errorf("%w: %q has no valid host", ErrInvalidURI, uri)
	}

	if !validUser(p.user, wildcarded) {
		return parsed{}, fmt.Errorf("%w: %q has an invalid user part", ErrInvalidURI, uri)
	}

	if wildcarded && strings.Contains(p.host, wildcardDelimiter) {
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

	subscriber, params := body[:paramStart], body[paramStart:]

	if wildcarded {
		start, end := strings.Index(subscriber, wildcardDelimiter), strings.LastIndex(subscriber, wildcardDelimiter)
		if strings.Trim(stripVisualSeparators(subscriber[:start]+subscriber[end+1:]), "+") == "" {
			return parsed{}, fmt.Errorf("%w: %q: a tel wildcard needs a fixed part", ErrInvalidWildcard, uri)
		}

		subscriber = stripVisualSeparators(subscriber[:start]) + subscriber[start:end+1] + stripVisualSeparators(subscriber[end+1:])
	} else {
		subscriber = stripVisualSeparators(subscriber)
	}

	if !validNumber(subscriber, wildcarded) {
		return parsed{}, fmt.Errorf("%w: %q has no valid number", ErrInvalidURI, uri)
	}

	p.user, p.rest = subscriber, phoneContext(params)

	return p, nil
}

func phoneContext(params string) string {
	for _, param := range strings.Split(params, ";") {
		name, value, _ := strings.Cut(param, "=")
		if strings.EqualFold(name, "phone-context") {
			return ";phone-context=" + strings.ToLower(value)
		}
	}

	return ""
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

func validHostport(hostport string) bool {
	host, port := hostport, ""

	if strings.HasPrefix(hostport, "[") {
		end := strings.IndexByte(hostport, ']')
		if end < 0 {
			return false
		}

		host, port = hostport[1:end], strings.TrimPrefix(hostport[end+1:], ":")
		if hostport[end+1:] != "" && !strings.HasPrefix(hostport[end+1:], ":") {
			return false
		}

		if host == "" || strings.Trim(host, "0123456789abcdef:.") != "" {
			return false
		}
	} else {
		if i := strings.LastIndexByte(hostport, ':'); i >= 0 {
			host, port = hostport[:i], hostport[i+1:]
			if port == "" {
				return false
			}
		}

		if host == "" || strings.HasPrefix(host, ".") || strings.Contains(host, "..") {
			return false
		}

		for _, r := range host {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '.' {
				return false
			}
		}
	}

	return strings.Trim(port, "0123456789") == "" && len(port) <= 5
}

func validUser(user string, wildcarded bool) bool {
	if wildcarded {
		start, end := strings.Index(user, wildcardDelimiter), strings.LastIndex(user, wildcardDelimiter)
		user = user[:start] + user[end+1:]
	}

	for _, r := range user {
		if r <= ' ' || r == 0x7f || strings.ContainsRune(`<>"@`, r) {
			return false
		}
	}

	return true
}

func validNumber(number string, wildcarded bool) bool {
	fixed := number
	if wildcarded {
		start, end := strings.Index(number, wildcardDelimiter), strings.LastIndex(number, wildcardDelimiter)
		fixed = number[:start] + number[end+1:]
	}

	digits := strings.TrimPrefix(fixed, "+")
	if digits == "" && !wildcarded {
		return false
	}

	return strings.Trim(digits, "0123456789abcdefABCDEF*#") == ""
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
