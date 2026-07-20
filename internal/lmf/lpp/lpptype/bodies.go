// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package lpptype

import "github.com/ellanetworks/core/internal/per"

// =====================================================================
// RequestCapabilities (TS 37.355 §6.3)
// =====================================================================

type RequestCapabilities struct {
	CriticalExtensions RequestCapabilitiesCriticalExtensions
}

type RequestCapabilitiesCriticalExtensions struct {
	C1                       *RequestCapabilitiesCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                `per:",choice:1,optional"`
}

type RequestCapabilitiesCriticalExtensionsC1 struct {
	RequestCapabilitiesR9 *RequestCapabilitiesR9IEs `per:",choice:0,optional"`
	Spare3                *per.Null                 `per:",choice:1,optional"`
	Spare2                *per.Null                 `per:",choice:2,optional"`
	Spare1                *per.Null                 `per:",choice:3,optional"`
}

// RequestCapabilities-r9-IEs: extensible SEQUENCE with 5 root optional fields.
type RequestCapabilitiesR9IEs struct {
	_                            [0]struct{}               `per:"extseq"`
	CommonIEsRequestCapabilities *per.Null                 `per:",optional"`
	AGNSSRequestCapabilities     *AGNSSRequestCapabilities `per:",optional"`
	OTDOARequestCapabilities     *per.Null                 `per:",optional"`
	ECIDRequestCapabilities      *per.Null                 `per:",optional"`
	EPDURequestCapabilities      *per.Null                 `per:",optional"`
}

// =====================================================================
// ProvideCapabilities (TS 37.355 §6.3)
// =====================================================================

type ProvideCapabilities struct {
	CriticalExtensions ProvideCapabilitiesCriticalExtensions
}

type ProvideCapabilitiesCriticalExtensions struct {
	C1                       *ProvideCapabilitiesCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                `per:",choice:1,optional"`
}

type ProvideCapabilitiesCriticalExtensionsC1 struct {
	ProvideCapabilitiesR9 *ProvideCapabilitiesR9IEs `per:",choice:0,optional"`
	Spare3                *per.Null                 `per:",choice:1,optional"`
	Spare2                *per.Null                 `per:",choice:2,optional"`
	Spare1                *per.Null                 `per:",choice:3,optional"`
}

// ProvideCapabilities-r9-IEs: extensible SEQUENCE with 5 root optional fields.
type ProvideCapabilitiesR9IEs struct {
	_                            [0]struct{}               `per:"extseq"`
	CommonIEsProvideCapabilities *per.Null                 `per:",optional"`
	AGNSSProvideCapabilities     *AGNSSProvideCapabilities `per:",optional"`
	OTDOAProvideCapabilities     *per.Null                 `per:",optional"`
	ECIDProvideCapabilities      *per.Null                 `per:",optional"`
	EPDUProvideCapabilities      *per.Null                 `per:",optional"`
}

// =====================================================================
// RequestAssistanceData (TS 37.355 §6.3)
// =====================================================================

type RequestAssistanceData struct {
	CriticalExtensions RequestAssistanceDataCriticalExtensions
}

type RequestAssistanceDataCriticalExtensions struct {
	C1                       *RequestAssistanceDataCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                  `per:",choice:1,optional"`
}

type RequestAssistanceDataCriticalExtensionsC1 struct {
	RequestAssistanceDataR9 *RequestAssistanceDataR9IEs `per:",choice:0,optional"`
	Spare3                  *per.Null                   `per:",choice:1,optional"`
	Spare2                  *per.Null                   `per:",choice:2,optional"`
	Spare1                  *per.Null                   `per:",choice:3,optional"`
}

// RequestAssistanceData-r9-IEs: extensible SEQUENCE with 4 root optional fields
// (no ECID). OTDOA/EPDU are unused but must be modelled so the optional-field
// bitmap has the spec-mandated width (TS 37.355).
type RequestAssistanceDataR9IEs struct {
	_                              [0]struct{} `per:"extseq"`
	CommonIEsRequestAssistanceData *per.Null   `per:",optional"`
	AGNSSRequestAssistanceData     *per.Null   `per:",optional"`
	OTDOARequestAssistanceData     *per.Null   `per:",optional"`
	EPDURequestAssistanceData      *per.Null   `per:",optional"`
}

// =====================================================================
// ProvideAssistanceData (TS 37.355 §6.3)
// =====================================================================

type ProvideAssistanceData struct {
	CriticalExtensions ProvideAssistanceDataCriticalExtensions
}

