// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package sepp

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ellanetworks/core/internal/roaming/wire"
)

const CallbackPathRoot = "/roaming-callbacks/v1"

const (
	MinSEPPID = 1
	MaxSEPPID = 63
)

type CallbackNF string

const (
	CallbackAMF CallbackNF = "amf"
	CallbackSMF CallbackNF = "smf"
)

type CallbackKind string

const (
	CallbackUECMDereg CallbackKind = "uecm-dereg"
	CallbackSDMChange CallbackKind = "sdm-change"
)

func (k CallbackKind) Method() string {
	return http.MethodPost
}

func (k CallbackKind) NotFound(detail string) wire.ProblemDetails {
	if k == CallbackUECMDereg {
		return wire.NewProblem(http.StatusNotFound, wire.CauseContextNotFound, detail)
	}

	return wire.NewProblem(http.StatusNotFound, wire.CauseSubscriptionNotFound, detail)
}

var ErrCallbackURI = errors.New("sepp: not a roaming callback URI")

type Callback struct {
	NF     CallbackNF
	SEPPID int
	Supi   string
	Kind   CallbackKind
}

func callbackAllowed(nf CallbackNF, kind CallbackKind) bool {
	switch nf {
	case CallbackAMF:
		return kind == CallbackUECMDereg || kind == CallbackSDMChange
	case CallbackSMF:
		return kind == CallbackSDMChange
	default:
		return false
	}
}

func (c Callback) validate() error {
	if !callbackAllowed(c.NF, c.Kind) {
		return fmt.Errorf("%w: %s %s is not on the allow-list", ErrCallbackURI, c.NF, c.Kind)
	}

	if c.SEPPID < MinSEPPID || c.SEPPID > MaxSEPPID {
		return fmt.Errorf("%w: node %d is outside [%d, %d]", ErrCallbackURI, c.SEPPID, MinSEPPID, MaxSEPPID)
	}

	if _, ok := wire.IMSIFromSupi(c.Supi); !ok {
		return fmt.Errorf("%w: SUPI %q is not imsi-<digits>", ErrCallbackURI, c.Supi)
	}

	return nil
}

func (c Callback) Path() string {
	return CallbackPathRoot + "/" + c.Supi + "/" + string(c.Kind)
}

func (c Callback) URI(op wire.PlmnID) (string, error) {
	if err := c.validate(); err != nil {
		return "", err
	}

	if !op.Valid() {
		return "", fmt.Errorf("%w: operator PLMN %s", ErrCallbackURI, op)
	}

	u := url.URL{Scheme: "https", Host: CallbackHost(c.NF, c.SEPPID, op), Path: c.Path()}

	return u.String(), nil
}

func CallbackHost(nf CallbackNF, seppID int, op wire.PlmnID) string {
	return op.NFFQDN(string(nf) + strconv.Itoa(seppID))
}

func ParseCallbackHost(host string, op wire.PlmnID) (CallbackNF, int, error) {
	h := host
	if u, err := url.Parse("https://" + host); err == nil && u.Port() != "" {
		h = u.Hostname()
	}

	name, ok := wire.ParseNetworkFQDN(h)
	if !ok || !name.InPLMN(op) {
		return "", 0, fmt.Errorf("%w: host %q is not in the network of %s", ErrCallbackURI, host, op)
	}

	for _, nf := range []CallbackNF{CallbackAMF, CallbackSMF} {
		digits, ok := strings.CutPrefix(name.Label, string(nf))
		if !ok {
			continue
		}

		id, err := strconv.Atoi(digits)
		if err != nil || strconv.Itoa(id) != digits || id < MinSEPPID || id > MaxSEPPID {
			break
		}

		return nf, id, nil
	}

	return "", 0, fmt.Errorf("%w: host %q is not an AMF or SMF callback host", ErrCallbackURI, host)
}

func ParseCallback(method, host, escapedPath string, op wire.PlmnID) (Callback, error) {
	nf, id, err := ParseCallbackHost(host, op)
	if err != nil {
		return Callback{}, err
	}

	rest, ok := strings.CutPrefix(escapedPath, CallbackPathRoot+"/")
	if !ok || strings.Contains(escapedPath, "%") || hasDotSegment(escapedPath) {
		return Callback{}, fmt.Errorf("%w: path %q", ErrCallbackURI, escapedPath)
	}

	parts := strings.Split(rest, "/")
	if len(parts) != 2 {
		return Callback{}, fmt.Errorf("%w: path %q", ErrCallbackURI, escapedPath)
	}

	c := Callback{NF: nf, SEPPID: id, Supi: parts[0], Kind: CallbackKind(parts[1])}
	if err := c.validate(); err != nil {
		return Callback{}, err
	}

	if method != c.Kind.Method() {
		return Callback{}, fmt.Errorf("%w: method %s", ErrCallbackURI, method)
	}

	return c, nil
}
