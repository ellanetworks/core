// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package eps

import "github.com/ellanetworks/core/nas"

// AdditionalUpdateResult is the additional update result value (TS 24.301 §9.9.3.0A).
type AdditionalUpdateResult uint8

// Additional update result values (TS 24.301 table 9.9.3.0A.1).
const (
	AdditionalUpdateResultNone                   AdditionalUpdateResult = 0
	AdditionalUpdateResultCSFallbackNotPreferred AdditionalUpdateResult = 1
	AdditionalUpdateResultSMSOnly                AdditionalUpdateResult = 2
)

var additionalUpdateResultNames = map[uint8]string{
	uint8(AdditionalUpdateResultNone):                   "no additional information",
	uint8(AdditionalUpdateResultCSFallbackNotPreferred): "CS Fallback not preferred",
	uint8(AdditionalUpdateResultSMSOnly):                "SMS only",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (r AdditionalUpdateResult) Name() string { return additionalUpdateResultNames[uint8(r)] }

func (r AdditionalUpdateResult) String() string {
	return enumString(uint8(r), additionalUpdateResultNames)
}

// SMSServicesStatus is the SMS services status value (TS 24.301 §9.9.3.4B).
type SMSServicesStatus uint8

// SMS services status values (TS 24.301 table 9.9.3.4B.1).
const (
	SMSServicesNotAvailable       SMSServicesStatus = 0
	SMSServicesNotAvailableInPLMN SMSServicesStatus = 1
	SMSServicesNetworkFailure     SMSServicesStatus = 2
	SMSServicesCongestion         SMSServicesStatus = 3
)

var smsServicesStatusNames = map[uint8]string{
	uint8(SMSServicesNotAvailable):       "SMS services not available",
	uint8(SMSServicesNotAvailableInPLMN): "SMS services not available in this PLMN",
	uint8(SMSServicesNetworkFailure):     "Network failure",
	uint8(SMSServicesCongestion):         "Congestion",
}

// Name returns the value's spec description, or the empty string when the value
// is not one TS 24.301 assigns.
func (s SMSServicesStatus) Name() string { return smsServicesStatusNames[uint8(s)] }

func (s SMSServicesStatus) String() string { return enumString(uint8(s), smsServicesStatusNames) }

type csDomainIEs struct {
	lai                    **nas.LAI
	msIdentity             **MobileIdentity
	additionalUpdateResult **AdditionalUpdateResult
	smsServicesStatus      **SMSServicesStatus
}

func (m *AttachAccept) csDomainIEs() csDomainIEs {
	return csDomainIEs{&m.LAI, &m.MSIdentity, &m.AdditionalUpdateResult, &m.SMSServicesStatus}
}

func (m *TrackingAreaUpdateAccept) csDomainIEs() csDomainIEs {
	return csDomainIEs{&m.LAI, &m.MSIdentity, &m.AdditionalUpdateResult, &m.SMSServicesStatus}
}

func (c csDomainIEs) parse(iei uint8, value []byte) (bool, error) {
	switch iei {
	case ieiLocationAreaID:
		parsed, err := nas.ParseLAI(value)
		if err != nil {
			return false, err
		}

		*c.lai = &parsed
	case ieiMSIdentity:
		parsed, err := ParseMobileIdentity(value)
		if err != nil {
			return false, err
		}

		*c.msIdentity = &parsed
	case ieiAdditionalUpdateResult:
		v := tv1Value(value)
		if v == nil {
			return false, nil
		}

		r := AdditionalUpdateResult(*v & 0x03)
		*c.additionalUpdateResult = &r
	case ieiSMSServicesStatus:
		v := tv1Value(value)
		if v == nil {
			return false, nil
		}

		s := SMSServicesStatus(*v & 0x07)
		*c.smsServicesStatus = &s
	default:
		return false, nil
	}

	return true, nil
}

func (c csDomainIEs) appendLocation(o *nas.OptionalWriter) error {
	if lai := *c.lai; lai != nil {
		raw, err := lai.MarshalBinary()
		if err != nil {
			return err
		}

		o.TV3(ieiLocationAreaID, raw)
	}

	if id := *c.msIdentity; id != nil {
		raw, err := id.MarshalBinary()
		if err != nil {
			return err
		}

		o.TLV(ieiMSIdentity, raw)
	}

	return nil
}

func (c csDomainIEs) appendResult(o *nas.OptionalWriter) {
	if r := *c.additionalUpdateResult; r != nil {
		o.TV1(ieiAdditionalUpdateResult, uint8(*r)&0x03)
	}

	if s := *c.smsServicesStatus; s != nil {
		o.TV1(ieiSMSServicesStatus, uint8(*s)&0x07)
	}
}
