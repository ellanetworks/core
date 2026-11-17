// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"bytes"
	"reflect"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

type extCodec interface {
	per.Marshaler
	per.Unmarshaler
	Diagnose(*ap.Diagnosis[ProtocolIEID]) bool
}

type typedExtCase struct {
	name  string
	id    ProtocolIEID
	crit  Criticality
	value []byte
	wire  func(raw rawIE) extCodec
	fresh func() extCodec
	typed func(v extCodec) per.Marshaler
}

func plmnVec() PLMNIdentity { return PLMNIdentity{0x00, 0xf1, 0x10} }

func ecgiVec() EUTRANCGI { return EUTRANCGI{PLMNIdentity: plmnVec(), CellID: 0x1234567} }

func taiVec() TAI { return TAI{PLMNIdentity: plmnVec(), TAC: 7} }

func globalENBIDVec() GlobalENBID {
	return GlobalENBID{PLMNIdentity: plmnVec(), ENBID: ENBID{MacroENBID: Ptr(MacroENBID(0x12345))}}
}

func gnbVec() GlobalRANNodeID {
	return GlobalRANNodeID{GNB: &GNB{GlobalGNBID: GlobalGNBID{PLMNIdentity: plmnVec(), GNBID: GNBIdentity{GNBID: &GNBID{Value: 0x101, Bits: 22}}}}}
}

func encodedVec(t *testing.T, m per.Marshaler) []byte {
	t.Helper()

	return ieRaw(t, m)
}

func sourceContainerVec() *SourceeNBToTargeteNBTransparentContainer {
	return &SourceeNBToTargeteNBTransparentContainer{
		RRCContainer:         RRCContainer{0x01},
		TargetCellID:         ecgiVec(),
		UEHistoryInformation: UEHistoryInformation{{UTRANCell: Ptr(LastVisitedUTRANCellInformation{0x02})}},
	}
}

func sourceContainerCase(t *testing.T, name string, id ProtocolIEID, typed per.Marshaler, get func(*SourceeNBToTargeteNBTransparentContainer) per.Marshaler) typedExtCase {
	t.Helper()

	return typedExtCase{
		name: name, id: id, crit: CriticalityIgnore, value: encodedVec(t, typed),
		wire: func(raw rawIE) extCodec {
			c := sourceContainerVec()
			c.iEExtensions.Append(raw)

			return c
		},
		fresh: func() extCodec { return &SourceeNBToTargeteNBTransparentContainer{} },
		typed: func(v extCodec) per.Marshaler { return get(v.(*SourceeNBToTargeteNBTransparentContainer)) },
	}
}

func erabInfoCase(t *testing.T, name string, id ProtocolIEID, typed per.Marshaler, get func(*ERABInformationListItem) per.Marshaler) typedExtCase {
	t.Helper()

	return typedExtCase{
		name: name, id: id, crit: CriticalityIgnore, value: encodedVec(t, typed),
		wire: func(raw rawIE) extCodec {
			c := &ERABInformationListItem{ERABID: 5}
			c.iEExtensions.Append(raw)

			return c
		},
		fresh: func() extCodec { return &ERABInformationListItem{} },
		typed: func(v extCodec) per.Marshaler { return get(v.(*ERABInformationListItem)) },
	}
}

func hrlCase(t *testing.T, name string, id ProtocolIEID, typed per.Marshaler, get func(*HandoverRestrictionList) per.Marshaler) typedExtCase {
	t.Helper()

	return typedExtCase{
		name: name, id: id, crit: CriticalityIgnore, value: encodedVec(t, typed),
		wire: func(raw rawIE) extCodec {
			c := &HandoverRestrictionList{ServingPLMN: plmnVec()}
			c.iEExtensions.Append(raw)

			return c
		},
		fresh: func() extCodec { return &HandoverRestrictionList{} },
		typed: func(v extCodec) per.Marshaler { return get(v.(*HandoverRestrictionList)) },
	}
}

