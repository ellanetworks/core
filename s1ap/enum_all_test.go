// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/asn1spec"
)

type enumShape struct {
	Root       []string
	Extensible bool
	Extension  []string
}

type enumSpecCase struct {
	asn1 string
	got  enumShape
	typ  reflect.Type
}

func enumSpec[T per.Integer](asn1 string, e *per.Enum[T]) enumSpecCase {
	return enumSpecCase{
		asn1: asn1,
		got:  enumShape{Root: e.Root, Extensible: e.Extensible || len(e.Extension) > 0, Extension: e.Extension},
		typ:  reflect.TypeFor[T](),
	}
}

func asn1Enum(t *testing.T, ref string) enumShape {
	t.Helper()

	s := vendoredSpec(t)

	typeName, component, _ := strings.Cut(ref, ".")

	ta := s.Type(typeName)
	if ta == nil {
		t.Fatalf("ASN.1 type %s not found", typeName)
	}

	ty := ta.Type

	if component != "" {
		c := ty.Component(component)
		if c == nil {
			t.Fatalf("%s has no component %s", typeName, component)
		}

		ty = c.Type
	}

	ty, err := s.Underlying(ty)
	if err != nil {
		t.Fatal(err)
	}

	if ty.Kind != asn1spec.Enumerated {
		t.Fatalf("%s is %s, not ENUMERATED", ref, ty.Kind)
	}

	var out enumShape

	for _, it := range ty.Enum {
		if it.HasNumber {
			t.Fatalf("%s: %s has an explicit number", ref, it.Name)
		}

		if it.Extension {
			out.Extension = append(out.Extension, it.Name)
		} else {
			out.Root = append(out.Root, it.Name)
		}
	}

	out.Extensible = ty.Extensible

	return out
}

