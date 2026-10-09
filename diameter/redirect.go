// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type RedirectHostUsage uint32

const (
	DontCache           RedirectHostUsage = 0
	AllSession          RedirectHostUsage = 1
	AllRealm            RedirectHostUsage = 2
	RealmAndApplication RedirectHostUsage = 3
	AllApplication      RedirectHostUsage = 4
	AllHost             RedirectHostUsage = 5
	AllUser             RedirectHostUsage = 6
)

func (u RedirectHostUsage) String() string {
	switch u {
	case DontCache:
		return "DONT_CACHE"
	case AllSession:
		return "ALL_SESSION"
	case AllRealm:
		return "ALL_REALM"
	case RealmAndApplication:
		return "REALM_AND_APPLICATION"
	case AllApplication:
		return "ALL_APPLICATION"
	case AllHost:
		return "ALL_HOST"
	case AllUser:
		return "ALL_USER"
	default:
		return fmt.Sprintf("RedirectHostUsage(%d)", uint32(u))
	}
}

func (u RedirectHostUsage) valid() bool {
	return u <= AllUser
}

var ErrInvalidRedirect = errors.New("diameter: invalid redirect")

type Redirect struct {
	Hosts        []URI
	Usage        RedirectHostUsage
	MaxCacheTime time.Duration
}

func NewRedirectAnswer(req *Message, id Identity, r Redirect) (*Message, error) {
	switch {
	case len(r.Hosts) == 0:
		return nil, fmt.Errorf("%w: no Redirect-Host", ErrInvalidRedirect)
	case !r.Usage.valid():
		return nil, fmt.Errorf("%w: Redirect-Host-Usage %s", ErrInvalidRedirect, r.Usage)
	case r.Usage != DontCache && (r.MaxCacheTime < time.Second || r.MaxCacheTime%time.Second != 0 || r.MaxCacheTime/time.Second > math.MaxUint32):
		return nil, fmt.Errorf("%w: Redirect-Max-Cache-Time %s", ErrInvalidRedirect, r.MaxCacheTime)
	}

	ans := NewAnswer(req, id, ResultRedirectIndication)

	for _, h := range r.Hosts {
		if !isDiameterIdentity(h.Host) {
			return nil, fmt.Errorf("%w: Redirect-Host %q", ErrInvalidRedirect, h.Host)
		}

		ans.AVPs = append(ans.AVPs, UTF8String(AVPRedirectHost, AVPFlagMandatory, 0, h.String()))
	}

	if r.Usage != DontCache {
		ans.AVPs = append(ans.AVPs,
			Unsigned32(AVPRedirectHostUsage, AVPFlagMandatory, 0, uint32(r.Usage)),
			Unsigned32(AVPRedirectMaxCacheTime, AVPFlagMandatory, 0, uint32(r.MaxCacheTime/time.Second)),
		)
	}

	return ans, nil
}

func ParseRedirect(ans *Message) (Redirect, error) {
	if code, ok := protocolResult(ans); !ok || code != ResultRedirectIndication {
		return Redirect{}, fmt.Errorf("%w: not a DIAMETER_REDIRECT_INDICATION answer", ErrInvalidRedirect)
	}

	var r Redirect

	for _, a := range FindAll(ans.AVPs, AVPRedirectHost, 0) {
		u, err := ParseURI(a.UTF8String())
		if err != nil {
			return Redirect{}, fmt.Errorf("%w: %w", ErrInvalidRedirect, err)
		}

		r.Hosts = append(r.Hosts, u)
	}

	if len(r.Hosts) == 0 {
		return Redirect{}, fmt.Errorf("%w: no Redirect-Host", ErrInvalidRedirect)
	}

	if a, ok := ans.Find(AVPRedirectHostUsage, 0); ok {
		v, err := a.Unsigned32()
		if err != nil || !RedirectHostUsage(v).valid() {
			return Redirect{}, fmt.Errorf("%w: Redirect-Host-Usage", ErrInvalidRedirect)
		}

		r.Usage = RedirectHostUsage(v)
	}

	if r.Usage == DontCache {
		return r, nil
	}

	a, ok := ans.Find(AVPRedirectMaxCacheTime, 0)
	if !ok {
		return Redirect{}, fmt.Errorf("%w: Redirect-Host-Usage %s without Redirect-Max-Cache-Time", ErrInvalidRedirect, r.Usage)
	}

	seconds, err := a.Unsigned32()
	if err != nil {
		return Redirect{}, fmt.Errorf("%w: Redirect-Max-Cache-Time", ErrInvalidRedirect)
	}

	r.MaxCacheTime = time.Duration(seconds) * time.Second

	return r, nil
}

func protocolResult(ans *Message) (uint32, bool) {
	if ans.Flags&FlagError == 0 {
		return 0, false
	}

	a, ok := ans.Find(AVPResultCode, 0)
	if !ok {
		return 0, false
	}

	code, err := a.Unsigned32()

	return code, err == nil
}

var cachePrecedence = []RedirectHostUsage{AllSession, AllUser, RealmAndApplication, AllRealm, AllApplication}

type cacheKey struct {
	usage RedirectHostUsage
	value string
}

type cacheEntry struct {
	target *peer
	host   string
	timer  *time.Timer
}

func requestCacheKey(usage RedirectHostUsage, req *Message) (cacheKey, bool) {
	var value string

	switch usage {
	case AllSession:
		value = avpString(req, AVPSessionID)
	case AllUser:
		value = avpString(req, AVPUserName)
	case AllRealm:
		value = strings.ToLower(avpString(req, AVPDestinationRealm))
	case RealmAndApplication:
		if realm := avpString(req, AVPDestinationRealm); realm != "" {
			value = strings.ToLower(realm) + "/" + strconv.FormatUint(uint64(req.ApplicationID), 10)
		}
	case AllApplication:
		value = strconv.FormatUint(uint64(req.ApplicationID), 10)
	default:
		return cacheKey{}, false
	}

	return cacheKey{usage: usage, value: value}, value != ""
}

func hostCacheKey(host string) cacheKey {
	return cacheKey{usage: AllHost, value: strings.ToLower(host)}
}

func avpString(m *Message, code uint32) string {
	a, ok := m.Find(code, 0)
	if !ok {
		return ""
	}

	return a.UTF8String()
}

func (n *Node) cacheLookupLocked(req *Message) (*cacheEntry, bool) {
	for _, usage := range cachePrecedence {
		key, ok := requestCacheKey(usage, req)
		if !ok {
			continue
		}

		if e, ok := n.cache[key]; ok {
			return e, true
		}
	}

	return nil, false
}

func (n *Node) cacheStoreLocked(key cacheKey, target *peer, host string, ttl time.Duration) {
	n.cacheDeleteLocked(key)

	e := &cacheEntry{target: target, host: host}
	e.timer = time.AfterFunc(ttl, func() {
		n.mu.Lock()
		defer n.mu.Unlock()

		if n.cache[key] == e {
			n.cacheDeleteLocked(key)
		}
	})

	n.cache[key] = e
	target.refs++
}

func (n *Node) cacheDeleteLocked(key cacheKey) {
	e, ok := n.cache[key]
	if !ok {
		return
	}

	e.timer.Stop()
	delete(n.cache, key)
	n.releaseLocked(e.target)
}

func (n *Node) cacheForgetPeerLocked(p *peer) {
	for key, e := range n.cache {
		if e.target == p || (key.usage == AllHost && p.host != "" && key.value == p.host) {
			n.cacheDeleteLocked(key)
		}
	}
}