func lastVisitedCase(t *testing.T, name string, id ProtocolIEID, typed per.Marshaler, get func(*LastVisitedEUTRANCellInformation) per.Marshaler) typedExtCase {
	t.Helper()

	return typedExtCase{
		name: name, id: id, crit: CriticalityIgnore, value: encodedVec(t, typed),
		wire: func(raw rawIE) extCodec {
			c := &LastVisitedEUTRANCellInformation{GlobalCellID: ecgiVec(), TimeUEStayedInCell: 30}
			c.iEExtensions.Append(raw)

			return c
		},
		fresh: func() extCodec { return &LastVisitedEUTRANCellInformation{} },
		typed: func(v extCodec) per.Marshaler { return get(v.(*LastVisitedEUTRANCellInformation)) },
	}
}

func typedExtensionCases(t *testing.T) []typedExtCase {
	t.Helper()

	sourceTLA := tlaOf(10, 0, 0, 1)
	nodeTLA := tlaOf(0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1)
	pscell := PSCellInformation{NCGI: NRCGI{PLMNIdentity: plmnVec(), NRCellIdentity: 0x123456789}}

	return []typedExtCase{
		{
			name: "ServedGUMMEIsItem/GUMMEIType", id: IDGUMMEIType, crit: CriticalityIgnore, value: []byte{0x80},
			wire: func(raw rawIE) extCodec {
				c := &ServedGUMMEIsItem{ServedPLMNs: []PLMNIdentity{plmnVec()}, ServedGroupIDs: []MMEGroupID{{0x80, 0x01}}, ServedMMECs: []MMECode{1}}
				c.iEExtensions.Append(raw)

				return c
			},
			fresh: func() extCodec { return &ServedGUMMEIsItem{} },
			typed: func(v extCodec) per.Marshaler { return v.(*ServedGUMMEIsItem).GUMMEIType },
		},
		hrlCase(t, "HandoverRestrictionList/NRrestrictioninEPSasSecondaryRAT", IDNRrestrictioninEPSasSecondaryRAT, Ptr(NRrestrictedinEPSasSecondaryRAT), func(c *HandoverRestrictionList) per.Marshaler { return c.NRrestrictioninEPSasSecondaryRAT }),
		hrlCase(t, "HandoverRestrictionList/UnlicensedSpectrumRestriction", IDUnlicensedSpectrumRestriction, Ptr(UnlicensedRestricted), func(c *HandoverRestrictionList) per.Marshaler { return c.UnlicensedSpectrumRestriction }),
		hrlCase(t, "HandoverRestrictionList/CNTypeRestrictions", IDCNTypeRestrictions, &CNTypeRestrictions{{PLMNIdentity: plmnVec(), CNType: CNTypeEPCForbidden}}, func(c *HandoverRestrictionList) per.Marshaler { return &c.CNTypeRestrictions }),
		hrlCase(t, "HandoverRestrictionList/NRrestrictionin5GS", IDNRrestrictionin5GS, Ptr(NRrestrictedin5GS), func(c *HandoverRestrictionList) per.Marshaler { return c.NRrestrictionin5GS }),
		hrlCase(t, "HandoverRestrictionList/LastNG-RANPLMNIdentity", IDLastNGRANPLMNIdentity, Ptr(plmnVec()), func(c *HandoverRestrictionList) per.Marshaler { return c.LastNGRANPLMNIdentity }),
		hrlCase(t, "HandoverRestrictionList/RAT-Restrictions", IDRATRestrictions, &RATRestrictions{{PLMNIdentity: plmnVec(), RATRestrictionInformation: RATRestrictionInformation{Value: 0x80}}}, func(c *HandoverRestrictionList) per.Marshaler { return &c.RATRestrictions }),
		{
			name: "UserLocationInformation/PSCellInformation", id: IDPSCellInformation, crit: CriticalityIgnore, value: encodedVec(t, &pscell),
			wire: func(raw rawIE) extCodec {
				c := &UserLocationInformation{EUTRANCGI: ecgiVec(), TAI: taiVec()}
				c.iEExtensions.Append(raw)

				return c
			},
			fresh: func() extCodec { return &UserLocationInformation{} },
			typed: func(v extCodec) per.Marshaler { return v.(*UserLocationInformation).PSCellInformation },
		},
		{
			name: "UserLocationInformation/LTE-NTN-TAI-Information", id: IDLTENTNTAIInformation, crit: CriticalityIgnore,
			value: encodedVec(t, &LTENTNTAIInformation{ServingPLMN: plmnVec(), TACListInLTENTN: []TAC{7, 8}, UELocationDerivedTAC: Ptr(TAC(8))}),
			wire: func(raw rawIE) extCodec {
				c := &UserLocationInformation{EUTRANCGI: ecgiVec(), TAI: taiVec()}
				c.iEExtensions.Append(raw)

				return c
			},
			fresh: func() extCodec { return &UserLocationInformation{} },
			typed: func(v extCodec) per.Marshaler { return v.(*UserLocationInformation).LTENTNTAIInformation },
		},
		{
			name: "RequestType/RequestTypeAdditionalInfo", id: IDRequestTypeAdditionalInfo, crit: CriticalityIgnore, value: []byte{0x00},
			wire: func(raw rawIE) extCodec {
				c := &RequestType{}
				c.iEExtensions.Append(raw)

				return c
			},
			fresh: func() extCodec { return &RequestType{} },
			typed: func(v extCodec) per.Marshaler { return v.(*RequestType).RequestTypeAdditionalInfo },
		},
		erabInfoCase(t, "E-RABInformationListItem/DAPSRequestInfo", IDDAPSRequestInfo, &DAPSRequestInfo{}, func(c *ERABInformationListItem) per.Marshaler { return c.DAPSRequestInfo }),
		erabInfoCase(t, "E-RABInformationListItem/SourceTransportLayerAddress", IDSourceTransportLayerAddress, &sourceTLA, func(c *ERABInformationListItem) per.Marshaler { return c.SourceTransportLayerAddress }),
		erabInfoCase(t, "E-RABInformationListItem/SecurityIndication", IDSecurityIndication, &SecurityIndication{IntegrityProtectionIndication: IntegrityProtectionIndication(1)}, func(c *ERABInformationListItem) per.Marshaler { return c.SecurityIndication }),
		erabInfoCase(t, "E-RABInformationListItem/SourceNodeTransportLayerAddress", IDSourceNodeTransportLayerAddress, &nodeTLA, func(c *ERABInformationListItem) per.Marshaler { return c.SourceNodeTransportLayerAddress }),
		lastVisitedCase(t, "LastVisitedEUTRANCellInformation/Time-UE-StayedInCell-EnhancedGranularity", IDTimeUEStayedInCellEnhancedGranularity, Ptr(TimeUEStayedInCellEnhancedGranularity(40950)), func(c *LastVisitedEUTRANCellInformation) per.Marshaler {
			return c.TimeUEStayedInCellEnhancedGranularity
		}),
		lastVisitedCase(t, "LastVisitedEUTRANCellInformation/HO-Cause", IDHOCause, Ptr(CauseRadioNetworkTimeCriticalHandover.Cause()), func(c *LastVisitedEUTRANCellInformation) per.Marshaler { return c.HOCause }),
		lastVisitedCase(t, "LastVisitedEUTRANCellInformation/lastVisitedPSCellList", IDLastVisitedPSCellList, &LastVisitedPSCellList{{PSCellID: &pscell, TimeStay: 12}}, func(c *LastVisitedEUTRANCellInformation) per.Marshaler { return &c.LastVisitedPSCellList }),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/MobilityInformation", IDMobilityInformation, Ptr(MobilityInformation(0xdeadbeef)), func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler { return c.MobilityInformation }),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/uE-HistoryInformationFromTheUE", IDUEHistoryInformationFromTheUE, &UEHistoryInformationFromTheUE{0x01, 0x02}, func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler {
			return c.UEHistoryInformationFromTheUE
		}),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/IMSvoiceEPSfallbackfrom5G", IDIMSvoiceEPSfallbackfrom5G, Ptr(IMSvoiceEPSfallbackfrom5GTrue), func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler { return c.IMSvoiceEPSfallbackfrom5G }),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/AdditionalRRMPriorityIndex", IDAdditionalRRMPriorityIndex, Ptr(AdditionalRRMPriorityIndex(0x80000001)), func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler {
			return c.AdditionalRRMPriorityIndex
		}),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/ContextatSource", IDContextatSource, &ContextatSource{SourceNGRANNodeID: gnbVec(), RANUENGAPID: 0xfffffffe}, func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler { return c.ContextatSource }),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/IntersystemMeasurementConfiguration", IDIntersystemMeasurementConfiguration, &IntersystemMeasurementConfiguration{
			RSRP: Ptr(uint8(127)),
			InterSystemMeasurementParameters: InterSystemMeasurementParameters{MeasurementDuration: 100, InterSystemMeasurementList: InterSystemMeasurementList{
				{FreqBandIndicatorNR: 78, SSBFrequencies: 632628, SubcarrierSpacingSSB: SubcarrierSpacingSSBkHz480, SMTC: []byte{0x01}},
			}},
		}, func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler {
			return c.IntersystemMeasurementConfiguration
		}),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/SourceNodeID", IDSourceNodeID, &SourceNodeID{SourceNgRanNodeID: &SourceNgRanNodeID{GlobalRANNodeID: gnbVec(), SelectedTAI: FiveGSTAI{PLMNIdentity: plmnVec(), TAC: 0x10203}}}, func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler {
			return c.SourceNodeID
		}),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/EmergencyIndicator", IDEmergencyIndicator, Ptr(EmergencyIndicatorTrue), func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler { return c.EmergencyIndicator }),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/UEContextReferenceatSourceeNB", IDUEContextReferenceatSourceeNB, Ptr(ENBUES1APID(0xabcdef)), func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler {
			return c.UEContextReferenceatSourceeNB
		}),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/SourceSNID", IDSourceSNID, Ptr(gnbVec()), func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler { return c.SourceSNID }),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/Direct-Forwarding-Path-Availability", IDDirectForwardingPathAvailability, Ptr(DirectForwardingPathAvailable), func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler {
			return c.DirectForwardingPathAvailability
		}),
		sourceContainerCase(t, "SourceeNB-ToTargeteNB/TimeBasedHandoverInformation", IDTimeBasedHandoverInformation, &TimeBasedHandoverInformation{HOWindowStart: 1048575, HOWindowDuration: 6000}, func(c *SourceeNBToTargeteNBTransparentContainer) per.Marshaler {
			return c.TimeBasedHandoverInformation
		}),
		{
			name: "SupportedTAs-Item/RAT-Type", id: IDRATType, crit: CriticalityReject, value: []byte{0x87},
			wire: func(raw rawIE) extCodec {
				c := &SupportedTAItem{TAC: 1, BroadcastPLMNs: BPLMNs{plmnVec()}}
				c.iEExtensions.Append(raw)

				return c
			},
			fresh: func() extCodec { return &SupportedTAItem{} },
			typed: func(v extCodec) per.Marshaler { return v.(*SupportedTAItem).RATType },
		},
		{
			name: "TraceActivation/MDTConfigurationNR", id: IDMDTConfigurationNR, crit: CriticalityIgnore, value: []byte{0x02, 0xab, 0xcd},
			wire: func(raw rawIE) extCodec {
				c := &TraceActivation{TraceCollectionEntityIPAddress: tlaOf(10, 0, 0, 2)}
				c.iEExtensions.Append(raw)

				return c
			},
			fresh: func() extCodec { return &TraceActivation{} },
			typed: func(v extCodec) per.Marshaler { return v.(*TraceActivation).MDTConfigurationNR },
		},
		{
			name: "TraceActivation/TraceCollectionEntityURI", id: IDTraceCollectionEntityURI, crit: CriticalityIgnore, value: append([]byte{0x05}, "a.b/c"...),
			wire: func(raw rawIE) extCodec {
				c := &TraceActivation{TraceCollectionEntityIPAddress: tlaOf(10, 0, 0, 2)}
				c.iEExtensions.Append(raw)

				return c
			},
			fresh: func() extCodec { return &TraceActivation{} },
			typed: func(v extCodec) per.Marshaler { return v.(*TraceActivation).TraceCollectionEntityURI },
		},
		{
			name: "SONConfigurationTransfer/RequestedTNLInfo", id: IDRequestedTNLInfo, crit: CriticalityIgnore, value: []byte{0x00, 0x00, 0xf1, 0x10},
			wire: func(raw rawIE) extCodec {
				c := &SONConfigurationTransfer{
					TargeteNBID:    TargeteNBID{GlobalENBID: globalENBIDVec(), SelectedTAI: taiVec()},
					SourceeNBID:    SourceeNBID{GlobalENBID: globalENBIDVec(), SelectedTAI: taiVec()},
					SONInformation: SONInformation{SONInformationRequest: Ptr(SONInformationRequestDeactivateMuting)},
				}
				c.iEExtensions.Append(raw)

				return c
			},
			fresh: func() extCodec { return &SONConfigurationTransfer{} },
			typed: func(v extCodec) per.Marshaler { return v.(*SONConfigurationTransfer).RequestedTNLInfo },
		},
		{
			name: "X2TNLConfigurationInfo/eNBX2ExtendedTransportLayerAddresses", id: IDENBX2ExtendedTransportLayerAddresses, crit: CriticalityIgnore,
			value: encodedVec(t, &ENBX2ExtTLAs{{IPsecTLA: Ptr(tlaOf(192, 0, 2, 1)), GTPTLAa: ENBX2GTPTLAs{tlaOf(192, 0, 2, 2)}}}),
			wire: func(raw rawIE) extCodec {
				c := &X2TNLConfigurationInfo{ENBX2TransportLayerAddresses: ENBX2TLAs{tlaOf(192, 0, 2, 3)}}
				c.iEExtensions.Append(raw)

				return c
			},
			fresh: func() extCodec { return &X2TNLConfigurationInfo{} },
			typed: func(v extCodec) per.Marshaler {
				return &v.(*X2TNLConfigurationInfo).ENBX2ExtendedTransportLayerAddresses
			},
		},
	}
}

