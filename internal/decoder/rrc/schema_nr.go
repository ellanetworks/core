// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package rrc

var nrTypes = map[string]node{}

func init() {
	nrTypes["AccessStratumRelease"] = enumerated{values: []string{"rel15", "rel16", "rel17", "rel18", "spare4", "spare3", "spare2", "spare1"}, extensible: true}

	nrTypes["AerialParameters-r18"] = sequence{
		extensible: true,
		fields: []field{
			{name: "aerialUE-Capability-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "altitudeMeas-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "altitudeBasedSSB-ToMeasure-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eventAxHy-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "flightPathReporting-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "flightPathAvailabilityIndicationUAI-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multipleCellsMeasExtension-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nr-NS-PmaxListAerial-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simulMultiTriggerSingleMeasReport-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sl-A2X-Service-r18", typ: enumerated{values: []string{"brid", "daa", "bridAndDAA"}, extensible: false}, optional: true},
		},
	}

	nrTypes["AggregatedBandwidth"] = enumerated{values: []string{"mhz50", "mhz100", "mhz150", "mhz200", "mhz250", "mhz300", "mhz350", "mhz400", "mhz450", "mhz500", "mhz550", "mhz600", "mhz650", "mhz700", "mhz750", "mhz800"}, extensible: false}

	nrTypes["AppLayerMeasParameters-r17"] = sequence{
		extensible: true,
		fields: []field{
			{name: "qoe-Streaming-MeasReport-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "qoe-MTSI-MeasReport-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "qoe-VR-MeasReport-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ran-VisibleQoE-Streaming-MeasReport-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ran-VisibleQoE-VR-MeasReport-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-MeasurementReportAppLayer-Seg-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["BAP-Parameters-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "flowControlBH-RLC-ChannelBased-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "flowControlRouting-ID-Based-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["BAP-Parameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bapHeaderRewriting-Rerouting-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "bapHeaderRewriting-Routing-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["BandCombination"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandList", typ: sequenceOf{lb: 1, ub: 32, elem: deferred{name: "BandParameters", reg: nrTypes}}},
			{name: "featureSetCombination", typ: deferred{name: "FeatureSetCombinationId", reg: nrTypes}},
			{name: "ca-ParametersEUTRA", typ: deferred{name: "CA-ParametersEUTRA", reg: nrTypes}, optional: true},
			{name: "ca-ParametersNR", typ: deferred{name: "CA-ParametersNR", reg: nrTypes}, optional: true},
			{name: "mrdc-Parameters", typ: deferred{name: "MRDC-Parameters", reg: nrTypes}, optional: true},
			{name: "supportedBandwidthCombinationSet", typ: bitString{lb: 1, ub: 32, extensible: false}, optional: true},
			{name: "powerClass-v1530", typ: enumerated{values: []string{"pc2"}, extensible: false}, optional: true},
		},
	}

	nrTypes["BandCombinationList"] = sequenceOf{lb: 1, ub: 65536, elem: deferred{name: "BandCombination", reg: nrTypes}}

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

	nrTypes["BandParameters"] = choice{
		extensible: false,
		alternatives: []field{
			{name: "eutra", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "bandEUTRA", typ: deferred{name: "FreqBandIndicatorEUTRA", reg: nrTypes}},
					{name: "ca-BandwidthClassDL-EUTRA", typ: deferred{name: "CA-BandwidthClassEUTRA", reg: nrTypes}, optional: true},
					{name: "ca-BandwidthClassUL-EUTRA", typ: deferred{name: "CA-BandwidthClassEUTRA", reg: nrTypes}, optional: true},
				},
			}},
			{name: "nr", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "bandNR", typ: deferred{name: "FreqBandIndicatorNR", reg: nrTypes}},
					{name: "ca-BandwidthClassDL-NR", typ: deferred{name: "CA-BandwidthClassNR", reg: nrTypes}, optional: true},
					{name: "ca-BandwidthClassUL-NR", typ: deferred{name: "CA-BandwidthClassNR", reg: nrTypes}, optional: true},
				},
			}},
		},
	}

	nrTypes["BandSidelink-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "freqBandSidelink-r16", typ: deferred{name: "FreqBandIndicatorNR", reg: nrTypes}},
			{name: "sl-Reception-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "harq-RxProcessSidelink-r16", typ: enumerated{values: []string{"n16", "n24", "n32", "n48", "n64"}, extensible: false}},
					{name: "pscch-RxSidelink-r16", typ: enumerated{values: []string{"value1", "value2"}, extensible: false}},
					{name: "scs-CP-PatternRxSidelink-r16", typ: choice{
						extensible: false,
						alternatives: []field{
							{name: "fr1-r16", typ: sequence{
								extensible: false,
								fields: []field{
									{name: "scs-15kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
									{name: "scs-30kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
									{name: "scs-60kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
								},
							}},
							{name: "fr2-r16", typ: sequence{
								extensible: false,
								fields: []field{
									{name: "scs-60kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
									{name: "scs-120kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
								},
							}},
						},
					}, optional: true},
					{name: "extendedCP-RxSidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "sl-TransmissionMode1-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "harq-TxProcessModeOneSidelink-r16", typ: enumerated{values: []string{"n8", "n16"}, extensible: false}},
					{name: "scs-CP-PatternTxSidelinkModeOne-r16", typ: choice{
						extensible: false,
						alternatives: []field{
							{name: "fr1-r16", typ: sequence{
								extensible: false,
								fields: []field{
									{name: "scs-15kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
									{name: "scs-30kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
									{name: "scs-60kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
								},
							}},
							{name: "fr2-r16", typ: sequence{
								extensible: false,
								fields: []field{
									{name: "scs-60kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
									{name: "scs-120kHz-r16", typ: bitString{lb: 16, ub: 16, extensible: false}, optional: true},
								},
							}},
						},
					}},
					{name: "extendedCP-TxSidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "harq-ReportOnPUCCH-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "sync-Sidelink-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "gNB-Sync-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "gNB-GNSS-UE-SyncWithPriorityOnGNB-ENB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "gNB-GNSS-UE-SyncWithPriorityOnGNSS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "sl-Tx-256QAM-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "psfch-FormatZeroSidelink-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "psfch-RxNumber", typ: enumerated{values: []string{"n5", "n15", "n25", "n32", "n35", "n45", "n50", "n64"}, extensible: false}},
					{name: "psfch-TxNumber", typ: enumerated{values: []string{"n4", "n8", "n16"}, extensible: false}},
				},
			}, optional: true},
			{name: "lowSE-64QAM-MCS-TableSidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "enb-sync-Sidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["BandSidelinkEUTRA-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "freqBandSidelinkEUTRA-r16", typ: deferred{name: "FreqBandIndicatorEUTRA", reg: nrTypes}},
			{name: "gnb-ScheduledMode3SidelinkEUTRA-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "gnb-ScheduledMode3DelaySidelinkEUTRA-r16", typ: enumerated{values: []string{"ms0", "ms0dot25", "ms0dot5", "ms0dot625", "ms0dot75", "ms1", "ms1dot25", "ms1dot5", "ms1dot75", "ms2", "ms2dot5", "ms3", "ms4", "ms5", "ms6", "ms8", "ms10", "ms20"}, extensible: false}},
				},
			}, optional: true},
			{name: "gnb-ScheduledMode4SidelinkEUTRA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["CA-BandwidthClassEUTRA"] = enumerated{values: []string{"a", "b", "c", "d", "e", "f"}, extensible: true}

	nrTypes["CA-BandwidthClassNR"] = enumerated{values: []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q"}, extValues: []string{"r2-v1730", "r3-v1730", "r4-v1730", "r5-v1730", "r6-v1730", "r7-v1730", "r8-v1730", "r9-v1730", "r10-v1730", "r11-v1730", "r12-v1730", "v-v1770", "w-v1770"}, extensible: true}

	nrTypes["CA-ParametersEUTRA"] = sequence{
		extensible: true,
		fields: []field{
			{name: "multipleTimingAdvance", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simultaneousRx-Tx", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedNAICS-2CRS-AP", typ: bitString{lb: 1, ub: 8, extensible: false}, optional: true},
			{name: "additionalRx-Tx-PerformanceReq", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-CA-PowerClass-N", typ: enumerated{values: []string{"class2"}, extensible: false}, optional: true},
			{name: "supportedBandwidthCombinationSetEUTRA-v1530", typ: bitString{lb: 1, ub: 32, extensible: false}, optional: true},
		},
	}

	nrTypes["CA-ParametersNR"] = sequence{
		extensible: true,
		fields: []field{
			{name: "dummy", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "parallelTxSRS-PUCCH-PUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "parallelTxPRACH-SRS-PUCCH-PUSCH", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simultaneousRxTxInterBandCA", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simultaneousRxTxSUL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "diffNumerologyAcrossPUCCH-Group", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "diffNumerologyWithinPUCCH-GroupSmallerSCS", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedNumberTAG", typ: enumerated{values: []string{"n2", "n3", "n4"}, extensible: false}, optional: true},
		},
	}

	nrTypes["DummyA"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxNumberNZP-CSI-RS-PerCC", typ: integer{lb: 1, ub: 32, extensible: false}},
			{name: "maxNumberPortsAcrossNZP-CSI-RS-PerCC", typ: enumerated{values: []string{"p2", "p4", "p8", "p12", "p16", "p24", "p32", "p40", "p48", "p56", "p64", "p72", "p80", "p88", "p96", "p104", "p112", "p120", "p128", "p136", "p144", "p152", "p160", "p168", "p176", "p184", "p192", "p200", "p208", "p216", "p224", "p232", "p240", "p248", "p256"}, extensible: false}},
			{name: "maxNumberCS-IM-PerCC", typ: enumerated{values: []string{"n1", "n2", "n4", "n8", "n16", "n32"}, extensible: false}},
			{name: "maxNumberSimultaneousCSI-RS-ActBWP-AllCC", typ: enumerated{values: []string{"n5", "n6", "n7", "n8", "n9", "n10", "n12", "n14", "n16", "n18", "n20", "n22", "n24", "n26", "n28", "n30", "n32", "n34", "n36", "n38", "n40", "n42", "n44", "n46", "n48", "n50", "n52", "n54", "n56", "n58", "n60", "n62", "n64"}, extensible: false}},
			{name: "totalNumberPortsSimultaneousCSI-RS-ActBWP-AllCC", typ: enumerated{values: []string{"p8", "p12", "p16", "p24", "p32", "p40", "p48", "p56", "p64", "p72", "p80", "p88", "p96", "p104", "p112", "p120", "p128", "p136", "p144", "p152", "p160", "p168", "p176", "p184", "p192", "p200", "p208", "p216", "p224", "p232", "p240", "p248", "p256"}, extensible: false}},
		},
	}

	nrTypes["DummyB"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxNumberTxPortsPerResource", typ: enumerated{values: []string{"p2", "p4", "p8", "p12", "p16", "p24", "p32"}, extensible: false}},
			{name: "maxNumberResources", typ: integer{lb: 1, ub: 64, extensible: false}},
			{name: "totalNumberTxPorts", typ: integer{lb: 2, ub: 256, extensible: false}},
			{name: "supportedCodebookMode", typ: enumerated{values: []string{"mode1", "mode1AndMode2"}, extensible: false}},
			{name: "maxNumberCSI-RS-PerResourceSet", typ: integer{lb: 1, ub: 8, extensible: false}},
		},
	}

	nrTypes["DummyC"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxNumberTxPortsPerResource", typ: enumerated{values: []string{"p8", "p16", "p32"}, extensible: false}},
			{name: "maxNumberResources", typ: integer{lb: 1, ub: 64, extensible: false}},
			{name: "totalNumberTxPorts", typ: integer{lb: 2, ub: 256, extensible: false}},
			{name: "supportedCodebookMode", typ: enumerated{values: []string{"mode1", "mode2", "both"}, extensible: false}},
			{name: "supportedNumberPanels", typ: enumerated{values: []string{"n2", "n4"}, extensible: false}},
			{name: "maxNumberCSI-RS-PerResourceSet", typ: integer{lb: 1, ub: 8, extensible: false}},
		},
	}

	nrTypes["DummyD"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxNumberTxPortsPerResource", typ: enumerated{values: []string{"p4", "p8", "p12", "p16", "p24", "p32"}, extensible: false}},
			{name: "maxNumberResources", typ: integer{lb: 1, ub: 64, extensible: false}},
			{name: "totalNumberTxPorts", typ: integer{lb: 2, ub: 256, extensible: false}},
			{name: "parameterLx", typ: integer{lb: 2, ub: 4, extensible: false}},
			{name: "amplitudeScalingType", typ: enumerated{values: []string{"wideband", "widebandAndSubband"}, extensible: false}},
			{name: "amplitudeSubsetRestriction", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "maxNumberCSI-RS-PerResourceSet", typ: integer{lb: 1, ub: 8, extensible: false}},
		},
	}

	nrTypes["DummyE"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxNumberTxPortsPerResource", typ: enumerated{values: []string{"p4", "p8", "p12", "p16", "p24", "p32"}, extensible: false}},
			{name: "maxNumberResources", typ: integer{lb: 1, ub: 64, extensible: false}},
			{name: "totalNumberTxPorts", typ: integer{lb: 2, ub: 256, extensible: false}},
			{name: "parameterLx", typ: integer{lb: 2, ub: 4, extensible: false}},
			{name: "amplitudeScalingType", typ: enumerated{values: []string{"wideband", "widebandAndSubband"}, extensible: false}},
			{name: "maxNumberCSI-RS-PerResourceSet", typ: integer{lb: 1, ub: 8, extensible: false}},
		},
	}

	nrTypes["DummyF"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxNumberPeriodicCSI-ReportPerBWP", typ: integer{lb: 1, ub: 4, extensible: false}},
			{name: "maxNumberAperiodicCSI-ReportPerBWP", typ: integer{lb: 1, ub: 4, extensible: false}},
			{name: "maxNumberSemiPersistentCSI-ReportPerBWP", typ: integer{lb: 0, ub: 4, extensible: false}},
			{name: "simultaneousCSI-ReportsAllCC", typ: integer{lb: 5, ub: 32, extensible: false}},
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

	nrTypes["DummyI"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedSRS-TxPortSwitch", typ: enumerated{values: []string{"t1r2", "t1r4", "t2r4", "t1r4-t2r4", "tr-equal"}, extensible: false}},
			{name: "txSwitchImpactToRx", typ: enumerated{values: []string{"true"}, extensible: false}, optional: true},
		},
	}

	nrTypes["ERedCapParameters-r18"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportOfERedCap-r18", typ: enumerated{values: []string{"supported"}, extensible: false}},
			{name: "eRedCapNotReducedBB-BW-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eRedCapIgnoreCapabilityFiltering-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["EUTRA-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "supportedBandListEUTRA", typ: sequenceOf{lb: 1, ub: 256, elem: deferred{name: "FreqBandIndicatorEUTRA", reg: nrTypes}}},
			{name: "eutra-ParametersCommon", typ: deferred{name: "EUTRA-ParametersCommon", reg: nrTypes}, optional: true},
			{name: "eutra-ParametersXDD-Diff", typ: deferred{name: "EUTRA-ParametersXDD-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["EUTRA-ParametersCommon"] = sequence{
		extensible: true,
		fields: []field{
			{name: "mfbi-EUTRA", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "modifiedMPR-BehaviorEUTRA", typ: bitString{lb: 32, ub: 32, extensible: false}, optional: true},
			{name: "multiNS-Pmax-EUTRA", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "rs-SINR-MeasEUTRA", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["EUTRA-ParametersXDD-Diff"] = sequence{
		extensible: true,
		fields: []field{
			{name: "rsrqMeasWidebandEUTRA", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["FeatureSet"] = choice{
		extensible: false,
		alternatives: []field{
			{name: "eutra", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "downlinkSetEUTRA", typ: deferred{name: "FeatureSetEUTRA-DownlinkId", reg: nrTypes}},
					{name: "uplinkSetEUTRA", typ: deferred{name: "FeatureSetEUTRA-UplinkId", reg: nrTypes}},
				},
			}},
			{name: "nr", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "downlinkSetNR", typ: deferred{name: "FeatureSetDownlinkId", reg: nrTypes}},
					{name: "uplinkSetNR", typ: deferred{name: "FeatureSetUplinkId", reg: nrTypes}},
				},
			}},
		},
	}

	nrTypes["FeatureSetCombination"] = sequenceOf{lb: 1, ub: 32, elem: deferred{name: "FeatureSetsPerBand", reg: nrTypes}}

	nrTypes["FeatureSetCombinationId"] = integer{lb: 0, ub: 1024, extensible: false}

	nrTypes["FeatureSetDownlink"] = sequence{
		extensible: false,
		fields: []field{
			{name: "featureSetListPerDownlinkCC", typ: sequenceOf{lb: 1, ub: 32, elem: deferred{name: "FeatureSetDownlinkPerCC-Id", reg: nrTypes}}},
			{name: "intraBandFreqSeparationDL", typ: deferred{name: "FreqSeparationClass", reg: nrTypes}, optional: true},
			{name: "scalingFactor", typ: enumerated{values: []string{"f0p4", "f0p75", "f0p8"}, extensible: false}, optional: true},
			{name: "dummy8", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "scellWithoutSSB", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-RS-MeasSCellWithoutSSB", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dummy1", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "type1-3-CSS", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdcch-MonitoringAnyOccasions", typ: enumerated{values: []string{"withoutDCI-Gap", "withDCI-Gap"}, extensible: false}, optional: true},
			{name: "dummy2", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-SpecificUL-DL-Assignment", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "searchSpaceSharingCA-DL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "timeDurationForQCL", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "scs-60kHz", typ: enumerated{values: []string{"s7", "s14", "s28"}, extensible: false}, optional: true},
					{name: "scs-120kHz", typ: enumerated{values: []string{"s14", "s28"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "pdsch-ProcessingType1-DifferentTB-PerSlot", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "scs-15kHz", typ: enumerated{values: []string{"upto2", "upto4", "upto7"}, extensible: false}, optional: true},
					{name: "scs-30kHz", typ: enumerated{values: []string{"upto2", "upto4", "upto7"}, extensible: false}, optional: true},
					{name: "scs-60kHz", typ: enumerated{values: []string{"upto2", "upto4", "upto7"}, extensible: false}, optional: true},
					{name: "scs-120kHz", typ: enumerated{values: []string{"upto2", "upto4", "upto7"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "dummy3", typ: deferred{name: "DummyA", reg: nrTypes}, optional: true},
			{name: "dummy4", typ: sequenceOf{lb: 1, ub: 16, elem: deferred{name: "DummyB", reg: nrTypes}}, optional: true},
			{name: "dummy5", typ: sequenceOf{lb: 1, ub: 16, elem: deferred{name: "DummyC", reg: nrTypes}}, optional: true},
			{name: "dummy6", typ: sequenceOf{lb: 1, ub: 16, elem: deferred{name: "DummyD", reg: nrTypes}}, optional: true},
			{name: "dummy7", typ: sequenceOf{lb: 1, ub: 16, elem: deferred{name: "DummyE", reg: nrTypes}}, optional: true},
		},
	}

	nrTypes["FeatureSetDownlinkId"] = integer{lb: 0, ub: 1024, extensible: false}

	nrTypes["FeatureSetDownlinkPerCC"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedSubcarrierSpacingDL", typ: deferred{name: "SubcarrierSpacing", reg: nrTypes}},
			{name: "supportedBandwidthDL", typ: deferred{name: "SupportedBandwidth", reg: nrTypes}},
			{name: "channelBW-90mhz", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "maxNumberMIMO-LayersPDSCH", typ: deferred{name: "MIMO-LayersDL", reg: nrTypes}, optional: true},
			{name: "supportedModulationOrderDL", typ: deferred{name: "ModulationOrder", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["FeatureSetDownlinkPerCC-Id"] = integer{lb: 1, ub: 1024, extensible: false}

	nrTypes["FeatureSetEUTRA-DownlinkId"] = integer{lb: 0, ub: 256, extensible: false}

	nrTypes["FeatureSetEUTRA-UplinkId"] = integer{lb: 0, ub: 256, extensible: false}

	nrTypes["FeatureSetUplink"] = sequence{
		extensible: false,
		fields: []field{
			{name: "featureSetListPerUplinkCC", typ: sequenceOf{lb: 1, ub: 32, elem: deferred{name: "FeatureSetUplinkPerCC-Id", reg: nrTypes}}},
			{name: "scalingFactor", typ: enumerated{values: []string{"f0p4", "f0p75", "f0p8"}, extensible: false}, optional: true},
			{name: "dummy3", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "intraBandFreqSeparationUL", typ: deferred{name: "FreqSeparationClass", reg: nrTypes}, optional: true},
			{name: "searchSpaceSharingCA-UL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dummy1", typ: deferred{name: "DummyI", reg: nrTypes}, optional: true},
			{name: "supportedSRS-Resources", typ: deferred{name: "SRS-Resources", reg: nrTypes}, optional: true},
			{name: "twoPUCCH-Group", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dynamicSwitchSUL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "simultaneousTxSUL-NonSUL", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-ProcessingType1-DifferentTB-PerSlot", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "scs-15kHz", typ: enumerated{values: []string{"upto2", "upto4", "upto7"}, extensible: false}, optional: true},
					{name: "scs-30kHz", typ: enumerated{values: []string{"upto2", "upto4", "upto7"}, extensible: false}, optional: true},
					{name: "scs-60kHz", typ: enumerated{values: []string{"upto2", "upto4", "upto7"}, extensible: false}, optional: true},
					{name: "scs-120kHz", typ: enumerated{values: []string{"upto2", "upto4", "upto7"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "dummy2", typ: deferred{name: "DummyF", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["FeatureSetUplinkId"] = integer{lb: 0, ub: 1024, extensible: false}

	nrTypes["FeatureSetUplinkPerCC"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportedSubcarrierSpacingUL", typ: deferred{name: "SubcarrierSpacing", reg: nrTypes}},
			{name: "supportedBandwidthUL", typ: deferred{name: "SupportedBandwidth", reg: nrTypes}},
			{name: "channelBW-90mhz", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mimo-CB-PUSCH", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "maxNumberMIMO-LayersCB-PUSCH", typ: deferred{name: "MIMO-LayersUL", reg: nrTypes}, optional: true},
					{name: "maxNumberSRS-ResourcePerSet", typ: integer{lb: 1, ub: 2, extensible: false}},
				},
			}, optional: true},
			{name: "maxNumberMIMO-LayersNonCB-PUSCH", typ: deferred{name: "MIMO-LayersUL", reg: nrTypes}, optional: true},
			{name: "supportedModulationOrderUL", typ: deferred{name: "ModulationOrder", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["FeatureSetUplinkPerCC-Id"] = integer{lb: 1, ub: 1024, extensible: false}

	nrTypes["FeatureSets"] = sequence{
		extensible: true,
		fields: []field{
			{name: "featureSetsDownlink", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "FeatureSetDownlink", reg: nrTypes}}, optional: true},
			{name: "featureSetsDownlinkPerCC", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "FeatureSetDownlinkPerCC", reg: nrTypes}}, optional: true},
			{name: "featureSetsUplink", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "FeatureSetUplink", reg: nrTypes}}, optional: true},
			{name: "featureSetsUplinkPerCC", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "FeatureSetUplinkPerCC", reg: nrTypes}}, optional: true},
		},
	}

	nrTypes["FeatureSetsPerBand"] = sequenceOf{lb: 1, ub: 128, elem: deferred{name: "FeatureSet", reg: nrTypes}}

	nrTypes["FreqBandIndicatorEUTRA"] = integer{lb: 1, ub: 256, extensible: false}

	nrTypes["FreqBandIndicatorNR"] = integer{lb: 1, ub: 1024, extensible: false}

	nrTypes["FreqBandInformation"] = choice{
		extensible: false,
		alternatives: []field{
			{name: "bandInformationEUTRA", typ: deferred{name: "FreqBandInformationEUTRA", reg: nrTypes}},
			{name: "bandInformationNR", typ: deferred{name: "FreqBandInformationNR", reg: nrTypes}},
		},
	}

	nrTypes["FreqBandInformationEUTRA"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandEUTRA", typ: deferred{name: "FreqBandIndicatorEUTRA", reg: nrTypes}},
			{name: "ca-BandwidthClassDL-EUTRA", typ: deferred{name: "CA-BandwidthClassEUTRA", reg: nrTypes}, optional: true},
			{name: "ca-BandwidthClassUL-EUTRA", typ: deferred{name: "CA-BandwidthClassEUTRA", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["FreqBandInformationNR"] = sequence{
		extensible: false,
		fields: []field{
			{name: "bandNR", typ: deferred{name: "FreqBandIndicatorNR", reg: nrTypes}},
			{name: "maxBandwidthRequestedDL", typ: deferred{name: "AggregatedBandwidth", reg: nrTypes}, optional: true},
			{name: "maxBandwidthRequestedUL", typ: deferred{name: "AggregatedBandwidth", reg: nrTypes}, optional: true},
			{name: "maxCarriersRequestedDL", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
			{name: "maxCarriersRequestedUL", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
		},
	}

	nrTypes["FreqBandList"] = sequenceOf{lb: 1, ub: 1280, elem: deferred{name: "FreqBandInformation", reg: nrTypes}}

	nrTypes["FreqSeparationClass"] = enumerated{values: []string{"mhz800", "mhz1200", "mhz1400"}, extValues: []string{"mhz400-v1650", "mhz600-v1650"}, extensible: true}

	nrTypes["GeneralParametersMRDC-XDD-Diff"] = sequence{
		extensible: true,
		fields: []field{
			{name: "splitSRB-WithOneUL-Path", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "splitDRB-withUL-Both-MCG-SCG", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "srb3", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dummy", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["GeneralParametersMRDC-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "f1c-OverEUTRA-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["HighSpeedParameters-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measurementEnhancement-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "demodulationEnhancement-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["HighSpeedParameters-v1650"] = choice{
		extensible: false,
		alternatives: []field{
			{name: "intraNR-MeasurementEnhancement-r16", typ: enumerated{values: []string{"supported"}, extensible: false}},
			{name: "interRAT-MeasurementEnhancement-r16", typ: enumerated{values: []string{"supported"}, extensible: false}},
		},
	}

	nrTypes["HighSpeedParameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measurementEnhancementCA-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "measurementEnhancementInterFreq-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["IMS-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "ims-ParametersCommon", typ: deferred{name: "IMS-ParametersCommon", reg: nrTypes}, optional: true},
			{name: "ims-ParametersFRX-Diff", typ: deferred{name: "IMS-ParametersFRX-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["IMS-Parameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ims-ParametersFR2-2-r17", typ: deferred{name: "IMS-ParametersFR2-2-r17", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["IMS-ParametersCommon"] = sequence{
		extensible: true,
		fields: []field{
			{name: "voiceOverEUTRA-5GC", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["IMS-ParametersFR2-2-r17"] = sequence{
		extensible: true,
		fields: []field{
			{name: "voiceOverNR-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["IMS-ParametersFRX-Diff"] = sequence{
		extensible: true,
		fields: []field{
			{name: "voiceOverNR", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["InterRAT-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "eutra", typ: deferred{name: "EUTRA-Parameters", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MAC-Parameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mac-ParametersCommon", typ: deferred{name: "MAC-ParametersCommon", reg: nrTypes}, optional: true},
			{name: "mac-ParametersXDD-Diff", typ: deferred{name: "MAC-ParametersXDD-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MAC-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mac-ParametersFRX-Diff-r16", typ: deferred{name: "MAC-ParametersFRX-Diff-r16", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MAC-Parameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mac-ParametersFR2-2-r17", typ: deferred{name: "MAC-ParametersFR2-2-r17", reg: nrTypes}, optional: true},
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

	nrTypes["MAC-ParametersFR2-2-r17"] = sequence{
		extensible: true,
		fields: []field{
			{name: "directMCG-SCellActivation-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directMCG-SCellActivationResume-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directSCG-SCellActivation-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directSCG-SCellActivationResume-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "drx-Adaptation-r17", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "non-SharedSpectrumChAccess-r17", typ: deferred{name: "MinTimeGapFR2-2-r17", reg: nrTypes}, optional: true},
					{name: "sharedSpectrumChAccess-r17", typ: deferred{name: "MinTimeGapFR2-2-r17", reg: nrTypes}, optional: true},
				},
			}, optional: true},
		},
	}

	nrTypes["MAC-ParametersFRX-Diff-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "directMCG-SCellActivation-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directMCG-SCellActivationResume-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directSCG-SCellActivation-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directSCG-SCellActivationResume-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "drx-Adaptation-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "non-SharedSpectrumChAccess-r16", typ: deferred{name: "MinTimeGap-r16", reg: nrTypes}, optional: true},
					{name: "sharedSpectrumChAccess-r16", typ: deferred{name: "MinTimeGap-r16", reg: nrTypes}, optional: true},
				},
			}, optional: true},
		},
	}

	nrTypes["MAC-ParametersSidelink-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "mac-ParametersSidelinkCommon-r16", typ: deferred{name: "MAC-ParametersSidelinkCommon-r16", reg: nrTypes}, optional: true},
			{name: "mac-ParametersSidelinkXDD-Diff-r16", typ: deferred{name: "MAC-ParametersSidelinkXDD-Diff-r16", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MAC-ParametersSidelinkCommon-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "lcp-RestrictionSidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "multipleConfiguredGrantsSidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MAC-ParametersSidelinkXDD-Diff-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "multipleSR-ConfigurationsSidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "logicalChannelSR-DelayTimerSidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
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

	nrTypes["MBS-Parameters-r17"] = sequence{
		extensible: false,
		fields: []field{
			{name: "maxMRB-Add-r17", typ: integer{lb: 1, ub: 16, extensible: false}, optional: true},
		},
	}

	nrTypes["MIMO-LayersDL"] = enumerated{values: []string{"twoLayers", "fourLayers", "eightLayers"}, extensible: false}

	nrTypes["MIMO-LayersUL"] = enumerated{values: []string{"oneLayer", "twoLayers", "fourLayers"}, extensible: false}

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

	nrTypes["MRDC-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "singleUL-Transmission", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dynamicPowerSharingENDC", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "tdm-Pattern", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-SharingEUTRA-NR", typ: enumerated{values: []string{"tdm", "fdm", "both"}, extensible: false}, optional: true},
			{name: "ul-SwitchingTimeEUTRA-NR", typ: enumerated{values: []string{"type1", "type2"}, extensible: false}, optional: true},
			{name: "simultaneousRxTxInterBandENDC", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "asyncIntraBandENDC", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersCommon", typ: deferred{name: "MeasAndMobParametersCommon", reg: nrTypes}, optional: true},
			{name: "measAndMobParametersXDD-Diff", typ: deferred{name: "MeasAndMobParametersXDD-Diff", reg: nrTypes}, optional: true},
			{name: "measAndMobParametersFRX-Diff", typ: deferred{name: "MeasAndMobParametersFRX-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MeasAndMobParameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersFR2-2-r17", typ: deferred{name: "MeasAndMobParametersFR2-2-r17", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersCommon"] = sequence{
		extensible: true,
		fields: []field{
			{name: "supportedGapPattern", typ: bitString{lb: 22, ub: 22, extensible: false}, optional: true},
			{name: "ssb-RLM", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ssb-AndCSI-RS-RLM", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersFR2-2-r17"] = sequence{
		extensible: true,
		fields: []field{
			{name: "handoverInterF-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "handoverLTE-EPC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "handoverLTE-5GC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "idleInactiveNR-MeasReport-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersFRX-Diff"] = sequence{
		extensible: true,
		fields: []field{
			{name: "ss-SINR-Meas", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-RSRP-AndRSRQ-MeasWithSSB", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-RSRP-AndRSRQ-MeasWithoutSSB", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-SINR-Meas", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "csi-RS-RLM", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-Common", typ: deferred{name: "MeasAndMobParametersMRDC-Common", reg: nrTypes}, optional: true},
			{name: "measAndMobParametersMRDC-XDD-Diff", typ: deferred{name: "MeasAndMobParametersMRDC-XDD-Diff", reg: nrTypes}, optional: true},
			{name: "measAndMobParametersMRDC-FRX-Diff", typ: deferred{name: "MeasAndMobParametersMRDC-FRX-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-Common"] = sequence{
		extensible: false,
		fields: []field{
			{name: "independentGapConfig", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-Common-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "condPSCellChangeParametersCommon-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "condPSCellChangeFDD-TDD-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "condPSCellChangeFR1-FR2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "pscellT312-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-Common-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "condPSCellChangeParameters-r17", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "inter-SN-condPSCellChangeFDD-TDD-NRDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "inter-SN-condPSCellChangeFR1-FR2-NRDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "inter-SN-condPSCellChangeFDD-TDD-ENDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "inter-SN-condPSCellChangeFR1-FR2-ENDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "mn-InitiatedCondPSCellChange-FR1FDD-ENDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "mn-InitiatedCondPSCellChange-FR1TDD-ENDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "mn-InitiatedCondPSCellChange-FR2TDD-ENDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "sn-InitiatedCondPSCellChange-FR1FDD-ENDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "sn-InitiatedCondPSCellChange-FR1TDD-ENDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "sn-InitiatedCondPSCellChange-FR2TDD-ENDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "condHandoverWithSCG-ENDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "condHandoverWithSCG-NEDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-Common-v1730"] = sequence{
		extensible: false,
		fields: []field{
			{name: "independentGapConfig-maxCC-r17", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "fr1-Only-r17", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
					{name: "fr2-Only-r17", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
					{name: "fr1-AndFR2-r17", typ: integer{lb: 1, ub: 32, extensible: false}, optional: true},
				},
			}},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-Common-v1810"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mn-ConfiguredMN-TriggerSCPAC-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mn-ConfiguredSN-TriggerSCPAC-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sn-ConfiguredSCPAC-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mn-ConfiguredMN-TriggerSCPAC-afterSCG-release-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mn-ConfiguredReferenceConfigSCPAC-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sn-ConfiguredReferenceConfigSCPAC-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "condHandoverWithCandSCG-Addition-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "condHandoverWithCandSCG-FR1-FR2-Change-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "condHandoverWithCandSCG-FDD-TDD-Change-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-FRX-Diff"] = sequence{
		extensible: false,
		fields: []field{
			{name: "simultaneousRxDataSSB-DiffNumerology", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-XDD-Diff"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sftd-MeasPSCell", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sftd-MeasNR-Cell", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-XDD-Diff-v1560"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sftd-MeasPSCell-NEDC", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-v1560"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-XDD-Diff-v1560", typ: deferred{name: "MeasAndMobParametersMRDC-XDD-Diff-v1560", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-Common-v1610", typ: deferred{name: "MeasAndMobParametersMRDC-Common-v1610", reg: nrTypes}, optional: true},
			{name: "interNR-MeasEUTRA-IAB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-Common-v1700", typ: deferred{name: "MeasAndMobParametersMRDC-Common-v1700", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-v1730"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-Common-v1730", typ: deferred{name: "MeasAndMobParametersMRDC-Common-v1730", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersMRDC-v1810"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-Common-v1810", typ: deferred{name: "MeasAndMobParametersMRDC-Common-v1810", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["MeasAndMobParametersXDD-Diff"] = sequence{
		extensible: true,
		fields: []field{
			{name: "intraAndInterF-MeasAndReport", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eventA-MeasAndReport", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MinTimeGap-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "scs-15kHz-r16", typ: enumerated{values: []string{"sl1", "sl3"}, extensible: false}, optional: true},
			{name: "scs-30kHz-r16", typ: enumerated{values: []string{"sl1", "sl6"}, extensible: false}, optional: true},
			{name: "scs-60kHz-r16", typ: enumerated{values: []string{"sl1", "sl12"}, extensible: false}, optional: true},
			{name: "scs-120kHz-r16", typ: enumerated{values: []string{"sl2", "sl24"}, extensible: false}, optional: true},
		},
	}

	nrTypes["MinTimeGapFR2-2-r17"] = sequence{
		extensible: false,
		fields: []field{
			{name: "scs-120kHz-r17", typ: enumerated{values: []string{"sl2", "sl24"}, extensible: false}, optional: true},
			{name: "scs-480kHz-r17", typ: enumerated{values: []string{"sl8", "sl96"}, extensible: false}, optional: true},
			{name: "scs-960kHz-r17", typ: enumerated{values: []string{"sl16", "sl192"}, extensible: false}, optional: true},
		},
	}

	nrTypes["ModulationOrder"] = enumerated{values: []string{"bpsk-halfpi", "bpsk", "qpsk", "qam16", "qam64", "qam256"}, extensible: false}

	nrTypes["NAICS-Capability-Entry"] = sequence{
		extensible: true,
		fields: []field{
			{name: "numberOfNAICS-CapableCC", typ: integer{lb: 1, ub: 5, extensible: false}},
			{name: "numberOfAggregatedPRB", typ: enumerated{values: []string{"n50", "n75", "n100", "n125", "n150", "n175", "n200", "n225", "n250", "n275", "n300", "n350", "n400", "n450", "n500", "spare"}, extensible: false}},
		},
	}

	nrTypes["NCR-Parameters-r18"] = sequence{
		extensible: false,
		fields: []field{
			{name: "inactiveStateNCR-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportedNumberOfDRBs-NCR-r18", typ: enumerated{values: []string{"n1", "n16"}, extensible: false}, optional: true},
			{name: "dummy", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["NRDC-Parameters"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersNRDC", typ: deferred{name: "MeasAndMobParametersMRDC", reg: nrTypes}, optional: true},
			{name: "generalParametersNRDC", typ: deferred{name: "GeneralParametersMRDC-XDD-Diff", reg: nrTypes}, optional: true},
			{name: "fdd-Add-UE-NRDC-Capabilities", typ: deferred{name: "UE-MRDC-CapabilityAddXDD-Mode", reg: nrTypes}, optional: true},
			{name: "tdd-Add-UE-NRDC-Capabilities", typ: deferred{name: "UE-MRDC-CapabilityAddXDD-Mode", reg: nrTypes}, optional: true},
			{name: "fr1-Add-UE-NRDC-Capabilities", typ: deferred{name: "UE-MRDC-CapabilityAddFRX-Mode", reg: nrTypes}, optional: true},
			{name: "fr2-Add-UE-NRDC-Capabilities", typ: deferred{name: "UE-MRDC-CapabilityAddFRX-Mode", reg: nrTypes}, optional: true},
			{name: "dummy2", typ: octetString{hasUB: false}, optional: true},
			{name: "dummy", typ: sequence{
				extensible: false,
				fields:     []field{},
			}, optional: true},
		},
	}

	nrTypes["NRDC-Parameters-v1570"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sfn-SyncNRDC", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["NRDC-Parameters-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersNRDC-v1610", typ: deferred{name: "MeasAndMobParametersMRDC-v1610", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["NRDC-Parameters-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "f1c-OverNR-RRC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "measAndMobParametersNRDC-v1700", typ: deferred{name: "MeasAndMobParametersMRDC-v1700", reg: nrTypes}},
		},
	}

	nrTypes["NTN-Parameters-r17"] = sequence{
		extensible: false,
		fields: []field{
			{name: "inactiveStateNTN-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ra-SDT-NTN-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "srb-SDT-NTN-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "measAndMobParametersNTN-r17", typ: deferred{name: "MeasAndMobParameters", reg: nrTypes}, optional: true},
			{name: "mac-ParametersNTN-r17", typ: deferred{name: "MAC-Parameters", reg: nrTypes}, optional: true},
			{name: "phy-ParametersNTN-r17", typ: deferred{name: "Phy-Parameters", reg: nrTypes}, optional: true},
			{name: "fdd-Add-UE-NR-CapabilitiesNTN-r17", typ: deferred{name: "UE-NR-CapabilityAddXDD-Mode", reg: nrTypes}, optional: true},
			{name: "fr1-Add-UE-NR-CapabilitiesNTN-r17", typ: deferred{name: "UE-NR-CapabilityAddFRX-Mode", reg: nrTypes}, optional: true},
			{name: "ue-BasedPerfMeas-ParametersNTN-r17", typ: deferred{name: "UE-BasedPerfMeas-Parameters-r16", reg: nrTypes}, optional: true},
			{name: "son-ParametersNTN-r17", typ: deferred{name: "SON-Parameters-r16", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["NTN-Parameters-v1820"] = sequence{
		extensible: false,
		fields: []field{
			{name: "fr2-Add-UE-NR-CapabilitiesNTN-r18", typ: deferred{name: "UE-NR-CapabilityAddFRX-Mode", reg: nrTypes}, optional: true},
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

	nrTypes["PDCP-ParametersMRDC"] = sequence{
		extensible: false,
		fields: []field{
			{name: "pdcp-DuplicationSplitSRB", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdcp-DuplicationSplitDRB", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["PDCP-ParametersMRDC-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "scg-DRB-NR-IAB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
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

	nrTypes["Phy-ParametersMRDC"] = sequence{
		extensible: true,
		fields: []field{
			{name: "naics-Capability-List", typ: sequenceOf{lb: 1, ub: 8, elem: deferred{name: "NAICS-Capability-Entry", reg: nrTypes}}, optional: true},
		},
	}

	nrTypes["Phy-ParametersSharedSpectrumChAccess-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "ss-SINR-Meas-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sp-CSI-ReportPUCCH-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sp-CSI-ReportPUSCH-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dynamicSFI-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mux-SR-HARQ-ACK-CSI-PUCCH-OncePerSlot-r16", typ: sequence{
				extensible: false,
				fields: []field{
					{name: "sameSymbol-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
					{name: "diffSymbol-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
				},
			}, optional: true},
			{name: "mux-SR-HARQ-ACK-PUCCH-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mux-SR-HARQ-ACK-CSI-PUCCH-MultiPerSlot-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mux-HARQ-ACK-PUSCH-DiffSymbol-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pucch-Repetition-F1-3-4-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "type1-PUSCH-RepetitionMultiSlots-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "type2-PUSCH-RepetitionMultiSlots-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pusch-RepetitionMultiSlots-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pdsch-RepetitionMultiSlots-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "downlinkSPS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "configuredUL-GrantType1-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "configuredUL-GrantType2-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "pre-EmptIndication-DL-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
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

	nrTypes["PowSav-Parameters-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "powSav-ParametersCommon-r16", typ: deferred{name: "PowSav-ParametersCommon-r16", reg: nrTypes}, optional: true},
			{name: "powSav-ParametersFRX-Diff-r16", typ: deferred{name: "PowSav-ParametersFRX-Diff-r16", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["PowSav-Parameters-v1700"] = sequence{
		extensible: true,
		fields: []field{
			{name: "powSav-ParametersFR2-2-r17", typ: deferred{name: "PowSav-ParametersFR2-2-r17", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["PowSav-ParametersCommon-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "drx-Preference-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "maxCC-Preference-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "releasePreference-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "minSchedulingOffsetPreference-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["PowSav-ParametersFR2-2-r17"] = sequence{
		extensible: true,
		fields: []field{
			{name: "maxBW-Preference-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "maxMIMO-LayerPreference-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["PowSav-ParametersFRX-Diff-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "maxBW-Preference-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "maxMIMO-LayerPreference-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["RAT-Type"] = enumerated{values: []string{"nr", "eutra-nr", "eutra", "utra-fdd-v1610"}, extensible: true}

	nrTypes["RF-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "supportedBandListNR", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "BandNR", reg: nrTypes}}},
			{name: "supportedBandCombinationList", typ: deferred{name: "BandCombinationList", reg: nrTypes}, optional: true},
			{name: "appliedFreqBandListFilter", typ: deferred{name: "FreqBandList", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["RF-ParametersMRDC"] = sequence{
		extensible: true,
		fields: []field{
			{name: "supportedBandCombinationList", typ: deferred{name: "BandCombinationList", reg: nrTypes}, optional: true},
			{name: "appliedFreqBandListFilter", typ: deferred{name: "FreqBandList", reg: nrTypes}, optional: true},
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

	nrTypes["RLC-ParametersSidelink-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "am-WithLongSN-Sidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "um-WithLongSN-Sidelink-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["RedCapParameters-r17"] = sequence{
		extensible: false,
		fields: []field{
			{name: "supportOfRedCap-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "supportOf16DRB-RedCap-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["RedCapParameters-v1740"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ncd-SSB-ForRedCapInitialBWP-SDT-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
		},
	}

	nrTypes["SDAP-Parameters"] = sequence{
		extensible: true,
		fields: []field{
			{name: "as-ReflectiveQoS", typ: enumerated{values: []string{"true"}, extensible: false}, optional: true},
		},
	}

	nrTypes["SON-Parameters-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "rach-Report-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
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

	nrTypes["SidelinkParameters-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sidelinkParametersNR-r16", typ: deferred{name: "SidelinkParametersNR-r16", reg: nrTypes}, optional: true},
			{name: "sidelinkParametersEUTRA-r16", typ: deferred{name: "SidelinkParametersEUTRA-r16", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["SidelinkParametersEUTRA-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "sl-ParametersEUTRA1-r16", typ: octetString{hasUB: false}, optional: true},
			{name: "sl-ParametersEUTRA2-r16", typ: octetString{hasUB: false}, optional: true},
			{name: "sl-ParametersEUTRA3-r16", typ: octetString{hasUB: false}, optional: true},
			{name: "supportedBandListSidelinkEUTRA-r16", typ: sequenceOf{lb: 1, ub: 256, elem: deferred{name: "BandSidelinkEUTRA-r16", reg: nrTypes}}, optional: true},
		},
	}

	nrTypes["SidelinkParametersNR-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "rlc-ParametersSidelink-r16", typ: deferred{name: "RLC-ParametersSidelink-r16", reg: nrTypes}, optional: true},
			{name: "mac-ParametersSidelink-r16", typ: deferred{name: "MAC-ParametersSidelink-r16", reg: nrTypes}, optional: true},
			{name: "fdd-Add-UE-Sidelink-Capabilities-r16", typ: deferred{name: "UE-SidelinkCapabilityAddXDD-Mode-r16", reg: nrTypes}, optional: true},
			{name: "tdd-Add-UE-Sidelink-Capabilities-r16", typ: deferred{name: "UE-SidelinkCapabilityAddXDD-Mode-r16", reg: nrTypes}, optional: true},
			{name: "supportedBandListSidelink-r16", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "BandSidelink-r16", reg: nrTypes}}, optional: true},
		},
	}

	nrTypes["SubcarrierSpacing"] = enumerated{values: []string{"kHz15", "kHz30", "kHz60", "kHz120", "kHz240", "kHz480-v1700", "kHz960-v1700", "spare1"}, extensible: false}

	nrTypes["SupportedBandwidth"] = choice{
		extensible: false,
		alternatives: []field{
			{name: "fr1", typ: enumerated{values: []string{"mhz5", "mhz10", "mhz15", "mhz20", "mhz25", "mhz30", "mhz40", "mhz50", "mhz60", "mhz80", "mhz100"}, extensible: false}},
			{name: "fr2", typ: enumerated{values: []string{"mhz50", "mhz100", "mhz200", "mhz400"}, extensible: false}},
		},
	}

	nrTypes["UE-BasedPerfMeas-Parameters-r16"] = sequence{
		extensible: true,
		fields: []field{
			{name: "barometerMeasReport-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "immMeasBT-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "immMeasWLAN-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "loggedMeasBT-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "loggedMeasurements-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "loggedMeasWLAN-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "orientationMeasReport-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "speedMeasReport-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "gnss-Location-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ulPDCP-Delay-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
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

	nrTypes["UE-MRDC-Capability"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC", typ: deferred{name: "MeasAndMobParametersMRDC", reg: nrTypes}, optional: true},
			{name: "phy-ParametersMRDC-v1530", typ: deferred{name: "Phy-ParametersMRDC", reg: nrTypes}, optional: true},
			{name: "rf-ParametersMRDC", typ: deferred{name: "RF-ParametersMRDC", reg: nrTypes}},
			{name: "generalParametersMRDC", typ: deferred{name: "GeneralParametersMRDC-XDD-Diff", reg: nrTypes}, optional: true},
			{name: "fdd-Add-UE-MRDC-Capabilities", typ: deferred{name: "UE-MRDC-CapabilityAddXDD-Mode", reg: nrTypes}, optional: true},
			{name: "tdd-Add-UE-MRDC-Capabilities", typ: deferred{name: "UE-MRDC-CapabilityAddXDD-Mode", reg: nrTypes}, optional: true},
			{name: "fr1-Add-UE-MRDC-Capabilities", typ: deferred{name: "UE-MRDC-CapabilityAddFRX-Mode", reg: nrTypes}, optional: true},
			{name: "fr2-Add-UE-MRDC-Capabilities", typ: deferred{name: "UE-MRDC-CapabilityAddFRX-Mode", reg: nrTypes}, optional: true},
			{name: "featureSetCombinations", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "FeatureSetCombination", reg: nrTypes}}, optional: true},
			{name: "pdcp-ParametersMRDC-v1530", typ: deferred{name: "PDCP-ParametersMRDC", reg: nrTypes}, optional: true},
			{name: "lateNonCriticalExtension", typ: octetString{hasUB: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-MRDC-Capability-v1560", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-MRDC-Capability-v1560"] = sequence{
		extensible: false,
		fields: []field{
			{name: "receivedFilters", typ: octetString{hasUB: false}, optional: true},
			{name: "measAndMobParametersMRDC-v1560", typ: deferred{name: "MeasAndMobParametersMRDC-v1560", reg: nrTypes}, optional: true},
			{name: "fdd-Add-UE-MRDC-Capabilities-v1560", typ: deferred{name: "UE-MRDC-CapabilityAddXDD-Mode-v1560", reg: nrTypes}, optional: true},
			{name: "tdd-Add-UE-MRDC-Capabilities-v1560", typ: deferred{name: "UE-MRDC-CapabilityAddXDD-Mode-v1560", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-MRDC-Capability-v1610", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-MRDC-Capability-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-v1610", typ: deferred{name: "MeasAndMobParametersMRDC-v1610", reg: nrTypes}, optional: true},
			{name: "generalParametersMRDC-v1610", typ: deferred{name: "GeneralParametersMRDC-v1610", reg: nrTypes}, optional: true},
			{name: "pdcp-ParametersMRDC-v1610", typ: deferred{name: "PDCP-ParametersMRDC-v1610", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-MRDC-Capability-v1700", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-MRDC-Capability-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-v1700", typ: deferred{name: "MeasAndMobParametersMRDC-v1700", reg: nrTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-MRDC-Capability-v1730", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-MRDC-Capability-v1730"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-v1730", typ: deferred{name: "MeasAndMobParametersMRDC-v1730", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-MRDC-Capability-v1800", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-MRDC-Capability-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "requirementTypeIndication-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "measAndMobParametersMRDC-v1810", typ: deferred{name: "MeasAndMobParametersMRDC-v1810", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: sequence{
				extensible: false,
				fields:     []field{},
			}, optional: true},
		},
	}

	nrTypes["UE-MRDC-CapabilityAddFRX-Mode"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-FRX-Diff", typ: deferred{name: "MeasAndMobParametersMRDC-FRX-Diff", reg: nrTypes}},
		},
	}

	nrTypes["UE-MRDC-CapabilityAddXDD-Mode"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-XDD-Diff", typ: deferred{name: "MeasAndMobParametersMRDC-XDD-Diff", reg: nrTypes}, optional: true},
			{name: "generalParametersMRDC-XDD-Diff", typ: deferred{name: "GeneralParametersMRDC-XDD-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-MRDC-CapabilityAddXDD-Mode-v1560"] = sequence{
		extensible: false,
		fields: []field{
			{name: "measAndMobParametersMRDC-XDD-Diff-v1560", typ: deferred{name: "MeasAndMobParametersMRDC-XDD-Diff-v1560", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability"] = sequence{
		extensible: false,
		fields: []field{
			{name: "accessStratumRelease", typ: deferred{name: "AccessStratumRelease", reg: nrTypes}},
			{name: "pdcp-Parameters", typ: deferred{name: "PDCP-Parameters", reg: nrTypes}},
			{name: "rlc-Parameters", typ: deferred{name: "RLC-Parameters", reg: nrTypes}, optional: true},
			{name: "mac-Parameters", typ: deferred{name: "MAC-Parameters", reg: nrTypes}, optional: true},
			{name: "phy-Parameters", typ: deferred{name: "Phy-Parameters", reg: nrTypes}},
			{name: "rf-Parameters", typ: deferred{name: "RF-Parameters", reg: nrTypes}},
			{name: "measAndMobParameters", typ: deferred{name: "MeasAndMobParameters", reg: nrTypes}, optional: true},
			{name: "fdd-Add-UE-NR-Capabilities", typ: deferred{name: "UE-NR-CapabilityAddXDD-Mode", reg: nrTypes}, optional: true},
			{name: "tdd-Add-UE-NR-Capabilities", typ: deferred{name: "UE-NR-CapabilityAddXDD-Mode", reg: nrTypes}, optional: true},
			{name: "fr1-Add-UE-NR-Capabilities", typ: deferred{name: "UE-NR-CapabilityAddFRX-Mode", reg: nrTypes}, optional: true},
			{name: "fr2-Add-UE-NR-Capabilities", typ: deferred{name: "UE-NR-CapabilityAddFRX-Mode", reg: nrTypes}, optional: true},
			{name: "featureSets", typ: deferred{name: "FeatureSets", reg: nrTypes}, optional: true},
			{name: "featureSetCombinations", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "FeatureSetCombination", reg: nrTypes}}, optional: true},
			{name: "lateNonCriticalExtension", typ: octetString{hasUB: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1530", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "fdd-Add-UE-NR-Capabilities-v1530", typ: deferred{name: "UE-NR-CapabilityAddXDD-Mode-v1530", reg: nrTypes}, optional: true},
			{name: "tdd-Add-UE-NR-Capabilities-v1530", typ: deferred{name: "UE-NR-CapabilityAddXDD-Mode-v1530", reg: nrTypes}, optional: true},
			{name: "dummy", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "interRAT-Parameters", typ: deferred{name: "InterRAT-Parameters", reg: nrTypes}, optional: true},
			{name: "inactiveState", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "delayBudgetReporting", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1540", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1540"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sdap-Parameters", typ: deferred{name: "SDAP-Parameters", reg: nrTypes}, optional: true},
			{name: "overheatingInd", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ims-Parameters", typ: deferred{name: "IMS-Parameters", reg: nrTypes}, optional: true},
			{name: "fr1-Add-UE-NR-Capabilities-v1540", typ: deferred{name: "UE-NR-CapabilityAddFRX-Mode-v1540", reg: nrTypes}, optional: true},
			{name: "fr2-Add-UE-NR-Capabilities-v1540", typ: deferred{name: "UE-NR-CapabilityAddFRX-Mode-v1540", reg: nrTypes}, optional: true},
			{name: "fr1-fr2-Add-UE-NR-Capabilities", typ: deferred{name: "UE-NR-CapabilityAddFRX-Mode", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1550", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1550"] = sequence{
		extensible: false,
		fields: []field{
			{name: "reducedCP-Latency", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1560", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1560"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nrdc-Parameters", typ: deferred{name: "NRDC-Parameters", reg: nrTypes}, optional: true},
			{name: "receivedFilters", typ: octetString{hasUB: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1570", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1570"] = sequence{
		extensible: false,
		fields: []field{
			{name: "nrdc-Parameters-v1570", typ: deferred{name: "NRDC-Parameters-v1570", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1610", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "inDeviceCoexInd-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dl-DedicatedMessageSegmentation-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nrdc-Parameters-v1610", typ: deferred{name: "NRDC-Parameters-v1610", reg: nrTypes}, optional: true},
			{name: "powSav-Parameters-r16", typ: deferred{name: "PowSav-Parameters-r16", reg: nrTypes}, optional: true},
			{name: "fr1-Add-UE-NR-Capabilities-v1610", typ: deferred{name: "UE-NR-CapabilityAddFRX-Mode-v1610", reg: nrTypes}, optional: true},
			{name: "fr2-Add-UE-NR-Capabilities-v1610", typ: deferred{name: "UE-NR-CapabilityAddFRX-Mode-v1610", reg: nrTypes}, optional: true},
			{name: "bh-RLF-Indication-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "directSN-AdditionFirstRRC-IAB-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "bap-Parameters-r16", typ: deferred{name: "BAP-Parameters-r16", reg: nrTypes}, optional: true},
			{name: "referenceTimeProvision-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "sidelinkParameters-r16", typ: deferred{name: "SidelinkParameters-r16", reg: nrTypes}, optional: true},
			{name: "highSpeedParameters-r16", typ: deferred{name: "HighSpeedParameters-r16", reg: nrTypes}, optional: true},
			{name: "mac-Parameters-v1610", typ: deferred{name: "MAC-Parameters-v1610", reg: nrTypes}, optional: true},
			{name: "mcgRLF-RecoveryViaSCG-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "resumeWithStoredMCG-SCells-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "resumeWithStoredSCG-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "resumeWithSCG-Config-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-BasedPerfMeas-Parameters-r16", typ: deferred{name: "UE-BasedPerfMeas-Parameters-r16", reg: nrTypes}, optional: true},
			{name: "son-Parameters-r16", typ: deferred{name: "SON-Parameters-r16", reg: nrTypes}, optional: true},
			{name: "onDemandSIB-Connected-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1640", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1640"] = sequence{
		extensible: false,
		fields: []field{
			{name: "redirectAtResumeByNAS-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "phy-ParametersSharedSpectrumChAccess-r16", typ: deferred{name: "Phy-ParametersSharedSpectrumChAccess-r16", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1650", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1650"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mpsPriorityIndication-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "highSpeedParameters-v1650", typ: deferred{name: "HighSpeedParameters-v1650", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1690", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1690"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ul-RRC-Segmentation-r16", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1700", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1700"] = sequence{
		extensible: false,
		fields: []field{
			{name: "inactiveStatePO-Determination-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "highSpeedParameters-v1700", typ: deferred{name: "HighSpeedParameters-v1700", reg: nrTypes}, optional: true},
			{name: "powSav-Parameters-v1700", typ: deferred{name: "PowSav-Parameters-v1700", reg: nrTypes}, optional: true},
			{name: "mac-Parameters-v1700", typ: deferred{name: "MAC-Parameters-v1700", reg: nrTypes}, optional: true},
			{name: "ims-Parameters-v1700", typ: deferred{name: "IMS-Parameters-v1700", reg: nrTypes}, optional: true},
			{name: "measAndMobParameters-v1700", typ: deferred{name: "MeasAndMobParameters-v1700", reg: nrTypes}},
			{name: "appLayerMeasParameters-r17", typ: deferred{name: "AppLayerMeasParameters-r17", reg: nrTypes}, optional: true},
			{name: "redCapParameters-r17", typ: deferred{name: "RedCapParameters-r17", reg: nrTypes}, optional: true},
			{name: "ra-SDT-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "srb-SDT-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "gNB-SideRTT-BasedPDC-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "bh-RLF-DetectionRecovery-Indication-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nrdc-Parameters-v1700", typ: deferred{name: "NRDC-Parameters-v1700", reg: nrTypes}, optional: true},
			{name: "bap-Parameters-v1700", typ: deferred{name: "BAP-Parameters-v1700", reg: nrTypes}, optional: true},
			{name: "musim-GapPreference-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "musimLeaveConnected-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mbs-Parameters-r17", typ: deferred{name: "MBS-Parameters-r17", reg: nrTypes}},
			{name: "nonTerrestrialNetwork-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ntn-ScenarioSupport-r17", typ: enumerated{values: []string{"gso", "ngso"}, extensible: false}, optional: true},
			{name: "sliceInfoforCellReselection-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ue-RadioPagingInfo-r17", typ: deferred{name: "UE-RadioPagingInfo-r17", reg: nrTypes}, optional: true},
			{name: "ul-GapFR2-Pattern-r17", typ: bitString{lb: 4, ub: 4, extensible: false}, optional: true},
			{name: "ntn-Parameters-r17", typ: deferred{name: "NTN-Parameters-r17", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1740", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1740"] = sequence{
		extensible: false,
		fields: []field{
			{name: "redCapParameters-v1740", typ: deferred{name: "RedCapParameters-v1740", reg: nrTypes}},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1750", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1750"] = sequence{
		extensible: false,
		fields: []field{
			{name: "crossCarrierSchedulingConfigurationRelease-r17", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1800", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1800"] = sequence{
		extensible: false,
		fields: []field{
			{name: "airToGroundNetwork-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "eRedCapParameters-r18", typ: deferred{name: "ERedCapParameters-r18", reg: nrTypes}, optional: true},
			{name: "ncr-Parameters-r18", typ: deferred{name: "NCR-Parameters-r18", reg: nrTypes}, optional: true},
			{name: "softSatelliteSwitchResyncNTN-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "hardSatelliteSwitchResyncNTN-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mt-SDT-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "mt-SDT-NTN-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "inDeviceCoexIndAutonomousDenial-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "inDeviceCoexIndFDM-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "inDeviceCoexIndTDM-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "musim-GapPriorityPreference-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "musim-CapabilityRestriction-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "dummy", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ra-InsteadCG-SDT-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "resumeAfterSDT-Release-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "ul-TrafficInfo-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "aerialParameters-r18", typ: deferred{name: "AerialParameters-r18", reg: nrTypes}, optional: true},
			{name: "ntn-VSAT-AntennaType-r18", typ: enumerated{values: []string{"electronic", "mechanical"}, extensible: false}, optional: true},
			{name: "ntn-VSAT-MobilityType-r18", typ: enumerated{values: []string{"fixed", "mobile"}, extensible: false}, optional: true},
			{name: "ntn-Parameters-v1820", typ: deferred{name: "NTN-Parameters-v1820", reg: nrTypes}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1830", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1830"] = sequence{
		extensible: false,
		fields: []field{
			{name: "sib19-Support-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: deferred{name: "UE-NR-Capability-v1860", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-Capability-v1860"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ntn-CHO-OnlyLocationTimeTrigger-r18", typ: enumerated{values: []string{"supported"}, extensible: false}, optional: true},
			{name: "nonCriticalExtension", typ: sequence{
				extensible: false,
				fields:     []field{},
			}, optional: true},
		},
	}

	nrTypes["UE-NR-CapabilityAddFRX-Mode"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phy-ParametersFRX-Diff", typ: deferred{name: "Phy-ParametersFRX-Diff", reg: nrTypes}, optional: true},
			{name: "measAndMobParametersFRX-Diff", typ: deferred{name: "MeasAndMobParametersFRX-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-CapabilityAddFRX-Mode-v1540"] = sequence{
		extensible: false,
		fields: []field{
			{name: "ims-ParametersFRX-Diff", typ: deferred{name: "IMS-ParametersFRX-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-CapabilityAddFRX-Mode-v1610"] = sequence{
		extensible: false,
		fields: []field{
			{name: "powSav-ParametersFRX-Diff-r16", typ: deferred{name: "PowSav-ParametersFRX-Diff-r16", reg: nrTypes}, optional: true},
			{name: "mac-ParametersFRX-Diff-r16", typ: deferred{name: "MAC-ParametersFRX-Diff-r16", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-CapabilityAddXDD-Mode"] = sequence{
		extensible: false,
		fields: []field{
			{name: "phy-ParametersXDD-Diff", typ: deferred{name: "Phy-ParametersXDD-Diff", reg: nrTypes}, optional: true},
			{name: "mac-ParametersXDD-Diff", typ: deferred{name: "MAC-ParametersXDD-Diff", reg: nrTypes}, optional: true},
			{name: "measAndMobParametersXDD-Diff", typ: deferred{name: "MeasAndMobParametersXDD-Diff", reg: nrTypes}, optional: true},
		},
	}

	nrTypes["UE-NR-CapabilityAddXDD-Mode-v1530"] = sequence{
		extensible: false,
		fields: []field{
			{name: "eutra-ParametersXDD-Diff", typ: deferred{name: "EUTRA-ParametersXDD-Diff", reg: nrTypes}},
		},
	}

	nrTypes["UE-RadioPagingInfo-r17"] = sequence{
		extensible: true,
		fields: []field{
			{name: "pei-SubgroupingSupportBandList-r17", typ: sequenceOf{lb: 1, ub: 1024, elem: deferred{name: "FreqBandIndicatorNR", reg: nrTypes}}, optional: true},
		},
	}

	nrTypes["UE-SidelinkCapabilityAddXDD-Mode-r16"] = sequence{
		extensible: false,
		fields: []field{
			{name: "mac-ParametersSidelinkXDD-Diff-r16", typ: deferred{name: "MAC-ParametersSidelinkXDD-Diff-r16", reg: nrTypes}, optional: true},
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