func enumCases() []enumSpecCase {
	return []enumSpecCase{
		enumSpec("TriggeringMessage", &enumTriggeringMessage),
		enumSpec("HandoverType", &enumHandoverType),
		enumSpec("CNDomain", &enumCNDomain),
		enumSpec("Pre-emptionCapability", &enumPreemptionCapability),
		enumSpec("Pre-emptionVulnerability", &enumPreemptionVulnerability),
		enumSpec("TypeOfError", &enumTypeOfError),
		enumSpec("EventType", &enumEventType),
		enumSpec("ReportArea", &enumReportArea),
		enumSpec("BearerType", &enumBearerType),
		enumSpec("Ethernet-Type", &enumEthernetType),
		enumSpec("IntegrityProtectionIndication", &enumIntegrityProtectionIndication),
		enumSpec("PagingPriority", &enumPagingPriority),
		enumSpec("UE-RetentionInformation", &enumUERetentionInformation),
		enumSpec("PagingDRX", &enumPagingDRX),
		enumSpec("ForbiddenInterRATs", &enumForbiddenInterRATs),
		enumSpec("Data-Forwarding-Not-Possible", &enumDataForwardingNotPossible),
		enumSpec("TimeToWait", &enumTimeToWait),
		enumSpec("TraceDepth", &enumTraceDepth),
		enumSpec("RRC-Establishment-Cause", &enumRRCEstablishmentCause),
		enumSpec("GWContextReleaseIndication", &enumGWContextReleaseIndication),
		enumSpec("DL-Forwarding", &enumDLForwarding),
		enumSpec("Cell-Size", &enumCellSize),
		enumSpec("Direct-Forwarding-Path-Availability", &enumDirectForwardingPathAvailability),
		enumSpec("ResetAll", &enumResetAll),
		enumSpec("OverloadAction", &enumOverloadAction),
		enumSpec("GUMMEIType", &enumGUMMEIType),
		enumSpec("NRrestrictioninEPSasSecondaryRAT", &enumNRrestrictioninEPSasSecondaryRAT),
		enumSpec("UnlicensedSpectrumRestriction", &enumUnlicensedSpectrumRestriction),
		enumSpec("NRrestrictionin5GS", &enumNRrestrictionin5GS),
		enumSpec("CNType", &enumCNType),
		enumSpec("RequestTypeAdditionalInfo", &enumRequestTypeAdditionalInfo),
		enumSpec("DAPSRequestInfo.dAPSIndicator", &enumDAPSIndicator),
		enumSpec("IMSvoiceEPSfallbackfrom5G", &enumIMSvoiceEPSfallbackfrom5G),
		enumSpec("EmergencyIndicator", &enumEmergencyIndicator),
		enumSpec("InterSystemMeasurementItem.subcarrierSpacingSSB", &enumSubcarrierSpacingSSB),
		enumSpec("RAT-Type", &enumRATType),
		enumSpec("CoarseUELocationRequested", &enumCoarseUELocationRequested),
		enumSpec("TimeRefDistribution", &enumTimeRefDistribution),
		enumSpec("SONInformationRequest", &enumSONInformationRequest),
		enumSpec("SynchronisationStatus", &enumSynchronisationStatus),
		enumSpec("MutingAvailabilityIndication", &enumMutingAvailabilityIndication),
		enumSpec("MutingPatternInformation.muting-pattern-period", &enumMutingPatternPeriod),
		enumSpec("ListeningSubframePattern.pattern-period", &enumPatternPeriod),
		enumSpec("ConcurrentWarningMessageIndicator", &enumConcurrentWarningMessageIndicator),
		enumSpec("KillAllWarningMessages", &enumKillAllWarningMessages),
		enumSpec("CauseRadioNetwork", &enumCauseRadioNetwork),
		enumSpec("CauseTransport", &enumCauseTransport),
		enumSpec("CauseNas", &enumCauseNas),
		enumSpec("CauseProtocol", &enumCauseProtocol),
		enumSpec("CauseMisc", &enumCauseMisc),
		enumSpec("AdditionalCSFallbackIndicator", &enumAdditionalCSFallbackIndicator),
		enumSpec("AerialUEsubscriptionInformation", &enumAerialUEsubscriptionInformation),
		enumSpec("CE-ModeBRestricted", &enumCEModeBRestricted),
		enumSpec("CE-mode-B-SupportIndicator", &enumCEmodeBSupportIndicator),
		enumSpec("CSFallbackIndicator", &enumCSFallbackIndicator),
		enumSpec("CSGMembershipStatus", &enumCSGMembershipStatus),
		enumSpec("CellAccessMode", &enumCellAccessMode),
		enumSpec("Coverage-Level", &enumCoverageLevel),
		enumSpec("DLNASPDUDeliveryAckRequest", &enumDLNASPDUDeliveryAckRequest),
		enumSpec("EDT-Session", &enumEDTSession),
		enumSpec("EndIndication", &enumEndIndication),
		enumSpec("EnhancedCoverageRestricted", &enumEnhancedCoverageRestricted),
		enumSpec("IAB-Authorized", &enumIABAuthorized),
		enumSpec("IAB-Node-Indication", &enumIABNodeIndication),
		enumSpec("IAB-Supported", &enumIABSupported),
		enumSpec("LTE-M-Indication", &enumLTEMIndication),
		enumSpec("MMERelaySupportIndicator", &enumMMERelaySupportIndicator),
		enumSpec("ManagementBasedMDTAllowed", &enumManagementBasedMDTAllowed),
		enumSpec("NB-IoT-DefaultPagingDRX", &enumNBIoTDefaultPagingDRX),
		enumSpec("NB-IoT-PagingDRX", &enumNBIoTPagingDRX),
		enumSpec("NotifySourceeNB", &enumNotifySourceeNB),
		enumSpec("PS-ServiceNotAvailable", &enumPSServiceNotAvailable),
		enumSpec("PagingCause", &enumPagingCause),
		enumSpec("PendingDataIndication", &enumPendingDataIndication),
		enumSpec("RelayNode-Indicator", &enumRelayNodeIndicator),
		enumSpec("SRVCCHOIndication", &enumSRVCCHOIndication),
		enumSpec("SRVCCOperationNotPossible", &enumSRVCCOperationNotPossible),
		enumSpec("SRVCCOperationPossible", &enumSRVCCOperationPossible),
		enumSpec("SecondaryRATDataUsageRequest", &enumSecondaryRATDataUsageRequest),
		enumSpec("Subscription-Based-UE-DifferentiationInfo.periodicCommunicationIndicator", &enumSubscriptionBasedUEDifferentiationInfoPeriodicCommunicationIndicator),
		enumSpec("Subscription-Based-UE-DifferentiationInfo.stationaryIndication", &enumSubscriptionBasedUEDifferentiationInfoStationaryIndication),
		enumSpec("Subscription-Based-UE-DifferentiationInfo.trafficProfile", &enumSubscriptionBasedUEDifferentiationInfoTrafficProfile),
		enumSpec("Subscription-Based-UE-DifferentiationInfo.batteryIndication", &enumSubscriptionBasedUEDifferentiationInfoBatteryIndication),
		enumSpec("UECapabilityInfoRequest", &enumUECapabilityInfoRequest),
		enumSpec("UEUserPlaneCIoTSupportIndicator", &enumUEUserPlaneCIoTSupportIndicator),
		enumSpec("ExpectedHOInterval", &enumExpectedHOInterval),
		enumSpec("NB-IoT-Paging-eDRX-Cycle", &enumNBIoTPagingeDRXCycle),
		enumSpec("NB-IoT-PagingTimeWindow", &enumNBIoTPagingTimeWindow),
		enumSpec("VehicleUE", &enumVehicleUE),
		enumSpec("PedestrianUE", &enumPedestrianUE),
		enumSpec("Paging-eDRX-Cycle", &enumPagingeDRXCycle),
		enumSpec("PagingTimeWindow", &enumPagingTimeWindow),
		enumSpec("ProSeDirectDiscovery", &enumProSeDirectDiscovery),
		enumSpec("ProSeDirectCommunication", &enumProSeDirectCommunication),
		enumSpec("ProSeUEtoNetworkRelaying", &enumProSeUEtoNetworkRelaying),
		enumSpec("PagingProbabilityInformation", &enumPagingProbabilityInformation),
		enumSpec("NextPagingAreaScope", &enumNextPagingAreaScope),
		enumSpec("SourceOfUEActivityBehaviourInformation", &enumSourceOfUEActivityBehaviourInformation),
		enumSpec("SecondaryRATType", &enumSecondaryRATType),
		enumSpec("Range", &enumRangeType),
	}
}

