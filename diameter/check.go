// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
)

type AVPKey struct {
	Code     uint32
	VendorID uint32
}

type Rule struct {
	Required  bool
	Multiple  bool
	MinLength int
}

type Rules map[AVPKey]Rule

type AVPError struct {
	ResultCode uint32
	AVP        AVP
}

func (e *AVPError) Error() string {
	return fmt.Sprintf("diameter: AVP %d (vendor %d) rejected with result %d", e.AVP.Code, e.AVP.VendorID, e.ResultCode)
}

func NewAVPError(resultCode uint32, a AVP) error {
	return &AVPError{ResultCode: resultCode, AVP: a}
}

func BaseRequestRules() Rules {
	return Rules{
		{Code: AVPSessionID}:                   {Required: true},
		{Code: AVPDRMP}:                        {},
		{Code: AVPVendorSpecificApplicationID}: {},
		{Code: AVPAuthSessionState}:            {Required: true, MinLength: 4},
		{Code: AVPOriginHost}:                  {Required: true},
		{Code: AVPOriginRealm}:                 {Required: true},
		{Code: AVPDestinationHost}:             {},
		{Code: AVPDestinationRealm}:            {Required: true},
		{Code: AVPOriginStateID}:               {},
		{Code: AVPProxyInfo}:                   {Multiple: true},
		{Code: AVPRouteRecord}:                 {Multiple: true},
	}
}

func (r Rules) With(extra Rules) Rules {
	merged := maps.Clone(r)
	maps.Copy(merged, extra)

	return merged
}

func (r Rules) Check(m *Message) error {
	seen := make(map[AVPKey]int, len(m.AVPs))

	for _, a := range m.AVPs {
		key := AVPKey{Code: a.Code, VendorID: a.VendorID}

		rule, known := r[key]
		if !known {
			if a.Flags&AVPFlagMandatory != 0 {
				return NewAVPError(ResultAVPUnsupported, a)
			}

			continue
		}

		seen[key]++

		if seen[key] > 1 && !rule.Multiple {
			return NewAVPError(ResultAVPOccursTooManyTimes, a)
		}

		if rule.Required && len(a.Data) == 0 {
			return NewAVPError(ResultInvalidAVPValue, a)
		}
	}

	missing := slices.SortedFunc(maps.Keys(r), func(a, b AVPKey) int {
		return cmp.Or(cmp.Compare(a.VendorID, b.VendorID), cmp.Compare(a.Code, b.Code))
	})

	for _, key := range missing {
		if rule := r[key]; rule.Required && seen[key] == 0 {
			return NewAVPError(ResultMissingAVP, OctetString(key.Code, AVPFlagMandatory, key.VendorID, make([]byte, rule.MinLength)))
		}
	}

	return nil
}
