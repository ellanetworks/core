// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package wire

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
)

const (
	LinkRel5GAKA      = "5g-aka"
	LinkRelEAPSession = "eap-session"
)

var servingNetworkNamePattern = regexp.MustCompile(`^5G:mnc([0-9]{3})[.]mcc([0-9]{3})[.]3gppnetwork[.]org(:[A-F0-9]{11})?$`)

type AuthType string

const (
	AuthType5GAKA       AuthType = "5G_AKA"
	AuthTypeEAPAKAPrime AuthType = "EAP_AKA_PRIME"
	AuthTypeEAPTLS      AuthType = "EAP_TLS"
	AuthTypeEAPTTLS     AuthType = "EAP_TTLS"
)

func (a AuthType) Known() bool {
	return oneOf(a, AuthType5GAKA, AuthTypeEAPAKAPrime, AuthTypeEAPTLS, AuthTypeEAPTTLS)
}

type AuthResult string

const (
	AuthResultSuccess AuthResult = "AUTHENTICATION_SUCCESS"
	AuthResultFailure AuthResult = "AUTHENTICATION_FAILURE"
	AuthResultOngoing AuthResult = "AUTHENTICATION_ONGOING"
)

func (a AuthResult) Known() bool {
	return oneOf(a, AuthResultSuccess, AuthResultFailure, AuthResultOngoing)
}

type ServingNetwork struct {
	Mcc string
	Mnc string
	Nid string
}

func ParseServingNetworkName(s string) (ServingNetwork, error) {
	m := servingNetworkNamePattern.FindStringSubmatch(s)
	if m == nil {
		return ServingNetwork{}, fmt.Errorf("wire: %q is not a serving network name", s)
	}

	sn := ServingNetwork{Mcc: m[2], Mnc: m[1]}
	if m[3] != "" {
		sn.Nid = m[3][1:]
	}

	return sn, nil
}

func (sn ServingNetwork) Matches(p PlmnID) bool {
	return sn.Nid == "" && p.Valid() && sn.Mcc == p.Mcc && sn.Mnc == p.mnc3()
}

func checkHex(param string, s string, size int, optional bool) error {
	if s == "" {
		if optional {
			return nil
		}

		return Missing(param)
	}

	if len(s) != 2*size {
		return Incorrect(param, optional, "not %d hexadecimal digits", 2*size)
	}

	if _, err := hex.DecodeString(s); err != nil {
		return Incorrect(param, optional, "not %d hexadecimal digits", 2*size)
	}

	return nil
}

func decodeHex(dst []byte, s string) error {
	b, err := hex.DecodeString(s)
	if err != nil || len(b) != len(dst) {
		return fmt.Errorf("wire: %q is not %d hexadecimal octets", s, len(dst))
	}

	copy(dst, b)

	return nil
}

type ResynchronizationInfo struct {
	Rand string `json:"rand"`
	Auts string `json:"auts"`
}

func NewResynchronizationInfo(rand [16]byte, auts [14]byte) *ResynchronizationInfo {
	return &ResynchronizationInfo{Rand: hex.EncodeToString(rand[:]), Auts: hex.EncodeToString(auts[:])}
}

func (r ResynchronizationInfo) Check(Policy) error {
	return firstError(checkHex("/rand", r.Rand, 16, false), checkHex("/auts", r.Auts, 14, false))
}

type AuthenticationInfo struct {
	SupiOrSuci            string                 `json:"supiOrSuci"`
	ServingNetworkName    string                 `json:"servingNetworkName"`
	ResynchronizationInfo *ResynchronizationInfo `json:"resynchronizationInfo,omitempty"`
	Pei                   string                 `json:"pei,omitempty"`
	RoutingIndicator      string                 `json:"routingIndicator,omitempty"`
	SupportedFeatures     SupportedFeatures      `json:"supportedFeatures,omitempty"`
}

func (a AuthenticationInfo) Check(p Policy) error {
	if err := firstError(
		present("/supiOrSuci", a.SupiOrSuci != ""),
		present("/servingNetworkName", a.ServingNetworkName != ""),
	); err != nil || p != Strict {
		return err
	}

	return firstError(
		checkString("/supiOrSuci", a.SupiOrSuci, ValidSupiOrSuci, false),
		checkPattern("/servingNetworkName", a.ServingNetworkName, servingNetworkNamePattern, false),
		CheckOptional("/resynchronizationInfo", a.ResynchronizationInfo, p),
		checkString("/pei", a.Pei, ValidPei, true),
		checkString("/routingIndicator", a.RoutingIndicator, ValidRoutingIndicator, true),
		Nest("/supportedFeatures", a.SupportedFeatures.Check(p)),
	)
}

type Av5gAka struct {
	Rand      string `json:"rand"`
	HxresStar string `json:"hxresStar"`
	Autn      string `json:"autn"`
}

func (a Av5gAka) Check(Policy) error {
	return firstError(
		checkHex("/rand", a.Rand, 16, false),
		checkHex("/hxresStar", a.HxresStar, 16, false),
		checkHex("/autn", a.Autn, 16, false),
	)
}

func (a Av5gAka) Vector() ([16]byte, [16]byte, [16]byte, error) {
	var rand, hxresStar, autn [16]byte

	err := firstError(decodeHex(rand[:], a.Rand), decodeHex(hxresStar[:], a.HxresStar), decodeHex(autn[:], a.Autn))

	return rand, hxresStar, autn, err
}

type AuthData struct {
	Av5gAka    *Av5gAka
	EapPayload []byte
}

