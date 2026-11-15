// SPDX-FileCopyrightText: Ella Networks Inc.
// SPDX-License-Identifier: BUSL-1.1

package s1ap

import (
	"github.com/ellanetworks/core/per"
	"github.com/ellanetworks/core/per/ap"
)

const maxnoofCellsinUEHistoryInfo = 16

type SourceeNBToTargeteNBTransparentContainer struct {
	RRCContainer                        RRCContainer
	ERABInformationList                 ERABInformationList `per:",optional"`
	TargetCellID                        EUTRANCGI
	SubscriberProfileIDforRFP           *SubscriberProfileIDforRFP `per:",optional"`
	UEHistoryInformation                UEHistoryInformation
	MobilityInformation                 *MobilityInformation                 `per:",extie:IDMobilityInformation,crit:ignore"`
	UEHistoryInformationFromTheUE       *UEHistoryInformationFromTheUE       `per:",extie:IDUEHistoryInformationFromTheUE,crit:ignore"`
	IMSvoiceEPSfallbackfrom5G           *IMSvoiceEPSfallbackfrom5G           `per:",extie:IDIMSvoiceEPSfallbackfrom5G,crit:ignore"`
	AdditionalRRMPriorityIndex          *AdditionalRRMPriorityIndex          `per:",extie:IDAdditionalRRMPriorityIndex,crit:ignore"`
	ContextatSource                     *ContextatSource                     `per:",extie:IDContextatSource,crit:ignore"`
	IntersystemMeasurementConfiguration *IntersystemMeasurementConfiguration `per:",extie:IDIntersystemMeasurementConfiguration,crit:ignore"`
	SourceNodeID                        *SourceNodeID                        `per:",extie:IDSourceNodeID,crit:ignore"`
	EmergencyIndicator                  *EmergencyIndicator                  `per:",extie:IDEmergencyIndicator,crit:ignore"`
	UEContextReferenceatSourceeNB       *ENBUES1APID                         `per:",extie:IDUEContextReferenceatSourceeNB,crit:ignore"`
	SourceSNID                          *GlobalRANNodeID                     `per:",extie:IDSourceSNID,crit:ignore"`
	DirectForwardingPathAvailability    *DirectForwardingPathAvailability    `per:",extie:IDDirectForwardingPathAvailability,crit:ignore"`
	TimeBasedHandoverInformation        *TimeBasedHandoverInformation        `per:",extie:IDTimeBasedHandoverInformation,crit:ignore"`
	iEExtensions                        extensionMeta                        `per:",exttable"`
	extensions                          per.ExtensionAdditions               `per:",extraw"`
}

func (c *SourceeNBToTargeteNBTransparentContainer) Marshal() (TransparentContainer, error) {
	w := per.NewWriter()

	if err := c.MarshalPER(w, per.Aligned); err != nil {
		return nil, err
	}

	w.AlignToByte()

	return TransparentContainer(w.Bytes()), nil
}

func DecodeSourceeNBToTargeteNBTransparentContainer(b []byte) (*SourceeNBToTargeteNBTransparentContainer, error) {
	return decodeValue[SourceeNBToTargeteNBTransparentContainer]("SourceeNB-ToTargeteNB-TransparentContainer", b)
}

func ParseSourceeNBToTargeteNBTransparentContainer(b []byte) (*SourceeNBToTargeteNBTransparentContainer, error) {
	return parseValue[SourceeNBToTargeteNBTransparentContainer]("SourceeNB-ToTargeteNB-TransparentContainer", b)
}

func (c *SourceeNBToTargeteNBTransparentContainer) Diagnostics() Diagnostics { return diagnosticsOf(c) }

type RRCContainer []byte

func (c RRCContainer) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *RRCContainer) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = RRCContainer(b)

	return nil
}

type DLForwarding uint8

const (
	DLForwardingProposed DLForwarding = iota
)

type ERABInformationListItem struct {
	ERABID                          ERABID
	DLForwarding                    *DLForwarding          `per:",optional"`
	DAPSRequestInfo                 *DAPSRequestInfo       `per:",extie:IDDAPSRequestInfo,crit:ignore"`
	SourceTransportLayerAddress     *TransportLayerAddress `per:",extie:IDSourceTransportLayerAddress,crit:ignore"`
	SecurityIndication              *SecurityIndication    `per:",extie:IDSecurityIndication,crit:ignore"`
	SourceNodeTransportLayerAddress *TransportLayerAddress `per:",extie:IDSourceNodeTransportLayerAddress,crit:ignore"`
	iEExtensions                    extensionMeta          `per:",exttable"`
	extensions                      per.ExtensionAdditions `per:",extraw"`
	containerCrit                   `per:"-"`
}

