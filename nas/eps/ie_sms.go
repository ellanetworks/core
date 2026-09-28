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

// NonEPSServices are the ATTACH ACCEPT and TRACKING AREA UPDATE ACCEPT elements
// that report the outcome for non-EPS services: the location area and MS
// identity of a combined procedure, the additional update result, and the SMS
// services status (TS 24.301 §8.2.1, §8.2.26, §5.5.1.3.4).
type NonEPSServices struct {
	LAI                    *nas.LAI
	MSIdentity             *MobileIdentity
	AdditionalUpdateResult *AdditionalUpdateResult
	SMSServicesStatus      *SMSServicesStatus
}

func (n *NonEPSServices) parse(iei uint8, value []byte) (bool, error) {
	switch iei {
	case ieiLocationAreaID:
		parsed, err := nas.ParseLAI(value)
		if err != nil {
			return false, err
		}

		n.LAI = &parsed
	case ieiMSIdentity:
		parsed, err := ParseMobileIdentity(value)
		if err != nil {
			return false, err
		}

		n.MSIdentity = &parsed
	case ieiAdditionalUpdateResult:
		v := tv1Value(value)
		if v == nil {
			return false, nil
		}

		r := AdditionalUpdateResult(*v & 0x03)
		n.AdditionalUpdateResult = &r
	case ieiSMSServicesStatus:
		v := tv1Value(value)
		if v == nil {
			return false, nil
		}

		s := SMSServicesStatus(*v & 0x07)
		n.SMSServicesStatus = &s
	default:
		return false, nil
	}

	return true, nil
}

func (n NonEPSServices) appendLocation(o *nas.OptionalWriter) error {
	if n.LAI != nil {
		raw, err := n.LAI.MarshalBinary()
		if err != nil {
			return err
		}

		o.TV3(ieiLocationAreaID, raw)
	}

	if n.MSIdentity != nil {
		raw, err := n.MSIdentity.MarshalBinary()
		if err != nil {
			return err
		}

		o.TLV(ieiMSIdentity, raw)
	}

	return nil
}

func (n NonEPSServices) appendResult(o *nas.OptionalWriter) {
	if n.AdditionalUpdateResult != nil {
		o.TV1(ieiAdditionalUpdateResult, uint8(*n.AdditionalUpdateResult)&0x03)
	}

	if n.SMSServicesStatus != nil {
		o.TV1(ieiSMSServicesStatus, uint8(*n.SMSServicesStatus)&0x07)
	}
}
