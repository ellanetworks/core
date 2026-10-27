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
	MinAMFPointer = 1
	MaxAMFPointer = 63
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
	NF         CallbackNF
	AMFPointer int
	Supi       string
	Kind       CallbackKind
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

	if c.AMFPointer < MinAMFPointer || c.AMFPointer > MaxAMFPointer {
		return fmt.Errorf("%w: AMF pointer %d is outside [%d, %d]", ErrCallbackURI, c.AMFPointer, MinAMFPointer, MaxAMFPointer)
	}

	if _, ok := wire.IMSIFromSupi(c.Supi); !ok {
		return fmt.Errorf("%w: SUPI %q is not imsi-<digits>", ErrCallbackURI, c.Supi)
	}

	return nil
}

func (c Callback) Path() string {
	return CallbackPathRoot + "/" + strconv.Itoa(c.AMFPointer) + "/" + string(c.NF) + "/" + c.Supi + "/" + string(c.Kind)
}

func (c Callback) URI(op wire.PlmnID) (string, error) {
	if err := c.validate(); err != nil {
		return "", err
	}

	if !op.Valid() {
		return "", fmt.Errorf("%w: operator PLMN %s", ErrCallbackURI, op)
	}

	u := url.URL{Scheme: "https", Host: CallbackHost(c.NF, op), Path: c.Path()}

	return u.String(), nil
}

func CallbackHost(nf CallbackNF, op wire.PlmnID) string {
	return op.NFFQDN(string(nf))
}

func ParseCallback(method, host, escapedPath string, op wire.PlmnID) (Callback, error) {
	rest, ok := strings.CutPrefix(escapedPath, CallbackPathRoot+"/")
	if !ok || strings.Contains(escapedPath, "%") || hasDotSegment(escapedPath) {
		return Callback{}, fmt.Errorf("%w: path %q", ErrCallbackURI, escapedPath)
	}

	parts := strings.Split(rest, "/")
	if len(parts) != 4 {
		return Callback{}, fmt.Errorf("%w: path %q", ErrCallbackURI, escapedPath)
	}

	ptr, err := strconv.Atoi(parts[0])
	if err != nil || strconv.Itoa(ptr) != parts[0] {
		return Callback{}, fmt.Errorf("%w: AMF pointer %q", ErrCallbackURI, parts[0])
	}

	c := Callback{NF: CallbackNF(parts[1]), AMFPointer: ptr, Supi: parts[2], Kind: CallbackKind(parts[3])}
	if err := c.validate(); err != nil {
		return Callback{}, err
	}

	if method != c.Kind.Method() {
		return Callback{}, fmt.Errorf("%w: method %s", ErrCallbackURI, method)
	}

	h := host
	if u, err := url.Parse("https://" + host); err == nil && u.Port() != "" {
		h = u.Hostname()
	}

	if !strings.EqualFold(strings.TrimSuffix(h, "."), CallbackHost(c.NF, op)) {
		return Callback{}, fmt.Errorf("%w: host %q does not serve %s callbacks of %s", ErrCallbackURI, host, c.NF, op)
	}

	return c, nil
}
