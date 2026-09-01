// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrc

var nrTypes = map[string]node{}

func init() {
	nrTypes["AccessStratumRelease"] = enumerated{values: []string{"rel15", "rel16", "rel17", "rel18", "spare4", "spare3", "spare2", "spare1"}, extensible: true}

	nrTypes["BandNR"] = sequence{
		extensible: true,
		fields: []field{
			{name: "bandNR", typ: deferred{name: "FreqBandIndicatorNR", reg: nrTypes}},
			{name: "modifiedMPR-Behaviour", typ: bitString{lb: 8, ub: 8, extensible: false}, optional: true},
			{name: "mimo-ParametersPerBand", typ: deferred{name: "MIMO-ParametersPerBand", reg: nrTypes}, optional: true},
			{name: "extendedCP", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multipleTCI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "bwp-WithoutRestriction", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "bwp-SameNumerology", typ: enumerated{values: []string{"upto2", "upto4"}, extensible: false}, optional: true},
			{name: "bwp-DiffNumerology", typ: enumerated{values: []string{"upto4"}, extensible: false}, optional: true},
			{name: "crossCarrierScheduling-SameSCS", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-256QAM-FR2", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-256QAM", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-PowerClass", typ: enumerated{values: []string{"pc1", "pc2", "pc3", "pc4"}, extensible: false}, optional: true},
			{name: "rateMatchingLTE-CRS", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "channelBWs-DL", typ: choice{
				extensible: false,
				alternatives: []field{
					{name: "fr1", typ: sequence{
						extensible: false,
						fields: []field{
							{name: "scs-15kHz", typ: bitString{lb: 10, ub: 10, extensible: false}, optional: true},
							{name: "scs-30kHz", typ: bitString{lb: 10, ub: 10, extensible: false}, optional: true},
							{name: "scs-60kHz", typ: bitString{lb: 10, ub: 10, extensible: false}, optional: true},
						},
					}},
					{name: "fr2", typ: sequence{
						extensible: false,
						fields: []field{
							{name: "scs-60kHz", typ: bitString{lb: 3, ub: 3, extensible: false}, optional: true},
							{name: "scs-120kHz", typ: bitString{lb: 3, ub: 3, extensible: false}, optional: true},
						},
					}},
				},
			}, optional: true},
			{name: "channelBWs-UL", typ: choice{
				extensible: false,
				alternatives: []field{
					{name: "fr1", typ: sequence{
						extensible: false,
						fields: []field{
							{name: "scs-15kHz", typ: bitString{lb: 10, ub: 10, extensible: false}, optional: true},
							{name: "scs-30kHz", typ: bitString{lb: 10, ub: 10, extensible: false}, optional: true},
							{name: "scs-60kHz", typ: bitString{lb: 10, ub: 10, extensible: false}, optional: true},
						},
					}},
					{name: "fr2", typ: sequence{
						extensible: false,
						fields: []field{
							{name: "scs-60kHz", typ: bitString{lb: 3, ub: 3, extensible: false}, optional: true},
							{name: "scs-120kHz", typ: bitString{lb: 3, ub: 3, extensible: false}, optional: true},
						},
					}},
				},
			}, optional: true},
		},
	}

	nrTypes["DummyG"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxNumberSSB-CSI-RS-ResourceOneTx", typ: enumerated{values: []string{"n8", "n16", "n32", "n64"}, extensible: false}},
			{name: "maxNumberSSB-CSI-RS-ResourceTwoTx", typ: enumerated{values: []string{"n0", "n4", "n8", "n16", "n32", "n64"}, extensible: false}},
			{name: "supportedCSI-RS-Density", typ: enumerated{values: []string{"one", "three", "oneAndThree"}, extensible: false}},
		},
	}

	nrTypes["DummyH"] = sequence{
		extensible: false,
		fields: []field{
			{name: "burstLength", typ: integer{lb: 1, ub: 2, extensible: false}},
			{name: "maxSimultaneousResourceSetsPerCC", typ: integer{lb: 1, ub: 8, extensible: false}},
			{name: "maxConfiguredResourceSetsPerCC", typ: integer{lb: 1, ub: 64, extensible: false}},
			{name: "maxConfiguredResourceSetsAllCC", typ: integer{lb: 1, ub: 128, extensible: false}},
		},
	}

	nrTypes["FreqBandIndicatorNR"] = integer{lb: 1, ub: 1024, extensible: false}

	nrTypes["MAC-Parameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mac-ParametersCommon", typ: deferred{name: "MAC-ParametersCommon", reg: nrTypes}, optional: true},
			{name: "mac-ParametersXDD-Diff", typ: deferred{name: "MAC-ParametersXDD-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MAC-ParametersCommon"] = sequence{
		extensible: true,
		fields: []field{
			{name: "lcp-Restriction", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dummy", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "lch-ToSCellRestriction", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MAC-ParametersXDD-Diff"] = sequence{
		extensible: true,
		fields: []field{
			{name: "skipUplinkTxDynamic", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "logicalChannelSR-DelayTimer", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "longDRX-Cycle", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "shortDRX-Cycle", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multipleSR-Configurations", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multipleConfiguredGrants", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MIMO-ParametersPerBand"] = sequence{
		extensible: true,
		fields: []field{
			{name: "tci-StatePDSCH", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "maxNumberConfiguredTCI-StatesPerCC", typ: enumerated{values: []string{"n4", "n8", "n16", "n32", "n64", "n128"}, extensible: false}, optional: true},
					{name: "maxNumberActiveTCI-PerBWP", typ: enumerated{values: []string{"n1", "n2", "n4", "n8"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "additionalActiveTCI-StatePDCCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-TransCoherence", typ: enumerated{values: []string{"nonCoherent", "partialCoherent", "fullCoherent"}, extensible: false}, optional: true},
			{name: "beamCorrespondenceWithoutUL-BeamSweeping", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "periodicBeamReport", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "aperiodicBeamReport", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sp-BeamReportPUCCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sp-BeamReportPUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dummy1", typ: deferred{name: "DummyG", reg: nrTypes}, optional: true},
			{name: "maxNumberRxBeam", typ: integer{lb: 2, ub: 8, extensible: false}, optional: true},
			{name: "maxNumberRxTxBeamSwitchDL", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "scs-15kHz", typ: enumerated{values: []string{"n4", "n7", "n14"}, extensible: false}, optional: true},
					{name: "scs-30kHz", typ: enumerated{values: []string{"n4", "n7", "n14"}, extensible: false}, optional: true},
					{name: "scs-60kHz", typ: enumerated{values: []string{"n4", "n7", "n14"}, extensible: false}, optional: true},
					{name: "scs-120kHz", typ: enumerated{values: []string{"n4", "n7", "n14"}, extensible: false}, optional: true},
					{name: "scs-240kHz", typ: enumerated{values: []string{"n4", "n7", "n14"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "maxNumberNonGroupBeamReporting", typ: enumerated{values: []string{"n1", "n2", "n4"}, extensible: false}, optional: true},
			{name: "groupBeamReporting", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "uplinkBeamManagement", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "maxNumberSRS-ResourcePerSet-BM", typ: enumerated{values: []string{"n2", "n4", "n8", "n16"}, extensible: false}},
					{name: "maxNumberSRS-ResourceSet", typ: integer{lb: 1, ub: 8, extensible: false}},
				},
			}, optional: true},
			{name: "maxNumberCSI-RS-BFD", typ: integer{lb: 1, ub: 64, extensible: false}, optional: true},
			{name: "maxNumberSSB-BFD", typ: integer{lb: 1, ub: 64, extensible: false}, optional: true},
			{name: "maxNumberCSI-RS-SSB-CBD", typ: integer{lb: 1, ub: 256, extensible: false}, optional: true},
			{name: "dummy2", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "twoPortsPTRS-UL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dummy5", typ: deferred{name: "SRS-Resources", reg: nrTypes}, optional: true},
			{name: "dummy3", typ: integer{lb: 1, ub: 4, extensible: false}, optional: true},
			{name: "beamReportTiming", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "scs-15kHz", typ: enumerated{values: []string{"sym2", "sym4", "sym8"}, extensible: false}, optional: true},
					{name: "scs-30kHz", typ: enumerated{values: []string{"sym4", "sym8", "sym14", "sym28"}, extensible: false}, optional: true},
					{name: "scs-60kHz", typ: enumerated{values: []string{"sym8", "sym14", "sym28"}, extensible: false}, optional: true},
					{name: "scs-120kHz", typ: enumerated{values: []string{"sym14", "sym28", "sym56"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "ptrs-DensityRecommendationSetDL", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "scs-15kHz", typ: deferred{name: "PTRS-DensityRecommendationDL", reg: nrTypes}, optional: true},
					{name: "scs-30kHz", typ: deferred{name: "PTRS-DensityRecommendationDL", reg: nrTypes}, optional: true},
					{name: "scs-60kHz", typ: deferred{name: "PTRS-DensityRecommendationDL", reg: nrTypes}, optional: true},
					{name: "scs-120kHz", typ: deferred{name: "PTRS-DensityRecommendationDL", reg: nrTypes}, optional: true},
				},
			}, optional: true},
			{name: "ptrs-DensityRecommendationSetUL", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "scs-15kHz", typ: deferred{name: "PTRS-DensityRecommendationUL", reg: nrTypes}, optional: true},
					{name: "scs-30kHz", typ: deferred{name: "PTRS-DensityRecommendationUL", reg: nrTypes}, optional: true},
					{name: "scs-60kHz", typ: deferred{name: "PTRS-DensityRecommendationUL", reg: nrTypes}, optional: true},
					{name: "scs-120kHz", typ: deferred{name: "PTRS-DensityRecommendationUL", reg: nrTypes}, optional: true},
				},
			}, optional: true},
			{name: "dummy4", typ: deferred{name: "DummyH", reg: nrTypes}, optional: true},
			{name: "aperiodicTRS", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["PDCP-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "supportedROHC-Profiles", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "profile0x0000", typ: boolean{}},
					{name: "profile0x0001", typ: boolean{}},
					{name: "profile0x0002", typ: boolean{}},
					{name: "profile0x0003", typ: boolean{}},
					{name: "profile0x0004", typ: boolean{}},
					{name: "profile0x0006", typ: boolean{}},
					{name: "profile0x0101", typ: boolean{}},
					{name: "profile0x0102", typ: boolean{}},
					{name: "profile0x0103", typ: boolean{}},
					{name: "profile0x0104", typ: boolean{}},
				},
			}},
			{name: "maxNumberROHC-ContextSessions", typ: enumerated{values: []string{"cs2", "cs4", "cs8", "cs12", "cs16", "cs24", "cs32", "cs48", "cs64", "cs128", "cs256", "cs512", "cs1024", "cs16384", "spare2", "spare1"}, extensible: false}},
			{name: "uplinkOnlyROHC-Profiles", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "continueROHC-Context", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "outOfOrderDelivery", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "shortSN", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdcp-DuplicationSRB", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdcp-DuplicationMCG-OrSCG-DRB", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["PTRS-DensityRecommendationDL"] = sequence{
		extensible: false,
		fields: []field{
			{name: "frequencyDensity1", typ: integer{lb: 1, ub: 276, extensible: false}},
			{name: "frequencyDensity2", typ: integer{lb: 1, ub: 276, extensible: false}},
			{name: "timeDensity1", typ: integer{lb: 0, ub: 29, extensible: false}},
			{name: "timeDensity2", typ: integer{lb: 0, ub: 29, extensible: false}},
			{name: "timeDensity3", typ: integer{lb: 0, ub: 29, extensible: false}},
		},
	}

	nrTypes["PTRS-DensityRecommendationUL"] = sequence{
		extensible: false,
		fields: []field{
			{name: "frequencyDensity1", typ: integer{lb: 1, ub: 276, extensible: false}},
			{name: "frequencyDensity2", typ: integer{lb: 1, ub: 276, extensible: false}},
			{name: "timeDensity1", typ: integer{lb: 0, ub: 29, extensible: false}},
			{name: "timeDensity2", typ: integer{lb: 0, ub: 29, extensible: false}},
			{name: "timeDensity3", typ: integer{lb: 0, ub: 29, extensible: false}},
			{name: "sampleDensity1", typ: integer{lb: 1, ub: 276, extensible: false}},
			{name: "sampleDensity2", typ: integer{lb: 1, ub: 276, extensible: false}},
			{name: "sampleDensity3", typ: integer{lb: 1, ub: 276, extensible: false}},
			{name: "sampleDensity4", typ: integer{lb: 1, ub: 276, extensible: false}},
			{name: "sampleDensity5", typ: integer{lb: 1, ub: 276, extensible: false}},
		},
	}

	nrTypes["Phy-Parameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phy-ParametersCommon", typ: deferred{name: "Phy-ParametersCommon", reg: nrTypes}, optional: true},
			{name: "phy-ParametersXDD-Diff", typ: deferred{name: "Phy-ParametersXDD-Diff", reg: nrTypes}, optional: true},
			{name: "phy-ParametersFRX-Diff", typ: deferred{name: "Phy-ParametersFRX-Diff", reg: nrTypes}, optional: true},
			{name: "phy-ParametersFR1", typ: deferred{name: "Phy-ParametersFR1", reg: nrTypes}, optional: true},
			{name: "phy-ParametersFR2", typ: deferred{name: "Phy-ParametersFR2", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["Phy-ParametersCommon"] = sequence{
		extensible: true,
		fields: []field{
			{name: "csi-RS-CFRA-ForHO", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dynamicPRB-BundlingDL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sp-CSI-ReportPUCCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sp-CSI-ReportPUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nzp-CSI-RS-IntefMgmt", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "type2-SP-CSI-Feedback-LongPUCCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "precoderGranularityCORESET", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dynamicHARQ-ACK-Codebook", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "semiStaticHARQ-ACK-Codebook", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "spatialBundlingHARQ-ACK", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dynamicBetaOffsetInd-HARQ-ACK-CSI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pucch-Repetition-F1-3-4", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ra-Type0-PUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dynamicSwitchRA-Type0-1-PDSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dynamicSwitchRA-Type0-1-PUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-MappingTypeA", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-MappingTypeB", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "interleavingVRB-ToPRB-PDSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "interSlotFreqHopping-PUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "type1-PUSCH-RepetitionMultiSlots", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "type2-PUSCH-RepetitionMultiSlots", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-RepetitionMultiSlots", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-RepetitionMultiSlots", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "downlinkSPS", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "configuredUL-GrantType1", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "configuredUL-GrantType2", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pre-EmptIndication-DL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "cbg-TransIndication-DL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "cbg-TransIndication-UL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "cbg-FlushIndication-DL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dynamicHARQ-ACK-CodeB-CBG-Retx-DL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rateMatchingResrcSetSemi-Static", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rateMatchingResrcSetDynamic", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "bwp-SwitchingDelay", typ: enumerated{values: []string{"type1", "type2"}, extensible: false}, optional: true},
		},
	}

	nrTypes["Phy-ParametersFR1"] = sequence{
		extensible: true,
		fields: []field{
			{name: "pdcch-MonitoringSingleOccasion", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "scs-60kHz", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-256QAM-FR1", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-RE-MappingFR1-PerSymbol", typ: enumerated{values: []string{"n10", "n20"}, extensible: false}, optional: true},
		},
	}

	nrTypes["Phy-ParametersFR2"] = sequence{
		extensible: true,
		fields: []field{
			{name: "dummy", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-RE-MappingFR2-PerSymbol", typ: enumerated{values: []string{"n6", "n20"}, extensible: false}, optional: true},
		},
	}

	nrTypes["Phy-ParametersFRX-Diff"] = sequence{
		extensible: true,
		fields: []field{
			{name: "dynamicSFI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dummy1", typ: bitString{lb: 2, ub: 2, extensible: false}, optional: true},
			{name: "twoFL-DMRS", typ: bitString{lb: 2, ub: 2, extensible: false}, optional: true},
			{name: "dummy2", typ: bitString{lb: 2, ub: 2, extensible: false}, optional: true},
			{name: "dummy3", typ: bitString{lb: 2, ub: 2, extensible: false}, optional: true},
			{name: "supportedDMRS-TypeDL", typ: enumerated{values: []string{"type1", "type1And2"}, extensible: false}, optional: true},
			{name: "supportedDMRS-TypeUL", typ: enumerated{values: []string{"type1", "type1And2"}, extensible: false}, optional: true},
			{name: "semiOpenLoopCSI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-ReportWithoutPMI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-ReportWithoutCQI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "onePortsPTRS", typ: bitString{lb: 2, ub: 2, extensible: false}, optional: true},
			{name: "twoPUCCH-F0-2-ConsecSymbols", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pucch-F2-WithFH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pucch-F3-WithFH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pucch-F4-WithFH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pucch-F0-2WithoutFH", typ: enumerated{values: []string{"notSupported"}, extensible: false}, optional: true},
			{name: "pucch-F1-3-4WithoutFH", typ: enumerated{values: []string{"notSupported"}, extensible: false}, optional: true},
			{name: "mux-SR-HARQ-ACK-CSI-PUCCH-MultiPerSlot", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "uci-CodeBlockSegmentation", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "onePUCCH-LongAndShortFormat", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "twoPUCCH-AnyOthersInSlot", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "intraSlotFreqHopping-PUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-LBRM", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdcch-BlindDetectionCA", typ: integer{lb: 4, ub: 16, extensible: false}, optional: true},
			{name: "tpc-PUSCH-RNTI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tpc-PUCCH-RNTI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tpc-SRS-RNTI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "absoluteTPC-Command", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "twoDifferentTPC-Loop-PUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "twoDifferentTPC-Loop-PUCCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-HalfPi-BPSK", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pucch-F3-4-HalfPi-BPSK", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "almostContiguousCP-OFDM-UL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sp-CSI-RS", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sp-CSI-IM", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tdd-MultiDL-UL-SwitchPerSlot", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multipleCORESET", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["Phy-ParametersXDD-Diff"] = sequence{
		extensible: true,
		fields: []field{
			{name: "dynamicSFI", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "twoPUCCH-F0-2-ConsecSymbols", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "twoDifferentTPC-Loop-PUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "twoDifferentTPC-Loop-PUCCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["RAT-Type"] = enumerated{values: []string{"nr", "eutra-nr", "eutra", "utra-fdd-v1610"}, extensible: true}

	nrTypes["RF-Parameters"] = sequence{
		extensible: true,
		stopAfter:  "supportedBandListNR",
		fields: []field{
			{name: "supportedBandListNR", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "BandNR", reg: nrTypes}}},
			{name: "supportedBandCombinationList", typ: unsupported{name: "supportedBandCombinationList"}, optional: true},
			{name: "appliedFreqBandListFilter", typ: unsupported{name: "appliedFreqBandListFilter"}, optional: true},
		},
	}

	nrTypes["RLC-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "am-WithShortSN", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "um-WithShortSN", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "um-WithLongSN", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["SRS-Resources"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxNumberAperiodicSRS-PerBWP", typ: enumerated{values: []string{"n1", "n2", "n4", "n8", "n16"}, extensible: false}},
			{name: "maxNumberAperiodicSRS-PerBWP-PerSlot", typ: integer{lb: 1, ub: 6, extensible: false}},
			{name: "maxNumberPeriodicSRS-PerBWP", typ: enumerated{values: []string{"n1", "n2", "n4", "n8", "n16"}, extensible: false}},
			{name: "maxNumberPeriodicSRS-PerBWP-PerSlot", typ: integer{lb: 1, ub: 6, extensible: false}},
			{name: "maxNumberSemiPersistentSRS-PerBWP", typ: enumerated{values: []string{"n1", "n2", "n4", "n8", "n16"}, extensible: false}},
			{name: "maxNumberSemiPersistentSRS-PerBWP-PerSlot", typ: integer{lb: 1, ub: 6, extensible: false}},
			{name: "maxNumberSRS-Ports-PerResource", typ: enumerated{values: []string{"n1", "n2", "n4"}, extensible: false}},
		},
	}

	nrTypes["UE-CapabilityRAT-Container"] = sequence{
		extensible: false,
		fields: []field{
			{name: "rat-Type", typ: deferred{name: "RAT-Type", reg: nrTypes}},
			{name: "ue-CapabilityRAT-Container", typ: octetString{hasUB: false}},
		},
	}

	nrTypes["UE-CapabilityRAT-ContainerList"] = sequenceOf{lb: 0, ub: 8, elem: deferred{name: "UE-CapabilityRAT-Container", reg: nrTypes}}

	nrTypes["UE-NR-Capability"] = sequence{
		extensible: false,
		stopAfter:  "rf-Parameters",
		fields: []field{
			{name: "accessStratumRelease", typ: deferred{name: "AccessStratumRelease", reg: nrTypes}},
			{name: "pdcp-Parameters", typ: deferred{name: "PDCP-Parameters", reg: nrTypes}},
			{name: "rlc-Parameters", typ: deferred{name: "RLC-Parameters", reg: nrTypes}, optional: true},
			{name: "mac-Parameters", typ: deferred{name: "MAC-Parameters", reg: nrTypes}, optional: true},
			{name: "phy-Parameters", typ: deferred{name: "Phy-Parameters", reg: nrTypes}},
			{name: "rf-Parameters", typ: deferred{name: "RF-Parameters", reg: nrTypes}},
			{name: "measAndMobParameters", typ: unsupported{name: "measAndMobParameters"}, optional: true},
			{name: "fdd-Add-UE-NR-Capabilities", typ: unsupported{name: "fdd-Add-UE-NR-Capabilities"}, optional: true},
			{name: "tdd-Add-UE-NR-Capabilities", typ: unsupported{name: "tdd-Add-UE-NR-Capabilities"}, optional: true},
			{name: "fr1-Add-UE-NR-Capabilities", typ: unsupported{name: "fr1-Add-UE-NR-Capabilities"}, optional: true},
			{name: "fr2-Add-UE-NR-Capabilities", typ: unsupported{name: "fr2-Add-UE-NR-Capabilities"}, optional: true},
			{name: "featureSets", typ: unsupported{name: "featureSets"}, optional: true},
			{name: "featureSetCombinations", typ: unsupported{name: "featureSetCombinations"}, optional: true},
			{name: "lateNonCriticalExtension", typ: unsupported{name: "lateNonCriticalExtension"}, optional: true},
			{name: "nonCriticalExtension", typ: unsupported{name: "nonCriticalExtension"}, optional: true},
		},
	}

	nrTypes["UERadioAccessCapabilityInformation"] = sequence{
		extensible: false,
		fields: []field{
			{name: "criticalExtensions", typ: choice{
				extensible: false,
				alternatives: []field{
					{name: "c1", typ: choice{
						extensible: false,
						alternatives: []field{
							{name: "ueRadioAccessCapabilityInformation", typ: deferred{name: "UERadioAccessCapabilityInformation-IEs", reg: nrTypes}},
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

	nrTypes["UERadioAccessCapabilityInformation-IEs"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ue-RadioAccessCapabilityInfo", typ: octetString{hasUB: false}},
			{name: "nonCriticalExtension", typ: sequence{
				extensible: false,
				fields:     []field{},
			}, optional: true},
		},
	}
}
