// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package ngap

import (
	"encoding/hex"
	"fmt"
	"reflect"

	"github.com/ellanetworks/core/internal/decoder/rrc"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/ngap"
)

type typedIE struct {
	id    ngap.ProtocolIEID
	crit  ngap.Criticality
	value any
}

func appendTyped(ies []IE, typed []typedIE) []IE {
	for _, t := range typed {
		if utils.Present(t.value) {
			ies = append(ies, ie(t.id, t.crit, render(t.value)))
		}
	}

	return ies
}

func wireOrder(raw []byte, ies []IE) []IE {
	pdu, err := ngap.ParseRawPDU(raw)
	if err != nil {
		return ies
	}

	wire := pdu.IEs()
	ids := make([]int64, len(wire))

	for i, w := range wire {
		ids[i] = int64(w.ID)
	}

	return utils.WireOrder(ids, ies, func(e IE) int64 { return e.ID.Value })
}

func renderPresent(v any) any {
	if !utils.Present(v) {
		return nil
	}

	return render(v)
}

func render(v any) any {
	return utils.Render(v, leaf)
}

func leaf(v any) (any, bool) {
	switch t := v.(type) {
	case ngap.PLMNIdentity:
		return plmnIDToDecoder(t), true
	case ngap.TAC:
		return fmt.Sprintf("%06x", uint32(t)), true
	case ngap.TAI:
		return tai(t), true
	case ngap.NRCGI:
		return nrCGI(t), true
	case ngap.EUTRACGI:
		return eutraCGI(t), true
	case ngap.NGRANCGI:
		return ngranCGI(t), true
	case ngap.SNSSAI:
		return buildSNSSAIValue(t), true
	case ngap.AllowedNSSAI:
		out := make([]SNSSAI, 0, len(t))
		for _, item := range t {
			out = append(out, buildSNSSAIValue(item.SNSSAI))
		}

		return out, true
	case ngap.GUAMI:
		return guami(t), true
	case ngap.TimeStamp:
		return timeStamp(&t), true
	case ngap.Cause:
		return cause(t), true
	case ngap.CriticalityDiagnostics:
		return criticalityDiagnostics(t), true
	case ngap.UserLocationInformation:
		return userLocationInformation(t), true
	case ngap.NASPDU:
		return libNASPDU(t), true
	case ngap.UERadioCapability:
		return rrc.DescribeNGAP(t), true
	case ngap.MobilityRestrictionList:
		return mobilityRestrictionList(t), true
	case ngap.UEAggregateMaximumBitRate:
		return UEAggregateMaximumBitRate{Downlink: int64(t.DL), Uplink: int64(t.UL), Unit: "bps"}, true
	case ngap.UESecurityCapabilities:
		return libUESecurityCapabilities(t), true
	case ngap.SecurityKey:
		return hex.EncodeToString(t[:]), true
	case ngap.SecurityAlgorithms:
		return utils.BitStringUint(uint64(t.Uint16()), 16), true
	case ngap.RATRestrictionInformation:
		return ratRestrictionInformation(t), true
	case ngap.GlobalRANNodeID:
		return buildGlobalRANNodeID(t), true
	case ngap.FiveGSTMSI:
		return buildFiveGSTMSI(t), true
	case ngap.GTPTunnel:
		return GTPTunnel{GTPTEID: uint32(t.GTPTEID), TransportLayerAddress: transportLayerAddressText(t.TransportLayerAddress)}, true
	case ngap.ExtendedAMFName:
		return extendedAMFName(t), true
	case ngap.LocationReportingRequestType:
		return libLocationReportingRequestType(t), true
	case ngap.SourceToTargetAMFInformationReroute:
		return sourceToTargetAMFInformationReroute(t), true
	case ngap.ProtocolIEID:
		return ieEnum(t), true
	case ngap.Criticality:
		return criticalityToEnum(t), true
	case ngap.ProcedureCode:
		return procedureCodeToEnum(t), true
	case ngap.SingleContainer:
		return ie(t.ID, t.Criticality, rawIEValue{Hex: hex.EncodeToString([]byte(t.Value))}), true
	case ngap.GNBIDValue:
		return utils.BitStringUint(uint64(t.Value), t.Bits), true
	}

	if bits, ok := bitStringWidths[reflect.TypeOf(v)]; ok {
		return utils.BitStringUint(reflect.ValueOf(v).Uint(), bits), true
	}

	return nil, false
}

var bitStringWidths = map[reflect.Type]int{
	reflect.TypeFor[ngap.NRCellIdentity]():                                        ngap.NRCellIdentityBits,
	reflect.TypeFor[ngap.EUTRACellIdentity]():                                     ngap.EUTRACellIdentityBits,
	reflect.TypeFor[ngap.MessageIdentifier]():                                     16,
	reflect.TypeFor[ngap.SerialNumber]():                                          16,
	reflect.TypeFor[ngap.DataCodingScheme]():                                      8,
	reflect.TypeFor[ngap.HomeENBID]():                                             28,
	reflect.TypeFor[ngap.NID]():                                                   44,
	reflect.TypeFor[ngap.InterfacesToTrace]():                                     8,
	reflect.TypeFor[ngap.N3IWFIDValue]():                                          16,
	reflect.TypeFor[ngap.MacroNgENBID]():                                          20,
	reflect.TypeFor[ngap.ShortMacroNgENBID]():                                     18,
	reflect.TypeFor[ngap.LongMacroNgENBID]():                                      21,
	reflect.TypeFor[ngap.AMFRegionID]():                                           8,
	reflect.TypeFor[ngap.AMFSetID]():                                              10,
	reflect.TypeFor[ngap.AMFPointer]():                                            6,
	reflect.TypeFor[ngap.CandidateRelayUEID]():                                    24,
	reflect.TypeFor[ngap.MeasurementsToActivate]():                                8,
	reflect.TypeFor[ngap.CAGID]():                                                 32,
	reflect.TypeFor[ngap.MDTLocationInformation]():                                8,
	reflect.TypeFor[ngap.MaskedIMEISV]():                                          64,
	reflect.TypeFor[ngap.UEIdentityIndexValueIndexLength10]():                     10,
	reflect.TypeFor[ngap.PeriodicRegistrationUpdateTimer]():                       8,
	reflect.TypeFor[ngap.ExtendedUEIdentityIndexValue]():                          16,
	reflect.TypeFor[ngap.FurtherExtendedUEIdentityIndexValue]():                   20,
	reflect.TypeFor[ngap.ScheduledCommunicationTimeDayofWeek]():                   7,
	reflect.TypeFor[ngap.ReportCharacteristics]():                                 32,
	reflect.TypeFor[ngap.ClockQualityAcceptanceCriteriaClockFrequencyStability](): 16,
	reflect.TypeFor[ngap.ENBIDMacroENBID]():                                       20,
	reflect.TypeFor[ngap.ENBIDHomeENBID]():                                        28,
	reflect.TypeFor[ngap.ENBIDShortmacroENBID]():                                  18,
	reflect.TypeFor[ngap.ENBIDLongmacroENBID]():                                   21,
}
