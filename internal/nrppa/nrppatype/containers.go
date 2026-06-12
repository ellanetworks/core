// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package nrppatype

// Open-type / extension containers.
//
// For the E-CID MVP these are never populated: every CHOICE choice-Extension
// alternative and every SEQUENCE iE-Extensions field is modelled as an empty
// open-type struct (mirroring ngapType's `ProtocolIESingleContainer*ExtIEs
// struct{}` pattern). The field types exist so the surrounding CHOICE/SEQUENCE
// has the right shape, but the values are always nil/absent on the wire.
//
// Limitation: a peer that actually carries one of these extensions cannot be
// decoded; supporting that would require the full ProtocolExtensionField
// modelling (id + criticality + skippable open type).

// --- ProtocolIE-Single-Container { ... } choice-Extension open types ---

// Cause-ExtensionIE.
type ProtocolIESingleContainerCauseExtensionIE struct{}

// NG-RANCell-ExtensionIE.
type ProtocolIESingleContainerNGRANCellExtensionIE struct{}

// MeasuredResultsValue-ExtensionIE.
type ProtocolIESingleContainerMeasuredResultsValueExtensionIE struct{}

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