func TestTypedExtensionsDecodeFromTheWire(t *testing.T) {
	for _, tc := range typedExtensionCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			in := encodedVec(t, tc.wire(rawIE{ID: tc.id, Criticality: tc.crit, Value: tc.value}))

			got := tc.fresh()
			if err := got.UnmarshalPER(per.NewReader(in), per.Aligned); err != nil {
				t.Fatalf("decode: %v", err)
			}

			typed := tc.typed(got)
			if isNilMarshaler(typed) {
				t.Fatal("the extension was not decoded into its typed field")
			}

			if v := encodedVec(t, typed); !bytes.Equal(v, tc.value) {
				t.Fatalf("typed value encodes as % x, want % x", v, tc.value)
			}

			if got.Diagnose(&ap.Diagnosis[ProtocolIEID]{}) {
				t.Fatal("a spec-defined extension is reported as not comprehended")
			}

			if out := encodedVec(t, got); !bytes.Equal(out, in) {
				t.Fatalf("re-encoded\n got % x\nwant % x", out, in)
			}
		})
	}
}

func isNilMarshaler(m per.Marshaler) bool {
	v := reflect.ValueOf(m)
	if m == nil || v.IsNil() {
		return true
	}

	return v.Elem().Kind() == reflect.Slice && v.Elem().Len() == 0
}
