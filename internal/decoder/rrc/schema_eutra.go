// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrc

var eutraTypes = map[string]node{}

func init() {
	eutraTypes["AccessStratumRelease"] = enumerated{values: []string{"rel8", "rel9", "rel10", "rel11", "rel12", "rel13", "rel14", "rel15"}, extValues: []string{"rel16", "rel17", "rel18"}, extensible: true}

	eutraTypes["FreqBandIndicator"] = integer{lb: 1, ub: 64, extensible: false}

	eutraTypes["PDCP-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "supportedROHC-Profiles", typ: deferred{name: "ROHC-ProfileSupportList-r15", reg: eutraTypes}},
			{name: "maxNumberROHC-ContextSessions", typ: enumerated{values: []string{"cs2", "cs4", "cs8", "cs12", "cs16", "cs24", "cs32", "cs48", "cs64", "cs128", "cs256", "cs512", "cs1024", "cs16384", "spare2", "spare1"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-TxAntennaSelectionSupported", typ: boolean{}},
			{name: "ue-SpecificRefSigsSupported", typ: boolean{}},
		},
	}

	eutraTypes["RAT-Type"] = enumerated{values: []string{"eutra", "utra", "geran-cs", "geran-ps", "cdma2000-1XRTT", "nr", "eutra-nr", "spare1"}, extensible: true}

	eutraTypes["RF-Parameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListEUTRA", typ: deferred{name: "SupportedBandListEUTRA", reg: eutraTypes}},
		},
	}

	eutraTypes["ROHC-ProfileSupportList-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "profile0x0001-r15", typ: boolean{}},
			{name: "profile0x0002-r15", typ: boolean{}},
			{name: "profile0x0003-r15", typ: boolean{}},
			{name: "profile0x0004-r15", typ: boolean{}},
			{name: "profile0x0006-r15", typ: boolean{}},
			{name: "profile0x0101-r15", typ: boolean{}},
			{name: "profile0x0102-r15", typ: boolean{}},
			{name: "profile0x0103-r15", typ: boolean{}},
			{name: "profile0x0104-r15", typ: boolean{}},
		},
	}

	eutraTypes["RRC-TransactionIdentifier"] = integer{lb: 0, ub: 3, extensible: false}

	eutraTypes["SupportedBandEUTRA"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandEUTRA", typ: deferred{name: "FreqBandIndicator", reg: eutraTypes}},
			{name: "halfDuplex", typ: boolean{}},
		},
	}

	eutraTypes["SupportedBandListEUTRA"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandEUTRA", reg: eutraTypes}}

	eutraTypes["UE-CapabilityRAT-Container"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rat-Type", typ: deferred{name: "RAT-Type", reg: eutraTypes}},
			{name: "ueCapabilityRAT-Container", typ: octetString{hasUB: false}},
		},
	}

	eutraTypes["UE-CapabilityRAT-ContainerList"] = sequenceOf{lb: 0, ub: 8, elem: deferred{name: "UE-CapabilityRAT-Container", reg: eutraTypes}}

	eutraTypes["UE-EUTRA-Capability"] = sequence{
		extensible: false,
		stopAfter:  "rf-Parameters",
		fields: []field{
			{name: "accessStratumRelease", typ: deferred{name: "AccessStratumRelease", reg: eutraTypes}},
			{name: "ue-Category", typ: integer{lb: 1, ub: 5, extensible: false}},
			{name: "pdcp-Parameters", typ: deferred{name: "PDCP-Parameters", reg: eutraTypes}},
			{name: "phyLayerParameters", typ: deferred{name: "PhyLayerParameters", reg: eutraTypes}},
			{name: "rf-Parameters", typ: deferred{name: "RF-Parameters", reg: eutraTypes}},
			{name: "measParameters", typ: unsupported{name: "measParameters"}},
			{name: "featureGroupIndicators", typ: unsupported{name: "featureGroupIndicators"}, optional: true},
			{name: "interRAT-Parameters", typ: unsupported{name: "interRAT-Parameters"}},
			{name: "nonCriticalExtension", typ: unsupported{name: "nonCriticalExtension"}, optional: true},
		},
	}

	eutraTypes["UECapabilityInformation"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rrc-TransactionIdentifier", typ: deferred{name: "RRC-TransactionIdentifier", reg: eutraTypes}},
			{name: "criticalExtensions", typ: choice{
				extensible: false,
				alternatives: []field{
					{name: "c1", typ: choice{
						extensible: false,
						alternatives: []field{
							{name: "ueCapabilityInformation-r8", typ: deferred{name: "UECapabilityInformation-r8-IEs", reg: eutraTypes}},
							{name: "spare7", typ: null{}},
							{name: "spare6", typ: null{}},
							{name: "spare5", typ: null{}},
							{name: "spare4", typ: null{}},
							{name: "spare3", typ: null{}},
							{name: "spare2", typ: null{}},
							{name: "spare1", typ: null{}},
						},
					}},
					{name: "criticalExtensionsFuture", typ: sequence{
						extensible: false,
						fields:     []field{},
					}},
				},
			}},
		},
	}

	eutraTypes["UECapabilityInformation-r8-IEs"] = sequence{
		extensible: false,
		stopAfter:  "ue-CapabilityRAT-ContainerList",
		fields: []field{
			{name: "ue-CapabilityRAT-ContainerList", typ: deferred{name: "UE-CapabilityRAT-ContainerList", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: unsupported{name: "nonCriticalExtension"}, optional: true},
		},
	}

	eutraTypes["UERadioAccessCapabilityInformation"] = sequence{
		extensible: false,
		fields: []field{
			{name: "criticalExtensions", typ: choice{
				extensible: false,
				alternatives: []field{
					{name: "c1", typ: choice{
						extensible: false,
						alternatives: []field{
							{name: "ueRadioAccessCapabilityInformation-r8", typ: deferred{name: "UERadioAccessCapabilityInformation-r8-IEs", reg: eutraTypes}},
							{name: "spare7", typ: null{}},
							{name: "spare6", typ: null{}},
							{name: "spare5", typ: null{}},
							{name: "spare4", typ: null{}},
							{name: "spare3", typ: null{}},
							{name: "spare2", typ: null{}},
							{name: "spare1", typ: null{}},
						},
					}},
					{name: "criticalExtensionsFuture", typ: sequence{
						extensible: false,
						fields:     []field{},
					}},
				},
			}},
		},
	}

	eutraTypes["UERadioAccessCapabilityInformation-r8-IEs"] = sequence{
		extensible: false,
		stopAfter:  "ue-RadioAccessCapabilityInfo",
		fields: []field{
			{name: "ue-RadioAccessCapabilityInfo", typ: octetString{hasUB: false}},
			{name: "nonCriticalExtension", typ: unsupported{name: "nonCriticalExtension"}, optional: true},
		},
	}
}
