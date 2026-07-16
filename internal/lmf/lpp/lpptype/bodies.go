// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

// =====================================================================
// RequestCapabilities (TS 37.355 §6.3)
// =====================================================================

type RequestCapabilities struct {
	CriticalExtensions RequestCapabilitiesCriticalExtensions
}

type RequestCapabilitiesCriticalExtensions struct {
	Present                  int
	C1                       *RequestCapabilitiesCriticalExtensionsC1
	CriticalExtensionsFuture *struct{}
}

type RequestCapabilitiesCriticalExtensionsC1 struct {
	Present               int
	RequestCapabilitiesR9 *RequestCapabilitiesR9IEs
	Spare3                *struct{}
	Spare2                *struct{}
	Spare1                *struct{}
}

// RequestCapabilities-r9-IEs: extensible SEQUENCE with 5 root optional fields.
type RequestCapabilitiesR9IEs struct {
	CommonIEsRequestCapabilities *struct{}
	AGNSSRequestCapabilities     *AGNSSRequestCapabilities
	OTDOARequestCapabilities     *struct{}
	ECIDRequestCapabilities      *struct{}
	EPDURequestCapabilities      *struct{}
}

// =====================================================================
// ProvideCapabilities (TS 37.355 §6.3)
// =====================================================================

type ProvideCapabilities struct {
	CriticalExtensions ProvideCapabilitiesCriticalExtensions
}

type ProvideCapabilitiesCriticalExtensions struct {
	Present                  int
	C1                       *ProvideCapabilitiesCriticalExtensionsC1
	CriticalExtensionsFuture *struct{}
}

type ProvideCapabilitiesCriticalExtensionsC1 struct {
	Present               int
	ProvideCapabilitiesR9 *ProvideCapabilitiesR9IEs
	Spare3                *struct{}
	Spare2                *struct{}
	Spare1                *struct{}
}

// ProvideCapabilities-r9-IEs: extensible SEQUENCE with 5 root optional fields.
type ProvideCapabilitiesR9IEs struct {
	CommonIEsProvideCapabilities *struct{}
	AGNSSProvideCapabilities     *AGNSSProvideCapabilities
	OTDOAProvideCapabilities     *struct{}
	ECIDProvideCapabilities      *struct{}
	EPDUProvideCapabilities      *struct{}
}

// =====================================================================
// RequestAssistanceData (TS 37.355 §6.3)
// =====================================================================

type RequestAssistanceData struct {
	CriticalExtensions RequestAssistanceDataCriticalExtensions
}

type RequestAssistanceDataCriticalExtensions struct {
	Present                  int
	C1                       *RequestAssistanceDataCriticalExtensionsC1
	CriticalExtensionsFuture *struct{}
}

type RequestAssistanceDataCriticalExtensionsC1 struct {
	Present                 int
	RequestAssistanceDataR9 *RequestAssistanceDataR9IEs
	Spare3                  *struct{}
	Spare2                  *struct{}
	Spare1                  *struct{}
}

type RequestAssistanceDataR9IEs struct {
	CommonIEsRequestAssistanceData *struct{}
	AGNSSRequestAssistanceData     *struct{}
}

// =====================================================================
// ProvideAssistanceData (TS 37.355 §6.3)
// =====================================================================

type ProvideAssistanceData struct {
	CriticalExtensions ProvideAssistanceDataCriticalExtensions
}

type ProvideAssistanceDataCriticalExtensions struct {
	Present                  int
	C1                       *ProvideAssistanceDataCriticalExtensionsC1
	CriticalExtensionsFuture *struct{}
}

type ProvideAssistanceDataCriticalExtensionsC1 struct {
	Present                 int
	ProvideAssistanceDataR9 *ProvideAssistanceDataR9IEs
	Spare3                  *struct{}
	Spare2                  *struct{}
	Spare1                  *struct{}
}

type ProvideAssistanceDataR9IEs struct {
	CommonIEsProvideAssistanceData *struct{}
	AGNSSProvideAssistanceData     *AGNSSProvideAssistanceData
}

// =====================================================================
// RequestLocationInformation (TS 37.355 §6.3)
// =====================================================================

type RequestLocationInformation struct {
	CriticalExtensions RequestLocationInformationCriticalExtensions
}

type RequestLocationInformationCriticalExtensions struct {
	Present                  int
	C1                       *RequestLocationInformationCriticalExtensionsC1
	CriticalExtensionsFuture *struct{}
}

type RequestLocationInformationCriticalExtensionsC1 struct {
	Present                      int
	RequestLocationInformationR9 *RequestLocationInformationR9IEs
	Spare3                       *struct{}
	Spare2                       *struct{}
	Spare1                       *struct{}
}

// RequestLocationInformation-r9-IEs: extensible SEQUENCE.
type RequestLocationInformationR9IEs struct {
	CommonIEsRequestLocationInformation *CommonIEsRequestLocationInformation
	AGNSSRequestLocationInformation     *AGNSSRequestLocationInformation
}

// =====================================================================
// ProvideLocationInformation (TS 37.355 §6.3)
// =====================================================================

type ProvideLocationInformation struct {
	CriticalExtensions ProvideLocationInformationCriticalExtensions
}

type ProvideLocationInformationCriticalExtensions struct {
	Present                  int
	C1                       *ProvideLocationInformationCriticalExtensionsC1
	CriticalExtensionsFuture *struct{}
}

type ProvideLocationInformationCriticalExtensionsC1 struct {
	Present                      int
	ProvideLocationInformationR9 *ProvideLocationInformationR9IEs
	Spare3                       *struct{}
	Spare2                       *struct{}
	Spare1                       *struct{}
}

// ProvideLocationInformation-r9-IEs: extensible SEQUENCE.
type ProvideLocationInformationR9IEs struct {
	CommonIEsProvideLocationInformation *CommonIEsProvideLocationInformation
	AGNSSProvideLocationInformation     *AGNSSProvideLocationInformation
}
