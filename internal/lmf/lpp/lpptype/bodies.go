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
	_                            [0]struct{}                   `per:"extseq"`
	CommonIEsRequestCapabilities *CommonIEsRequestCapabilities `per:",optional"`
	AGNSSRequestCapabilities     *AGNSSRequestCapabilities     `per:",optional"`
	OTDOARequestCapabilities     *OTDOARequestCapabilities     `per:",optional"`
	ECIDRequestCapabilities      *ECIDRequestCapabilities      `per:",optional"`
	EPDURequestCapabilities      *EPDUSequence                 `per:",optional"`
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
	_                            [0]struct{}                   `per:"extseq"`
	CommonIEsProvideCapabilities *CommonIEsProvideCapabilities `per:",optional"`
	AGNSSProvideCapabilities     *AGNSSProvideCapabilities     `per:",optional"`
	OTDOAProvideCapabilities     *OTDOAProvideCapabilities     `per:",optional"`
	ECIDProvideCapabilities      *ECIDProvideCapabilities      `per:",optional"`
	EPDUProvideCapabilities      *EPDUSequence                 `per:",optional"`
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

// RequestAssistanceData-r9-IEs: extensible SEQUENCE.
// TS 37.355 §6.3 line 1147-1156.
type RequestAssistanceDataR9IEs struct {
	_                              [0]struct{}                     `per:"extseq"`
	CommonIEsRequestAssistanceData *CommonIEsRequestAssistanceData `per:",optional"`
	AGNSSRequestAssistanceData     *AGNSSRequestAssistanceData     `per:",optional"`
	OTDOARequestAssistanceData     *OTDOARequestAssistanceData     `per:",optional"`
	EPDURequestAssistanceData      *EPDUSequence                   `per:",optional"`
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

// ProvideAssistanceData-r9-IEs: extensible SEQUENCE.
// TS 37.355 §6.3 line 2693-2707.
type ProvideAssistanceDataR9IEs struct {
	_                              [0]struct{}                     `per:"extseq"`
	CommonIEsProvideAssistanceData *CommonIEsProvideAssistanceData `per:",optional"`
	AGNSSProvideAssistanceData     *AGNSSProvideAssistanceData     `per:",optional"`
	OTDOAProvideAssistanceData     *OTDOAProvideAssistanceData     `per:",optional"`
	EPDUProvideAssistanceData      *EPDUSequence                   `per:",optional"`
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

// RequestLocationInformation-r9-IEs: extensible SEQUENCE with 5 root optional fields.
// TS 37.355 §6.3 line 2781-2793.
type RequestLocationInformationR9IEs struct {
	_                                   [0]struct{}                          `per:"extseq"`
	CommonIEsRequestLocationInformation *CommonIEsRequestLocationInformation `per:",optional"`
	AGNSSRequestLocationInformation     *AGNSSRequestLocationInformation     `per:",optional"`
	OTDOARequestLocationInformation     *OTDOARequestLocationInformation     `per:",optional"`
	ECIDRequestLocationInformation      *ECIDRequestLocationInformation      `per:",optional"`
	EPDURequestLocationInformation      *EPDUSequence                        `per:",optional"`
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

// ProvideLocationInformation-r9-IEs: extensible SEQUENCE.
// TS 37.355 §6.3 line 2935-2943.
type ProvideLocationInformationR9IEs struct {
	_                                   [0]struct{}                          `per:"extseq"`
	CommonIEsProvideLocationInformation *CommonIEsProvideLocationInformation `per:",optional"`
	AGNSSProvideLocationInformation     *AGNSSProvideLocationInformation     `per:",optional"`
	OTDOAProvideLocationInformation     *OTDOAProvideLocationInformation     `per:",optional"`
	ECIDProvideLocationInformation      *ECIDProvideLocationInformation      `per:",optional"`
	EPDUProvideLocationInformation      *EPDUSequence                        `per:",optional"`
}
