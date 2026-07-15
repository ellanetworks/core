// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

// =====================================================================
// RequestCapabilities (TS 37.355 §6.3)
// =====================================================================

type RequestCapabilities struct {
	CriticalExtensions RequestCapabilitiesCriticalExtensions `aper:"valueLB:0,valueUB:1"`
}

type RequestCapabilitiesCriticalExtensions struct {
	Present                  int
	C1                       *RequestCapabilitiesCriticalExtensionsC1 `aper:"valueLB:0,valueUB:3"`
	CriticalExtensionsFuture *struct{}
}

type RequestCapabilitiesCriticalExtensionsC1 struct {
	Present               int
	RequestCapabilitiesR9 *RequestCapabilitiesR9IEs `aper:"valueExt"`
	Spare3                *struct{}
	Spare2                *struct{}
	Spare1                *struct{}
}

// RequestCapabilities-r9-IEs: extensible SEQUENCE with 5 root optional fields.
type RequestCapabilitiesR9IEs struct {
	CommonIEsRequestCapabilities *struct{}                 `aper:"optional"`
	AGNSSRequestCapabilities     *AGNSSRequestCapabilities `aper:"optional,valueExt"`
	OTDOARequestCapabilities     *struct{}                 `aper:"optional"`
	ECIDRequestCapabilities      *struct{}                 `aper:"optional"`
	EPDURequestCapabilities      *struct{}                 `aper:"optional"`
}

// =====================================================================
// ProvideCapabilities (TS 37.355 §6.3)
// =====================================================================

type ProvideCapabilities struct {
	CriticalExtensions ProvideCapabilitiesCriticalExtensions `aper:"valueLB:0,valueUB:1"`
}

type ProvideCapabilitiesCriticalExtensions struct {
	Present                  int
	C1                       *ProvideCapabilitiesCriticalExtensionsC1 `aper:"valueLB:0,valueUB:3"`
	CriticalExtensionsFuture *struct{}
}

type ProvideCapabilitiesCriticalExtensionsC1 struct {
	Present               int
	ProvideCapabilitiesR9 *ProvideCapabilitiesR9IEs `aper:"valueExt"`
	Spare3                *struct{}
	Spare2                *struct{}
	Spare1                *struct{}
}

// ProvideCapabilities-r9-IEs: extensible SEQUENCE with 5 root optional fields.
type ProvideCapabilitiesR9IEs struct {
	CommonIEsProvideCapabilities *struct{}                 `aper:"optional"`
	AGNSSProvideCapabilities     *AGNSSProvideCapabilities `aper:"optional,valueExt"`
	OTDOAProvideCapabilities     *struct{}                 `aper:"optional"`
	ECIDProvideCapabilities      *struct{}                 `aper:"optional"`
	EPDUProvideCapabilities      *struct{}                 `aper:"optional"`
}

// =====================================================================
// RequestAssistanceData (TS 37.355 §6.3)
// =====================================================================

type RequestAssistanceData struct {
	CriticalExtensions RequestAssistanceDataCriticalExtensions `aper:"valueLB:0,valueUB:1"`
}

type RequestAssistanceDataCriticalExtensions struct {
	Present                  int
	C1                       *RequestAssistanceDataCriticalExtensionsC1 `aper:"valueLB:0,valueUB:3"`
	CriticalExtensionsFuture *struct{}
}

type RequestAssistanceDataCriticalExtensionsC1 struct {
	Present                 int
	RequestAssistanceDataR9 *RequestAssistanceDataR9IEs `aper:"valueExt"`
	Spare3                  *struct{}
	Spare2                  *struct{}
	Spare1                  *struct{}
}

type RequestAssistanceDataR9IEs struct {
	CommonIEsRequestAssistanceData *struct{} `aper:"optional"`
	AGNSSRequestAssistanceData     *struct{} `aper:"optional"`
}

// =====================================================================
// ProvideAssistanceData (TS 37.355 §6.3)
// =====================================================================

type ProvideAssistanceData struct {
	CriticalExtensions ProvideAssistanceDataCriticalExtensions `aper:"valueLB:0,valueUB:1"`
}

type ProvideAssistanceDataCriticalExtensions struct {
	Present                  int
	C1                       *ProvideAssistanceDataCriticalExtensionsC1 `aper:"valueLB:0,valueUB:3"`
	CriticalExtensionsFuture *struct{}
}

type ProvideAssistanceDataCriticalExtensionsC1 struct {
	Present                 int
	ProvideAssistanceDataR9 *ProvideAssistanceDataR9IEs `aper:"valueExt"`
	Spare3                  *struct{}
	Spare2                  *struct{}
	Spare1                  *struct{}
}

type ProvideAssistanceDataR9IEs struct {
	CommonIEsProvideAssistanceData *struct{}                   `aper:"optional"`
	AGNSSProvideAssistanceData     *AGNSSProvideAssistanceData `aper:"optional,valueExt"`
}

// =====================================================================
// RequestLocationInformation (TS 37.355 §6.3)
// =====================================================================

type RequestLocationInformation struct {
	CriticalExtensions RequestLocationInformationCriticalExtensions `aper:"valueLB:0,valueUB:1"`
}

type RequestLocationInformationCriticalExtensions struct {
	Present                  int
	C1                       *RequestLocationInformationCriticalExtensionsC1 `aper:"valueLB:0,valueUB:3"`
	CriticalExtensionsFuture *struct{}
}

type RequestLocationInformationCriticalExtensionsC1 struct {
	Present                      int
	RequestLocationInformationR9 *RequestLocationInformationR9IEs `aper:"valueExt"`
	Spare3                       *struct{}
	Spare2                       *struct{}
	Spare1                       *struct{}
}

// RequestLocationInformation-r9-IEs: extensible SEQUENCE.
type RequestLocationInformationR9IEs struct {
	CommonIEsRequestLocationInformation *CommonIEsRequestLocationInformation `aper:"optional,valueExt"`
	AGNSSRequestLocationInformation     *AGNSSRequestLocationInformation     `aper:"optional,valueExt"`
}

// =====================================================================
// ProvideLocationInformation (TS 37.355 §6.3)
// =====================================================================

type ProvideLocationInformation struct {
	CriticalExtensions ProvideLocationInformationCriticalExtensions `aper:"valueLB:0,valueUB:1"`
}

type ProvideLocationInformationCriticalExtensions struct {
	Present                  int
	C1                       *ProvideLocationInformationCriticalExtensionsC1 `aper:"valueLB:0,valueUB:3"`
	CriticalExtensionsFuture *struct{}
}

type ProvideLocationInformationCriticalExtensionsC1 struct {
	Present                      int
	ProvideLocationInformationR9 *ProvideLocationInformationR9IEs `aper:"valueExt"`
	Spare3                       *struct{}
	Spare2                       *struct{}
	Spare1                       *struct{}
}

// ProvideLocationInformation-r9-IEs: extensible SEQUENCE.
type ProvideLocationInformationR9IEs struct {
	CommonIEsProvideLocationInformation *CommonIEsProvideLocationInformation `aper:"optional,valueExt"`
	AGNSSProvideLocationInformation     *AGNSSProvideLocationInformation     `aper:"optional"`
}
