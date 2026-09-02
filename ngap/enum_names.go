// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

// ASN.1 enumeration item names from TS 38.413 §9.4.5 and §9.4.6. Each Name
// returns the empty string for a value the module does not name.

var criticalityNames = map[Criticality]string{
	CriticalityReject: "reject",
	CriticalityIgnore: "ignore",
	CriticalityNotify: "notify",
}

func (c Criticality) Name() string { return criticalityNames[c] }

var triggeringMessageNames = map[TriggeringMessage]string{
	TriggeringInitiatingMessage:   "initiating-message",
	TriggeringSuccessfulOutcome:   "successful-outcome",
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
	RRCCauseMOVoiceCall:        "mo-VoiceCall",
	RRCCauseMOVideoCall:        "mo-VideoCall",
	RRCCauseMOSMS:              "mo-SMS",
	RRCCauseMPSPriorityAccess:  "mps-PriorityAccess",
	RRCCauseMCSPriorityAccess:  "mcs-PriorityAccess",
}

func (c RRCEstablishmentCause) Name() string { return rrcEstablishmentCauseNames[c] }

var ueContextRequestNames = map[UEContextRequest]string{
	UEContextRequested: "requested",
}

func (u UEContextRequest) Name() string { return ueContextRequestNames[u] }

var ueRetentionInformationNames = map[UERetentionInformation]string{
	UERetentionUesRetained: "ues-retained",
}

func (u UERetentionInformation) Name() string { return ueRetentionInformationNames[u] }

var pagingOriginNames = map[PagingOrigin]string{
	PagingOriginNon3GPP: "non-3gpp",
}

func (o PagingOrigin) Name() string { return pagingOriginNames[o] }

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

var pduSessionTypeNames = map[PDUSessionType]string{
	PDUSessionTypeIPv4:         "ipv4",
	PDUSessionTypeIPv6:         "ipv6",
	PDUSessionTypeIPv4v6:       "ipv4v6",
	PDUSessionTypeEthernet:     "ethernet",
	PDUSessionTypeUnstructured: "unstructured",
}

func (t PDUSessionType) Name() string { return pduSessionTypeNames[t] }

var integrityProtectionResultNames = map[IntegrityProtectionResult]string{
	IntegrityProtectionPerformed:    "performed",
	IntegrityProtectionNotPerformed: "not-performed",
}

func (i IntegrityProtectionResult) Name() string { return integrityProtectionResultNames[i] }

var confidentialityProtectionResultNames = map[ConfidentialityProtectionResult]string{
	ConfidentialityProtectionPerformed:    "performed",
	ConfidentialityProtectionNotPerformed: "not-performed",
}

func (c ConfidentialityProtectionResult) Name() string {
	return confidentialityProtectionResultNames[c]
}
