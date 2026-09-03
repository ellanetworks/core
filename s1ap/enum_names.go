// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

// ASN.1 enumeration item names from TS 36.413 §9.3.4 and §9.3.5. Each Name
// returns the empty string for a value the module does not name.

var criticalityNames = map[Criticality]string{
	CriticalityReject: "reject",
	CriticalityIgnore: "ignore",
	CriticalityNotify: "notify",
}

func (c Criticality) Name() string { return criticalityNames[c] }

var triggeringMessageNames = map[TriggeringMessage]string{
	TriggeringInitiatingMessage: "initiating-message",
	TriggeringSuccessfulOutcome: "successful-outcome",
	// TS 36.413 §9.3.5 misspells this ASN.1 identifier; the label reads as intended.
	TriggeringUnsuccessfulOutcome: "unsuccessful-outcome",
}

func (t TriggeringMessage) Name() string { return triggeringMessageNames[t] }

var typeOfErrorNames = map[TypeOfError]string{
	TypeOfErrorNotUnderstood: "not-understood",
	TypeOfErrorMissing:       "missing",
}

func (t TypeOfError) Name() string { return typeOfErrorNames[t] }

// Name returns the name of the Cause CHOICE alternative the group selects.
func (g CauseGroup) Name() string { return causeGroupNames[g] }

var pagingDRXNames = map[PagingDRX]string{
	PagingDRXv32:  "v32",
	PagingDRXv64:  "v64",
	PagingDRXv128: "v128",
	PagingDRXv256: "v256",
}

func (d PagingDRX) Name() string { return pagingDRXNames[d] }

var timeToWaitNames = map[TimeToWait]string{
	TimeToWaitV1s:  "v1s",
	TimeToWaitV2s:  "v2s",
	TimeToWaitV5s:  "v5s",
	TimeToWaitV10s: "v10s",
	TimeToWaitV20s: "v20s",
	TimeToWaitV60s: "v60s",
}

func (t TimeToWait) Name() string { return timeToWaitNames[t] }

var rrcEstablishmentCauseNames = map[RRCEstablishmentCause]string{
	RRCCauseEmergency:          "emergency",
	RRCCauseHighPriorityAccess: "highPriorityAccess",
	RRCCauseMTAccess:           "mt-Access",
	RRCCauseMOSignalling:       "mo-Signalling",
	RRCCauseMOData:             "mo-Data",
}

func (c RRCEstablishmentCause) Name() string { return rrcEstablishmentCauseNames[c] }

var handoverTypeNames = map[HandoverType]string{
	HandoverTypeIntraLTE:    "intralte",
	HandoverTypeLTEtoUTRAN:  "ltetoutran",
	HandoverTypeLTEtoGERAN:  "ltetogeran",
	HandoverTypeUTRANtoLTE:  "utrantolte",
	HandoverTypeGERANtoLTE:  "gerantolte",
	HandoverTypeEPSToFiveGS: "eps-to-5gs",
	HandoverTypeFiveGSToEPS: "fivegs-to-eps",
}

func (t HandoverType) Name() string { return handoverTypeNames[t] }

var cnDomainNames = map[CNDomain]string{
	CNDomainPS: "ps",
	CNDomainCS: "cs",
}

func (d CNDomain) Name() string { return cnDomainNames[d] }

var pagingPriorityNames = map[PagingPriority]string{
	PagingPriorityLevel1: "priolevel1",
	PagingPriorityLevel2: "priolevel2",
	PagingPriorityLevel3: "priolevel3",
	PagingPriorityLevel4: "priolevel4",
	PagingPriorityLevel5: "priolevel5",
	PagingPriorityLevel6: "priolevel6",
	PagingPriorityLevel7: "priolevel7",
	PagingPriorityLevel8: "priolevel8",
}

func (p PagingPriority) Name() string { return pagingPriorityNames[p] }

var directForwardingPathAvailabilityNames = map[DirectForwardingPathAvailability]string{
	DirectForwardingPathAvailable: "directPathAvailable",
}

func (d DirectForwardingPathAvailability) Name() string {
	return directForwardingPathAvailabilityNames[d]
}

var ueRetentionInformationNames = map[UERetentionInformation]string{
	UERetentionUesRetained: "ues-retained",
}

func (u UERetentionInformation) Name() string { return ueRetentionInformationNames[u] }

var forbiddenInterRATsNames = map[ForbiddenInterRATs]string{
	ForbiddenInterRATsAll:      "all",
	ForbiddenInterRATsGERAN:    "geran",
	ForbiddenInterRATsUTRAN:    "utran",
	ForbiddenInterRATsCDMA2000: "cdma2000",
}

func (f ForbiddenInterRATs) Name() string { return forbiddenInterRATsNames[f] }

var enbIDKindNames = map[ENBIDKind]string{
	ENBIDMacro:      "macroENB-ID",
	ENBIDHome:       "homeENB-ID",
	ENBIDShortMacro: "short-macroENB-ID",
	ENBIDLongMacro:  "long-macroENB-ID",
}

// Name returns the name of the ENB-ID CHOICE alternative the kind selects.
func (k ENBIDKind) Name() string { return enbIDKindNames[k] }

var eventTypeNames = map[EventType]string{
	EventTypeDirect:                "direct",
	EventTypeChangeOfServeCell:     "change-of-serve-cell",
	EventTypeStopChangeOfServeCell: "stop-change-of-serve-cell",
}

func (e EventType) Name() string { return eventTypeNames[e] }

var reportAreaNames = map[ReportArea]string{
	ReportAreaECGI: "ecgi",
}

func (r ReportArea) Name() string { return reportAreaNames[r] }
