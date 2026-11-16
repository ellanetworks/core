// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"encoding/hex"
	"reflect"

	"github.com/ellanetworks/core/internal/decoder/rrc"
	"github.com/ellanetworks/core/internal/decoder/utils"
	"github.com/ellanetworks/core/s1ap"
)

type typedIE struct {
	id    s1ap.ProtocolIEID
	crit  s1ap.Criticality
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
	pdu, err := s1ap.ParseRawPDU(raw)
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

func render(v any) any {
	return utils.Render(v, leaf)
}

func leaf(v any) (any, bool) {
	switch t := v.(type) {
	case s1ap.PLMNIdentity:
		return plmnToID(t), true
	case s1ap.TAI:
		return tai(t), true
	case s1ap.EUTRANCGI:
		return eutranCGI(t), true
	case s1ap.STMSI:
		return stmsi(t), true
	case s1ap.UES1APIDs:
		return ues1apIDs(t), true
	case s1ap.GUMMEI:
		return gummei(t), true
	case s1ap.GlobalENBID:
		return globalENBID(t), true
	case s1ap.Cause:
		return cause(t), true
	case s1ap.CriticalityDiagnostics:
		return criticalityDiagnostics(t), true
	case s1ap.UserLocationInformation:
		return userLocationInformation(t), true
	case s1ap.HandoverRestrictionList:
		return handoverRestrictionList(t), true
	case s1ap.TargetID:
		return targetID(t), true
	case s1ap.SecurityContext:
		return securityContext(t), true
	case s1ap.TraceActivation:
		return traceActivation(t), true
	case s1ap.NASPDU:
		return nasPDU(t), true
	case s1ap.UERadioCapability:
		return rrc.DescribeS1AP(t), true
	case s1ap.UESecurityCapabilities:
		return UESecurityCapabilities{
			EncryptionAlgorithms:          securityAlgorithms(t.EncryptionAlgorithms.Value, "EEA"),
			IntegrityProtectionAlgorithms: securityAlgorithms(t.IntegrityProtectionAlgorithms.Value, "EIA"),
		}, true
	case s1ap.SecurityKey:
		return hex.EncodeToString(t[:]), true
	case s1ap.SecurityAlgorithms:
		return utils.BitStringUint(uint64(t.Value), 16), true
	case s1ap.ProtocolIEID:
		return ieEnum(t), true
	case s1ap.Criticality:
		return criticalityToEnum(t), true
	case s1ap.ProcedureCode:
		return procedureCodeToEnum(t), true
	case s1ap.SingleContainer:
		return ie(t.ID, t.Criticality, rawIEValue{Hex: hex.EncodeToString([]byte(t.Value))}), true
	case s1ap.GNBID:
		return utils.BitStringUint(uint64(t.Value), t.Bits), true
	}

	if bits, ok := bitStringWidths[reflect.TypeOf(v)]; ok {
		return utils.BitStringUint(reflect.ValueOf(v).Uint(), bits), true
	}

	return nil, false
}

var bitStringWidths = map[reflect.Type]int{
	reflect.TypeFor[s1ap.CSGId]():                                   27,
	reflect.TypeFor[s1ap.ExtendedUEIdentityIndexValue]():            14,
	reflect.TypeFor[s1ap.MaskedIMEISV]():                            64,
	reflect.TypeFor[s1ap.NBIoTUEIdentityIndexValue]():               12,
	reflect.TypeFor[s1ap.UEApplicationLayerMeasurementCapability](): 8,
	reflect.TypeFor[s1ap.ScheduledCommunicationTimeDayofWeek]():     7,
	reflect.TypeFor[s1ap.NRCellIdentity]():                          36,
	reflect.TypeFor[s1ap.MobilityInformation]():                     32,
	reflect.TypeFor[s1ap.AdditionalRRMPriorityIndex]():              32,
	reflect.TypeFor[s1ap.MessageIdentifier]():                       16,
	reflect.TypeFor[s1ap.SerialNumber]():                            16,
	reflect.TypeFor[s1ap.DataCodingScheme]():                        8,
	reflect.TypeFor[s1ap.MacroENBID]():                              20,
	reflect.TypeFor[s1ap.HomeENBID]():                               28,
	reflect.TypeFor[s1ap.ShortMacroENBID]():                         18,
	reflect.TypeFor[s1ap.LongMacroENBID]():                          21,
	reflect.TypeFor[s1ap.CellIdentity]():                            28,
	reflect.TypeFor[s1ap.InterfacesToTrace]():                       8,
}