func TestEnumLiteralsMatchASN1(t *testing.T) {
	for _, c := range enumCases() {
		t.Run(c.asn1, func(t *testing.T) {
			want := asn1Enum(t, c.asn1)

			if !slices.Equal(c.got.Root, want.Root) || c.got.Extensible != want.Extensible || !slices.Equal(c.got.Extension, want.Extension) {
				t.Errorf("modeled as %+v, spec says %+v", c.got, want)
			}
		})
	}
}

func TestEnumCasesAreComplete(t *testing.T) {
	cases := enumCases()

	types := make([]reflect.Type, len(cases))
	for i, c := range cases {
		types[i] = c.typ
	}

	checkComplete(t, "per.Enum", scannedDecls(t).enums, types)
}

func TestCriticalityLiteralsMatchASN1(t *testing.T) {
	want := asn1Enum(t, "Criticality")

	got := enumShape{}
	for _, c := range []Criticality{CriticalityReject, CriticalityIgnore, CriticalityNotify} {
		got.Root = append(got.Root, c.String())
	}

	if !slices.Equal(got.Root, want.Root) || want.Extensible {
		t.Errorf("Criticality = %+v, spec says %+v", got, want)
	}

	for i, c := range []Criticality{CriticalityReject, CriticalityIgnore, CriticalityNotify} {
		if int(c) != i {
			t.Errorf("%s has index %d, spec says %d", c, int(c), i)
		}
	}
}