type ProvideAssistanceDataCriticalExtensions struct {
	C1                       *ProvideAssistanceDataCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                  `per:",choice:1,optional"`
}

type ProvideAssistanceDataCriticalExtensionsC1 struct {
	ProvideAssistanceDataR9 *ProvideAssistanceDataR9IEs `per:",choice:0,optional"`
	Spare3                  *per.Null                   `per:",choice:1,optional"`
	Spare2                  *per.Null                   `per:",choice:2,optional"`
	Spare1                  *per.Null                   `per:",choice:3,optional"`
}

// ProvideAssistanceData-r9-IEs: extensible SEQUENCE with 4 root optional fields
// (no ECID). OTDOA/EPDU are unused but must be modelled so the optional-field
// bitmap has the spec-mandated width (TS 37.355).
type ProvideAssistanceDataR9IEs struct {
	_                              [0]struct{}                 `per:"extseq"`
	CommonIEsProvideAssistanceData *per.Null                   `per:",optional"`
	AGNSSProvideAssistanceData     *AGNSSProvideAssistanceData `per:",optional"`
	OTDOAProvideAssistanceData     *per.Null                   `per:",optional"`
	EPDUProvideAssistanceData      *per.Null                   `per:",optional"`
}

// =====================================================================
// RequestLocationInformation (TS 37.355 §6.3)
// =====================================================================

type RequestLocationInformation struct {
	CriticalExtensions RequestLocationInformationCriticalExtensions
}

type RequestLocationInformationCriticalExtensions struct {
	C1                       *RequestLocationInformationCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                       `per:",choice:1,optional"`
}

type RequestLocationInformationCriticalExtensionsC1 struct {
	RequestLocationInformationR9 *RequestLocationInformationR9IEs `per:",choice:0,optional"`
	Spare3                       *per.Null                        `per:",choice:1,optional"`
	Spare2                       *per.Null                        `per:",choice:2,optional"`
	Spare1                       *per.Null                        `per:",choice:3,optional"`
}

// RequestLocationInformation-r9-IEs: extensible SEQUENCE with 5 root optional
// fields. OTDOA/ECID/EPDU are unused but must be modelled so the optional-field
// bitmap has the spec-mandated width (TS 37.355).
type RequestLocationInformationR9IEs struct {
	_                                   [0]struct{}                          `per:"extseq"`
	CommonIEsRequestLocationInformation *CommonIEsRequestLocationInformation `per:",optional"`
	AGNSSRequestLocationInformation     *AGNSSRequestLocationInformation     `per:",optional"`
	OTDOARequestLocationInformation     *per.Null                            `per:",optional"`
	ECIDRequestLocationInformation      *per.Null                            `per:",optional"`
	EPDURequestLocationInformation      *per.Null                            `per:",optional"`
}

// =====================================================================
// ProvideLocationInformation (TS 37.355 §6.3)
// =====================================================================

type ProvideLocationInformation struct {
	CriticalExtensions ProvideLocationInformationCriticalExtensions
}

type ProvideLocationInformationCriticalExtensions struct {
	C1                       *ProvideLocationInformationCriticalExtensionsC1 `per:",choice:0,optional"`
	CriticalExtensionsFuture *per.Null                                       `per:",choice:1,optional"`
}

type ProvideLocationInformationCriticalExtensionsC1 struct {
	ProvideLocationInformationR9 *ProvideLocationInformationR9IEs `per:",choice:0,optional"`
	Spare3                       *per.Null                        `per:",choice:1,optional"`
	Spare2                       *per.Null                        `per:",choice:2,optional"`
	Spare1                       *per.Null                        `per:",choice:3,optional"`
}

// ProvideLocationInformation-r9-IEs: extensible SEQUENCE with 5 root optional
// fields. OTDOA/ECID/EPDU are unused but must be modelled so the optional-field
// bitmap has the spec-mandated width (TS 37.355).
type ProvideLocationInformationR9IEs struct {
	_                                   [0]struct{}                          `per:"extseq"`
	CommonIEsProvideLocationInformation *CommonIEsProvideLocationInformation `per:",optional"`
	AGNSSProvideLocationInformation     *per.Null                            `per:",optional"`
	OTDOAProvideLocationInformation     *per.Null                            `per:",optional"`
	ECIDProvideLocationInformation      *per.Null                            `per:",optional"`
	EPDUProvideLocationInformation      *per.Null                            `per:",optional"`
}