type ERABInformationList []ERABInformationListItem

func (l ERABInformationList) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return encodeSingleContainerList(w, enc, maxnoofERABs, IDERABInformationListItem, CriticalityIgnore, l)
}

func (l *ERABInformationList) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := decodeItemList[ERABInformationListItem](r, enc, maxnoofERABs, IDERABInformationListItem, CriticalityIgnore)
	if err != nil {
		return err
	}

	*l = items

	return nil
}

type SubscriberProfileIDforRFP uint16

func (s SubscriberProfileIDforRFP) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 1, HasLB: true, UB: 256, HasUB: true}, int64(s))
}

func (s *SubscriberProfileIDforRFP) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 1, HasLB: true, UB: 256, HasUB: true})
	if err != nil {
		return err
	}

	*s = SubscriberProfileIDforRFP(v)

	return nil
}

type UEHistoryInformation []LastVisitedCellItem

func (u UEHistoryInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return marshalSeqOf(w, enc, 1, maxnoofCellsinUEHistoryInfo, []LastVisitedCellItem(u))
}

func (u *UEHistoryInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	items, err := unmarshalSeqOf[LastVisitedCellItem](r, enc, 1, maxnoofCellsinUEHistoryInfo)
	if err != nil {
		return err
	}

	*u = items

	return nil
}

type LastVisitedCellItem struct {
	EUTRANCell *LastVisitedEUTRANCellInformation `per:",choice:0"`
	UTRANCell  *LastVisitedUTRANCellInformation  `per:",choice:1"`
	GERANCell  *LastVisitedGERANCellInformation  `per:",choice:2"`
	NGRANCell  *LastVisitedNGRANCellInformation  `per:",choice:0,ext"`
	Ext        *per.ExtensionAlternative         `per:",extalt"`
}

type LastVisitedEUTRANCellInformation struct {
	GlobalCellID                          EUTRANCGI
	CellType                              CellType
	TimeUEStayedInCell                    TimeUEStayedInCell
	TimeUEStayedInCellEnhancedGranularity *TimeUEStayedInCellEnhancedGranularity `per:",extie:IDTimeUEStayedInCellEnhancedGranularity,crit:ignore"`
	HOCause                               *Cause                                 `per:",extie:IDHOCause,crit:ignore"`
	LastVisitedPSCellList                 LastVisitedPSCellList                  `per:",extie:IDLastVisitedPSCellList,crit:ignore"`
	iEExtensions                          extensionMeta                          `per:",exttable"`
	extensions                            per.ExtensionAdditions                 `per:",extraw"`
}

type CellType struct {
	CellSize     CellSize
	iEExtensions extensionMeta          `per:",exttable"`
	extensions   per.ExtensionAdditions `per:",extraw"`
}

type CellSize uint8

const (
	CellSizeVerySmall CellSize = iota
	CellSizeSmall
	CellSizeMedium
	CellSizeLarge
)

type TimeUEStayedInCell uint16

func (t TimeUEStayedInCell) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeInteger(w, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true}, int64(t))
}

func (t *TimeUEStayedInCell) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	v, err := per.DecodeInteger(r, enc, per.Bounds{LB: 0, HasLB: true, UB: 4095, HasUB: true})
	if err != nil {
		return err
	}

	*t = TimeUEStayedInCell(v)

	return nil
}

type LastVisitedUTRANCellInformation []byte

func (c LastVisitedUTRANCellInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *LastVisitedUTRANCellInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = LastVisitedUTRANCellInformation(b)

	return nil
}

type LastVisitedNGRANCellInformation []byte

func (c LastVisitedNGRANCellInformation) MarshalPER(w *per.Writer, enc per.Encoding) error {
	return per.EncodeOctetString(w, enc, 0, 0, true, false, false, c)
}

func (c *LastVisitedNGRANCellInformation) UnmarshalPER(r *per.Reader, enc per.Encoding) error {
	b, err := per.DecodeOctetString(r, enc, 0, 0, true, false, false)
	if err != nil {
		return err
	}

	*c = LastVisitedNGRANCellInformation(b)

	return nil
}

type LastVisitedGERANCellInformation struct {
	Undefined *per.Null                 `per:",choice:0"`
	Ext       *per.ExtensionAlternative `per:",extalt"`
}

func (l *ERABInformationList) Diagnose(d *ap.Diagnosis[ProtocolIEID]) bool {
	return diagnoseItemList(d, IDERABInformationListItem, CriticalityIgnore, l)
}
