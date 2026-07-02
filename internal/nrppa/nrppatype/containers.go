// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppatype

// Open-type / extension containers.
//
// Most choice-Extension / iE-Extensions containers are empty structs (the
// values are always nil/absent on the wire). The exceptions are those that
// carry NR measurement types over the wire: MeasuredResultsValue-ExtensionIE
// is modelled as a full ProtocolIEField (id + criticality + open type) so that
// gNB-reported SS-RSRP, SS-RSRQ, CSI-RSRP and CSI-RSRQ decode correctly.
//
// The remaining empty structs mirror ngapType's `struct{}` pattern: the types
// exist so surrounding CHOICE/SEQUENCE have the right shape, but the values
// are always nil/absent on the wire for the E-CID MVP.

// --- ProtocolIE-Single-Container { ... } choice-Extension open types ---

// Cause-ExtensionIE.
type ProtocolIESingleContainerCauseExtensionIE struct{}

// NG-RANCell-ExtensionIE.
type ProtocolIESingleContainerNGRANCellExtensionIE struct{}

// MeasuredResultsValue-ExtensionIE.
type ProtocolIESingleContainerMeasuredResultsValueExtensionIE struct {
	MeasuredResultsValueExtIEs *MeasuredResultsValueExtIEs
}

// --- ProtocolExtensionContainer { ... } iE-Extensions open types ---

// E-CID-MeasurementResult-ExtIEs.
type ProtocolExtensionContainerECIDMeasurementResultExtIEs struct{}

// NG-RANAccessPointPosition-ExtIEs.
type ProtocolExtensionContainerNGRANAccessPointPositionExtIEs struct{}

// NG-RAN-CGI-ExtIEs.
type ProtocolExtensionContainerNGRANCGIExtIEs struct{}

// MeasurementQuantitiesValue-ExtIEs (MeasurementQuantities-Item iE-Extensions).
type ProtocolExtensionContainerMeasurementQuantitiesItemExtIEs struct{}

// CriticalityDiagnostics-ExtIEs.
type ProtocolExtensionContainerCriticalityDiagnosticsExtIEs struct{}

// CriticalityDiagnostics-IE-List-ExtIEs (per-item iE-Extensions).
type ProtocolExtensionContainerCriticalityDiagnosticsIEListExtIEs struct{}

// ResultRSRP-EUTRA-Item-ExtIEs.
type ProtocolExtensionContainerResultRSRPEUTRAItemExtIEs struct{}

// ResultRSRQ-EUTRA-Item-ExtIEs.
type ProtocolExtensionContainerResultRSRQEUTRAItemExtIEs struct{}

// ResultSS-RSRP-Item-ExtIEs.
type ProtocolExtensionContainerResultSSRSRPItemExtIEs struct{}

// ResultSS-RSRP-PerSSB-Item-ExtIEs.
type ProtocolExtensionContainerResultSSRSRPPerSSBItemExtIEs struct{}

// ResultSS-RSRQ-Item-ExtIEs.
type ProtocolExtensionContainerResultSSRSRQItemExtIEs struct{}

// ResultSS-RSRQ-PerSSB-Item-ExtIEs.
type ProtocolExtensionContainerResultSSRSRQPerSSBItemExtIEs struct{}

// CGI-NR-ExtIEs.
type ProtocolExtensionContainerCGINRExtIEs struct{}

// ResultCSI-RSRP-Item-ExtIEs.
type ProtocolExtensionContainerResultCSIRSRPItemExtIEs struct{}

// ResultCSI-RSRQ-Item-ExtIEs.
type ProtocolExtensionContainerResultCSIRSRQItemExtIEs struct{}
