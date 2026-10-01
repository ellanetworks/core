// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package cx

import (
	"fmt"
	"strings"
)

type AuthorizationType uint32

const (
	AuthorizationRegistration                AuthorizationType = 0
	AuthorizationDeregistration              AuthorizationType = 1
	AuthorizationRegistrationAndCapabilities AuthorizationType = 2
)

func (t AuthorizationType) String() string {
	return enumName("AuthorizationType", uint32(t), "REGISTRATION", "DE_REGISTRATION", "REGISTRATION_AND_CAPABILITIES")
}

type AssignmentType uint32

const (
	AssignmentNoAssignment                     AssignmentType = 0
	AssignmentRegistration                     AssignmentType = 1
	AssignmentReRegistration                   AssignmentType = 2
	AssignmentUnregisteredUser                 AssignmentType = 3
	AssignmentTimeoutDeregistration            AssignmentType = 4
	AssignmentUserDeregistration               AssignmentType = 5
	AssignmentTimeoutDeregistrationStoreServer AssignmentType = 6
	AssignmentUserDeregistrationStoreServer    AssignmentType = 7
	AssignmentAdministrativeDeregistration     AssignmentType = 8
	AssignmentAuthenticationFailure            AssignmentType = 9
	AssignmentAuthenticationTimeout            AssignmentType = 10
	AssignmentDeregistrationTooMuchData        AssignmentType = 11
)

func (t AssignmentType) String() string {
	return enumName("AssignmentType", uint32(t),
		"NO_ASSIGNMENT", "REGISTRATION", "RE_REGISTRATION", "UNREGISTERED_USER", "TIMEOUT_DEREGISTRATION",
		"USER_DEREGISTRATION", "TIMEOUT_DEREGISTRATION_STORE_SERVER_NAME", "USER_DEREGISTRATION_STORE_SERVER_NAME",
		"ADMINISTRATIVE_DEREGISTRATION", "AUTHENTICATION_FAILURE", "AUTHENTICATION_TIMEOUT", "DEREGISTRATION_TOO_MUCH_DATA")
}

type ReasonCode uint32

const (
	ReasonPermanentTermination ReasonCode = 0
	ReasonNewServerAssigned    ReasonCode = 1
	ReasonServerChange         ReasonCode = 2
	ReasonRemoveSCSCF          ReasonCode = 3
)

func (c ReasonCode) String() string {
	return enumName("ReasonCode", uint32(c), "PERMANENT_TERMINATION", "NEW_SERVER_ASSIGNED", "SERVER_CHANGE", "REMOVE_S-CSCF")
}

type Features uint32

const (
	FeatureSharedIFCSets    Features = 1 << 0
	FeatureAliasIndication  Features = 1 << 1
	FeatureIMSRestoration   Features = 1 << 2
	FeaturePCSCFRestoration Features = 1 << 3
)

func (f Features) String() string {
	return bitNames(uint32(f), "SiFC", "AliasInd", "IMSRestorationInd", "P-CSCF-Restoration-mechanism")
}

type AuthenticationScheme string

const (
	SchemeDigestAKAv1MD5   AuthenticationScheme = "Digest-AKAv1-MD5"
	SchemeDigestAKAv2MD5   AuthenticationScheme = "Digest-AKAv2-MD5"
	SchemeSIPDigest        AuthenticationScheme = "SIP Digest"
	SchemeNASSBundled      AuthenticationScheme = "NASS-Bundled"
	SchemeEarlyIMSSecurity AuthenticationScheme = "Early-IMS-Security"
	SchemeUnknown          AuthenticationScheme = "Unknown"
)

func (s AuthenticationScheme) IsAKA() bool {
	const prefix = "Digest-AKA"

	return len(s) > len(prefix) && strings.EqualFold(string(s[:len(prefix)]), prefix)
}

type IdentityType uint8

const (
	IdentityDistinctPublicUserIdentity IdentityType = 0
	IdentityDistinctPSI                IdentityType = 1
	IdentityWildcardedPSI              IdentityType = 2
	IdentityNonDistinctIMPU            IdentityType = 3
	IdentityWildcardedIMPU             IdentityType = 4
)

func (t IdentityType) String() string {
	return enumName("IdentityType", uint32(t),
		"DISTINCT_PUBLIC_USER_IDENTITY", "DISTINCT_PSI", "WILDCARDED_PSI", "NON_DISTINCT_IMPU", "WILDCARDED_IMPU")
}

type ProfilePart uint8

const (
	ProfilePartRegistered   ProfilePart = 0
	ProfilePartUnregistered ProfilePart = 1
)

func (p ProfilePart) String() string {
	return enumName("ProfilePart", uint32(p), "REGISTERED", "UNREGISTERED")
}

type SessionCase uint8

const (
	SessionCaseOriginatingRegistered   SessionCase = 0
	SessionCaseTerminatingRegistered   SessionCase = 1
	SessionCaseTerminatingUnregistered SessionCase = 2
	SessionCaseOriginatingUnregistered SessionCase = 3
	SessionCaseOriginatingCDIV         SessionCase = 4
)

func (c SessionCase) String() string {
	return enumName("SessionCase", uint32(c),
		"ORIGINATING_REGISTERED", "TERMINATING_REGISTERED", "TERMINATING_UNREGISTERED", "ORIGINATING_UNREGISTERED", "ORIGINATING_CDIV")
}

type RegistrationType uint8

const (
	RegistrationTypeInitial        RegistrationType = 0
	RegistrationTypeReRegistration RegistrationType = 1
	RegistrationTypeDeregistration RegistrationType = 2
)

func (t RegistrationType) String() string {
	return enumName("RegistrationType", uint32(t), "INITIAL_REGISTRATION", "RE-REGISTRATION", "DE-REGISTRATION")
}

type DefaultHandling uint8

const (
	DefaultHandlingSessionContinued  DefaultHandling = 0
	DefaultHandlingSessionTerminated DefaultHandling = 1
)

func (h DefaultHandling) String() string {
	return enumName("DefaultHandling", uint32(h), "SESSION_CONTINUED", "SESSION_TERMINATED")
}

func enumName(typeName string, v uint32, names ...string) string {
	if int(v) < len(names) {
		return names[v]
	}

	return fmt.Sprintf("%s(%d)", typeName, v)
}

func bitNames(v uint32, names ...string) string {
	if v == 0 {
		return "0"
	}

	var parts []string

	for i, name := range names {
		if v&(1<<i) != 0 {
			parts = append(parts, name)
			v &^= 1 << i
		}
	}

	if v != 0 {
		parts = append(parts, fmt.Sprintf("%#x", v))
	}

	return strings.Join(parts, "|")
}