func (d *AuthData) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)

	switch {
	case bytes.Equal(trimmed, []byte("null")):
		*d = AuthData{}

		return nil
	case len(trimmed) > 0 && trimmed[0] == '{':
		var av Av5gAka
		if err := json.Unmarshal(trimmed, &av); err != nil {
			return err
		}

		*d = AuthData{Av5gAka: &av}

		return nil
	default:
		var eap []byte
		if err := json.Unmarshal(trimmed, &eap); err != nil {
			return err
		}

		*d = AuthData{EapPayload: eap}

		return nil
	}
}

func (d AuthData) empty() bool {
	return d.Av5gAka == nil && d.EapPayload == nil
}

type Link struct {
	Href string `json:"href,omitempty"`
}

type Links []Link

func (l *Links) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] == '[' {
		var list []Link
		if err := json.Unmarshal(trimmed, &list); err != nil {
			return err
		}

		*l = list

		return nil
	}

	if bytes.Equal(trimmed, []byte("null")) {
		*l = nil

		return nil
	}

	var one Link
	if err := json.Unmarshal(trimmed, &one); err != nil {
		return err
	}

	*l = Links{one}

	return nil
}

type UEAuthenticationCtx struct {
	AuthType           AuthType         `json:"authType"`
	AuthData           AuthData         `json:"5gAuthData"`
	Links              map[string]Links `json:"_links"`
	ServingNetworkName string           `json:"servingNetworkName,omitempty"`
}

func (c UEAuthenticationCtx) Href(rel string) (string, bool) {
	for _, l := range c.Links[rel] {
		if l.Href != "" {
			return l.Href, true
		}
	}

	return "", false
}

func (c UEAuthenticationCtx) Check(p Policy) error {
	if err := firstError(
		CheckEnum("/authType", c.AuthType, p, false),
		present("/5gAuthData", !c.AuthData.empty()),
		present("/_links", c.Links != nil),
	); err != nil {
		return err
	}

	switch c.AuthType {
	case AuthType5GAKA:
		if c.AuthData.Av5gAka == nil {
			return Incorrect("/5gAuthData", false, "5G_AKA without Av5gAka")
		}

		if err := c.AuthData.Av5gAka.Check(p); err != nil {
			return Nest("/5gAuthData", err)
		}

		href, ok := c.Href(LinkRel5GAKA)
		if !ok {
			return Missing("/_links/" + LinkRel5GAKA)
		}

		if err := checkAbsoluteURI("/_links/"+LinkRel5GAKA+"/href", href, false); err != nil {
			return err
		}
	case AuthTypeEAPAKAPrime, AuthTypeEAPTLS, AuthTypeEAPTTLS:
		if c.AuthData.EapPayload == nil {
			return Incorrect("/5gAuthData", false, "%s without EapPayload", c.AuthType)
		}
	}

	if p != Strict {
		return nil
	}

	for _, rel := range sortedKeys(c.Links) {
		if len(c.Links[rel]) == 0 {
			return Incorrect("/_links/"+rel, false, "empty link list")
		}
	}

	return checkPattern("/servingNetworkName", c.ServingNetworkName, servingNetworkNamePattern, true)
}

type ConfirmationData struct {
	ResStar           *string           `json:"resStar"`
	SupportedFeatures SupportedFeatures `json:"supportedFeatures,omitempty"`
}

func ConfirmationWithResStar(resStar [16]byte) ConfirmationData {
	s := hex.EncodeToString(resStar[:])

	return ConfirmationData{ResStar: &s}
}

func (c ConfirmationData) Check(p Policy) error {
	if c.ResStar != nil {
		if err := checkHex("/resStar", *c.ResStar, 16, false); err != nil {
			return err
		}
	}

	if p == Strict {
		return Nest("/supportedFeatures", c.SupportedFeatures.Check(p))
	}

	return nil
}

type ConfirmationDataResponse struct {
	AuthResult AuthResult `json:"authResult"`
	Supi       string     `json:"supi,omitempty"`
	Kseaf      string     `json:"kseaf,omitempty"`
}

func (c ConfirmationDataResponse) Check(p Policy) error {
	if err := CheckEnum("/authResult", c.AuthResult, p, false); err != nil {
		return err
	}

	if err := checkHex("/kseaf", c.Kseaf, 32, c.AuthResult != AuthResultSuccess); err != nil {
		return err
	}

	return checkString("/supi", c.Supi, ValidSupi, true)
}

var ErrAuthenticationFailed = errors.New("wire: authentication failed")

func (c ConfirmationDataResponse) Success(supiOrSuci string) (string, [32]byte, error) {
	var kseaf [32]byte

	if c.AuthResult != AuthResultSuccess {
		return "", kseaf, fmt.Errorf("%w: %s", ErrAuthenticationFailed, c.AuthResult)
	}

	if err := decodeHex(kseaf[:], c.Kseaf); err != nil {
		return "", kseaf, Missing("/kseaf")
	}

	if ValidSupi(supiOrSuci) {
		if c.Supi != "" && c.Supi != supiOrSuci {
			return "", kseaf, Incorrect("/supi", false, "%q is not the requested SUPI", c.Supi)
		}

		return supiOrSuci, kseaf, nil
	}

	suci, err := ParseSuci(supiOrSuci)
	if err != nil {
		return "", kseaf, err
	}

	if c.Supi == "" {
		return "", kseaf, Missing("/supi")
	}

	if _, ok := HomePLMN(c.Supi, []PlmnID{suci.PlmnID()}); !ok {
		return "", kseaf, Incorrect("/supi", false, "%q is not in the SUCI home network %s", c.Supi, suci.PlmnID())
	}

	return c.Supi, kseaf, nil
}
