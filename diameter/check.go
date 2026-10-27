// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package diameter

import (
	"cmp"
	"errors"
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
	AVPs       []AVP
}

func (e *AVPError) Error() string {
	if len(e.AVPs) == 0 {
		return fmt.Sprintf("diameter: rejected with result %d", e.ResultCode)
	}

	first := e.AVPs[0]

	return fmt.Sprintf("diameter: AVP %d (vendor %d) rejected with result %d", first.Code, first.VendorID, e.ResultCode)
}

func NewAVPError(resultCode uint32, first AVP, rest ...AVP) error {
	return &AVPError{ResultCode: resultCode, AVPs: append([]AVP{first}, rest...)}
}

func BaseRequestRules() Rules {
	return Rules{
		{Code: AVPSessionID}:                   {Required: true, MinLength: 1},
		{Code: AVPDRMP}:                        {},
		{Code: AVPVendorSpecificApplicationID}: {},
		{Code: AVPAuthSessionState}:            {Required: true, MinLength: 4},
		{Code: AVPOriginHost}:                  {Required: true, MinLength: 1},
		{Code: AVPOriginRealm}:                 {Required: true, MinLength: 1},
		{Code: AVPDestinationHost}:             {},
		{Code: AVPDestinationRealm}:            {Required: true, MinLength: 1},
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
	return r.check(m.AVPs)
}

func (r Rules) CheckGrouped(a AVP) ([]AVP, error) {
	inner, err := a.Grouped()
	if err != nil {
		return nil, err
	}

	if err := r.check(inner); err != nil {
		return nil, WithinGrouped(a, err)
	}

	return inner, nil
}

func WithinGrouped(parent AVP, err error) error {
	var avpErr *AVPError
	if !errors.As(err, &avpErr) || len(avpErr.AVPs) == 0 ||
		avpErr.AVPs[0].Code == parent.Code && avpErr.AVPs[0].VendorID == parent.VendorID {
		return err
	}

	return NewAVPError(avpErr.ResultCode, Grouped(parent.Code, parent.Flags, parent.VendorID, avpErr.AVPs...))
}

func (r Rules) check(avps []AVP) error {
	seen := make(map[AVPKey]int, len(avps))

	for _, a := range avps {
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

		if len(a.Data) < rule.MinLength {
			return NewAVPError(ResultInvalidAVPLength, a)
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
