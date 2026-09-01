// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrc

var eutraTypes = map[string]node{}

func init() {
	eutraTypes["AccessStratumRelease"] = enumerated{values: []string{"rel8", "rel9", "rel10", "rel11", "rel12", "rel13", "rel14", "rel15"}, extValues: []string{"rel16", "rel17", "rel18"}, extensible: true}

	eutraTypes["BandCombination-r14"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandIndication-r14", reg: eutraTypes}}

	eutraTypes["BandCombinationList-r14"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "BandCombination-r14", reg: eutraTypes}}

	eutraTypes["BandCombinationListEUTRA-r10"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandInfoEUTRA", reg: eutraTypes}}

	eutraTypes["BandCombinationParameters-r10"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-r10", reg: eutraTypes}}

	eutraTypes["BandCombinationParameters-r11"] = sequence{
		extensible: true,
		fields: []field{
			{name: "bandParameterList-r11", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-r11", reg: eutraTypes}}},
			{name: "supportedBandwidthCombinationSet-r11", typ: deferred{name: "SupportedBandwidthCombinationSet-r10", reg: eutraTypes}, optional: true},
			{name: "multipleTimingAdvance-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simultaneousRx-Tx-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "bandInfoEUTRA-r11", typ: deferred{name: "BandInfoEUTRA", reg: eutraTypes}},
		},
	}

	eutraTypes["BandCombinationParameters-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "differentFallbackSupported-r13", typ: enumerated{values: []string{"true"}, extensible: false}, optional: true},
			{name: "bandParameterList-r13", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-r13", reg: eutraTypes}}},
			{name: "supportedBandwidthCombinationSet-r13", typ: deferred{name: "SupportedBandwidthCombinationSet-r10", reg: eutraTypes}, optional: true},
			{name: "multipleTimingAdvance-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simultaneousRx-Tx-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "bandInfoEUTRA-r13", typ: deferred{name: "BandInfoEUTRA", reg: eutraTypes}},
			{name: "dc-Support-r13", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "asynchronous-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "supportedCellGrouping-r13", typ: choice{
						extensible: false,
						alternatives: []field{
							{name: "threeEntries-r13", typ: bitString{lb: 3, ub: 3, extensible: false}},
							{name: "fourEntries-r13", typ: bitString{lb: 7, ub: 7, extensible: false}},
							{name: "fiveEntries-r13", typ: bitString{lb: 15, ub: 15, extensible: false}},
						},
					}, optional: true},
				},
			}, optional: true},
			{name: "supportedNAICS-2CRS-AP-r13", typ: bitString{lb: 1, ub: 8, extensible: false}, optional: true},
			{name: "commSupportedBandsPerBC-r13", typ: bitString{lb: 1, ub: 64, extensible: false}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1090"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-v1090", reg: eutraTypes}}

	eutraTypes["BandCombinationParameters-v1130"] = sequence{
		extensible: true,
		fields: []field{
			{name: "multipleTimingAdvance-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simultaneousRx-Tx-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "bandParameterList-r11", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-v1130", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1250"] = sequence{
		extensible: true,
		fields: []field{
			{name: "dc-Support-r12", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "asynchronous-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "supportedCellGrouping-r12", typ: choice{
						extensible: false,
						alternatives: []field{
							{name: "threeEntries-r12", typ: bitString{lb: 3, ub: 3, extensible: false}},
							{name: "fourEntries-r12", typ: bitString{lb: 7, ub: 7, extensible: false}},
							{name: "fiveEntries-r12", typ: bitString{lb: 15, ub: 15, extensible: false}},
						},
					}, optional: true},
				},
			}, optional: true},
			{name: "supportedNAICS-2CRS-AP-r12", typ: bitString{lb: 1, ub: 8, extensible: false}, optional: true},
			{name: "commSupportedBandsPerBC-r12", typ: bitString{lb: 1, ub: 64, extensible: false}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1270"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandParameterList-v1270", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-v1270", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1320"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandParameterList-v1320", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-v1320", reg: eutraTypes}}, optional: true},
			{name: "additionalRx-Tx-PerformanceReq-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandParameterList-v1430", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-v1430", reg: eutraTypes}}, optional: true},
			{name: "v2x-SupportedTxBandCombListPerBC-r14", typ: bitString{lb: 1, ub: 384, extensible: false}, optional: true},
			{name: "v2x-SupportedRxBandCombListPerBC-r14", typ: bitString{lb: 1, ub: 384, extensible: false}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1450"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandParameterList-v1450", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-v1450", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandParameterList-v1530", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-v1530", reg: eutraTypes}}, optional: true},
			{name: "spt-Parameters-r15", typ: deferred{name: "SPT-Parameters-r15", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measGapInfoNR-r16", typ: deferred{name: "MeasGapInfoNR-r16", reg: eutraTypes}, optional: true},
			{name: "bandParameterList-v1610", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandParameters-v1610", reg: eutraTypes}}, optional: true},
			{name: "interFreqDAPS-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "interFreqAsyncDAPS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "interFreqMultiUL-TransmissionDAPS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1630"] = sequence{
		extensible: false,
		fields: []field{
			{name: "v2x-SupportedTxBandCombListPerBC-v1630", typ: bitString{lb: 1, ub: 512, extensible: false}, optional: true},
			{name: "v2x-SupportedRxBandCombListPerBC-v1630", typ: bitString{lb: 1, ub: 512, extensible: false}, optional: true},
			{name: "scalingFactorTxSidelink-r16", typ: sequenceOf{lb: 1, ub: 512, elem: deferred{name: "ScalingFactorSidelink-r16", reg: eutraTypes}}, optional: true},
			{name: "scalingFactorRxSidelink-r16", typ: sequenceOf{lb: 1, ub: 512, elem: deferred{name: "ScalingFactorSidelink-r16", reg: eutraTypes}}, optional: true},
			{name: "interBandPowerSharingSyncDAPS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "interBandPowerSharingAsyncDAPS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["BandCombinationParameters-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measGapInfoNR-r18", typ: deferred{name: "MeasGapInfoNR-r18", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["BandCombinationParametersExt-r10"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandwidthCombinationSet-r10", typ: deferred{name: "SupportedBandwidthCombinationSet-r10", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["BandIndication-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandEUTRA-r14", typ: deferred{name: "FreqBandIndicator-r11", reg: eutraTypes}},
			{name: "ca-BandwidthClassDL-r14", typ: deferred{name: "CA-BandwidthClass-r10", reg: eutraTypes}},
			{name: "ca-BandwidthClassUL-r14", typ: deferred{name: "CA-BandwidthClass-r10", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["BandInfoEUTRA"] = sequence{
		extensible: false,
		fields: []field{
			{name: "interFreqBandList", typ: deferred{name: "InterFreqBandList", reg: eutraTypes}},
			{name: "interRAT-BandList", typ: deferred{name: "InterRAT-BandList", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["BandListEUTRA"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "BandInfoEUTRA", reg: eutraTypes}}

	eutraTypes["BandParameters-r10"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandEUTRA-r10", typ: deferred{name: "FreqBandIndicator", reg: eutraTypes}},
			{name: "bandParametersUL-r10", typ: deferred{name: "BandParametersUL-r10", reg: eutraTypes}, optional: true},
			{name: "bandParametersDL-r10", typ: deferred{name: "BandParametersDL-r10", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["BandParameters-r11"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandEUTRA-r11", typ: deferred{name: "FreqBandIndicator-r11", reg: eutraTypes}},
			{name: "bandParametersUL-r11", typ: deferred{name: "BandParametersUL-r10", reg: eutraTypes}, optional: true},
			{name: "bandParametersDL-r11", typ: deferred{name: "BandParametersDL-r10", reg: eutraTypes}, optional: true},
			{name: "supportedCSI-Proc-r11", typ: enumerated{values: []string{"n1", "n3", "n4"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["BandParameters-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandEUTRA-r13", typ: deferred{name: "FreqBandIndicator-r11", reg: eutraTypes}},
			{name: "bandParametersUL-r13", typ: deferred{name: "BandParametersUL-r13", reg: eutraTypes}, optional: true},
			{name: "bandParametersDL-r13", typ: deferred{name: "BandParametersDL-r13", reg: eutraTypes}, optional: true},
			{name: "supportedCSI-Proc-r13", typ: enumerated{values: []string{"n1", "n3", "n4"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["BandParameters-v1090"] = sequence{
		extensible: true,
		fields: []field{
			{name: "bandEUTRA-v1090", typ: deferred{name: "FreqBandIndicator-v9e0", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["BandParameters-v1130"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedCSI-Proc-r11", typ: enumerated{values: []string{"n1", "n3", "n4"}, extensible: false}},
		},
	}

	eutraTypes["BandParameters-v1270"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandParametersDL-v1270", typ: sequenceOf{lb: 1, ub: 16, elem: deferred{name: "CA-MIMO-ParametersDL-v1270", reg: eutraTypes}}},
		},
	}

	eutraTypes["BandParameters-v1320"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandParametersDL-v1320", typ: deferred{name: "MIMO-CA-ParametersPerBoBC-r13", reg: eutraTypes}},
		},
	}

	eutraTypes["BandParameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandParametersDL-v1430", typ: deferred{name: "MIMO-CA-ParametersPerBoBC-v1430", reg: eutraTypes}, optional: true},
			{name: "ul-256QAM-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-256QAM-perCC-InfoList-r14", typ: sequenceOf{lb: 2, ub: 32, elem: deferred{name: "UL-256QAM-perCC-Info-r14", reg: eutraTypes}}, optional: true},
			{name: "srs-CapabilityPerBandPairList-r14", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SRS-CapabilityPerBandPair-r14", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["BandParameters-v1450"] = sequence{
		extensible: false,
		fields: []field{
			{name: "must-CapabilityPerBand-r14", typ: deferred{name: "MUST-Parameters-r14", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["BandParameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-TxAntennaSelection-SRS-1T4R-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-TxAntennaSelection-SRS-2T4R-2Pairs-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-TxAntennaSelection-SRS-2T4R-3Pairs-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dl-1024QAM-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "qcl-TypeC-Operation-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "qcl-CRI-BasedCSI-Reporting-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "stti-SPT-BandParameters-r15", typ: deferred{name: "STTI-SPT-BandParameters-r15", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["BandParameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "intraFreqDAPS-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "intraFreqAsyncDAPS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "dummy", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "intraFreqTwoTAGs-DAPS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "addSRS-FrequencyHopping-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "addSRS-AntennaSwitching-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "addSRS-1T2R-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "addSRS-1T4R-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "addSRS-2T4R-2pairs-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "addSRS-2T4R-3pairs-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "srs-CapabilityPerBandPairList-v1610", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SRS-CapabilityPerBandPair-v1610", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["BandParametersDL-r10"] = sequenceOf{lb: 1, ub: 16, elem: deferred{name: "CA-MIMO-ParametersDL-r10", reg: eutraTypes}}

	eutraTypes["BandParametersDL-r13"] = deferred{name: "CA-MIMO-ParametersDL-r13", reg: eutraTypes}

	eutraTypes["BandParametersRxA2X-r18"] = sequence{
		extensible: false,
		fields: []field{
			{name: "a2x-BandwidthClassRxSL-r18", typ: deferred{name: "V2X-BandwidthClassSL-r14", reg: eutraTypes}},
		},
	}

	eutraTypes["BandParametersRxSL-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "v2x-BandwidthClassRxSL-r14", typ: deferred{name: "V2X-BandwidthClassSL-r14", reg: eutraTypes}},
			{name: "v2x-HighReception-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["BandParametersTxA2X-r18"] = sequence{
		extensible: false,
		fields: []field{
			{name: "a2x-BandwidthClassTxSL-r18", typ: deferred{name: "V2X-BandwidthClassSL-r14", reg: eutraTypes}},
		},
	}

	eutraTypes["BandParametersTxSL-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "v2x-BandwidthClassTxSL-r14", typ: deferred{name: "V2X-BandwidthClassSL-r14", reg: eutraTypes}},
			{name: "v2x-eNB-Scheduled-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "v2x-HighPower-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["BandParametersUL-r10"] = sequenceOf{lb: 1, ub: 16, elem: deferred{name: "CA-MIMO-ParametersUL-r10", reg: eutraTypes}}

	eutraTypes["BandParametersUL-r13"] = deferred{name: "CA-MIMO-ParametersUL-r10", reg: eutraTypes}

	eutraTypes["BandclassCDMA2000"] = enumerated{values: []string{"bc0", "bc1", "bc2", "bc3", "bc4", "bc5", "bc6", "bc7", "bc8", "bc9", "bc10", "bc11", "bc12", "bc13", "bc14", "bc15", "bc16", "bc17", "bc18-v9a0", "bc19-v9a0", "bc20-v9a0", "bc21-v9a0", "spare10", "spare9", "spare8", "spare7", "spare6", "spare5", "spare4", "spare3", "spare2", "spare1"}, extensible: true}

	eutraTypes["CA-BandwidthClass-r10"] = enumerated{values: []string{"a", "b", "c", "d", "e", "f"}, extensible: true}

	eutraTypes["CA-MIMO-ParametersDL-r10"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ca-BandwidthClassDL-r10", typ: deferred{name: "CA-BandwidthClass-r10", reg: eutraTypes}},
			{name: "supportedMIMO-CapabilityDL-r10", typ: deferred{name: "MIMO-CapabilityDL-r10", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["CA-MIMO-ParametersDL-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ca-BandwidthClassDL-r13", typ: deferred{name: "CA-BandwidthClass-r10", reg: eutraTypes}},
			{name: "supportedMIMO-CapabilityDL-r13", typ: deferred{name: "MIMO-CapabilityDL-r10", reg: eutraTypes}, optional: true},
			{name: "fourLayerTM3-TM4-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "intraBandContiguousCC-InfoList-r13", typ: sequenceOf{lb: 1, ub: 32, elem: deferred{name: "IntraBandContiguousCC-Info-r12", reg: eutraTypes}}},
		},
	}

	eutraTypes["CA-MIMO-ParametersDL-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedMIMO-CapabilityDL-r15", typ: deferred{name: "MIMO-CapabilityDL-r10", reg: eutraTypes}, optional: true},
			{name: "fourLayerTM3-TM4-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "intraBandContiguousCC-InfoList-r15", typ: sequenceOf{lb: 1, ub: 32, elem: deferred{name: "IntraBandContiguousCC-Info-r12", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["CA-MIMO-ParametersDL-v1270"] = sequence{
		extensible: false,
		fields: []field{
			{name: "intraBandContiguousCC-InfoList-r12", typ: sequenceOf{lb: 1, ub: 5, elem: deferred{name: "IntraBandContiguousCC-Info-r12", reg: eutraTypes}}},
		},
	}

	eutraTypes["CA-MIMO-ParametersUL-r10"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ca-BandwidthClassUL-r10", typ: deferred{name: "CA-BandwidthClass-r10", reg: eutraTypes}},
			{name: "supportedMIMO-CapabilityUL-r10", typ: deferred{name: "MIMO-CapabilityUL-r10", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["CA-MIMO-ParametersUL-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedMIMO-CapabilityUL-r15", typ: deferred{name: "MIMO-CapabilityUL-r10", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["CE-MultiTB-Parameters-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdsch-MultiTB-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-MultiTB-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-MultiTB-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-MultiTB-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-MultiTB-64QAM-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-MultiTB-EarlyTermination-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-MultiTB-FrequencyHopping-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-MultiTB-HARQ-AckBundling-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-MultiTB-Interleaving-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-MultiTB-SubPRB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["CE-Parameters-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ce-ModeA-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-ModeB-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["CE-Parameters-v1320"] = sequence{
		extensible: false,
		fields: []field{
			{name: "intraFreqA3-CE-ModeA-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "intraFreqA3-CE-ModeB-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "intraFreqHO-CE-ModeA-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "intraFreqHO-CE-ModeB-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["CE-Parameters-v1350"] = sequence{
		extensible: false,
		fields: []field{
			{name: "unicastFrequencyHopping-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["CE-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ce-SwitchWithoutHO-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["CE-ResourceResvParameters-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "subframeResourceResvDL-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "subframeResourceResvDL-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "subframeResourceResvUL-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "subframeResourceResvUL-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "slotSymbolResourceResvDL-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "slotSymbolResourceResvDL-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "slotSymbolResourceResvUL-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "slotSymbolResourceResvUL-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "subcarrierPuncturingCE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "subcarrierPuncturingCE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["CSG-ProximityIndicationParameters-r9"] = sequence{
		extensible: false,
		fields: []field{
			{name: "intraFreqProximityIndication-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "interFreqProximityIndication-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "utran-ProximityIndication-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["DC-Parameters-r12"] = sequence{
		extensible: false,
		fields: []field{
			{name: "drb-TypeSplit-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "drb-TypeSCG-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["DC-Parameters-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdcp-TransferSplitUL-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-SSTD-Meas-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["DL-UL-CCs-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxNumberDL-CCs-r15", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
			{name: "maxNumberUL-CCs-r15", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
		},
	}

	eutraTypes["EUTRA-5GC-Parameters-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eutra-5GC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-EPC-HO-EUTRA-5GC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ho-EUTRA-5GC-FDD-TDD-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ho-InterfreqEUTRA-5GC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ims-VoiceOverMCG-BearerEUTRA-5GC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "inactiveState-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "reflectiveQoS-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["EUTRA-5GC-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ce-InactiveState-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-EUTRA-5GC-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["FeMBMS-Unicast-Parameters-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "unicast-fembmsMixedSCell-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "emptyUnicastRegion-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["FeatureSetDL-PerCC-Id-r15"] = integer{lb: 0, ub: 32, extensible: false}

	eutraTypes["FeatureSetDL-PerCC-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "fourLayerTM3-TM4-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedMIMO-CapabilityDL-MRDC-r15", typ: deferred{name: "MIMO-CapabilityDL-r10", reg: eutraTypes}, optional: true},
			{name: "supportedCSI-Proc-r15", typ: enumerated{values: []string{"n1", "n3", "n4"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["FeatureSetDL-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mimo-CA-ParametersPerBoBC-r15", typ: deferred{name: "MIMO-CA-ParametersPerBoBC-r15", reg: eutraTypes}, optional: true},
			{name: "featureSetPerCC-ListDL-r15", typ: sequenceOf{lb: 1, ub: 32, elem: deferred{name: "FeatureSetDL-PerCC-Id-r15", reg: eutraTypes}}},
		},
	}

	eutraTypes["FeatureSetUL-PerCC-Id-r15"] = integer{lb: 0, ub: 32, extensible: false}

	eutraTypes["FeatureSetUL-PerCC-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedMIMO-CapabilityUL-r15", typ: deferred{name: "MIMO-CapabilityUL-r10", reg: eutraTypes}, optional: true},
			{name: "ul-256QAM-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["FeatureSetUL-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "featureSetPerCC-ListUL-r15", typ: sequenceOf{lb: 1, ub: 32, elem: deferred{name: "FeatureSetUL-PerCC-Id-r15", reg: eutraTypes}}},
		},
	}

	eutraTypes["FeatureSetsEUTRA-r15"] = sequence{
		extensible: true,
		fields: []field{
			{name: "featureSetsDL-r15", typ: sequenceOf{lb: 1, ub: 256, elem: deferred{name: "FeatureSetDL-r15", reg: eutraTypes}}, optional: true},
			{name: "featureSetsDL-PerCC-r15", typ: sequenceOf{lb: 1, ub: 32, elem: deferred{name: "FeatureSetDL-PerCC-r15", reg: eutraTypes}}, optional: true},
			{name: "featureSetsUL-r15", typ: sequenceOf{lb: 1, ub: 256, elem: deferred{name: "FeatureSetUL-r15", reg: eutraTypes}}, optional: true},
			{name: "featureSetsUL-PerCC-r15", typ: sequenceOf{lb: 1, ub: 32, elem: deferred{name: "FeatureSetUL-PerCC-r15", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["FreqBandIndicator"] = integer{lb: 1, ub: 64, extensible: false}

	eutraTypes["FreqBandIndicator-r11"] = integer{lb: 1, ub: 256, extensible: false}

	eutraTypes["FreqBandIndicator-v9e0"] = integer{lb: 65, ub: 256, extensible: false}

	eutraTypes["FreqBandIndicatorListEUTRA-r12"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "FreqBandIndicator-r11", reg: eutraTypes}}

	eutraTypes["FreqBandIndicatorNR-r15"] = integer{lb: 1, ub: 1024, extensible: false}

	eutraTypes["HighSpeedEnhParameters-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measurementEnhancements-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "demodulationEnhancements-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "prach-Enhancements-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["HighSpeedEnhParameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measurementEnhancementsSCell-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "measurementEnhancements2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "demodulationEnhancements2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "interRAT-enhancementNR-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersCDMA2000-1XRTT"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandList1XRTT", typ: deferred{name: "SupportedBandList1XRTT", reg: eutraTypes}},
			{name: "tx-Config1XRTT", typ: enumerated{values: []string{"single", "dual"}, extensible: false}},
			{name: "rx-Config1XRTT", typ: enumerated{values: []string{"single", "dual"}, extensible: false}},
		},
	}

	eutraTypes["IRAT-ParametersCDMA2000-1XRTT-v1020"] = sequence{
		extensible: false,
		fields: []field{
			{name: "e-CSFB-dual-1XRTT-r10", typ: enumerated{values: []string{"supported"}, extensible: false}},
		},
	}

	eutraTypes["IRAT-ParametersCDMA2000-1XRTT-v920"] = sequence{
		extensible: false,
		fields: []field{
			{name: "e-CSFB-1XRTT-r9", typ: enumerated{values: []string{"supported"}, extensible: false}},
			{name: "e-CSFB-ConcPS-Mob1XRTT-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersCDMA2000-HRPD"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListHRPD", typ: deferred{name: "SupportedBandListHRPD", reg: eutraTypes}},
			{name: "tx-ConfigHRPD", typ: enumerated{values: []string{"single", "dual"}, extensible: false}},
			{name: "rx-ConfigHRPD", typ: enumerated{values: []string{"single", "dual"}, extensible: false}},
		},
	}

	eutraTypes["IRAT-ParametersCDMA2000-v1130"] = sequence{
		extensible: false,
		fields: []field{
			{name: "cdma2000-NW-Sharing-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersGERAN"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListGERAN", typ: deferred{name: "SupportedBandListGERAN", reg: eutraTypes}},
			{name: "interRAT-PS-HO-ToGERAN", typ: boolean{}},
		},
	}

	eutraTypes["IRAT-ParametersGERAN-v920"] = sequence{
		extensible: false,
		fields: []field{
			{name: "dtm-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "e-RedirectionGERAN-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersNR-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "en-DC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eventB2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedBandListEN-DC-r15", typ: deferred{name: "SupportedBandListNR-r15", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersNR-v1540"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eutra-5GC-HO-ToNR-FDD-FR1-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-5GC-HO-ToNR-TDD-FR1-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-5GC-HO-ToNR-FDD-FR2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-5GC-HO-ToNR-TDD-FR2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-EPC-HO-ToNR-FDD-FR1-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-EPC-HO-ToNR-TDD-FR1-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-EPC-HO-ToNR-FDD-FR2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-EPC-HO-ToNR-TDD-FR2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ims-VoiceOverNR-FR1-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ims-VoiceOverNR-FR2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sa-NR-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedBandListNR-SA-r15", typ: deferred{name: "SupportedBandListNR-r15", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersNR-v1560"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ng-EN-DC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersNR-v1570"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ss-SINR-Meas-NR-FR1-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ss-SINR-Meas-NR-FR2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersNR-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nr-HO-ToEN-DC-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-EUTRA-5GC-HO-ToNR-FDD-FR1-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-EUTRA-5GC-HO-ToNR-TDD-FR1-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-EUTRA-5GC-HO-ToNR-FDD-FR2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-EUTRA-5GC-HO-ToNR-TDD-FR2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersNR-v1660"] = sequence{
		extensible: false,
		fields: []field{
			{name: "extendedBand-n77-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersNR-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eutra-5GC-HO-ToNR-TDD-FR2-2-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-EPC-HO-ToNR-TDD-FR2-2-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-EUTRA-5GC-HO-ToNR-TDD-FR2-2-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ims-VoiceOverNR-FR2-2-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersNR-v1710"] = sequence{
		extensible: false,
		fields: []field{
			{name: "extendedBand-n77-2-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["IRAT-ParametersUTRA-FDD"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListUTRA-FDD", typ: deferred{name: "SupportedBandListUTRA-FDD", reg: eutraTypes}},
		},
	}

	eutraTypes["IRAT-ParametersUTRA-TDD-v1020"] = sequence{
		extensible: false,
		fields: []field{
			{name: "e-RedirectionUTRA-TDD-r10", typ: enumerated{values: []string{"supported"}, extensible: false}},
		},
	}

	eutraTypes["IRAT-ParametersUTRA-TDD128"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListUTRA-TDD128", typ: deferred{name: "SupportedBandListUTRA-TDD128", reg: eutraTypes}},
		},
	}

	eutraTypes["IRAT-ParametersUTRA-TDD384"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListUTRA-TDD384", typ: deferred{name: "SupportedBandListUTRA-TDD384", reg: eutraTypes}},
		},
	}

	eutraTypes["IRAT-ParametersUTRA-TDD768"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListUTRA-TDD768", typ: deferred{name: "SupportedBandListUTRA-TDD768", reg: eutraTypes}},
		},
	}

	eutraTypes["IRAT-ParametersUTRA-v920"] = sequence{
		extensible: false,
		fields: []field{
			{name: "e-RedirectionUTRA-r9", typ: enumerated{values: []string{"supported"}, extensible: false}},
		},
	}

	eutraTypes["IRAT-ParametersWLAN-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListWLAN-r13", typ: sequenceOf{lb: 1, ub: 8, elem: deferred{name: "WLAN-BandIndicator-r13", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["InterFreqBandInfo"] = sequence{
		extensible: false,
		fields: []field{
			{name: "interFreqNeedForGaps", typ: boolean{}},
		},
	}

	eutraTypes["InterFreqBandList"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "InterFreqBandInfo", reg: eutraTypes}}

	eutraTypes["InterRAT-BandInfo"] = sequence{
		extensible: false,
		fields: []field{
			{name: "interRAT-NeedForGaps", typ: boolean{}},
		},
	}

	eutraTypes["InterRAT-BandInfoNR-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "interRAT-NeedForGapsNR-r16", typ: boolean{}},
		},
	}

	eutraTypes["InterRAT-BandInfoNR-r18"] = sequence{
		extensible: false,
		fields: []field{
			{name: "interRAT-NeedForInterruptionNR-r18", typ: enumerated{values: []string{"no-gap-with-interruption", "no-gap-no-interruption"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["InterRAT-BandList"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "InterRAT-BandInfo", reg: eutraTypes}}

	eutraTypes["InterRAT-BandListNR-r16"] = sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "InterRAT-BandInfoNR-r16", reg: eutraTypes}}

	eutraTypes["InterRAT-BandListNR-r18"] = sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "InterRAT-BandInfoNR-r18", reg: eutraTypes}}

	eutraTypes["IntraBandContiguousCC-Info-r12"] = sequence{
		extensible: false,
		fields: []field{
			{name: "fourLayerTM3-TM4-perCC-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedMIMO-CapabilityDL-r12", typ: deferred{name: "MIMO-CapabilityDL-r10", reg: eutraTypes}, optional: true},
			{name: "supportedCSI-Proc-r12", typ: enumerated{values: []string{"n1", "n3", "n4"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["LAA-Parameters-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "crossCarrierSchedulingLAA-DL-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-RS-DRS-RRM-MeasurementsLAA-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "downlinkLAA-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "endingDwPTS-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "secondSlotStartingPosition-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tm9-LAA-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tm10-LAA-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["LAA-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "crossCarrierSchedulingLAA-UL-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "uplinkLAA-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "twoStepSchedulingTimingInfo-r14", typ: enumerated{values: []string{"nPlus1", "nPlus2", "nPlus3"}, extensible: false}, optional: true},
			{name: "uss-BlindDecodingAdjustment-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "uss-BlindDecodingReduction-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "outOfSequenceGrantHandling-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["LAA-Parameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "aul-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "laa-PUSCH-Mode1-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "laa-PUSCH-Mode2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "laa-PUSCH-Mode3-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["LWA-Parameters-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "lwa-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "lwa-SplitBearer-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "wlan-MAC-Address-r13", typ: octetString{lb: 6, ub: 6, hasUB: true}, optional: true},
			{name: "lwa-BufferSize-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["LWA-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "lwa-HO-WithoutWT-Change-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "lwa-UL-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "wlan-PeriodicMeas-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "wlan-ReportAnyWLAN-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "wlan-SupportedDataRate-r14", typ: integer{lb: 1, ub: 2048, extensible: false}, optional: true},
		},
	}

	eutraTypes["LWA-Parameters-v1440"] = sequence{
		extensible: false,
		fields: []field{
			{name: "lwa-RLC-UM-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["LWIP-Parameters-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "lwip-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["LWIP-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "lwip-Aggregation-DL-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "lwip-Aggregation-UL-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["LowerMSD-MRDC-r18"] = sequence{
		extensible: false,
		fields: []field{
			{name: "aggressorband1-r18", typ: deferred{name: "FreqBandIndicatorNR-r15", reg: eutraTypes}},
			{name: "aggressorband2-r18", typ: deferred{name: "FreqBandIndicator-r11", reg: eutraTypes}, optional: true},
			{name: "msd-Information-r18", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "MSD-Information-r18", reg: eutraTypes}}},
		},
	}

	eutraTypes["MAC-Parameters-r12"] = sequence{
		extensible: false,
		fields: []field{
			{name: "logicalChannelSR-ProhibitTimer-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "longDRX-Command-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MAC-Parameters-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "extendedMAC-LengthField-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "extendedLongDRX-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MAC-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "shortSPS-IntervalFDD-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "shortSPS-IntervalTDD-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "skipUplinkDynamic-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "skipUplinkSPS-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multipleUplinkSPS-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dataInactMon-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MAC-Parameters-v1440"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rai-Support-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MAC-Parameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "min-Proc-TimelineSubslot-r15", typ: sequenceOf{lb: 1, ub: 3, elem: deferred{name: "ProcessingTimelineSet-r15", reg: eutraTypes}}, optional: true},
			{name: "skipSubframeProcessing-r15", typ: deferred{name: "SkipSubframeProcessing-r15", reg: eutraTypes}, optional: true},
			{name: "earlyData-UP-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dormantSCellState-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directSCellActivation-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directSCellHibernation-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "extendedLCID-Duplication-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sps-ServingCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MAC-Parameters-v1550"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eLCID-Support-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MAC-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "directMCG-SCellActivationResume-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directSCG-SCellActivationResume-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "earlyData-UP-5GC-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rai-SupportEnh-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MAC-Parameters-v1630"] = sequence{
		extensible: false,
		fields: []field{
			{name: "directSCG-SCellActivationNEDC-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MBMS-Parameters-r11"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mbms-SCell-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mbms-NonServingCell-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MBMS-Parameters-v1250"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mbms-AsyncDC-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MBMS-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "fembmsDedicatedCell-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "fembmsMixedCell-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "subcarrierSpacingMBMS-khz7dot5-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "subcarrierSpacingMBMS-khz1dot25-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MBMS-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mbms-ScalingFactor2dot5-r16", typ: enumerated{values: []string{"n2", "n4", "n6", "n8"}, extensible: false}, optional: true},
			{name: "mbms-ScalingFactor0dot37-r16", typ: enumerated{values: []string{"n12", "n16", "n20", "n24"}, extensible: false}, optional: true},
			{name: "mbms-SupportedBandInfoList-r16", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "MBMS-SupportedBandInfo-r16", reg: eutraTypes}}},
		},
	}

	eutraTypes["MBMS-Parameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mbms-SupportedBandInfoList-v1700", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "MBMS-SupportedBandInfo-v1700", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["MBMS-SupportedBandInfo-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "subcarrierSpacingMBMS-khz2dot5-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "subcarrierSpacingMBMS-khz0dot37-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "timeSeparationSlot2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "timeSeparationSlot4-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
		},
	}

	eutraTypes["MBMS-SupportedBandInfo-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pmch-Bandwidth-n40-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pmch-Bandwidth-n35-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pmch-Bandwidth-n30-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MCC"] = sequenceOf{lb: 3, ub: 3, elem: deferred{name: "MCC-MNC-Digit", reg: eutraTypes}}

	eutraTypes["MCC-MNC-Digit"] = integer{lb: 0, ub: 9, extensible: false}

	eutraTypes["MIMO-BeamformedCapabilities-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "k-Max-r13", typ: integer{lb: 1, ub: 8, extensible: false}},
			{name: "n-MaxList-r13", typ: bitString{lb: 1, ub: 7, extensible: false}, optional: true},
		},
	}

	eutraTypes["MIMO-BeamformedCapabilityList-r13"] = sequenceOf{lb: 1, ub: 4, elem: deferred{name: "MIMO-BeamformedCapabilities-r13", reg: eutraTypes}}

	eutraTypes["MIMO-CA-ParametersPerBoBC-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "parametersTM9-r13", typ: deferred{name: "MIMO-CA-ParametersPerBoBCPerTM-r13", reg: eutraTypes}, optional: true},
			{name: "parametersTM10-r13", typ: deferred{name: "MIMO-CA-ParametersPerBoBCPerTM-r13", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["MIMO-CA-ParametersPerBoBC-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "parametersTM9-r15", typ: deferred{name: "MIMO-CA-ParametersPerBoBCPerTM-r15", reg: eutraTypes}, optional: true},
			{name: "parametersTM10-r15", typ: deferred{name: "MIMO-CA-ParametersPerBoBCPerTM-r15", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["MIMO-CA-ParametersPerBoBC-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "parametersTM9-v1430", typ: deferred{name: "MIMO-CA-ParametersPerBoBCPerTM-v1430", reg: eutraTypes}, optional: true},
			{name: "parametersTM10-v1430", typ: deferred{name: "MIMO-CA-ParametersPerBoBCPerTM-v1430", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["MIMO-CA-ParametersPerBoBCPerTM-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nonPrecoded-r13", typ: deferred{name: "MIMO-NonPrecodedCapabilities-r13", reg: eutraTypes}, optional: true},
			{name: "beamformed-r13", typ: deferred{name: "MIMO-BeamformedCapabilityList-r13", reg: eutraTypes}, optional: true},
			{name: "dmrs-Enhancements-r13", typ: enumerated{values: []string{"different"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MIMO-CA-ParametersPerBoBCPerTM-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nonPrecoded-r13", typ: deferred{name: "MIMO-NonPrecodedCapabilities-r13", reg: eutraTypes}, optional: true},
			{name: "beamformed-r13", typ: deferred{name: "MIMO-BeamformedCapabilityList-r13", reg: eutraTypes}, optional: true},
			{name: "dmrs-Enhancements-r13", typ: enumerated{values: []string{"different"}, extensible: false}, optional: true},
			{name: "csi-ReportingNP-r14", typ: enumerated{values: []string{"different"}, extensible: false}, optional: true},
			{name: "csi-ReportingAdvanced-r14", typ: enumerated{values: []string{"different"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MIMO-CA-ParametersPerBoBCPerTM-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "csi-ReportingNP-r14", typ: enumerated{values: []string{"different"}, extensible: false}, optional: true},
			{name: "csi-ReportingAdvanced-r14", typ: enumerated{values: []string{"different"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MIMO-CapabilityDL-r10"] = enumerated{values: []string{"twoLayers", "fourLayers", "eightLayers"}, extensible: false}

	eutraTypes["MIMO-CapabilityUL-r10"] = enumerated{values: []string{"twoLayers", "fourLayers"}, extensible: false}

	eutraTypes["MIMO-NonPrecodedCapabilities-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "config1-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "config2-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "config3-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "config4-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MIMO-UE-BeamformedCapabilities-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "altCodebook-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mimo-BeamformedCapabilities-r13", typ: deferred{name: "MIMO-BeamformedCapabilityList-r13", reg: eutraTypes}},
		},
	}

	eutraTypes["MIMO-UE-Parameters-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "parametersTM9-r13", typ: deferred{name: "MIMO-UE-ParametersPerTM-r13", reg: eutraTypes}, optional: true},
			{name: "parametersTM10-r13", typ: deferred{name: "MIMO-UE-ParametersPerTM-r13", reg: eutraTypes}, optional: true},
			{name: "srs-EnhancementsTDD-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "srs-Enhancements-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "interferenceMeasRestriction-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MIMO-UE-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "parametersTM9-v1430", typ: deferred{name: "MIMO-UE-ParametersPerTM-v1430", reg: eutraTypes}, optional: true},
			{name: "parametersTM10-v1430", typ: deferred{name: "MIMO-UE-ParametersPerTM-v1430", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["MIMO-UE-ParametersPerTM-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nonPrecoded-r13", typ: deferred{name: "MIMO-NonPrecodedCapabilities-r13", reg: eutraTypes}, optional: true},
			{name: "beamformed-r13", typ: deferred{name: "MIMO-UE-BeamformedCapabilities-r13", reg: eutraTypes}, optional: true},
			{name: "channelMeasRestriction-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dmrs-Enhancements-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-RS-EnhancementsTDD-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MIMO-UE-ParametersPerTM-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nzp-CSI-RS-AperiodicInfo-r14", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "nMaxProc-r14", typ: integer{lb: 5, ub: 32, extensible: false}},
					{name: "nMaxResource-r14", typ: enumerated{values: []string{"n1", "n2", "n4", "n8"}, extensible: false}},
				},
			}, optional: true},
			{name: "nzp-CSI-RS-PeriodicInfo-r14", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "nMaxResource-r14", typ: enumerated{values: []string{"n1", "n2", "n4", "n8"}, extensible: false}},
				},
			}, optional: true},
			{name: "zp-CSI-RS-AperiodicInfo-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-dmrs-Enhancements-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "densityReductionNP-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "densityReductionBF-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "hybridCSI-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "semiOL-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-ReportingNP-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-ReportingAdvanced-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MMTEL-Parameters-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "delayBudgetReporting-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-Enhancements-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "recommendedBitRate-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "recommendedBitRateQuery-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MMTEL-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "recommendedBitRateMultiplier-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MNC"] = sequenceOf{lb: 2, ub: 3, elem: deferred{name: "MCC-MNC-Digit", reg: eutraTypes}}

	eutraTypes["MSD-Information-r18"] = sequence{
		extensible: false,
		fields: []field{
			{name: "msd-Type-r18", typ: enumerated{values: []string{"harmonic", "harmonicMixing", "crossBandIsolation", "imd2", "imd3", "imd4", "imd5", "all", "spare8", "spare7", "spare6", "spare5", "spare4", "spare3", "spare2", "spare1"}, extensible: false}},
			{name: "msd-PowerClass-r18", typ: enumerated{values: []string{"pc1dot5", "pc2", "pc3"}, extensible: false}},
			{name: "msd-Class-r18", typ: enumerated{values: []string{"classI", "classII", "classIII", "classIV", "classV", "classVI", "classVII", "classVIII"}, extensible: false}},
		},
	}

	eutraTypes["MUST-Parameters-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "must-TM234-UpTo2Tx-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "must-TM89-UpToOneInterferingLayer-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "must-TM10-UpToOneInterferingLayer-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "must-TM89-UpToThreeInterferingLayers-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "must-TM10-UpToThreeInterferingLayers-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasGapInfoNR-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "interRAT-BandListNR-EN-DC-r16", typ: deferred{name: "InterRAT-BandListNR-r16", reg: eutraTypes}, optional: true},
			{name: "interRAT-BandListNR-SA-r16", typ: deferred{name: "InterRAT-BandListNR-r16", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["MeasGapInfoNR-r18"] = sequence{
		extensible: false,
		fields: []field{
			{name: "interRAT-BandListNR-EN-DC-r18", typ: deferred{name: "InterRAT-BandListNR-r18", reg: eutraTypes}, optional: true},
			{name: "interRAT-BandListNR-SA-r18", typ: deferred{name: "InterRAT-BandListNR-r18", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["MeasParameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandListEUTRA", typ: deferred{name: "BandListEUTRA", reg: eutraTypes}},
		},
	}

	eutraTypes["MeasParameters-v1020"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandCombinationListEUTRA-r10", typ: deferred{name: "BandCombinationListEUTRA-r10", reg: eutraTypes}},
		},
	}

	eutraTypes["MeasParameters-v1130"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rsrqMeasWideband-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v11a0"] = sequence{
		extensible: false,
		fields: []field{
			{name: "benefitsFromInterruption-r11", typ: enumerated{values: []string{"true"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1250"] = sequence{
		extensible: false,
		fields: []field{
			{name: "timerT312-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "alternativeTimeToTrigger-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "incMonEUTRA-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "incMonUTRA-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "extendedMaxMeasId-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "extendedRSRQ-LowerRange-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rsrq-OnAllSymbols-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "crs-DiscoverySignalsMeas-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-RS-DiscoverySignalsMeas-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rs-SINR-Meas-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "allowedCellList-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "extendedMaxObjectId-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-PDCP-Delay-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "extendedFreqPriorities-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multiBandInfoReport-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rssi-AndChannelOccupancyReporting-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ceMeasurements-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ncsg-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "shortMeasurementGap-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "perServingCellMeasurementGap-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nonUniformGap-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1520"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measGapPatterns-r15", typ: bitString{lb: 8, ub: 8, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "qoe-MeasReport-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "qoe-MTSI-MeasReport-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ca-IdleModeMeasurements-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ca-IdleModeValidityArea-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "heightMeas-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multipleCellsMeasExtension-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandInfoNR-r16", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "MeasGapInfoNR-r16", reg: eutraTypes}}, optional: true},
			{name: "altFreqPriority-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-DL-ChannelQualityReporting-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-MeasRSS-Dedicated-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eutra-IdleInactiveMeasurements-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nr-IdleInactiveMeasFR1-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nr-IdleInactiveMeasFR2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "idleInactiveValidityAreaList-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "measGapPatterns-NRonly-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "measGapPatterns-NRonly-ENDC-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1630"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nr-IdleInactiveBeamMeasFR1-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nr-IdleInactiveBeamMeasFR2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-MeasRSS-DedicatedSameRBs-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sharedSpectrumMeasNR-EN-DC-r17", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "SharedSpectrumMeasNR-r17", reg: eutraTypes}}, optional: true},
			{name: "sharedSpectrumMeasNR-SA-r17", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "SharedSpectrumMeasNR-r17", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1770"] = sequence{
		extensible: false,
		fields: []field{
			{name: "gaplessMeas-FR2-maxCC-r17", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
		},
	}

	eutraTypes["MeasParameters-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandInfoNR-v1800", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "MeasGapInfoNR-r18", reg: eutraTypes}}},
		},
	}

	eutraTypes["MeasParameters-v1840"] = sequence{
		extensible: false,
		fields: []field{
			{name: "simultaneousRxDataSSB-DiffNumerology-FR1-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MobilityParameters-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "makeBeforeBreak-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rach-Less-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["MobilityParameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "cho-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "cho-FDD-TDD-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "cho-Failure-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "cho-TwoTriggerEvents-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NAICS-Capability-Entry-r12"] = sequence{
		extensible: true,
		fields: []field{
			{name: "numberOfNAICS-CapableCC-r12", typ: integer{lb: 1, ub: 5, extensible: false}},
			{name: "numberOfAggregatedPRB-r12", typ: enumerated{values: []string{"n50", "n75", "n100", "n125", "n150", "n175", "n200", "n225", "n250", "n275", "n300", "n350", "n400", "n450", "n500", "spare"}, extensible: false}},
		},
	}

	eutraTypes["NAICS-Capability-List-r12"] = sequenceOf{lb: 1, ub: 8, elem: deferred{name: "NAICS-Capability-Entry-r12", reg: eutraTypes}}

	eutraTypes["NTN-Parameters-r17"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ntn-Connectivity-EPC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-TA-Report-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-PUR-TimerDelay-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-OffsetTimingEnh-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-ScenarioSupport-r17", typ: enumerated{values: []string{"ngso", "gso"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NTN-Parameters-v1720"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ntn-SegmentedPrecompensationGaps-r17", typ: enumerated{values: []string{"sym1", "sl1", "sf1"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NTN-Parameters-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ntn-EventA4BasedCHO-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-LocationBasedCHO-EFC-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-LocationBasedCHO-EMC-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-TimeBasedCHO-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eventD1-MeasReportTrigger-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eventD2-MeasReportTrigger-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-LocationBasedMeasTrigger-EFC-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-LocationBasedMeasTrigger-EMC-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-TimeBasedMeasTrigger-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-RRC-HarqDisableSingleTB-CE-ModeA-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-RRC-HarqDisableMultiTB-CE-ModeA-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-RRC-HarqDisableSingleTB-CE-ModeB-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-OverriddenHarqDisableSingleTB-CE-ModeB-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-DCI-HarqDisableSingleTB-CE-ModeB-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-RRC-HarqDisableMultiTB-CE-ModeB-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-OverriddenHarqDisableMultiTB-CE-ModeB-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-DCI-HarqDisableMultiTB-CE-ModeB-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-SemiStaticHarqDisableSPS-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-UplinkHarq-ModeB-SingleTB-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-UplinkHarq-ModeB-MultiTB-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-HarqEnhScenarioSupport-r18", typ: enumerated{values: []string{"ngso", "gso"}, extensible: false}, optional: true},
			{name: "ntn-Triggered-GNSS-Fix-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-Autonomous-GNSS-Fix-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-UplinkTxExtension-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-GNSS-EnhScenarioSupport-r18", typ: enumerated{values: []string{"ngso", "gso"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NTN-Parameters-v1830"] = sequence{
		extensible: false,
		fields: []field{
			{name: "satelliteInfoConfigDedicated-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NeighCellSI-AcquisitionParameters-r9"] = sequence{
		extensible: false,
		fields: []field{
			{name: "intraFreqSI-AcquisitionForHO-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "interFreqSI-AcquisitionForHO-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "utran-SI-AcquisitionForHO-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NeighCellSI-AcquisitionParameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "reportCGI-NR-EN-DC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "reportCGI-NR-NoEN-DC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NeighCellSI-AcquisitionParameters-v1550"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eutra-CGI-Reporting-ENDC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "utra-GERAN-CGI-Reporting-ENDC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NeighCellSI-AcquisitionParameters-v15a0"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eutra-CGI-Reporting-NEDC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NeighCellSI-AcquisitionParameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eutra-SI-AcquisitionForHO-ENDC-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nr-AutonomousGaps-ENDC-FR1-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nr-AutonomousGaps-ENDC-FR2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nr-AutonomousGaps-FR1-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nr-AutonomousGaps-FR2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NeighCellSI-AcquisitionParameters-v1710"] = sequence{
		extensible: false,
		fields: []field{
			{name: "gNB-ID-Length-Reporting-NR-EN-DC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "gNB-ID-Length-Reporting-NR-NoEN-DC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["NonContiguousUL-RA-WithinCC-List-r10"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "NonContiguousUL-RA-WithinCC-r10", reg: eutraTypes}}

	eutraTypes["NonContiguousUL-RA-WithinCC-r10"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nonContiguousUL-RA-WithinCC-Info-r10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["Other-Parameters-r11"] = sequence{
		extensible: false,
		fields: []field{
			{name: "inDeviceCoexInd-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "powerPrefInd-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-Rx-TxTimeDiffMeasurements-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["Other-Parameters-v1360"] = sequence{
		extensible: false,
		fields: []field{
			{name: "inDeviceCoexInd-HardwareSharingInd-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["Other-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bwPrefInd-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rlm-ReportSupport-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["Other-Parameters-v1460"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nonCSG-SI-Reporting-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["Other-Parameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "assistInfoBitForLC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "timeReferenceProvision-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "flightPathPlan-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["Other-Parameters-v1540"] = sequence{
		extensible: false,
		fields: []field{
			{name: "inDeviceCoexInd-ENDC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["Other-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "resumeWithStoredMCG-SCells-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "resumeWithMCG-SCellConfig-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "resumeWithStoredSCG-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "resumeWithSCG-Config-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mcgRLF-RecoveryViaSCG-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "overheatingIndForSCG-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["Other-Parameters-v1650"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mpsPriorityIndication-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["Other-Parameters-v1690"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ul-RRC-Segmentation-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["OtherParameters-v1450"] = sequence{
		extensible: false,
		fields: []field{
			{name: "overheatingInd-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PDCP-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "supportedROHC-Profiles", typ: deferred{name: "ROHC-ProfileSupportList-r15", reg: eutraTypes}},
			{name: "maxNumberROHC-ContextSessions", typ: enumerated{values: []string{"cs2", "cs4", "cs8", "cs12", "cs16", "cs24", "cs32", "cs48", "cs64", "cs128", "cs256", "cs512", "cs1024", "cs16384", "spare2", "spare1"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PDCP-Parameters-v1130"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdcp-SN-Extension-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportRohcContextContinue-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PDCP-Parameters-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdcp-SN-Extension-18bits-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PDCP-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedUplinkOnlyROHC-Profiles-r14", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "profile0x0006-r14", typ: boolean{}},
				},
			}},
			{name: "maxNumberROHC-ContextSessions-r14", typ: enumerated{values: []string{"cs2", "cs4", "cs8", "cs12", "cs16", "cs24", "cs32", "cs48", "cs64", "cs128", "cs256", "cs512", "cs1024", "cs16384", "spare2", "spare1"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PDCP-Parameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedUDC-r15", typ: deferred{name: "SupportedUDC-r15", reg: eutraTypes}, optional: true},
			{name: "pdcp-Duplication-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PDCP-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdcp-VersionChangeWithoutHO-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ehc-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "continueEHC-Context-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "maxNumberEHC-Contexts-r16", typ: enumerated{values: []string{"cs2", "cs4", "cs8", "cs16", "cs32", "cs64", "cs128", "cs256", "cs512", "cs1024", "cs2048", "cs4096", "cs8192", "cs16384", "cs32768", "cs65536"}, extensible: false}, optional: true},
			{name: "jointEHC-ROHC-Config-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PDCP-ParametersNR-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rohc-Profiles-r15", typ: deferred{name: "ROHC-ProfileSupportList-r15", reg: eutraTypes}},
			{name: "rohc-ContextMaxSessions-r15", typ: enumerated{values: []string{"cs2", "cs4", "cs8", "cs12", "cs16", "cs24", "cs32", "cs48", "cs64", "cs128", "cs256", "cs512", "cs1024", "cs16384", "spare2", "spare1"}, extensible: false}, optional: true},
			{name: "rohc-ProfilesUL-Only-r15", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "profile0x0006-r15", typ: boolean{}},
				},
			}},
			{name: "rohc-ContextContinue-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "outOfOrderDelivery-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sn-SizeLo-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ims-VoiceOverNR-PDCP-MCG-Bearer-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ims-VoiceOverNR-PDCP-SCG-Bearer-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PDCP-ParametersNR-v1560"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ims-VoNR-PDCP-SCG-NGENDC-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PLMN-Identity"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mcc", typ: deferred{name: "MCC", reg: eutraTypes}, optional: true},
			{name: "mnc", typ: deferred{name: "MNC", reg: eutraTypes}},
		},
	}

	eutraTypes["PUR-Parameters-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pur-CP-5GC-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-CP-5GC-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-UP-5GC-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-UP-5GC-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-CP-EPC-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-CP-EPC-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-UP-EPC-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-UP-EPC-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-CP-L1Ack-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-FrequencyHopping-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-PUSCH-NB-MaxTBS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-RSRP-Validation-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-SubPRB-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pur-SubPRB-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-TxAntennaSelectionSupported", typ: boolean{}},
			{name: "ue-SpecificRefSigsSupported", typ: boolean{}},
		},
	}

	eutraTypes["PhyLayerParameters-v1020"] = sequence{
		extensible: false,
		fields: []field{
			{name: "twoAntennaPortsForPUCCH-r10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tm9-With-8Tx-FDD-r10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pmi-Disabling-r10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "crossCarrierScheduling-r10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simultaneousPUCCH-PUSCH-r10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multiClusterPUSCH-WithinCC-r10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nonContiguousUL-RA-WithinCC-List-r10", typ: deferred{name: "NonContiguousUL-RA-WithinCC-List-r10", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1130"] = sequence{
		extensible: false,
		fields: []field{
			{name: "crs-InterfHandl-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ePDCCH-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multiACK-CSI-Reporting-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ss-CCH-InterfHandl-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tdd-SpecialSubframe-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "txDiv-PUCCH1b-ChSelect-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-CoMP-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1170"] = sequence{
		extensible: false,
		fields: []field{
			{name: "interBandTDD-CA-WithDifferentConfig-r11", typ: bitString{lb: 2, ub: 2, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1250"] = sequence{
		extensible: false,
		fields: []field{
			{name: "e-HARQ-Pattern-FDD-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "enhanced-4TxCodebook-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tdd-FDD-CA-PCellDuplex-r12", typ: bitString{lb: 2, ub: 2, extensible: false}, optional: true},
			{name: "phy-TDD-ReConfig-TDD-PCell-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "phy-TDD-ReConfig-FDD-PCell-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-FeedbackMode-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-SRS-PowerControl-SubframeSet-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-SubframeSet-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "noResourceRestrictionForTTIBundling-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "discoverySignalsInDeactSCell-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "naics-Capability-List-r12", typ: deferred{name: "NAICS-Capability-List-r12", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1280"] = sequence{
		extensible: false,
		fields: []field{
			{name: "alternativeTBS-Indices-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "aperiodicCSI-Reporting-r13", typ: bitString{lb: 2, ub: 2, extensible: false}, optional: true},
			{name: "codebook-HARQ-ACK-r13", typ: bitString{lb: 2, ub: 2, extensible: false}, optional: true},
			{name: "crossCarrierScheduling-B5C-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "fdd-HARQ-TimingTDD-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "maxNumberUpdatedCSI-Proc-r13", typ: integer{lb: 5, ub: 32, extensible: false}, optional: true},
			{name: "pucch-Format4-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pucch-Format5-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pucch-SCell-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "spatialBundling-HARQ-ACK-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedBlindDecoding-r13", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "maxNumberDecoding-r13", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
					{name: "pdcch-CandidateReductions-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "skipMonitoringDCI-Format0-1A-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "uci-PUSCH-Ext-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "crs-InterfMitigationTM10-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-CollisionHandling-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1320"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mimo-UE-Parameters-r13", typ: deferred{name: "MIMO-UE-Parameters-r13", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1330"] = sequence{
		extensible: false,
		fields: []field{
			{name: "cch-InterfMitigation-RefRecTypeA-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "cch-InterfMitigation-RefRecTypeB-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "cch-InterfMitigation-MaxNumCCs-r13", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
			{name: "crs-InterfMitigationTM1toTM9-r13", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ce-PUSCH-NB-MaxTBS-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-PDSCH-PUSCH-MaxBandwidth-r14", typ: enumerated{values: []string{"bw5", "bw20"}, extensible: false}, optional: true},
			{name: "ce-HARQ-AckBundling-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-PDSCH-TenProcesses-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-RetuningSymbols-r14", typ: enumerated{values: []string{"n0", "n1"}, extensible: false}, optional: true},
			{name: "ce-PDSCH-PUSCH-Enhancement-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-SchedulingEnhancement-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-SRS-Enhancement-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-PUCCH-Enhancement-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ce-ClosedLoopTxAntennaSelection-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tdd-SpecialSubframe-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tdd-TTI-Bundling-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dmrs-LessUpPTS-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mimo-UE-Parameters-v1430", typ: deferred{name: "MIMO-UE-Parameters-v1430", reg: eutraTypes}, optional: true},
			{name: "alternativeTBS-Index-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "feMBMS-Unicast-Parameters-r14", typ: deferred{name: "FeMBMS-Unicast-Parameters-r14", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1450"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ce-SRS-EnhancementWithoutComb4-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "crs-LessDwPTS-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "stti-SPT-Capabilities-r15", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "aperiodicCsi-ReportingSTTI-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "dmrs-BasedSPDCCH-MBSFN-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "dmrs-BasedSPDCCH-nonMBSFN-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "dmrs-PositionPattern-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "dmrs-SharingSubslotPDSCH-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "dmrs-RepetitionSubslotPDSCH-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "epdcch-SPT-differentCells-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "epdcch-STTI-differentCells-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "maxLayersSlotOrSubslotPUSCH-r15", typ: enumerated{values: []string{"oneLayer", "twoLayers", "fourLayers"}, extensible: false}, optional: true},
					{name: "maxNumberUpdatedCSI-Proc-SPT-r15", typ: integer{lb: 5, ub: 32, extensible: false}, optional: true},
					{name: "maxNumberUpdatedCSI-Proc-STTI-Comb77-r15", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
					{name: "maxNumberUpdatedCSI-Proc-STTI-Comb27-r15", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
					{name: "maxNumberUpdatedCSI-Proc-STTI-Comb22-Set1-r15", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
					{name: "maxNumberUpdatedCSI-Proc-STTI-Comb22-Set2-r15", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
					{name: "mimo-UE-ParametersSTTI-r15", typ: deferred{name: "MIMO-UE-Parameters-r13", reg: eutraTypes}, optional: true},
					{name: "mimo-UE-ParametersSTTI-v1530", typ: deferred{name: "MIMO-UE-Parameters-v1430", reg: eutraTypes}, optional: true},
					{name: "numberOfBlindDecodesUSS-r15", typ: integer{lb: 4, ub: 32, extensible: false}, optional: true},
					{name: "pdsch-SlotSubslotPDSCH-Decoding-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "powerUCI-SlotPUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "powerUCI-SubslotPUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "slotPDSCH-TxDiv-TM9and10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "subslotPDSCH-TxDiv-TM9and10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "spdcch-differentRS-types-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "srs-DCI7-TriggeringFS2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "sps-cyclicShift-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "spdcch-Reuse-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "sps-STTI-r15", typ: enumerated{values: []string{"slot", "subslot", "slotAndSubslot"}, extensible: false}, optional: true},
					{name: "tm8-slotPDSCH-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "tm9-slotSubslot-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "tm9-slotSubslotMBSFN-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "tm10-slotSubslot-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "tm10-slotSubslotMBSFN-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "txDiv-SPUCCH-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ul-AsyncHarqSharingDiff-TTI-Lengths-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "ce-Capabilities-r15", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "ce-CRS-IntfMitig-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-CQI-AlternativeTable-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-PDSCH-FlexibleStartPRB-CE-ModeA-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-PDSCH-FlexibleStartPRB-CE-ModeB-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-PDSCH-64QAM-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-PUSCH-FlexibleStartPRB-CE-ModeA-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-PUSCH-FlexibleStartPRB-CE-ModeB-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-PUSCH-SubPRB-Allocation-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-UL-HARQ-ACK-Feedback-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "shortCQI-ForSCellActivation-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mimo-CBSR-AdvancedCSI-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "crs-IntfMitig-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-PowerControlEnhancements-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "urllc-Capabilities-r15", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "pdsch-RepSubframe-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pdsch-RepSlot-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pdsch-RepSubslot-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pusch-SPS-MultiConfigSubframe-r15", typ: integer{lb: 0, ub: 6, extensible: false}, optional: true},
					{name: "pusch-SPS-MaxConfigSubframe-r15", typ: integer{lb: 0, ub: 31, extensible: false}, optional: true},
					{name: "pusch-SPS-MultiConfigSlot-r15", typ: integer{lb: 0, ub: 6, extensible: false}, optional: true},
					{name: "pusch-SPS-MaxConfigSlot-r15", typ: integer{lb: 0, ub: 31, extensible: false}, optional: true},
					{name: "pusch-SPS-MultiConfigSubslot-r15", typ: integer{lb: 0, ub: 6, extensible: false}, optional: true},
					{name: "pusch-SPS-MaxConfigSubslot-r15", typ: integer{lb: 0, ub: 31, extensible: false}, optional: true},
					{name: "pusch-SPS-SlotRepPCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pusch-SPS-SlotRepPSCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pusch-SPS-SlotRepSCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pusch-SPS-SubframeRepPCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pusch-SPS-SubframeRepPSCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pusch-SPS-SubframeRepSCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pusch-SPS-SubslotRepPCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pusch-SPS-SubslotRepPSCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pusch-SPS-SubslotRepSCell-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "semiStaticCFI-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "semiStaticCFI-Pattern-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "altMCS-Table-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1540"] = sequence{
		extensible: false,
		fields: []field{
			{name: "stti-SPT-Capabilities-v1540", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "slotPDSCH-TxDiv-TM8-r15", typ: enumerated{values: []string{"supported"}, extensible: false}},
				},
			}, optional: true},
			{name: "crs-IM-TM1-toTM9-OneRX-Port-v1540", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "cch-IM-RefRecTypeA-OneRX-Port-v1540", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1550"] = sequence{
		extensible: false,
		fields: []field{
			{name: "dmrs-OverheadReduction-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ce-Capabilities-v1610", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "ce-CSI-RS-Feedback-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-CSI-RS-FeedbackCodebookRestriction-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "crs-ChEstMPDCCH-CE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "crs-ChEstMPDCCH-CE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "crs-ChEstMPDCCH-CSI-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "crs-ChEstMPDCCH-ReciprocityTDD-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "etws-CMAS-RxInConnCE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "etws-CMAS-RxInConnCE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "mpdcch-InLteControlRegionCE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "mpdcch-InLteControlRegionCE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pdsch-InLteControlRegionCE-ModeA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "pdsch-InLteControlRegionCE-ModeB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "multiTB-Parameters-r16", typ: deferred{name: "CE-MultiTB-Parameters-r16", reg: eutraTypes}, optional: true},
					{name: "resourceResvParameters-r16", typ: deferred{name: "CE-ResourceResvParameters-r16", reg: eutraTypes}, optional: true},
				},
			}, optional: true},
			{name: "widebandPRG-Slot-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "widebandPRG-Subslot-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "widebandPRG-Subframe-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "addSRS-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "addSRS-FrequencyHopping-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "addSRS-AntennaSwitching-r16", typ: enumerated{values: []string{"useBasic"}, extensible: false}, optional: true},
					{name: "addSRS-CarrierSwitching-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "virtualCellID-BasicSRS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "virtualCellID-AddSRS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ce-Capabilities-v1700", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "ce-PDSCH-14HARQProcesses-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-PDSCH-14HARQProcesses-Alt2-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "ce-PDSCH-MaxTBS-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v1730"] = sequence{
		extensible: false,
		fields: []field{
			{name: "csi-SubframeSet2ForDormantSCell-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["PhyLayerParameters-v920"] = sequence{
		extensible: false,
		fields: []field{
			{name: "enhancedDualLayerFDD-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "enhancedDualLayerTDD-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["ProcessingTimelineSet-r15"] = enumerated{values: []string{"set1", "set2"}, extensible: false}

	eutraTypes["RAT-Type"] = enumerated{values: []string{"eutra", "utra", "geran-cs", "geran-ps", "cdma2000-1XRTT", "nr", "eutra-nr", "spare1"}, extensible: true}

	eutraTypes["RF-Parameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListEUTRA", typ: deferred{name: "SupportedBandListEUTRA", reg: eutraTypes}},
		},
	}

	eutraTypes["RF-Parameters-v1020"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandCombination-r10", typ: deferred{name: "SupportedBandCombination-r10", reg: eutraTypes}},
		},
	}

	eutraTypes["RF-Parameters-v1060"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandCombinationExt-r10", typ: deferred{name: "SupportedBandCombinationExt-r10", reg: eutraTypes}},
		},
	}

	eutraTypes["RF-Parameters-v1090"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandCombination-v1090", typ: deferred{name: "SupportedBandCombination-v1090", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1130"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandCombination-v1130", typ: deferred{name: "SupportedBandCombination-v1130", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1180"] = sequence{
		extensible: false,
		fields: []field{
			{name: "freqBandRetrieval-r11", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "requestedBands-r11", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "FreqBandIndicator-r11", reg: eutraTypes}}, optional: true},
			{name: "supportedBandCombinationAdd-r11", typ: deferred{name: "SupportedBandCombinationAdd-r11", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1250"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListEUTRA-v1250", typ: deferred{name: "SupportedBandListEUTRA-v1250", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombination-v1250", typ: deferred{name: "SupportedBandCombination-v1250", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationAdd-v1250", typ: deferred{name: "SupportedBandCombinationAdd-v1250", reg: eutraTypes}, optional: true},
			{name: "freqBandPriorityAdjustment-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1270"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandCombination-v1270", typ: deferred{name: "SupportedBandCombination-v1270", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationAdd-v1270", typ: deferred{name: "SupportedBandCombinationAdd-v1270", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eNB-RequestedParameters-r13", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "reducedIntNonContCombRequested-r13", typ: enumerated{values: []string{"true"}, extensible: false}, optional: true},
					{name: "requestedCCsDL-r13", typ: integer{lb: 2, ub: 32, extensible: false}, optional: true},
					{name: "requestedCCsUL-r13", typ: integer{lb: 2, ub: 32, extensible: false}, optional: true},
					{name: "skipFallbackCombRequested-r13", typ: enumerated{values: []string{"true"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "maximumCCsRetrieval-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "skipFallbackCombinations-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "reducedIntNonContComb-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedBandListEUTRA-v1310", typ: deferred{name: "SupportedBandListEUTRA-v1310", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationReduced-r13", typ: deferred{name: "SupportedBandCombinationReduced-r13", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1320"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandListEUTRA-v1320", typ: deferred{name: "SupportedBandListEUTRA-v1320", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombination-v1320", typ: deferred{name: "SupportedBandCombination-v1320", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationAdd-v1320", typ: deferred{name: "SupportedBandCombinationAdd-v1320", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationReduced-v1320", typ: deferred{name: "SupportedBandCombinationReduced-v1320", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandCombination-v1430", typ: deferred{name: "SupportedBandCombination-v1430", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationAdd-v1430", typ: deferred{name: "SupportedBandCombinationAdd-v1430", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationReduced-v1430", typ: deferred{name: "SupportedBandCombinationReduced-v1430", reg: eutraTypes}, optional: true},
			{name: "eNB-RequestedParameters-v1430", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "requestedDiffFallbackCombList-r14", typ: deferred{name: "BandCombinationList-r14", reg: eutraTypes}},
				},
			}, optional: true},
			{name: "diffFallbackCombReport-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1450"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandCombination-v1450", typ: deferred{name: "SupportedBandCombination-v1450", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationAdd-v1450", typ: deferred{name: "SupportedBandCombinationAdd-v1450", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationReduced-v1450", typ: deferred{name: "SupportedBandCombinationReduced-v1450", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sTTI-SPT-Supported-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedBandCombination-v1530", typ: deferred{name: "SupportedBandCombination-v1530", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationAdd-v1530", typ: deferred{name: "SupportedBandCombinationAdd-v1530", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationReduced-v1530", typ: deferred{name: "SupportedBandCombinationReduced-v1530", reg: eutraTypes}, optional: true},
			{name: "powerClass-14dBm-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1570"] = sequence{
		extensible: false,
		fields: []field{
			{name: "dl-1024QAM-ScalingFactor-r15", typ: enumerated{values: []string{"v1", "v1dot2", "v1dot25"}, extensible: false}},
			{name: "dl-1024QAM-TotalWeightedLayers-r15", typ: integer{lb: 0, ub: 10, extensible: false}},
		},
	}

	eutraTypes["RF-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandCombination-v1610", typ: deferred{name: "SupportedBandCombination-v1610", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationAdd-v1610", typ: deferred{name: "SupportedBandCombinationAdd-v1610", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationReduced-v1610", typ: deferred{name: "SupportedBandCombinationReduced-v1610", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1630"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedBandCombination-v1630", typ: deferred{name: "SupportedBandCombination-v1630", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationAdd-v1630", typ: deferred{name: "SupportedBandCombinationAdd-v1630", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationReduced-v1630", typ: deferred{name: "SupportedBandCombinationReduced-v1630", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RF-Parameters-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "multiNS-PmaxAerial-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedBandListEUTRA-v1800", typ: deferred{name: "SupportedBandListEUTRA-v1800", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombination-v1800", typ: deferred{name: "SupportedBandCombination-v1800", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationAdd-v1800", typ: deferred{name: "SupportedBandCombinationAdd-v1800", reg: eutraTypes}, optional: true},
			{name: "supportedBandCombinationReduced-v1800", typ: deferred{name: "SupportedBandCombinationReduced-v1800", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["RLC-Parameters-r12"] = sequence{
		extensible: false,
		fields: []field{
			{name: "extended-RLC-LI-Field-r12", typ: enumerated{values: []string{"supported"}, extensible: false}},
		},
	}

	eutraTypes["RLC-Parameters-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "extendedRLC-SN-SO-Field-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["RLC-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "extendedPollByte-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["RLC-Parameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "flexibleUM-AM-Combinations-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rlc-AM-Ooo-Delivery-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rlc-UM-Ooo-Delivery-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
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

	eutraTypes["SCPTM-Parameters-r13"] = sequence{
		extensible: false,
		fields: []field{
			{name: "scptm-ParallelReception-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "scptm-SCell-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "scptm-NonServingCell-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "scptm-AsyncDC-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SL-A2X-BandCombinationParameters-r18"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SL-A2X-BandParameters-r18", reg: eutraTypes}}

	eutraTypes["SL-A2X-BandParameters-r18"] = sequence{
		extensible: false,
		fields: []field{
			{name: "a2x-FreqBandEUTRA-r18", typ: deferred{name: "FreqBandIndicator-r11", reg: eutraTypes}},
			{name: "a2x-BandParametersTxSL-r18", typ: deferred{name: "BandParametersTxA2X-r18", reg: eutraTypes}, optional: true},
			{name: "a2x-BandParametersRxSL-r18", typ: deferred{name: "BandParametersRxA2X-r18", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["SL-A2X-SupportedBandCombination-r18"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "SL-A2X-BandCombinationParameters-r18", reg: eutraTypes}}

	eutraTypes["SL-Parameters-r12"] = sequence{
		extensible: false,
		fields: []field{
			{name: "commSimultaneousTx-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "commSupportedBands-r12", typ: deferred{name: "FreqBandIndicatorListEUTRA-r12", reg: eutraTypes}, optional: true},
			{name: "discSupportedBands-r12", typ: deferred{name: "SupportedBandInfoList-r12", reg: eutraTypes}, optional: true},
			{name: "discScheduledResourceAlloc-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "disc-UE-SelectedResourceAlloc-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "disc-SLSS-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "discSupportedProc-r12", typ: enumerated{values: []string{"n50", "n400"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SL-Parameters-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "discSysInfoReporting-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "commMultipleTx-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "discInterFreqTx-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "discPeriodicSLSS-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SL-Parameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "zoneBasedPoolSelection-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-AutonomousWithFullSensing-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-AutonomousWithPartialSensing-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sl-CongestionControl-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "v2x-TxWithShortResvInterval-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "v2x-numberTxRxTiming-r14", typ: integer{lb: 1, ub: 16, extensible: false}, optional: true},
			{name: "v2x-nonAdjacentPSCCH-PSSCH-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "slss-TxRx-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "v2x-SupportedBandCombinationList-r14", typ: deferred{name: "V2X-SupportedBandCombination-r14", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["SL-Parameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "slss-SupportedTxFreq-r15", typ: enumerated{values: []string{"single", "multiple"}, extensible: false}, optional: true},
			{name: "sl-64QAM-Tx-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sl-TxDiversity-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-CategorySL-r15", typ: deferred{name: "UE-CategorySL-r15", reg: eutraTypes}, optional: true},
			{name: "v2x-SupportedBandCombinationList-v1530", typ: deferred{name: "V2X-SupportedBandCombination-v1530", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["SL-Parameters-v1540"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sl-64QAM-Rx-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sl-RateMatchingTBSScaling-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sl-LowT2min-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "v2x-SensingReportingMode3-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SL-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sl-ParameterNR-r16", typ: octetString{hasUB: false}, optional: true},
			{name: "dummy", typ: deferred{name: "V2X-SupportedBandCombinationEUTRA-NR-r16", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["SL-Parameters-v1630"] = sequence{
		extensible: false,
		fields: []field{
			{name: "v2x-SupportedBandCombinationListEUTRA-NR-r16", typ: deferred{name: "V2X-SupportedBandCombinationEUTRA-NR-v1630", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["SL-Parameters-v1710"] = sequence{
		extensible: false,
		fields: []field{
			{name: "v2x-SupportedBandCombinationListEUTRA-NR-v1710", typ: deferred{name: "V2X-SupportedBandCombinationEUTRA-NR-v1710", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["SL-Parameters-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sl-A2X-SupportedBandCombinationList-r18", typ: deferred{name: "SL-A2X-SupportedBandCombination-r18", reg: eutraTypes}, optional: true},
			{name: "sl-A2X-Service-r18", typ: enumerated{values: []string{"brid", "daa", "bridAndDAA"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SON-Parameters-r9"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rach-Report-r9", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SON-Parameters-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rach-ReportForNR-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SPT-Parameters-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "frameStructureType-SPT-r15", typ: bitString{lb: 3, ub: 3, extensible: false}, optional: true},
			{name: "maxNumberCCs-SPT-r15", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
		},
	}

	eutraTypes["SRS-CapabilityPerBandPair-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "retuningInfo", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "rf-RetuningTimeDL-r14", typ: enumerated{values: []string{"n0", "n0dot5", "n1", "n1dot5", "n2", "n2dot5", "n3", "n3dot5", "n4", "n4dot5", "n5", "n5dot5", "n6", "n6dot5", "n7", "spare1"}, extensible: false}, optional: true},
					{name: "rf-RetuningTimeUL-r14", typ: enumerated{values: []string{"n0", "n0dot5", "n1", "n1dot5", "n2", "n2dot5", "n3", "n3dot5", "n4", "n4dot5", "n5", "n5dot5", "n6", "n6dot5", "n7", "spare1"}, extensible: false}, optional: true},
				},
			}},
		},
	}

	eutraTypes["SRS-CapabilityPerBandPair-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "addSRS-CarrierSwitching-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["STTI-SPT-BandParameters-r15"] = sequence{
		extensible: true,
		fields: []field{
			{name: "dl-1024QAM-Slot-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dl-1024QAM-SubslotTA-1-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dl-1024QAM-SubslotTA-2-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simultaneousTx-differentTx-duration-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sTTI-CA-MIMO-ParametersDL-r15", typ: deferred{name: "CA-MIMO-ParametersDL-r15", reg: eutraTypes}, optional: true},
			{name: "sTTI-CA-MIMO-ParametersUL-r15", typ: deferred{name: "CA-MIMO-ParametersUL-r15", reg: eutraTypes}},
			{name: "sTTI-FD-MIMO-Coexistence", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sTTI-MIMO-CA-ParametersPerBoBCs-r15", typ: deferred{name: "MIMO-CA-ParametersPerBoBC-r13", reg: eutraTypes}, optional: true},
			{name: "sTTI-MIMO-CA-ParametersPerBoBCs-v1530", typ: deferred{name: "MIMO-CA-ParametersPerBoBC-v1430", reg: eutraTypes}, optional: true},
			{name: "sTTI-SupportedCombinations-r15", typ: deferred{name: "STTI-SupportedCombinations-r15", reg: eutraTypes}, optional: true},
			{name: "sTTI-SupportedCSI-Proc-r15", typ: enumerated{values: []string{"n1", "n3", "n4"}, extensible: false}, optional: true},
			{name: "ul-256QAM-Slot-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-256QAM-Subslot-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["STTI-SupportedCombinations-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "combination-22-r15", typ: deferred{name: "DL-UL-CCs-r15", reg: eutraTypes}, optional: true},
			{name: "combination-77-r15", typ: deferred{name: "DL-UL-CCs-r15", reg: eutraTypes}, optional: true},
			{name: "combination-27-r15", typ: deferred{name: "DL-UL-CCs-r15", reg: eutraTypes}, optional: true},
			{name: "combination-22-27-r15", typ: sequenceOf{lb: 1, ub: 2, elem: deferred{name: "DL-UL-CCs-r15", reg: eutraTypes}}, optional: true},
			{name: "combination-77-22-r15", typ: sequenceOf{lb: 1, ub: 2, elem: deferred{name: "DL-UL-CCs-r15", reg: eutraTypes}}, optional: true},
			{name: "combination-77-27-r15", typ: sequenceOf{lb: 1, ub: 2, elem: deferred{name: "DL-UL-CCs-r15", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["ScalingFactorSidelink-r16"] = enumerated{values: []string{"f0p4", "f0p75", "f0p8", "f1"}, extensible: false}

	eutraTypes["SharedSpectrumMeasNR-r17"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nr-RSSI-ChannelOccupancyReporting-r17", typ: boolean{}},
		},
	}

	eutraTypes["SkipSubframeProcessing-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "skipProcessingDL-Slot-r15", typ: integer{lb: 0, ub: 3, extensible: false}, optional: true},
			{name: "skipProcessingDL-SubSlot-r15", typ: integer{lb: 0, ub: 3, extensible: false}, optional: true},
			{name: "skipProcessingUL-Slot-r15", typ: integer{lb: 0, ub: 3, extensible: false}, optional: true},
			{name: "skipProcessingUL-SubSlot-r15", typ: integer{lb: 0, ub: 3, extensible: false}, optional: true},
		},
	}

	eutraTypes["SupportedBandCombination-r10"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-r10", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1090"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1090", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1130"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1130", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1250"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1250", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1270"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1270", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1320"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1320", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1430"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1430", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1450"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1450", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1530"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1530", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1610"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1610", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1630"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1630", reg: eutraTypes}}

	eutraTypes["SupportedBandCombination-v1800"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParameters-v1800", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-r11"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-r11", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-v1250"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-v1250", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-v1270"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-v1270", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-v1320"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-v1320", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-v1430"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-v1430", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-v1450"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-v1450", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-v1530"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-v1530", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-v1610"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-v1610", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-v1630"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-v1630", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationAdd-v1800"] = sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandCombinationParameters-v1800", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationExt-r10"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "BandCombinationParametersExt-r10", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationReduced-r13"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "BandCombinationParameters-r13", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationReduced-v1320"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "BandCombinationParameters-v1320", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationReduced-v1430"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "BandCombinationParameters-v1430", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationReduced-v1450"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "BandCombinationParameters-v1450", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationReduced-v1530"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "BandCombinationParameters-v1530", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationReduced-v1610"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "BandCombinationParameters-v1610", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationReduced-v1630"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "BandCombinationParameters-v1630", reg: eutraTypes}}

	eutraTypes["SupportedBandCombinationReduced-v1800"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "BandCombinationParameters-v1800", reg: eutraTypes}}

	eutraTypes["SupportedBandEUTRA"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandEUTRA", typ: deferred{name: "FreqBandIndicator", reg: eutraTypes}},
			{name: "halfDuplex", typ: boolean{}},
		},
	}

	eutraTypes["SupportedBandEUTRA-v1250"] = sequence{
		extensible: false,
		fields: []field{
			{name: "dl-256QAM-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-64QAM-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SupportedBandEUTRA-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-PowerClass-5-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SupportedBandEUTRA-v1320"] = sequence{
		extensible: false,
		fields: []field{
			{name: "intraFreq-CE-NeedForGaps-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-PowerClass-N-r13", typ: enumerated{values: []string{"class1", "class2", "class4"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SupportedBandEUTRA-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "lowerMSD-MRDC-r18", typ: sequenceOf{lb: 1, ub: 256, elem: deferred{name: "LowerMSD-MRDC-r18", reg: eutraTypes}}, optional: true},
		},
	}

	eutraTypes["SupportedBandGERAN"] = enumerated{values: []string{"gsm450", "gsm480", "gsm710", "gsm750", "gsm810", "gsm850", "gsm900P", "gsm900E", "gsm900R", "gsm1800", "gsm1900", "spare5", "spare4", "spare3", "spare2", "spare1"}, extensible: true}

	eutraTypes["SupportedBandInfo-r12"] = sequence{
		extensible: false,
		fields: []field{
			{name: "support-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["SupportedBandInfoList-r12"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandInfo-r12", reg: eutraTypes}}

	eutraTypes["SupportedBandList1XRTT"] = sequenceOf{lb: 1, ub: 32, elem: deferred{name: "BandclassCDMA2000", reg: eutraTypes}}

	eutraTypes["SupportedBandListEUTRA"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandEUTRA", reg: eutraTypes}}

	eutraTypes["SupportedBandListEUTRA-v1250"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandEUTRA-v1250", reg: eutraTypes}}

	eutraTypes["SupportedBandListEUTRA-v1310"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandEUTRA-v1310", reg: eutraTypes}}

	eutraTypes["SupportedBandListEUTRA-v1320"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandEUTRA-v1320", reg: eutraTypes}}

	eutraTypes["SupportedBandListEUTRA-v1800"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandEUTRA-v1800", reg: eutraTypes}}

	eutraTypes["SupportedBandListGERAN"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandGERAN", reg: eutraTypes}}

	eutraTypes["SupportedBandListHRPD"] = sequenceOf{lb: 1, ub: 32, elem: deferred{name: "BandclassCDMA2000", reg: eutraTypes}}

	eutraTypes["SupportedBandListNR-r15"] = sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "SupportedBandNR-r15", reg: eutraTypes}}

	eutraTypes["SupportedBandListUTRA-FDD"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandUTRA-FDD", reg: eutraTypes}}

	eutraTypes["SupportedBandListUTRA-TDD128"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandUTRA-TDD128", reg: eutraTypes}}

	eutraTypes["SupportedBandListUTRA-TDD384"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandUTRA-TDD384", reg: eutraTypes}}

	eutraTypes["SupportedBandListUTRA-TDD768"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "SupportedBandUTRA-TDD768", reg: eutraTypes}}

	eutraTypes["SupportedBandNR-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandNR-r15", typ: deferred{name: "FreqBandIndicatorNR-r15", reg: eutraTypes}},
		},
	}

	eutraTypes["SupportedBandUTRA-FDD"] = enumerated{values: []string{"bandI", "bandII", "bandIII", "bandIV", "bandV", "bandVI", "bandVII", "bandVIII", "bandIX", "bandX", "bandXI", "bandXII", "bandXIII", "bandXIV", "bandXV", "bandXVI"}, extValues: []string{"bandXVII-8a0", "bandXVIII-8a0", "bandXIX-8a0", "bandXX-8a0", "bandXXI-8a0", "bandXXII-8a0", "bandXXIII-8a0", "bandXXIV-8a0", "bandXXV-8a0", "bandXXVI-8a0", "bandXXVII-8a0", "bandXXVIII-8a0", "bandXXIX-8a0", "bandXXX-8a0", "bandXXXI-8a0", "bandXXXII-8a0"}, extensible: true}

	eutraTypes["SupportedBandUTRA-TDD128"] = enumerated{values: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}, extensible: true}

	eutraTypes["SupportedBandUTRA-TDD384"] = enumerated{values: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}, extensible: true}

	eutraTypes["SupportedBandUTRA-TDD768"] = enumerated{values: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p"}, extensible: true}

	eutraTypes["SupportedBandwidthCombinationSet-r10"] = bitString{lb: 1, ub: 32, extensible: false}

	eutraTypes["SupportedOperatorDic-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "versionOfDictionary-r15", typ: integer{lb: 0, ub: 15, extensible: false}},
			{name: "associatedPLMN-ID-r15", typ: deferred{name: "PLMN-Identity", reg: eutraTypes}},
		},
	}

	eutraTypes["SupportedUDC-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedStandardDic-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedOperatorDic-r15", typ: deferred{name: "SupportedOperatorDic-r15", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-BasedNetwPerfMeasParameters-r10"] = sequence{
		extensible: false,
		fields: []field{
			{name: "loggedMeasurementsIdle-r10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "standaloneGNSS-Location-r10", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["UE-BasedNetwPerfMeasParameters-v1250"] = sequence{
		extensible: false,
		fields: []field{
			{name: "loggedMBSFNMeasurements-r12", typ: enumerated{values: []string{"supported"}, extensible: false}},
		},
	}

	eutraTypes["UE-BasedNetwPerfMeasParameters-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "locationReport-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["UE-BasedNetwPerfMeasParameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "loggedMeasBT-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "loggedMeasWLAN-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "immMeasBT-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "immMeasWLAN-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["UE-BasedNetwPerfMeasParameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ul-PDCP-AvgDelay-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["UE-BasedNetwPerfMeasParameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "loggedMeasIdleEventL1-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "loggedMeasIdleEventOutOfCoverage-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "loggedMeasUncomBarPre-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "immMeasUncomBarPre-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["UE-BasedNetwPerfMeasParameters-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sigBasedEUTRA-LoggedMeasOverrideProtect-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["UE-CapabilityRAT-Container"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rat-Type", typ: deferred{name: "RAT-Type", reg: eutraTypes}},
			{name: "ueCapabilityRAT-Container", typ: octetString{hasUB: false}},
		},
	}

	eutraTypes["UE-CapabilityRAT-ContainerList"] = sequenceOf{lb: 0, ub: 8, elem: deferred{name: "UE-CapabilityRAT-Container", reg: eutraTypes}}

	eutraTypes["UE-CategorySL-r15"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-CategorySL-C-TX-r15", typ: integer{lb: 1, ub: 5, extensible: false}},
			{name: "ue-CategorySL-C-RX-r15", typ: integer{lb: 1, ub: 4, extensible: false}},
		},
	}

	eutraTypes["UE-EUTRA-Capability"] = sequence{
		extensible: false,
		fields: []field{
			{name: "accessStratumRelease", typ: deferred{name: "AccessStratumRelease", reg: eutraTypes}},
			{name: "ue-Category", typ: integer{lb: 1, ub: 5, extensible: false}},
			{name: "pdcp-Parameters", typ: deferred{name: "PDCP-Parameters", reg: eutraTypes}},
			{name: "phyLayerParameters", typ: deferred{name: "PhyLayerParameters", reg: eutraTypes}},
			{name: "rf-Parameters", typ: deferred{name: "RF-Parameters", reg: eutraTypes}},
			{name: "measParameters", typ: deferred{name: "MeasParameters", reg: eutraTypes}},
			{name: "featureGroupIndicators", typ: bitString{lb: 32, ub: 32, extensible: false}, optional: true},
			{name: "interRAT-Parameters", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "utraFDD", typ: deferred{name: "IRAT-ParametersUTRA-FDD", reg: eutraTypes}, optional: true},
					{name: "utraTDD128", typ: deferred{name: "IRAT-ParametersUTRA-TDD128", reg: eutraTypes}, optional: true},
					{name: "utraTDD384", typ: deferred{name: "IRAT-ParametersUTRA-TDD384", reg: eutraTypes}, optional: true},
					{name: "utraTDD768", typ: deferred{name: "IRAT-ParametersUTRA-TDD768", reg: eutraTypes}, optional: true},
					{name: "geran", typ: deferred{name: "IRAT-ParametersGERAN", reg: eutraTypes}, optional: true},
					{name: "cdma2000-HRPD", typ: deferred{name: "IRAT-ParametersCDMA2000-HRPD", reg: eutraTypes}, optional: true},
					{name: "cdma2000-1xRTT", typ: deferred{name: "IRAT-ParametersCDMA2000-1XRTT", reg: eutraTypes}, optional: true},
				},
			}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v920-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1020-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-Category-v1020", typ: integer{lb: 6, ub: 8, extensible: false}, optional: true},
			{name: "phyLayerParameters-v1020", typ: deferred{name: "PhyLayerParameters-v1020", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1020", typ: deferred{name: "RF-Parameters-v1020", reg: eutraTypes}, optional: true},
			{name: "measParameters-v1020", typ: deferred{name: "MeasParameters-v1020", reg: eutraTypes}, optional: true},
			{name: "featureGroupIndRel10-r10", typ: bitString{lb: 32, ub: 32, extensible: false}, optional: true},
			{name: "interRAT-ParametersCDMA2000-v1020", typ: deferred{name: "IRAT-ParametersCDMA2000-1XRTT-v1020", reg: eutraTypes}, optional: true},
			{name: "ue-BasedNetwPerfMeasParameters-r10", typ: deferred{name: "UE-BasedNetwPerfMeasParameters-r10", reg: eutraTypes}, optional: true},
			{name: "interRAT-ParametersUTRA-TDD-v1020", typ: deferred{name: "IRAT-ParametersUTRA-TDD-v1020", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1060-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1060-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1060", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1060", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1060", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1060", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1060", typ: deferred{name: "RF-Parameters-v1060", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1090-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1090-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rf-Parameters-v1090", typ: deferred{name: "RF-Parameters-v1090", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1130-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1130-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdcp-Parameters-v1130", typ: deferred{name: "PDCP-Parameters-v1130", reg: eutraTypes}},
			{name: "phyLayerParameters-v1130", typ: deferred{name: "PhyLayerParameters-v1130", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1130", typ: deferred{name: "RF-Parameters-v1130", reg: eutraTypes}},
			{name: "measParameters-v1130", typ: deferred{name: "MeasParameters-v1130", reg: eutraTypes}},
			{name: "interRAT-ParametersCDMA2000-v1130", typ: deferred{name: "IRAT-ParametersCDMA2000-v1130", reg: eutraTypes}},
			{name: "otherParameters-r11", typ: deferred{name: "Other-Parameters-r11", reg: eutraTypes}},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1130", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1130", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1130", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1130", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1170-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1170-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1170", typ: deferred{name: "PhyLayerParameters-v1170", reg: eutraTypes}, optional: true},
			{name: "ue-Category-v1170", typ: integer{lb: 9, ub: 10, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1180-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1180-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rf-Parameters-v1180", typ: deferred{name: "RF-Parameters-v1180", reg: eutraTypes}, optional: true},
			{name: "mbms-Parameters-r11", typ: deferred{name: "MBMS-Parameters-r11", reg: eutraTypes}, optional: true},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1180", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1180", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1180", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1180", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v11a0-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v11a0-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-Category-v11a0", typ: integer{lb: 11, ub: 12, extensible: false}, optional: true},
			{name: "measParameters-v11a0", typ: deferred{name: "MeasParameters-v11a0", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1250-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1250-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1250", typ: deferred{name: "PhyLayerParameters-v1250", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1250", typ: deferred{name: "RF-Parameters-v1250", reg: eutraTypes}, optional: true},
			{name: "rlc-Parameters-r12", typ: deferred{name: "RLC-Parameters-r12", reg: eutraTypes}, optional: true},
			{name: "ue-BasedNetwPerfMeasParameters-v1250", typ: deferred{name: "UE-BasedNetwPerfMeasParameters-v1250", reg: eutraTypes}, optional: true},
			{name: "ue-CategoryDL-r12", typ: integer{lb: 0, ub: 14, extensible: false}, optional: true},
			{name: "ue-CategoryUL-r12", typ: integer{lb: 0, ub: 13, extensible: false}, optional: true},
			{name: "wlan-IW-Parameters-r12", typ: deferred{name: "WLAN-IW-Parameters-r12", reg: eutraTypes}, optional: true},
			{name: "measParameters-v1250", typ: deferred{name: "MeasParameters-v1250", reg: eutraTypes}, optional: true},
			{name: "dc-Parameters-r12", typ: deferred{name: "DC-Parameters-r12", reg: eutraTypes}, optional: true},
			{name: "mbms-Parameters-v1250", typ: deferred{name: "MBMS-Parameters-v1250", reg: eutraTypes}, optional: true},
			{name: "mac-Parameters-r12", typ: deferred{name: "MAC-Parameters-r12", reg: eutraTypes}, optional: true},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1250", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1250", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1250", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1250", reg: eutraTypes}, optional: true},
			{name: "sl-Parameters-r12", typ: deferred{name: "SL-Parameters-r12", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1260-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1260-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-CategoryDL-v1260", typ: integer{lb: 15, ub: 16, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1270-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1270-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rf-Parameters-v1270", typ: deferred{name: "RF-Parameters-v1270", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1280-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1280-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1280", typ: deferred{name: "PhyLayerParameters-v1280", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1310-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1310-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-CategoryDL-v1310", typ: enumerated{values: []string{"n17", "m1"}, extensible: false}, optional: true},
			{name: "ue-CategoryUL-v1310", typ: enumerated{values: []string{"n14", "m1"}, extensible: false}, optional: true},
			{name: "pdcp-Parameters-v1310", typ: deferred{name: "PDCP-Parameters-v1310", reg: eutraTypes}},
			{name: "rlc-Parameters-v1310", typ: deferred{name: "RLC-Parameters-v1310", reg: eutraTypes}},
			{name: "mac-Parameters-v1310", typ: deferred{name: "MAC-Parameters-v1310", reg: eutraTypes}, optional: true},
			{name: "phyLayerParameters-v1310", typ: deferred{name: "PhyLayerParameters-v1310", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1310", typ: deferred{name: "RF-Parameters-v1310", reg: eutraTypes}, optional: true},
			{name: "measParameters-v1310", typ: deferred{name: "MeasParameters-v1310", reg: eutraTypes}, optional: true},
			{name: "dc-Parameters-v1310", typ: deferred{name: "DC-Parameters-v1310", reg: eutraTypes}, optional: true},
			{name: "sl-Parameters-v1310", typ: deferred{name: "SL-Parameters-v1310", reg: eutraTypes}, optional: true},
			{name: "scptm-Parameters-r13", typ: deferred{name: "SCPTM-Parameters-r13", reg: eutraTypes}, optional: true},
			{name: "ce-Parameters-r13", typ: deferred{name: "CE-Parameters-r13", reg: eutraTypes}, optional: true},
			{name: "interRAT-ParametersWLAN-r13", typ: deferred{name: "IRAT-ParametersWLAN-r13", reg: eutraTypes}},
			{name: "laa-Parameters-r13", typ: deferred{name: "LAA-Parameters-r13", reg: eutraTypes}, optional: true},
			{name: "lwa-Parameters-r13", typ: deferred{name: "LWA-Parameters-r13", reg: eutraTypes}, optional: true},
			{name: "wlan-IW-Parameters-v1310", typ: deferred{name: "WLAN-IW-Parameters-v1310", reg: eutraTypes}},
			{name: "lwip-Parameters-r13", typ: deferred{name: "LWIP-Parameters-r13", reg: eutraTypes}},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1310", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1310", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1310", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1310", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1320-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1320-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ce-Parameters-v1320", typ: deferred{name: "CE-Parameters-v1320", reg: eutraTypes}, optional: true},
			{name: "phyLayerParameters-v1320", typ: deferred{name: "PhyLayerParameters-v1320", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1320", typ: deferred{name: "RF-Parameters-v1320", reg: eutraTypes}, optional: true},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1320", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1320", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1320", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1320", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1330-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1330-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-CategoryDL-v1330", typ: integer{lb: 18, ub: 19, extensible: false}, optional: true},
			{name: "phyLayerParameters-v1330", typ: deferred{name: "PhyLayerParameters-v1330", reg: eutraTypes}, optional: true},
			{name: "ue-CE-NeedULGaps-r13", typ: enumerated{values: []string{"true"}, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1340-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1340-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-CategoryUL-v1340", typ: integer{lb: 15, ub: 15, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1350-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1350-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-CategoryDL-v1350", typ: enumerated{values: []string{"oneBis"}, extensible: false}, optional: true},
			{name: "ue-CategoryUL-v1350", typ: enumerated{values: []string{"oneBis"}, extensible: false}, optional: true},
			{name: "ce-Parameters-v1350", typ: deferred{name: "CE-Parameters-v1350", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1360-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1360-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "other-Parameters-v1360", typ: deferred{name: "Other-Parameters-v1360", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1430-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1430-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1430", typ: deferred{name: "PhyLayerParameters-v1430", reg: eutraTypes}},
			{name: "ue-CategoryDL-v1430", typ: enumerated{values: []string{"m2"}, extensible: false}, optional: true},
			{name: "ue-CategoryUL-v1430", typ: enumerated{values: []string{"n16", "n17", "n18", "n19", "n20", "m2"}, extensible: false}, optional: true},
			{name: "ue-CategoryUL-v1430b", typ: enumerated{values: []string{"n21"}, extensible: false}, optional: true},
			{name: "mac-Parameters-v1430", typ: deferred{name: "MAC-Parameters-v1430", reg: eutraTypes}, optional: true},
			{name: "measParameters-v1430", typ: deferred{name: "MeasParameters-v1430", reg: eutraTypes}, optional: true},
			{name: "pdcp-Parameters-v1430", typ: deferred{name: "PDCP-Parameters-v1430", reg: eutraTypes}, optional: true},
			{name: "rlc-Parameters-v1430", typ: deferred{name: "RLC-Parameters-v1430", reg: eutraTypes}},
			{name: "rf-Parameters-v1430", typ: deferred{name: "RF-Parameters-v1430", reg: eutraTypes}, optional: true},
			{name: "laa-Parameters-v1430", typ: deferred{name: "LAA-Parameters-v1430", reg: eutraTypes}, optional: true},
			{name: "lwa-Parameters-v1430", typ: deferred{name: "LWA-Parameters-v1430", reg: eutraTypes}, optional: true},
			{name: "lwip-Parameters-v1430", typ: deferred{name: "LWIP-Parameters-v1430", reg: eutraTypes}, optional: true},
			{name: "otherParameters-v1430", typ: deferred{name: "Other-Parameters-v1430", reg: eutraTypes}},
			{name: "mmtel-Parameters-r14", typ: deferred{name: "MMTEL-Parameters-r14", reg: eutraTypes}, optional: true},
			{name: "mobilityParameters-r14", typ: deferred{name: "MobilityParameters-r14", reg: eutraTypes}, optional: true},
			{name: "ce-Parameters-v1430", typ: deferred{name: "CE-Parameters-v1430", reg: eutraTypes}},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1430", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1430", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1430", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1430", reg: eutraTypes}, optional: true},
			{name: "mbms-Parameters-v1430", typ: deferred{name: "MBMS-Parameters-v1430", reg: eutraTypes}, optional: true},
			{name: "sl-Parameters-v1430", typ: deferred{name: "SL-Parameters-v1430", reg: eutraTypes}, optional: true},
			{name: "ue-BasedNetwPerfMeasParameters-v1430", typ: deferred{name: "UE-BasedNetwPerfMeasParameters-v1430", reg: eutraTypes}, optional: true},
			{name: "highSpeedEnhParameters-r14", typ: deferred{name: "HighSpeedEnhParameters-r14", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1440-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1440-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "lwa-Parameters-v1440", typ: deferred{name: "LWA-Parameters-v1440", reg: eutraTypes}},
			{name: "mac-Parameters-v1440", typ: deferred{name: "MAC-Parameters-v1440", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1450-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1450-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1450", typ: deferred{name: "PhyLayerParameters-v1450", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1450", typ: deferred{name: "RF-Parameters-v1450", reg: eutraTypes}, optional: true},
			{name: "otherParameters-v1450", typ: deferred{name: "OtherParameters-v1450", reg: eutraTypes}},
			{name: "ue-CategoryDL-v1450", typ: integer{lb: 20, ub: 20, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1460-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1460-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-CategoryDL-v1460", typ: integer{lb: 21, ub: 21, extensible: false}, optional: true},
			{name: "otherParameters-v1460", typ: deferred{name: "Other-Parameters-v1460", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1510-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1510-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "irat-ParametersNR-r15", typ: deferred{name: "IRAT-ParametersNR-r15", reg: eutraTypes}, optional: true},
			{name: "featureSetsEUTRA-r15", typ: deferred{name: "FeatureSetsEUTRA-r15", reg: eutraTypes}, optional: true},
			{name: "pdcp-ParametersNR-r15", typ: deferred{name: "PDCP-ParametersNR-r15", reg: eutraTypes}, optional: true},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1510", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1510", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1510", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1510", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1520-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1520-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measParameters-v1520", typ: deferred{name: "MeasParameters-v1520", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1530-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1530-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measParameters-v1530", typ: deferred{name: "MeasParameters-v1530", reg: eutraTypes}, optional: true},
			{name: "otherParameters-v1530", typ: deferred{name: "Other-Parameters-v1530", reg: eutraTypes}, optional: true},
			{name: "neighCellSI-AcquisitionParameters-v1530", typ: deferred{name: "NeighCellSI-AcquisitionParameters-v1530", reg: eutraTypes}, optional: true},
			{name: "mac-Parameters-v1530", typ: deferred{name: "MAC-Parameters-v1530", reg: eutraTypes}, optional: true},
			{name: "phyLayerParameters-v1530", typ: deferred{name: "PhyLayerParameters-v1530", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1530", typ: deferred{name: "RF-Parameters-v1530", reg: eutraTypes}, optional: true},
			{name: "pdcp-Parameters-v1530", typ: deferred{name: "PDCP-Parameters-v1530", reg: eutraTypes}, optional: true},
			{name: "ue-CategoryDL-v1530", typ: integer{lb: 22, ub: 26, extensible: false}, optional: true},
			{name: "ue-BasedNetwPerfMeasParameters-v1530", typ: deferred{name: "UE-BasedNetwPerfMeasParameters-v1530", reg: eutraTypes}, optional: true},
			{name: "rlc-Parameters-v1530", typ: deferred{name: "RLC-Parameters-v1530", reg: eutraTypes}, optional: true},
			{name: "sl-Parameters-v1530", typ: deferred{name: "SL-Parameters-v1530", reg: eutraTypes}, optional: true},
			{name: "extendedNumberOfDRBs-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "reducedCP-Latency-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "laa-Parameters-v1530", typ: deferred{name: "LAA-Parameters-v1530", reg: eutraTypes}, optional: true},
			{name: "ue-CategoryUL-v1530", typ: integer{lb: 22, ub: 26, extensible: false}, optional: true},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1530", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1530", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1530", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1530", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1540-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1540-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1540", typ: deferred{name: "PhyLayerParameters-v1540", reg: eutraTypes}, optional: true},
			{name: "otherParameters-v1540", typ: deferred{name: "Other-Parameters-v1540", reg: eutraTypes}},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1540", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1540", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1540", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1540", reg: eutraTypes}, optional: true},
			{name: "sl-Parameters-v1540", typ: deferred{name: "SL-Parameters-v1540", reg: eutraTypes}, optional: true},
			{name: "irat-ParametersNR-v1540", typ: deferred{name: "IRAT-ParametersNR-v1540", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1550-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1550-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "neighCellSI-AcquisitionParameters-v1550", typ: deferred{name: "NeighCellSI-AcquisitionParameters-v1550", reg: eutraTypes}, optional: true},
			{name: "phyLayerParameters-v1550", typ: deferred{name: "PhyLayerParameters-v1550", reg: eutraTypes}},
			{name: "mac-Parameters-v1550", typ: deferred{name: "MAC-Parameters-v1550", reg: eutraTypes}},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1550", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1550", reg: eutraTypes}},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1550", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1550", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1560-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1560-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdcp-ParametersNR-v1560", typ: deferred{name: "PDCP-ParametersNR-v1560", reg: eutraTypes}},
			{name: "irat-ParametersNR-v1560", typ: deferred{name: "IRAT-ParametersNR-v1560", reg: eutraTypes}},
			{name: "appliedCapabilityFilterCommon-r15", typ: octetString{hasUB: false}, optional: true},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1560", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1560", reg: eutraTypes}},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1560", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1560", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1570-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1570-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rf-Parameters-v1570", typ: deferred{name: "RF-Parameters-v1570", reg: eutraTypes}, optional: true},
			{name: "irat-ParametersNR-v1570", typ: deferred{name: "IRAT-ParametersNR-v1570", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v15a0-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v15a0-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "neighCellSI-AcquisitionParameters-v15a0", typ: deferred{name: "NeighCellSI-AcquisitionParameters-v15a0", reg: eutraTypes}},
			{name: "eutra-5GC-Parameters-r15", typ: deferred{name: "EUTRA-5GC-Parameters-r15", reg: eutraTypes}, optional: true},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v15a0", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v15a0", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v15a0", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v15a0", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1610-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1610-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "highSpeedEnhParameters-v1610", typ: deferred{name: "HighSpeedEnhParameters-v1610", reg: eutraTypes}, optional: true},
			{name: "neighCellSI-AcquisitionParameters-v1610", typ: deferred{name: "NeighCellSI-AcquisitionParameters-v1610", reg: eutraTypes}, optional: true},
			{name: "mbms-Parameters-v1610", typ: deferred{name: "MBMS-Parameters-v1610", reg: eutraTypes}, optional: true},
			{name: "pdcp-Parameters-v1610", typ: deferred{name: "PDCP-Parameters-v1610", reg: eutraTypes}, optional: true},
			{name: "mac-Parameters-v1610", typ: deferred{name: "MAC-Parameters-v1610", reg: eutraTypes}, optional: true},
			{name: "phyLayerParameters-v1610", typ: deferred{name: "PhyLayerParameters-v1610", reg: eutraTypes}, optional: true},
			{name: "measParameters-v1610", typ: deferred{name: "MeasParameters-v1610", reg: eutraTypes}, optional: true},
			{name: "pur-Parameters-r16", typ: deferred{name: "PUR-Parameters-r16", reg: eutraTypes}, optional: true},
			{name: "eutra-5GC-Parameters-v1610", typ: deferred{name: "EUTRA-5GC-Parameters-v1610", reg: eutraTypes}, optional: true},
			{name: "otherParameters-v1610", typ: deferred{name: "Other-Parameters-v1610", reg: eutraTypes}, optional: true},
			{name: "dl-DedicatedMessageSegmentation-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mmtel-Parameters-v1610", typ: deferred{name: "MMTEL-Parameters-v1610", reg: eutraTypes}},
			{name: "irat-ParametersNR-v1610", typ: deferred{name: "IRAT-ParametersNR-v1610", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1610", typ: deferred{name: "RF-Parameters-v1610", reg: eutraTypes}, optional: true},
			{name: "mobilityParameters-v1610", typ: deferred{name: "MobilityParameters-v1610", reg: eutraTypes}, optional: true},
			{name: "ue-BasedNetwPerfMeasParameters-v1610", typ: deferred{name: "UE-BasedNetwPerfMeasParameters-v1610", reg: eutraTypes}},
			{name: "sl-Parameters-v1610", typ: deferred{name: "SL-Parameters-v1610", reg: eutraTypes}, optional: true},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1610", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1610", reg: eutraTypes}, optional: true},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1610", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1610", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1630-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1630-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rf-Parameters-v1630", typ: deferred{name: "RF-Parameters-v1630", reg: eutraTypes}, optional: true},
			{name: "sl-Parameters-v1630", typ: deferred{name: "SL-Parameters-v1630", reg: eutraTypes}, optional: true},
			{name: "earlySecurityReactivation-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mac-Parameters-v1630", typ: deferred{name: "MAC-Parameters-v1630", reg: eutraTypes}},
			{name: "measParameters-v1630", typ: deferred{name: "MeasParameters-v1630", reg: eutraTypes}, optional: true},
			{name: "fdd-Add-UE-EUTRA-Capabilities-v1630", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1630", reg: eutraTypes}},
			{name: "tdd-Add-UE-EUTRA-Capabilities-v1630", typ: deferred{name: "UE-EUTRA-CapabilityAddXDD-Mode-v1630", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1650-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1650-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "otherParameters-v1650", typ: deferred{name: "Other-Parameters-v1650", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1660-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1660-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "irat-ParametersNR-v1660", typ: deferred{name: "IRAT-ParametersNR-v1660", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1690-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1690-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "other-Parameters-v1690", typ: deferred{name: "Other-Parameters-v1690", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1700-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1700-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measParameters-v1700", typ: deferred{name: "MeasParameters-v1700", reg: eutraTypes}, optional: true},
			{name: "ue-BasedNetwPerfMeasParameters-v1700", typ: deferred{name: "UE-BasedNetwPerfMeasParameters-v1700", reg: eutraTypes}, optional: true},
			{name: "phyLayerParameters-v1700", typ: deferred{name: "PhyLayerParameters-v1700", reg: eutraTypes}},
			{name: "ntn-Parameters-r17", typ: deferred{name: "NTN-Parameters-r17", reg: eutraTypes}, optional: true},
			{name: "irat-ParametersNR-v1700", typ: deferred{name: "IRAT-ParametersNR-v1700", reg: eutraTypes}, optional: true},
			{name: "mbms-Parameters-v1700", typ: deferred{name: "MBMS-Parameters-v1700", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1710-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1710-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "irat-ParametersNR-v1710", typ: deferred{name: "IRAT-ParametersNR-v1710", reg: eutraTypes}},
			{name: "neighCellSI-AcquisitionParameters-v1710", typ: deferred{name: "NeighCellSI-AcquisitionParameters-v1710", reg: eutraTypes}, optional: true},
			{name: "sl-Parameters-v1710", typ: deferred{name: "SL-Parameters-v1710", reg: eutraTypes}, optional: true},
			{name: "sidelinkRequested-r17", typ: enumerated{values: []string{"true"}, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1720-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1720-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ntn-Parameters-v1720", typ: deferred{name: "NTN-Parameters-v1720", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1730-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1730-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1730", typ: deferred{name: "PhyLayerParameters-v1730", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1770-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1770-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measParameters-v1770", typ: deferred{name: "MeasParameters-v1770", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1800-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1800-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measParameters-v1800", typ: deferred{name: "MeasParameters-v1800", reg: eutraTypes}, optional: true},
			{name: "rf-Parameters-v1800", typ: deferred{name: "RF-Parameters-v1800", reg: eutraTypes}, optional: true},
			{name: "ntn-Parameters-v1800", typ: deferred{name: "NTN-Parameters-v1800", reg: eutraTypes}, optional: true},
			{name: "sl-Parameters-v1800", typ: deferred{name: "SL-Parameters-v1800", reg: eutraTypes}, optional: true},
			{name: "son-Parameters-v1800", typ: deferred{name: "SON-Parameters-v1800", reg: eutraTypes}},
			{name: "ue-BasedNetwPerfMeasParameters-v1800", typ: deferred{name: "UE-BasedNetwPerfMeasParameters-v1800", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1830-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1830-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ntn-Parameters-v1830", typ: deferred{name: "NTN-Parameters-v1830", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1840-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v1840-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measParameters-v1840", typ: deferred{name: "MeasParameters-v1840", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: sequence{
				extensible: false,
				fields:     []field{},
			}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v920-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v920", typ: deferred{name: "PhyLayerParameters-v920", reg: eutraTypes}},
			{name: "interRAT-ParametersGERAN-v920", typ: deferred{name: "IRAT-ParametersGERAN-v920", reg: eutraTypes}},
			{name: "interRAT-ParametersUTRA-v920", typ: deferred{name: "IRAT-ParametersUTRA-v920", reg: eutraTypes}, optional: true},
			{name: "interRAT-ParametersCDMA2000-v920", typ: deferred{name: "IRAT-ParametersCDMA2000-1XRTT-v920", reg: eutraTypes}, optional: true},
			{name: "deviceType-r9", typ: enumerated{values: []string{"noBenFromBatConsumpOpt"}, extensible: false}, optional: true},
			{name: "csg-ProximityIndicationParameters-r9", typ: deferred{name: "CSG-ProximityIndicationParameters-r9", reg: eutraTypes}},
			{name: "neighCellSI-AcquisitionParameters-r9", typ: deferred{name: "NeighCellSI-AcquisitionParameters-r9", reg: eutraTypes}},
			{name: "son-Parameters-r9", typ: deferred{name: "SON-Parameters-r9", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v940-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-Capability-v940-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "lateNonCriticalExtension", typ: octetString{hasUB: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-EUTRA-Capability-v1020-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1060"] = sequence{
		extensible: true,
		fields: []field{
			{name: "phyLayerParameters-v1060", typ: deferred{name: "PhyLayerParameters-v1020", reg: eutraTypes}, optional: true},
			{name: "featureGroupIndRel10-v1060", typ: bitString{lb: 32, ub: 32, extensible: false}, optional: true},
			{name: "interRAT-ParametersCDMA2000-v1060", typ: deferred{name: "IRAT-ParametersCDMA2000-1XRTT-v1020", reg: eutraTypes}, optional: true},
			{name: "interRAT-ParametersUTRA-TDD-v1060", typ: deferred{name: "IRAT-ParametersUTRA-TDD-v1020", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1130"] = sequence{
		extensible: true,
		fields: []field{
			{name: "phyLayerParameters-v1130", typ: deferred{name: "PhyLayerParameters-v1130", reg: eutraTypes}, optional: true},
			{name: "measParameters-v1130", typ: deferred{name: "MeasParameters-v1130", reg: eutraTypes}, optional: true},
			{name: "otherParameters-r11", typ: deferred{name: "Other-Parameters-r11", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1180"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mbms-Parameters-r11", typ: deferred{name: "MBMS-Parameters-r11", reg: eutraTypes}},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1250"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1250", typ: deferred{name: "PhyLayerParameters-v1250", reg: eutraTypes}, optional: true},
			{name: "measParameters-v1250", typ: deferred{name: "MeasParameters-v1250", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1310", typ: deferred{name: "PhyLayerParameters-v1310", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1320"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1320", typ: deferred{name: "PhyLayerParameters-v1320", reg: eutraTypes}, optional: true},
			{name: "scptm-Parameters-r13", typ: deferred{name: "SCPTM-Parameters-r13", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1430"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1430", typ: deferred{name: "PhyLayerParameters-v1430", reg: eutraTypes}, optional: true},
			{name: "mmtel-Parameters-r14", typ: deferred{name: "MMTEL-Parameters-r14", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1510"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdcp-ParametersNR-r15", typ: deferred{name: "PDCP-ParametersNR-r15", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "neighCellSI-AcquisitionParameters-v1530", typ: deferred{name: "NeighCellSI-AcquisitionParameters-v1530", reg: eutraTypes}, optional: true},
			{name: "reducedCP-Latency-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1540"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eutra-5GC-Parameters-r15", typ: deferred{name: "EUTRA-5GC-Parameters-r15", reg: eutraTypes}, optional: true},
			{name: "irat-ParametersNR-v1540", typ: deferred{name: "IRAT-ParametersNR-v1540", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1550"] = sequence{
		extensible: false,
		fields: []field{
			{name: "neighCellSI-AcquisitionParameters-v1550", typ: deferred{name: "NeighCellSI-AcquisitionParameters-v1550", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1560"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdcp-ParametersNR-v1560", typ: deferred{name: "PDCP-ParametersNR-v1560", reg: eutraTypes}},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v15a0"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1530", typ: deferred{name: "PhyLayerParameters-v1530", reg: eutraTypes}, optional: true},
			{name: "phyLayerParameters-v1540", typ: deferred{name: "PhyLayerParameters-v1540", reg: eutraTypes}, optional: true},
			{name: "phyLayerParameters-v1550", typ: deferred{name: "PhyLayerParameters-v1550", reg: eutraTypes}, optional: true},
			{name: "neighCellSI-AcquisitionParameters-v15a0", typ: deferred{name: "NeighCellSI-AcquisitionParameters-v15a0", reg: eutraTypes}},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phyLayerParameters-v1610", typ: deferred{name: "PhyLayerParameters-v1610", reg: eutraTypes}, optional: true},
			{name: "pur-Parameters-r16", typ: deferred{name: "PUR-Parameters-r16", reg: eutraTypes}, optional: true},
			{name: "measParameters-v1610", typ: deferred{name: "MeasParameters-v1610", reg: eutraTypes}, optional: true},
			{name: "eutra-5GC-Parameters-v1610", typ: deferred{name: "EUTRA-5GC-Parameters-v1610", reg: eutraTypes}, optional: true},
			{name: "irat-ParametersNR-v1610", typ: deferred{name: "IRAT-ParametersNR-v1610", reg: eutraTypes}, optional: true},
			{name: "neighCellSI-AcquisitionParameters-v1610", typ: deferred{name: "NeighCellSI-AcquisitionParameters-v1610", reg: eutraTypes}, optional: true},
			{name: "mobilityParameters-v1610", typ: deferred{name: "MobilityParameters-v1610", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UE-EUTRA-CapabilityAddXDD-Mode-v1630"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measParameters-v1630", typ: deferred{name: "MeasParameters-v1630", reg: eutraTypes}},
		},
	}

	eutraTypes["UE-RadioPagingInfo-r12"] = sequence{
		extensible: true,
		fields: []field{
			{name: "ue-Category-v1250", typ: integer{lb: 0, ub: 0, extensible: false}, optional: true},
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
		fields: []field{
			{name: "ue-CapabilityRAT-ContainerList", typ: deferred{name: "UE-CapabilityRAT-ContainerList", reg: eutraTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UECapabilityInformation-v8a0-IEs", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["UECapabilityInformation-v1250-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-RadioPagingInfo-r12", typ: deferred{name: "UE-RadioPagingInfo-r12", reg: eutraTypes}, optional: true},
			{name: "nonCriticalExtension", typ: sequence{
				extensible: false,
				fields:     []field{},
			}, optional: true},
		},
	}

	eutraTypes["UECapabilityInformation-v8a0-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "lateNonCriticalExtension", typ: octetString{hasUB: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UECapabilityInformation-v1250-IEs", reg: eutraTypes}, optional: true},
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
		fields: []field{
			{name: "ue-RadioAccessCapabilityInfo", typ: octetString{hasUB: false}},
			{name: "nonCriticalExtension", typ: sequence{
				extensible: false,
				fields:     []field{},
			}, optional: true},
		},
	}

	eutraTypes["UL-256QAM-perCC-Info-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ul-256QAM-perCC-r14", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["V2X-BandCombinationParameters-r14"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "V2X-BandParameters-r14", reg: eutraTypes}}

	eutraTypes["V2X-BandCombinationParameters-v1530"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "V2X-BandParameters-v1530", reg: eutraTypes}}

	eutraTypes["V2X-BandCombinationParametersEUTRA-NR-v1630"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandListSidelinkEUTRA-NR-r16", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "V2X-BandParametersEUTRA-NR-r16", reg: eutraTypes}}},
			{name: "bandListSidelinkEUTRA-NR-v1630", typ: sequenceOf{lb: 1, ub: 64, elem: deferred{name: "V2X-BandParametersEUTRA-NR-v1630", reg: eutraTypes}}},
		},
	}

	eutraTypes["V2X-BandCombinationParametersEUTRA-NR-v1710"] = sequenceOf{lb: 1, ub: 64, elem: deferred{name: "V2X-BandParametersEUTRA-NR-v1710", reg: eutraTypes}}

	eutraTypes["V2X-BandParameters-r14"] = sequence{
		extensible: false,
		fields: []field{
			{name: "v2x-FreqBandEUTRA-r14", typ: deferred{name: "FreqBandIndicator-r11", reg: eutraTypes}},
			{name: "bandParametersTxSL-r14", typ: deferred{name: "BandParametersTxSL-r14", reg: eutraTypes}, optional: true},
			{name: "bandParametersRxSL-r14", typ: deferred{name: "BandParametersRxSL-r14", reg: eutraTypes}, optional: true},
		},
	}

	eutraTypes["V2X-BandParameters-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "v2x-EnhancedHighReception-r15", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["V2X-BandParametersEUTRA-NR-r16"] = choice{
		extensible: false,
		alternatives: []field{
			{name: "eutra", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "v2x-BandParameters1-r16", typ: deferred{name: "V2X-BandParameters-r14", reg: eutraTypes}, optional: true},
					{name: "v2x-BandParameters2-r16", typ: deferred{name: "V2X-BandParameters-v1530", reg: eutraTypes}, optional: true},
				},
			}},
			{name: "nr", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "v2x-BandParametersNR-r16", typ: octetString{hasUB: false}, optional: true},
				},
			}},
		},
	}

	eutraTypes["V2X-BandParametersEUTRA-NR-v1630"] = choice{
		extensible: false,
		alternatives: []field{
			{name: "eutra", typ: null{}},
			{name: "nr", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "tx-Sidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "rx-Sidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}},
		},
	}

	eutraTypes["V2X-BandParametersEUTRA-NR-v1710"] = sequence{
		extensible: false,
		fields: []field{
			{name: "v2x-BandParametersEUTRA-NR-v1710", typ: octetString{hasUB: false}, optional: true},
		},
	}

	eutraTypes["V2X-BandwidthClass-r14"] = enumerated{values: []string{"a", "b", "c", "d", "e", "f"}, extValues: []string{"c1-v1530"}, extensible: true}

	eutraTypes["V2X-BandwidthClassSL-r14"] = sequenceOf{lb: 1, ub: 16, elem: deferred{name: "V2X-BandwidthClass-r14", reg: eutraTypes}}

	eutraTypes["V2X-SupportedBandCombination-r14"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "V2X-BandCombinationParameters-r14", reg: eutraTypes}}

	eutraTypes["V2X-SupportedBandCombination-v1530"] = sequenceOf{lb: 1, ub: 384, elem: deferred{name: "V2X-BandCombinationParameters-v1530", reg: eutraTypes}}

	eutraTypes["V2X-SupportedBandCombinationEUTRA-NR-r16"] = sequenceOf{lb: 1, ub: 512, elem: deferred{name: "V2X-BandParametersEUTRA-NR-r16", reg: eutraTypes}}

	eutraTypes["V2X-SupportedBandCombinationEUTRA-NR-v1630"] = sequenceOf{lb: 1, ub: 512, elem: deferred{name: "V2X-BandCombinationParametersEUTRA-NR-v1630", reg: eutraTypes}}

	eutraTypes["V2X-SupportedBandCombinationEUTRA-NR-v1710"] = sequenceOf{lb: 1, ub: 512, elem: deferred{name: "V2X-BandCombinationParametersEUTRA-NR-v1710", reg: eutraTypes}}

	eutraTypes["WLAN-BandIndicator-r13"] = enumerated{values: []string{"band2dot4", "band5", "band60-v1430", "spare5", "spare4", "spare3", "spare2", "spare1"}, extensible: true}

	eutraTypes["WLAN-IW-Parameters-r12"] = sequence{
		extensible: false,
		fields: []field{
			{name: "wlan-IW-RAN-Rules-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "wlan-IW-ANDSF-Policies-r12", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	eutraTypes["WLAN-IW-Parameters-v1310"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rclwi-r13", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}
}
